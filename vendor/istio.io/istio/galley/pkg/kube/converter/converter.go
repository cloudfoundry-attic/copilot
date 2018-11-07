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

package converter

import (
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	authn "istio.io/api/authentication/v1alpha1"
	"istio.io/istio/galley/pkg/kube/converter/legacy"
	"istio.io/istio/galley/pkg/runtime/resource"
)

// Fn is a conversion function that converts the given unstructured CRD into the destination resource.
type Fn func(destination resource.Info, name string, u *unstructured.Unstructured) (string, time.Time, proto.Message, error)

var converters = map[string]Fn{
	"identity":              identity,
	"legacy-mixer-resource": legacyMixerResource,
	"auth-policy-resource":  authPolicyResource,
}

// Get returns the named converter function, or panics if it is not found.
func Get(name string) Fn {
	fn, found := converters[name]
	if !found {
		panic(fmt.Sprintf("converter.Get: converter not found: %s", name))
	}

	return fn
}

func identity(destination resource.Info, name string, u *unstructured.Unstructured) (string, time.Time, proto.Message, error) {
	p, err := toProto(destination, u.Object["spec"])
	if err != nil {
		return "", time.Time{}, nil, err
	}

	return name, u.GetCreationTimestamp().Time, p, nil
}

func legacyMixerResource(_ resource.Info, name string, u *unstructured.Unstructured) (string, time.Time, proto.Message, error) {
	spec := u.Object["spec"]
	s := &types.Struct{}
	if err := toproto(s, spec); err != nil {
		return "", time.Time{}, nil, err
	}

	newName := fmt.Sprintf("%s/%s", u.GetKind(), name)

	return newName, u.GetCreationTimestamp().Time, &legacy.LegacyMixerResource{
		Name:     name,
		Kind:     u.GetKind(),
		Contents: s,
	}, nil
}

func authPolicyResource(destination resource.Info, name string, u *unstructured.Unstructured) (string, time.Time, proto.Message, error) {
	p, err := toProto(destination, u.Object["spec"])
	if err != nil {
		return "", time.Time{}, nil, err
	}

	policy, ok := p.(*authn.Policy)
	if !ok {
		return "", time.Time{}, nil, fmt.Errorf("object is not of type %v", destination.TypeURL)
	}

	// The pilot authentication plugin's config handling allows the mtls
	// peer method object value to be nil. See pilot/pkg/networking/plugin/authn/authentication.go#L68
	//
	// For example,
	//
	//     metadata:
	//       name: d-ports-mtls-enabled
	//     spec:
	//       targets:
	//       - name: d
	//         ports:
	//         - number: 80
	//       peers:
	//       - mtls:
	//
	// This translates to the following in-memory representation:
	//
	//     policy := &authn.Policy{
	//       Peers: []*authn.PeerAuthenticationMethod{{
	//         &authn.PeerAuthenticationMethod_Mtls{},
	//       }},
	//     }
	//
	// The PeerAuthenticationMethod_Mtls object with nil field is lost when
	// the proto is re-encoded for transport via MCP. As a workaround, fill
	// in the missing field value which is functionality equivalent.
	for _, peer := range policy.Peers {
		if mtls, ok := peer.Params.(*authn.PeerAuthenticationMethod_Mtls); ok && mtls.Mtls == nil {
			mtls.Mtls = &authn.MutualTls{}
		}
	}

	return name, u.GetCreationTimestamp().Time, p, nil
}
