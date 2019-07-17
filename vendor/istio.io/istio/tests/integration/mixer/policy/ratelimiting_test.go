//  Copyright 2018 Istio Authors
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package policy

import (
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"path"
	"strings"
	"testing"

	"istio.io/istio/pkg/test/env"
	"istio.io/istio/pkg/test/framework"
	"istio.io/istio/pkg/test/framework/components/bookinfo"
	"istio.io/istio/pkg/test/framework/components/environment"
	"istio.io/istio/pkg/test/framework/components/galley"
	"istio.io/istio/pkg/test/framework/components/ingress"
	"istio.io/istio/pkg/test/framework/components/istio"
	"istio.io/istio/pkg/test/framework/components/mixer"
	"istio.io/istio/pkg/test/framework/components/namespace"
	"istio.io/istio/pkg/test/framework/components/prometheus"
	"istio.io/istio/pkg/test/framework/components/redis"
	"istio.io/istio/pkg/test/framework/label"
	"istio.io/istio/pkg/test/framework/resource"
	util "istio.io/istio/tests/integration/mixer"
)

var (
	ist               istio.Instance
	bookinfoNamespace *namespace.Instance
	galInst           *galley.Instance
	redInst           *redis.Instance
	ingInst           *ingress.Instance
	promInst          *prometheus.Instance
)

func TestRateLimiting_RedisQuotaFixedWindow(t *testing.T) {
	testRedisQuota(t, bookinfo.RatingsRedisRateLimitFixed, "ratings")
}

func TestRateLimiting_RedisQuotaRollingWindow(t *testing.T) {
	testRedisQuota(t, bookinfo.RatingsRedisRateLimitRolling, "ratings")
}

func TestRateLimiting_DefaultLessThanOverride(t *testing.T) {
	framework.
		NewTest(t).
		// TODO(https://github.com/istio/istio/issues/12750)
		Label(label.Flaky).
		RequiresEnvironment(environment.Kube).
		Run(func(ctx framework.TestContext) {
			destinationService := "productpage"

			bookinfoNs, g, red, ing, prom := setupComponentsOrFail(t, ctx)
			defer deleteComponentsOrFail(t, ctx, g, bookinfoNs)
			bookInfoNameSpaceStr := bookinfoNs.Name()
			config := setupConfigOrFail(t, bookinfo.ProductPageRedisRateLimit, bookInfoNameSpaceStr,
				red, g, ctx)
			defer deleteConfigOrFail(t, config, g, ctx)
			util.AllowRuleSync(t)

			res := util.SendTraffic(ing, t, "Sending traffic...", "", 300)
			totalReqs := float64(res.DurationHistogram.Count)
			succReqs := float64(res.RetCodes[http.StatusOK])
			got429s := float64(res.RetCodes[http.StatusTooManyRequests])
			actualDuration := res.ActualDuration.Seconds() // can be a bit more than requested

			// Sending 600 requests at 10qps, and limit allowed is 50 for 30s, so we should see approx 100 requests go
			// through.
			want200s := 50.0
			// everything in excess of 200s should be 429s (ideally)
			want429s := totalReqs - want200s
			t.Logf("Expected Totals: 200s: %f (%f rps), 429s: %f (%f rps)", want200s, want200s/actualDuration,
				want429s, want429s/actualDuration)

			// As rate limit is applied at ingressgateway itself, fortio should see the limits too.
			want := math.Floor(want200s * 0.90)
			if succReqs < want {
				attributes := []string{fmt.Sprintf("%s=\"%s\"", util.GetDestinationLabel(),
					util.Fqdn(destinationService, bookInfoNameSpaceStr)),
					fmt.Sprintf("%s=\"%d\"", util.GetResponseCodeLabel(), 200),
					fmt.Sprintf("%s=\"%s\"", util.GetReporterCodeLabel(), "destination")}
				t.Logf("prometheus values for istio_requests_total for 200's:\n%s",
					util.PromDumpWithAttributes(prom, "istio_requests_total", attributes))
				t.Errorf("Bad metric value for successful requests (200s): got %f, want at least %f", succReqs, want)
			}

			// check resource exhausted
			// TODO: until https://github.com/istio/istio/issues/3028 is fixed, use 50% - should be only 5% or so
			want429s = math.Floor(want429s * 0.50)
			if got429s < want429s {
				attributes := []string{fmt.Sprintf("%s=\"%s\"", util.GetDestinationLabel(),
					util.Fqdn(destinationService, bookInfoNameSpaceStr)),
					fmt.Sprintf("%s=\"%d\"", util.GetResponseCodeLabel(), 429),
					fmt.Sprintf("%s=\"%s\"", util.GetReporterCodeLabel(), "destination")}
				t.Logf("prometheus values for istio_requests_total for 429's:\n%s",
					util.PromDumpWithAttributes(prom, "istio_requests_total", attributes))
				t.Errorf("Bad metric value for rate-limited requests (429s): got %f, want at least %f", got429s,
					want429s)
			}
		})
}

func testRedisQuota(t *testing.T, config bookinfo.ConfigFile, destinationService string) {
	framework.Run(t, func(ctx framework.TestContext) {
		bookinfoNs, g, red, ing, prom := setupComponentsOrFail(t, ctx)
		defer deleteComponentsOrFail(t, ctx, g, bookinfoNs)
		g.ApplyConfigOrFail(
			t,
			bookinfoNs,
			bookinfo.NetworkingReviewsV3Rule.LoadWithNamespaceOrFail(t, bookinfoNs.Name()),
		)
		defer g.DeleteConfigOrFail(t,
			bookinfoNs,
			bookinfo.NetworkingReviewsV3Rule.LoadWithNamespaceOrFail(t, bookinfoNs.Name()))
		bookInfoNameSpaceStr := bookinfoNs.Name()
		config := setupConfigOrFail(t, config, bookInfoNameSpaceStr, red, g, ctx)
		defer deleteConfigOrFail(t, config, g, ctx)
		util.AllowRuleSync(t)

		// This is the number of requests we allow to be missing to be reported, so as to make test stable.
		errorInRequestReportingAllowed := 5.0
		prior429s, prior200s := util.FetchRequestCount(t, prom, destinationService, "",
			bookInfoNameSpaceStr, 0)

		res := util.SendTraffic(ing, t, "Sending traffic...", "", 300)
		totalReqs := res.DurationHistogram.Count
		succReqs := float64(res.RetCodes[http.StatusOK])
		badReqs := res.RetCodes[http.StatusBadRequest]
		actualDuration := res.ActualDuration.Seconds() // can be a bit more than requested

		t.Log("Successfully sent request(s) to /productpage; checking metrics...")
		t.Logf("Fortio Summary: %d reqs (%f rps, %f 200s (%f rps), %d 400s - %+v)",
			totalReqs, res.ActualQPS, succReqs, succReqs/actualDuration, badReqs, res.RetCodes)

		// consider only successful requests (as recorded at productpage service)
		callsToRatings := succReqs
		want200s := 50.0
		// everything in excess of 200s should be 429s (ideally)
		want429s := callsToRatings - want200s
		t.Logf("Expected Totals: 200s: %f (%f rps), 429s: %f (%f rps)", want200s, want200s/actualDuration,
			want429s, want429s/actualDuration)
		// if we received less traffic than the expected enforced limit to ratings
		// then there is no way to determine if the rate limit was applied at all
		// and for how much traffic. log all metrics and abort test.
		if callsToRatings < want200s {
			attributes := []string{fmt.Sprintf("%s=\"%s\"", util.GetDestinationLabel(),
				util.Fqdn(destinationService, bookInfoNameSpaceStr))}
			t.Logf("full set of prometheus metrics for ratings:\n%s",
				util.PromDumpWithAttributes(prom, "istio_requests_total", attributes))
			t.Fatalf("Not enough traffic generated to exercise rate limit: %s_reqs=%f, want200s=%f",
				destinationService, callsToRatings, want200s)
		}

		got429s, got200s := util.FetchRequestCount(t, prom, destinationService, "", bookInfoNameSpaceStr,
			prior429s+prior200s+300-errorInRequestReportingAllowed)
		if got429s == 0 {
			attributes := []string{fmt.Sprintf("%s=\"%s\"", util.GetDestinationLabel(),
				util.Fqdn(destinationService, bookInfoNameSpaceStr)),
				fmt.Sprintf("%s=\"%d\"", util.GetResponseCodeLabel(), 429),
				fmt.Sprintf("%s=\"%s\"", util.GetReporterCodeLabel(), "destination")}
			t.Logf("prometheus values for istio_requests_total for 429's:\n%s",
				util.PromDumpWithAttributes(prom, "istio_requests_total", attributes))
			t.Errorf("Could not find 429s")
		}
		want429s = math.Floor(want429s * 0.90)
		got429s -= prior429s
		t.Logf("Actual 429s: %f (%f rps)", got429s, got429s/actualDuration)
		// check resource exhausted
		if got429s < want429s {
			attributes := []string{fmt.Sprintf("%s=\"%s\"", util.GetDestinationLabel(),
				util.Fqdn(destinationService, bookInfoNameSpaceStr)),
				fmt.Sprintf("%s=\"%d\"", util.GetResponseCodeLabel(), 429),
				fmt.Sprintf("%s=\"%s\"", util.GetReporterCodeLabel(), "destination")}
			t.Logf("prometheus values for istio_requests_total for 429's:\n%s",
				util.PromDumpWithAttributes(prom, "istio_requests_total", attributes))
			t.Errorf("Bad metric value for rate-limited requests (429s): got %f, want at least %f", got429s,
				want429s)
		}
		if got200s == 0 {
			attributes := []string{fmt.Sprintf("%s=\"%s\"", util.GetDestinationLabel(),
				util.Fqdn(destinationService, bookInfoNameSpaceStr)),
				fmt.Sprintf("%s=\"%d\"", util.GetResponseCodeLabel(), 200),
				fmt.Sprintf("%s=\"%s\"", util.GetReporterCodeLabel(), "destination")}
			t.Logf("prometheus values for istio_requests_total for 200's:\n%s",
				util.PromDumpWithAttributes(prom, "istio_requests_total", attributes))
			t.Errorf("Could not find successes value")
		}
		got200s -= prior200s
		t.Logf("Actual 200s: %f (%f rps), expecting ~1.666rps", got200s, got200s/actualDuration)
		// establish some baseline to protect against flakiness due to randomness in routing
		// and to allow for leniency in actual ceiling of enforcement (if 10 is the limit, but we allow slightly
		// less than 10, don't fail this test).
		want := math.Floor(want200s * 0.90)
		// check successes
		if got200s < want {
			attributes := []string{fmt.Sprintf("%s=\"%s\"", util.GetDestinationLabel(),
				util.Fqdn(destinationService, bookInfoNameSpaceStr)),
				fmt.Sprintf("%s=\"%d\"", util.GetResponseCodeLabel(), 200),
				fmt.Sprintf("%s=\"%s\"", util.GetReporterCodeLabel(), "destination")}
			t.Logf("prometheus values for istio_requests_total for 200's:\n%s",
				util.PromDumpWithAttributes(prom, "istio_requests_total", attributes))
			t.Errorf("Bad metric value for successful requests (200s): got %f, want at least %f", got200s, want)
		}
		want200s = math.Ceil(want200s * 1.05)
		if got200s > want200s {
			attributes := []string{fmt.Sprintf("%s=\"%s\"", util.GetDestinationLabel(),
				util.Fqdn(destinationService, bookInfoNameSpaceStr)),
				fmt.Sprintf("%s=\"%d\"", util.GetResponseCodeLabel(), 200),
				fmt.Sprintf("%s=\"%s\"", util.GetReporterCodeLabel(), "destination")}
			t.Logf("prometheus values for istio_requests_total for 200's:\n%s",
				util.PromDumpWithAttributes(prom, "istio_requests_total", attributes))
			t.Errorf("Bad metric value for successful requests (200s): got %f, want at most %f", got200s,
				want200s)
		}
	})
}

func setupComponentsOrFail(t *testing.T, ctx resource.Context) (bookinfoNs namespace.Instance, g galley.Instance,
	red redis.Instance, ing ingress.Instance, prom prometheus.Instance) {
	if bookinfoNamespace == nil {
		t.Fatalf("bookinfo namespace not allocated in setup")
	}
	bookinfoNs = *bookinfoNamespace
	if galInst == nil {
		t.Fatalf("galley not setup")
	}
	g = *galInst
	if redInst == nil {
		t.Fatalf("redis not setup")
	}
	red = *redInst
	if ingInst == nil {
		t.Fatalf("ingress not setup")
	}
	ing = *ingInst
	if promInst == nil {
		t.Fatalf("prometheus not setup")
	}
	prom = *promInst

	g.ApplyConfigOrFail(t, bookinfoNs,
		bookinfo.NetworkingBookinfoGateway.LoadGatewayFileWithNamespaceOrFail(t, bookinfoNs.Name()))
	g.ApplyConfigOrFail(
		t,
		bookinfoNs,
		bookinfo.GetDestinationRuleConfigFile(t, ctx).LoadWithNamespaceOrFail(t, bookinfoNs.Name()),
		bookinfo.NetworkingVirtualServiceAllV1.LoadWithNamespaceOrFail(t, bookinfoNs.Name()),
	)

	return
}

func deleteComponentsOrFail(t *testing.T, ctx resource.Context, g galley.Instance, bookinfoNs namespace.Instance) {
	defer g.DeleteConfigOrFail(t, bookinfoNs,
		bookinfo.NetworkingBookinfoGateway.LoadGatewayFileWithNamespaceOrFail(t, bookinfoNs.Name()))
	defer g.DeleteConfigOrFail(
		t,
		bookinfoNs,
		bookinfo.GetDestinationRuleConfigFile(t, ctx).LoadWithNamespaceOrFail(t, bookinfoNs.Name()),
		bookinfo.NetworkingVirtualServiceAllV1.LoadWithNamespaceOrFail(t, bookinfoNs.Name()))
}

func setupConfigOrFail(t *testing.T, config bookinfo.ConfigFile, bookInfoNameSpaceStr string,
	red redis.Instance, g galley.Instance, ctx resource.Context) string {
	p := path.Join(env.BookInfoRoot, string(config))
	content, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	con := string(content)

	con = strings.Replace(con, "redisServerUrl: redis-release-master:6379",
		"redisServerUrl: redis-release-master."+red.GetRedisNamespace()+":6379", -1)
	con = strings.Replace(con, "namespace: default",
		"namespace: "+bookInfoNameSpaceStr, -1)

	ns := namespace.ClaimOrFail(t, ctx, ist.Settings().SystemNamespace)
	g.ApplyConfigOrFail(t, ns, con)
	return con
}

func deleteConfigOrFail(t *testing.T, config string, g galley.Instance, ctx resource.Context) {
	ns := namespace.ClaimOrFail(t, ctx, ist.Settings().SystemNamespace)
	g.DeleteConfigOrFail(t, ns, config)
}

func TestMain(m *testing.M) {
	framework.
		NewSuite("mixer_policy_ratelimit", m).
		RequireEnvironment(environment.Kube).
		SetupOnEnv(environment.Kube, istio.Setup(&ist, nil)).
		Setup(testsetup).
		Run()
}

func testsetup(ctx resource.Context) error {
	bookinfoNs, err := namespace.New(ctx, "istio-bookinfo", true)
	if err != nil {
		return err
	}
	bookinfoNamespace = &bookinfoNs
	if _, err := bookinfo.Deploy(ctx, bookinfo.Config{Namespace: bookinfoNs, Cfg: bookinfo.BookInfo}); err != nil {
		return err
	}
	g, err := galley.New(ctx, galley.Config{})
	if err != nil {
		return err
	}
	galInst = &g
	if _, err = mixer.New(ctx, mixer.Config{Galley: g}); err != nil {
		return err
	}
	red, err := redis.New(ctx)
	if err != nil {
		return err
	}
	redInst = &red
	ing, err := ingress.New(ctx, ingress.Config{Istio: ist})
	if err != nil {
		return err
	}
	ingInst = &ing
	prom, err := prometheus.New(ctx)
	if err != nil {
		return err
	}
	promInst = &prom

	return nil
}
