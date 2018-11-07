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

package configdump

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/gogo/protobuf/jsonpb"

	"istio.io/istio/istioctl/pkg/util/configdump"
)

// ConfigWriter is a writer for processing responses from the Envoy Admin config_dump endpoint
type ConfigWriter struct {
	Stdout     io.Writer
	configDump *configdump.Wrapper
}

// Prime loads the config dump into the writer ready for printing
func (c *ConfigWriter) Prime(b []byte) error {
	cd := configdump.Wrapper{}
	// TODO(fisherxu): migrate this to jsonpb when issue fixed in golang
	// Issue to track -> https://github.com/golang/protobuf/issues/632
	err := json.Unmarshal(b, &cd)
	if err != nil {
		return fmt.Errorf("error unmarshalling config dump response from Envoy: %v", err)
	}
	c.configDump = &cd
	return nil
}

// PrintBootstrapDump prints just the bootstrap config dump to the ConfigWriter stdout
func (c *ConfigWriter) PrintBootstrapDump() error {
	if c.configDump == nil {
		return fmt.Errorf("config writer has not been primed")
	}
	bootstrapDump, err := c.configDump.GetBootstrapConfigDump()
	if err != nil {
		return err
	}
	jsonm := &jsonpb.Marshaler{Indent: "    "}
	if err := jsonm.Marshal(c.Stdout, bootstrapDump); err != nil {
		return fmt.Errorf("unable to marshal bootstrap in Envoy config dump")
	}
	return nil
}
