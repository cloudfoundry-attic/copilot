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

package cmd

import (
	"flag"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"istio.io/istio/galley/pkg/crd/validation"
	"istio.io/istio/galley/pkg/server"
	istiocmd "istio.io/istio/pkg/cmd"
	"istio.io/istio/pkg/collateral"
	"istio.io/istio/pkg/log"
	"istio.io/istio/pkg/probe"
	"istio.io/istio/pkg/version"
)

var (
	resyncPeriod   time.Duration
	kubeConfig     string
	loggingOptions = log.DefaultOptions()
)

// GetRootCmd returns the root of the cobra command-tree.
func GetRootCmd(args []string) *cobra.Command {

	var (
		serverArgs               = server.DefaultArgs()
		validationArgs           = validation.DefaultArgs()
		livenessProbeOptions     probe.Options
		readinessProbeOptions    probe.Options
		livenessProbeController  probe.Controller
		readinessProbeController probe.Controller
		monitoringPort           uint
		enableProfiling          bool
		pprofPort                uint
	)

	rootCmd := &cobra.Command{
		Use:          "galley",
		Short:        "Galley provides configuration management services for Istio.",
		Long:         "Galley provides configuration management services for Istio.",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("%q is an invalid argument", args[0])
			}
			err := log.Configure(loggingOptions)
			return err
		},
		Run: func(cmd *cobra.Command, args []string) {
			serverArgs.KubeConfig = kubeConfig
			serverArgs.ResyncPeriod = resyncPeriod
			serverArgs.CredentialOptions.CACertificateFile = validationArgs.CACertFile
			serverArgs.CredentialOptions.KeyFile = validationArgs.KeyFile
			serverArgs.CredentialOptions.CertificateFile = validationArgs.CertFile
			if livenessProbeOptions.IsValid() {
				livenessProbeController = probe.NewFileController(&livenessProbeOptions)
			}
			if readinessProbeOptions.IsValid() {
				readinessProbeController = probe.NewFileController(&readinessProbeOptions)
			}
			if !serverArgs.EnableServer && !validationArgs.EnableValidation {
				log.Fatala("Galley must be running under at least one mode: server or validation")
			}

			if err := validationArgs.Validate(); err != nil {
				log.Fatalf("Invalid validationArgs: %v", err)
			}

			if serverArgs.EnableServer {
				go server.RunServer(serverArgs, livenessProbeController, readinessProbeController)
			}
			if validationArgs.EnableValidation {
				go validation.RunValidation(validationArgs, kubeConfig, livenessProbeController, readinessProbeController)
			}
			galleyStop := make(chan struct{})
			go server.StartSelfMonitoring(galleyStop, monitoringPort)

			if enableProfiling {
				go server.StartProfiling(galleyStop, pprofPort)
			}

			go server.StartProbeCheck(livenessProbeController, readinessProbeController, galleyStop)
			istiocmd.WaitSignal(galleyStop)
		},
	}

	rootCmd.SetArgs(args)
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	rootCmd.PersistentFlags().StringVar(&kubeConfig, "kubeconfig", "",
		"Use a Kubernetes configuration file instead of in-cluster configuration")
	rootCmd.PersistentFlags().DurationVar(&resyncPeriod, "resyncPeriod", 0,
		"Resync period for rescanning Kubernetes resources")
	rootCmd.PersistentFlags().StringVar(&validationArgs.CertFile, "tlsCertFile", "/etc/certs/cert-chain.pem",
		"File containing the x509 Certificate for HTTPS.")
	rootCmd.PersistentFlags().StringVar(&validationArgs.KeyFile, "tlsKeyFile", "/etc/certs/key.pem",
		"File containing the x509 private key matching --tlsCertFile.")
	rootCmd.PersistentFlags().StringVar(&validationArgs.CACertFile, "caCertFile", "/etc/certs/root-cert.pem",
		"File containing the caBundle that signed the cert/key specified by --tlsCertFile and --tlsKeyFile.")
	rootCmd.PersistentFlags().StringVar(&livenessProbeOptions.Path, "livenessProbePath", server.DefaultLivenessProbeFilePath,
		"Path to the file for the Galley liveness probe.")
	rootCmd.PersistentFlags().DurationVar(&livenessProbeOptions.UpdateInterval, "livenessProbeInterval", server.DefaultProbeCheckInterval,
		"Interval of updating file for the Galley liveness probe.")
	rootCmd.PersistentFlags().StringVar(&readinessProbeOptions.Path, "readinessProbePath", server.DefaultReadinessProbeFilePath,
		"Path to the file for the Galley readiness probe.")
	rootCmd.PersistentFlags().DurationVar(&readinessProbeOptions.UpdateInterval, "readinessProbeInterval", server.DefaultProbeCheckInterval,
		"Interval of updating file for the Galley readiness probe.")
	rootCmd.PersistentFlags().UintVar(&monitoringPort, "monitoringPort", 9093,
		"Port to use for exposing self-monitoring information")
	rootCmd.PersistentFlags().UintVar(&pprofPort, "pprofPort", 9094, "Port to use for exposing profiling")
	rootCmd.PersistentFlags().BoolVar(&enableProfiling, "enableProfiling", false,
		"Enable profiling for Galley")

	// server config
	rootCmd.PersistentFlags().StringVarP(&serverArgs.APIAddress, "server-address", "", serverArgs.APIAddress,
		"Address to use for Galley's gRPC API, e.g. tcp://127.0.0.1:9092 or unix:///path/to/file")
	rootCmd.PersistentFlags().UintVarP(&serverArgs.MaxReceivedMessageSize, "server-maxReceivedMessageSize", "", serverArgs.MaxReceivedMessageSize,
		"Maximum size of individual gRPC messages")
	rootCmd.PersistentFlags().UintVarP(&serverArgs.MaxConcurrentStreams, "server-maxConcurrentStreams", "", serverArgs.MaxConcurrentStreams,
		"Maximum number of outstanding RPCs per connection")
	rootCmd.PersistentFlags().BoolVarP(&serverArgs.Insecure, "insecure", "", serverArgs.Insecure,
		"Use insecure gRPC communication")
	rootCmd.PersistentFlags().BoolVar(&serverArgs.EnableServer, "enable-server", serverArgs.EnableServer, "Run galley server mode")
	rootCmd.PersistentFlags().StringVarP(&serverArgs.AccessListFile, "accessListFile", "", serverArgs.AccessListFile,
		"The access list yaml file that contains the allowd mTLS peer ids.")
	rootCmd.PersistentFlags().StringVar(&serverArgs.ConfigPath, "configPath", serverArgs.ConfigPath,
		"Istio config file path")
	rootCmd.PersistentFlags().StringVar(&serverArgs.MeshConfigFile, "meshConfigFile", serverArgs.MeshConfigFile,
		"Path to the mesh config file")
	rootCmd.PersistentFlags().StringVar(&serverArgs.DomainSuffix, "domain", serverArgs.DomainSuffix,
		"DNS domain suffix")
	rootCmd.PersistentFlags().BoolVar(&serverArgs.DisableResourceReadyCheck, "disableResourceReadyCheck", serverArgs.DisableResourceReadyCheck,
		"Disable resource readiness checks. This allows Galley to start if not all resource types are supported")
	rootCmd.PersistentFlags().StringSliceVar(&serverArgs.ExcludedResourceKinds, "excludedResourceKinds",
		serverArgs.ExcludedResourceKinds, "Comma-separated list of resource kinds that should not generate source events")
	rootCmd.PersistentFlags().StringVar(&serverArgs.SinkAddress, "sinkAddress",
		serverArgs.SinkAddress, "Address of MCP Resource Sink server for Galley to connect to. Ex: 'foo.com:1234'")
	rootCmd.PersistentFlags().StringVar(&serverArgs.SinkAuthMode, "sinkAuthMode",
		serverArgs.SinkAuthMode, "Name of authentication plugin to use for connection to sink server.")

	serverArgs.IntrospectionOptions.AttachCobraFlags(rootCmd)

	// validation config
	rootCmd.PersistentFlags().StringVar(&validationArgs.WebhookConfigFile,
		"validation-webhook-config-file", "",
		"File that contains k8s validatingwebhookconfiguration yaml. Validation is disabled if file is not specified")
	rootCmd.PersistentFlags().UintVar(&validationArgs.Port, "validation-port", 443,
		"HTTPS port of the validation service. Must be 443 if service has more than one port ")
	rootCmd.PersistentFlags().BoolVar(&validationArgs.EnableValidation, "enable-validation", validationArgs.EnableValidation,
		"Run galley validation mode")
	rootCmd.PersistentFlags().StringVar(&validationArgs.DeploymentAndServiceNamespace, "deployment-namespace", "istio-system",
		"Namespace of the deployment for the validation pod")
	rootCmd.PersistentFlags().StringVar(&validationArgs.DeploymentName, "deployment-name", "istio-galley",
		"Name of the deployment for the validation pod")
	rootCmd.PersistentFlags().StringVar(&validationArgs.ServiceName, "service-name", "istio-galley",
		"Name of the validation service running in the same namespace as the deployment")
	rootCmd.PersistentFlags().StringVar(&validationArgs.WebhookName, "webhook-name", "istio-galley",
		"Name of the k8s validatingwebhookconfiguration")

	rootCmd.AddCommand(probeCmd())
	rootCmd.AddCommand(version.CobraCommand())
	rootCmd.AddCommand(collateral.CobraCommand(rootCmd, &doc.GenManHeader{
		Title:   "Istio Galley Server",
		Section: "galley CLI",
		Manual:  "Istio Galley Server",
	}))

	loggingOptions.AttachCobraFlags(rootCmd)

	return rootCmd
}
