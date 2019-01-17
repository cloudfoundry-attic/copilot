// Copyright 2018 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"istio.io/istio/pkg/cmd"
	"istio.io/istio/pkg/log"
	"istio.io/istio/security/pkg/nodeagent/cache"
	"istio.io/istio/security/pkg/nodeagent/sds"
	"istio.io/istio/security/pkg/nodeagent/secretfetcher"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	// name of authentication provider.
	caProvider = "CA_PROVIDER"

	// CA endpoint.
	caAddress = "CA_ADDR"

	// names of authentication provider's plugins.
	pluginNames = "Plugins"

	// The trust domain corresponds to the trust root of a system.
	// Refer to https://github.com/spiffe/spiffe/blob/master/standards/SPIFFE-ID.md#21-trust-domain
	trustDomain = "Trust_Domain"

	// The workload SDS mode allows node agent to provision credentials to workload proxy by sending
	// CSR to CA.
	enableWorkloadSDS = "ENABLE_WORKLOAD_SDS"

	// The ingress gateway SDS mode allows node agent to provision credentials to ingress gateway
	// proxy by watching kubernetes secrets.
	enableIngressGatewaySDS = "ENABLE_INGRESS_GATEWAY_SDS"

	// The environmental variable name for Vault CA address.
	vaultAddress = "VAULT_ADDR"

	// The environmental variable name for Vault auth path.
	vaultAuthPath = "VAULT_AUTH_PATH"

	// The environmental variable name for Vault role.
	vaultRole = "VAULT_ROLE"

	// The environmental variable name for Vault sign CSR path.
	vaultSignCsrPath = "VAULT_SIGN_CSR_PATH"

	// The environmental variable name for Vault TLS root certificate.
	vaultTLSRootCert = "VAULT_TLS_ROOT_CERT"
)

var (
	workloadSdsCacheOptions cache.Options
	gatewaySdsCacheOptions  cache.Options
	serverOptions           sds.Options
	gatewaySecretChan       chan struct{}
	loggingOptions          = log.DefaultOptions()

	// rootCmd defines the command for node agent.
	rootCmd = &cobra.Command{
		Use:   "nodeagent",
		Short: "Node agent",
		RunE: func(c *cobra.Command, args []string) error {
			if err := log.Configure(loggingOptions); err != nil {
				return err
			}
			gatewaySdsCacheOptions = workloadSdsCacheOptions

			if serverOptions.EnableIngressGatewaySDS && serverOptions.EnableWorkloadSDS &&
				serverOptions.IngressGatewayUDSPath == serverOptions.WorkloadUDSPath {
				log.Error("UDS paths for ingress gateway and workload are the same")
				os.Exit(1)
			}
			if serverOptions.CAProviderName == "" && serverOptions.EnableWorkloadSDS {
				log.Error("CA Provider is missing")
				os.Exit(1)
			}
			if serverOptions.CAEndpoint == "" && serverOptions.EnableWorkloadSDS {
				log.Error("CA Endpoint is missing")
				os.Exit(1)
			}

			stop := make(chan struct{})

			workloadSecretCache, gatewaySecretCache := newSecretCache(serverOptions)
			if workloadSecretCache != nil {
				defer workloadSecretCache.Close()
			}
			if gatewaySecretCache != nil {
				defer gatewaySecretCache.Close()
			}

			server, err := sds.NewServer(serverOptions, workloadSecretCache, gatewaySecretCache)
			defer server.Stop()
			if err != nil {
				log.Errorf("failed to create sds service: %v", err)
				return fmt.Errorf("failed to create sds service")
			}

			cmd.WaitSignal(stop)

			return nil
		},
	}
)

func newSecretCache(serverOptions sds.Options) (workloadSecretCache, gatewaySecretCache *cache.SecretCache) {
	if serverOptions.EnableWorkloadSDS {
		wSecretFetcher, err := secretfetcher.NewSecretFetcher(false, serverOptions.CAEndpoint,
			serverOptions.CAProviderName, true, []byte(serverOptions.VaultTLSRootCert),
			serverOptions.VaultAddress, serverOptions.VaultRole, serverOptions.VaultAuthPath,
			serverOptions.VaultSignCsrPath)
		if err != nil {
			log.Errorf("failed to create secretFetcher for workload proxy: %v", err)
			os.Exit(1)
		}
		workloadSdsCacheOptions.TrustDomain = serverOptions.TrustDomain
		workloadSdsCacheOptions.Plugins = sds.NewPlugins(serverOptions.PluginNames)
		workloadSecretCache = cache.NewSecretCache(wSecretFetcher, sds.NotifyProxy, workloadSdsCacheOptions)
	} else {
		workloadSecretCache = nil
	}

	if serverOptions.EnableIngressGatewaySDS {
		gSecretFetcher, err := secretfetcher.NewSecretFetcher(true, "", "", false, nil, "", "", "", "")
		if err != nil {
			log.Errorf("failed to create secretFetcher for gateway proxy: %v", err)
			os.Exit(1)
		}
		gatewaySecretChan = make(chan struct{})
		gSecretFetcher.Run(gatewaySecretChan)
		gatewaySecretCache = cache.NewSecretCache(gSecretFetcher, sds.NotifyProxy, gatewaySdsCacheOptions)
	} else {
		gatewaySecretCache = nil
	}
	return workloadSecretCache, gatewaySecretCache
}

func init() {
	pluginNames := os.Getenv(pluginNames)
	pns := []string{}
	if pluginNames != "" {
		pns = strings.Split(pluginNames, ",")
	}

	enableWorkloadSdsEnv := true
	val := os.Getenv(enableWorkloadSDS)
	if env, err := strconv.ParseBool(val); err == nil {
		enableWorkloadSdsEnv = env
	}
	enableIngressGatewaySdsEnv := false
	val = os.Getenv(enableIngressGatewaySDS)
	if env, err := strconv.ParseBool(val); err == nil {
		enableIngressGatewaySdsEnv = env
	}

	rootCmd.PersistentFlags().BoolVar(&serverOptions.EnableWorkloadSDS, "enableWorkloadSDS",
		enableWorkloadSdsEnv,
		"If true, node agent works as SDS server and provisions key/certificate to workload proxies.")
	rootCmd.PersistentFlags().StringVar(&serverOptions.WorkloadUDSPath, "workloadUDSPath",
		"/var/run/sds/uds_path", "Unix domain socket through which SDS server communicates with workload proxies")

	rootCmd.PersistentFlags().BoolVar(&serverOptions.EnableIngressGatewaySDS, "enableIngressGatewaySDS",
		enableIngressGatewaySdsEnv,
		"If true, node agent works as SDS server and watches kubernetes secrets for ingress gateway.")
	rootCmd.PersistentFlags().StringVar(&serverOptions.IngressGatewayUDSPath, "gatewayUdsPath",
		"/var/run/ingress_gateway/sds", "Unix domain socket through which SDS server communicates with ingress gateway proxies.")

	rootCmd.PersistentFlags().StringVar(&serverOptions.CAProviderName, "caProvider", os.Getenv(caProvider), "CA provider")
	rootCmd.PersistentFlags().StringVar(&serverOptions.CAEndpoint, "caEndpoint", os.Getenv(caAddress), "CA endpoint")

	rootCmd.PersistentFlags().StringVar(&serverOptions.TrustDomain, "trustDomain",
		os.Getenv(trustDomain), "The trust domain this node agent run in")
	rootCmd.PersistentFlags().StringArrayVar(&serverOptions.PluginNames, "pluginNames",
		pns, "authentication provider specific plugin names")

	rootCmd.PersistentFlags().StringVar(&serverOptions.CertFile, "sdsCertFile", "", "SDS gRPC TLS server-side certificate")
	rootCmd.PersistentFlags().StringVar(&serverOptions.KeyFile, "sdsKeyFile", "", "SDS gRPC TLS server-side key")

	rootCmd.PersistentFlags().DurationVar(&workloadSdsCacheOptions.SecretTTL, "secretTtl",
		24*time.Hour, "Secret's TTL")
	rootCmd.PersistentFlags().DurationVar(&workloadSdsCacheOptions.SecretRefreshGraceDuration, "secretRefreshGraceDuration",
		time.Hour, "Secret's Refresh Grace Duration")
	rootCmd.PersistentFlags().DurationVar(&workloadSdsCacheOptions.RotationInterval, "secretRotationInterval",
		10*time.Minute, "Secret rotation job running interval")
	rootCmd.PersistentFlags().DurationVar(&workloadSdsCacheOptions.EvictionDuration, "secretEvictionDuration",
		24*time.Hour, "Secret eviction time duration")

	rootCmd.PersistentFlags().StringVar(&serverOptions.VaultAddress, "vaultAddress", os.Getenv(vaultAddress),
		"Vault address")
	rootCmd.PersistentFlags().StringVar(&serverOptions.VaultRole, "vaultRole", os.Getenv(vaultRole),
		"Vault role")
	rootCmd.PersistentFlags().StringVar(&serverOptions.VaultAuthPath, "vaultAuthPath", os.Getenv(vaultAuthPath),
		"Vault auth path")
	rootCmd.PersistentFlags().StringVar(&serverOptions.VaultSignCsrPath, "vaultSignCsrPath", os.Getenv(vaultSignCsrPath),
		"Vault sign CSR path")
	rootCmd.PersistentFlags().StringVar(&serverOptions.VaultTLSRootCert, "vaultTLSRootCert", os.Getenv(vaultTLSRootCert),
		"Vault TLS root certificate")

	// Attach the Istio logging options to the command.
	loggingOptions.AttachCobraFlags(rootCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Errora(err)
		os.Exit(1)
	}
}
