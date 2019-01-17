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

package runtime

import (
	"flag"
	"fmt"
	"os"

	"istio.io/istio/pkg/test/framework/runtime/registries"
)

// init registers the command-line flags that we can exposed for "go test".
func init() {
	flag.StringVar(&globalSettings.WorkDir, "istio.test.work_dir", os.TempDir(),
		"Local working directory for creating logs/temp files. If left empty, os.TempDir() is used.")
	flag.StringVar((*string)(&globalSettings.Environment), "istio.test.env", string(globalSettings.Environment),
		fmt.Sprintf("Specify the environment to run the tests against. Allowed values are: %v",
			registries.GetSupportedEnvironments()))
	flag.BoolVar(&globalSettings.NoCleanup, "istio.test.noCleanup", globalSettings.NoCleanup,
		"Do not cleanup resources after test completion")

	globalSettings.LogOptions.AttachFlags(
		func(p *[]string, name string, value []string, usage string) {
			// TODO(ozben): Implement string array method for capturing the complete set of log settings.
		},
		flag.StringVar,
		flag.IntVar,
		flag.BoolVar)
}
