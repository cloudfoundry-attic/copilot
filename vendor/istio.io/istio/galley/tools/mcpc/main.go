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
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/types"
	"google.golang.org/grpc"
	"k8s.io/client-go/util/jsonpath"

	mcp "istio.io/api/mcp/v1alpha1"
	"istio.io/istio/galley/pkg/metadata"
	_ "istio.io/istio/galley/pkg/metadata" // Import the resource package to pull in all proto types.
	"istio.io/istio/pkg/mcp/client"
	"istio.io/istio/pkg/mcp/sink"
	"istio.io/istio/pkg/mcp/testing/monitoring"
)

var (
	serverAddr               = flag.String("server", "127.0.0.1:9901", "The server address")
	collectionList           = flag.String("sortedCollections", "", "The collectionList of resources to deploy")
	useWellKnownTypes        = flag.Bool("use-wkt", false, "use well known collectionList types")
	useWellKnownPilotTypes   = flag.Bool("use-wkt-pilot", false, "use well known collectionList types for pilot")
	useWellKnownMixerTypes   = flag.Bool("use-wkt-mixer", false, "use well known collectionList types for mixer")
	id                       = flag.String("id", "", "The node id for the client")
	useResourceSourceService = flag.Bool("use-source-service", true, "use the new resource source service")
	output                   = flag.String("output", "short", "output format. One of: long|short|stats|jsonpath=<template>")
)

var (
	shortHeader        = strings.Join([]string{"TYPE_URL", "INDEX", "KEY", "NAMESPACE", "NAME", "VERSION", "AGE"}, "\t")
	jsonpathHeader     = strings.Join([]string{"TYPE_URL", "INDEX", "KEY", "NAMESPACE", "NAME", "VERSION", "AGE", "JSONPATH"}, "\t")
	outputFormatWriter = tabwriter.NewWriter(os.Stdout, 4, 2, 4, ' ', 0)
	statsHeader        = strings.Join([]string{"COLLECTION", "APPLY", "ADD", "RE-ADD", "UPDATE", "DELETE"}, "\t")
)

type stat struct {
	apply  int64
	add    int64
	readd  int64 // added resource that already existed at the same version
	update int64
	delete int64

	// track the currently applied set of resources
	resources map[string]*mcp.Metadata
}

type updater struct {
	jp         *jsonpath.JSONPath
	stats      map[string]*stat
	totalApply int64

	sortedCollections []string // sorted list of sortedCollections
}

func asNamespaceAndName(key string) (string, string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}

func (u *updater) printShortChange(ch *sink.Change) {
	fmt.Printf("Incoming apply: %v\n", ch.Collection)
	if len(ch.Objects) == 0 {
		return
	}

	now := time.Now()

	fmt.Fprintln(outputFormatWriter, shortHeader)
	for i, o := range ch.Objects {
		age := ""
		if then, err := types.TimestampFromProto(o.Metadata.CreateTime); err == nil {
			age = now.Sub(then).Round(time.Millisecond).String()
		}
		namespace, name := asNamespaceAndName(o.Metadata.Name)

		parts := []string{
			o.TypeURL,
			strconv.Itoa(i),
			o.Metadata.Name,
			namespace, name,
			o.Metadata.Version,
			age,
		}

		fmt.Fprintln(outputFormatWriter, strings.Join(parts, "\t"))
	}
	outputFormatWriter.Flush()
}

// Update interface method implementation.
func (u *updater) printLongChange(ch *sink.Change) {
	fmt.Printf("Incoming apply: %v\n", ch.Collection)
	if len(ch.Objects) == 0 {
		return
	}

	for i, o := range ch.Objects {
		fmt.Printf("%s[%d]\n", ch.Collection, i)

		b, err := json.MarshalIndent(o, "  ", "  ")
		if err != nil {
			fmt.Printf("  Marshalling error: %v", err)
		} else {
			fmt.Printf("%s\n", string(b))
		}

		fmt.Printf("===============\n")
	}
}

func ageFromProto(now time.Time, t *types.Timestamp) string {
	age := ""
	if then, err := types.TimestampFromProto(t); err == nil {
		age = now.Sub(then).Round(time.Millisecond).String()
	}
	return age
}

func (u *updater) printJsonpathChange(ch *sink.Change) {
	fmt.Printf("Incoming apply: %v\n", ch.Collection)
	if len(ch.Objects) == 0 {
		return
	}

	now := time.Now()

	fmt.Fprintln(outputFormatWriter, jsonpathHeader)
	for i, o := range ch.Objects {
		age := ageFromProto(now, o.Metadata.CreateTime)

		namespace, name := asNamespaceAndName(o.Metadata.Name)

		output := ""
		m := jsonpb.Marshaler{}
		resourceStr, err := m.MarshalToString(o.Body)
		if err == nil {
			queryObj := map[string]interface{}{}
			if err := json.Unmarshal([]byte(resourceStr), &queryObj); err == nil {
				var b bytes.Buffer
				if err := u.jp.Execute(&b, queryObj); err == nil {
					output = b.String()
				}
			}
		}

		parts := []string{
			o.TypeURL,
			strconv.Itoa(i),
			o.Metadata.Name,
			namespace, name,
			o.Metadata.Version,
			age,
			output,
		}

		fmt.Fprintln(outputFormatWriter, strings.Join(parts, "\t"))
	}
	outputFormatWriter.Flush()
}

func (u *updater) printStats(ch *sink.Change) {
	stats, ok := u.stats[ch.Collection]
	if !ok {
		fmt.Printf("unknown collection: %v\n", ch.Collection)
		return
	}

	now := time.Now()

	u.totalApply++
	stats.apply++

	added := make(map[string]*mcp.Metadata, len(ch.Objects))

	for _, obj := range ch.Objects {
		added[obj.Metadata.Name] = obj.Metadata
	}

	// update add/update stats
	for name, metadata := range added {
		if prev, ok := stats.resources[name]; !ok {
			stats.add++
			stats.resources[name] = metadata
		} else if metadata.Version == prev.Version {
			stats.readd++
			stats.resources[name] = metadata
		} else {
			stats.update++
			stats.resources[name] = metadata
		}
	}

	// update delete stats
	for name := range stats.resources {
		if _, ok := added[name]; !ok {
			stats.delete++
			delete(stats.resources, name)
		}
	}

	// clear the screen
	print("\033[H\033[2J")

	fmt.Printf("Update at %v: apply=%v\n", time.Now().Format(time.RFC822), u.totalApply)

	fmt.Println("Current resource versions")
	parts := []string{"COLLECTION", "NAME", "VERSION", "AGE"}
	fmt.Fprintln(outputFormatWriter, strings.Join(parts, "\t"))

	for _, collection := range u.sortedCollections {
		stats := u.stats[collection]

		// sort the list of resources for consistent output
		resources := make([]string, 0, len(stats.resources))
		for name := range stats.resources {
			resources = append(resources, name)
		}
		sort.Strings(resources)

		for _, name := range resources {
			metadata := stats.resources[name]
			parts := []string{collection, name, metadata.Version, ageFromProto(now, metadata.CreateTime)}
			fmt.Fprintln(outputFormatWriter, strings.Join(parts, "\t"))
		}
	}
	outputFormatWriter.Flush()

	fmt.Printf("\n\n")

	fmt.Println("Change stats")
	fmt.Fprintln(outputFormatWriter, statsHeader)
	for _, collection := range u.sortedCollections {
		stats := u.stats[collection]
		parts := []string{
			collection,
			strconv.FormatInt(stats.apply, 10),
			strconv.FormatInt(stats.add, 10),
			strconv.FormatInt(stats.readd, 10),
			strconv.FormatInt(stats.update, 10),
			strconv.FormatInt(stats.delete, 10),
		}
		fmt.Fprintln(outputFormatWriter, strings.Join(parts, "\t"))
	}
	outputFormatWriter.Flush()
}

// Update interface method implementation.
func (u *updater) Apply(ch *sink.Change) error {
	switch *output {
	case "long":
		u.printLongChange(ch)
	case "short":
		u.printShortChange(ch)
	case "stats":
		u.printStats(ch)
	case "jsonpath":
		u.printJsonpathChange(ch)
	default:
		fmt.Printf("Change %v\n", ch.Collection)
	}
	return nil
}

func main() {
	flag.Parse()

	var jp *jsonpath.JSONPath
	if *output == "jsonpath" {
		parts := strings.Split(*output, "=")
		if len(parts) != 2 {
			fmt.Printf("unknown output option %q:\n", *output)
			os.Exit(-1)
		}

		if parts[0] != "jsonpath" {
			fmt.Printf("unknown output %q:\n", *output)
			os.Exit(-1)
		}

		jp = jsonpath.New("default")
		if err := jp.Parse(parts[1]); err != nil {
			fmt.Printf("Error parsing jsonpath: %v\n", err)
			os.Exit(-1)
		}
	}

	collectionsMap := make(map[string]struct{})

	if *collectionList != "" {
		for _, collection := range strings.Split(*collectionList, ",") {
			collectionsMap[collection] = struct{}{}
		}
	}

	for _, info := range metadata.Types.All() {
		collection := info.Collection.String()

		switch {
		// pilot sortedCollections
		case strings.HasPrefix(collection, "istio/networking/"):
			fallthrough
		case strings.HasPrefix(collection, "istio/authentication/"):
			fallthrough
		case strings.HasPrefix(collection, "istio/config/v1alpha2/httpapispecs"):
			fallthrough
		case strings.HasPrefix(collection, "istio/config/v1alpha2/httpapispecbindings"):
			fallthrough
		case strings.HasPrefix(collection, "istio/mixer/v1/config/client"):
			fallthrough
		case strings.HasPrefix(collection, "istio/rbac"):
			if *useWellKnownTypes || *useWellKnownPilotTypes {
				collectionsMap[collection] = struct{}{}
			}

		// mixer sortedCollections
		case strings.HasPrefix(collection, "istio/policy/"):
			fallthrough
		case strings.HasPrefix(collection, "istio/config/"):
			if *useWellKnownTypes || *useWellKnownMixerTypes {
				collectionsMap[collection] = struct{}{}
			}

		// default (k8s?)
		default:
			if *useWellKnownTypes {
				collectionsMap[collection] = struct{}{}
			}
		}
	}

	// de-dup types
	collections := make([]string, 0, len(collectionsMap))
	for collection := range collectionsMap {
		collections = append(collections, collection)
	}

	sort.Strings(collections)

	conn, err := grpc.Dial(*serverAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		os.Exit(-1)
	}

	u := &updater{
		jp:                jp,
		stats:             make(map[string]*stat, len(collections)),
		sortedCollections: collections,
	}
	for _, collection := range collections {
		u.stats[collection] = &stat{resources: make(map[string]*mcp.Metadata)}
	}

	options := &sink.Options{
		CollectionOptions: sink.CollectionOptionsFromSlice(collections),
		Updater:           u,
		ID:                *id,
		Reporter:          monitoring.NewInMemoryStatsContext(),
	}

	ctx := context.Background()

	if *useResourceSourceService {
		cl := mcp.NewResourceSourceClient(conn)
		c := sink.NewClient(cl, options)
		c.Run(ctx)
	} else {
		cl := mcp.NewAggregatedMeshConfigServiceClient(conn)
		c := client.New(cl, options)
		c.Run(ctx)
	}
}
