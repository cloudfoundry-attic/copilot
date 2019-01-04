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

package coredatamodel_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/onsi/gomega"

	authn "istio.io/api/authentication/v1alpha1"
	mcpapi "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/config/coredatamodel"
	"istio.io/istio/pilot/pkg/model"
	mcpclient "istio.io/istio/pkg/mcp/client"
)

var (
	gateway = &networking.Gateway{
		Servers: []*networking.Server{
			{
				Port: &networking.Port{
					Number:   443,
					Name:     "https",
					Protocol: "HTTP",
				},
				Hosts: []string{"*.secure.example.com"},
			},
		},
	}

	gateway2 = &networking.Gateway{
		Servers: []*networking.Server{
			{
				Port: &networking.Port{
					Number:   80,
					Name:     "http",
					Protocol: "HTTP",
				},
				Hosts: []string{"*.example.com"},
			},
		},
	}

	gateway3 = &networking.Gateway{
		Servers: []*networking.Server{
			{
				Port: &networking.Port{
					Number:   8080,
					Name:     "http",
					Protocol: "HTTP",
				},
				Hosts: []string{"foo.example.com"},
			},
		},
	}

	authnPolicy0 = &authn.Policy{
		Targets: []*authn.TargetSelector{{
			Name: "service-foo",
		}},
		Peers: []*authn.PeerAuthenticationMethod{{
			&authn.PeerAuthenticationMethod_Mtls{}},
		},
	}

	authnPolicy1 = &authn.Policy{
		Peers: []*authn.PeerAuthenticationMethod{{
			&authn.PeerAuthenticationMethod_Mtls{}},
		},
	}

	testControllerOptions = coredatamodel.Options{
		DomainSuffix: "cluster.local",
	}
)

func TestHasSynced(t *testing.T) {
	t.Skip("Pending: https://github.com/istio/istio/issues/7947")
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	g.Expect(controller.HasSynced()).To(gomega.BeFalse())
}

func TestConfigDescriptor(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	descriptors := controller.ConfigDescriptor()
	g.Expect(descriptors).To(gomega.Equal(model.IstioConfigTypes))
}

func TestListInvalidType(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	c, err := controller.List("bad-type", "some-phony-name-space.com")
	g.Expect(c).To(gomega.BeNil())
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring("list unknown type"))
}

func TestListCorrectTypeNoData(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	c, err := controller.List("virtual-service", "some-phony-name-space.com")
	g.Expect(c).To(gomega.BeNil())
	g.Expect(err).ToNot(gomega.HaveOccurred())
}

func TestListAllNameSpace(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	messages := convertToEnvelope(g, model.Gateway.MessageName, []proto.Message{gateway, gateway2, gateway3})
	message, message2, message3 := messages[0], messages[1], messages[2]
	change := convert(
		[]proto.Message{message, message2, message3},
		[]string{"namespace1/some-gateway1", "default/some-other-gateway", "some-other-gateway3"},
		model.Gateway.MessageName)

	err := controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	c, err := controller.List("gateway", "")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(c)).To(gomega.Equal(3))

	for _, conf := range c {
		g.Expect(conf.Type).To(gomega.Equal(model.Gateway.Type))
		if conf.Name == "some-gateway1" {
			g.Expect(conf.Spec).To(gomega.Equal(message))
			g.Expect(conf.Namespace).To(gomega.Equal("namespace1"))
		} else if conf.Name == "some-other-gateway" {
			g.Expect(conf.Namespace).To(gomega.Equal("default"))
			g.Expect(conf.Spec).To(gomega.Equal(message2))
		} else {
			g.Expect(conf.Name).To(gomega.Equal("some-other-gateway3"))
			g.Expect(conf.Namespace).To(gomega.Equal(""))
			g.Expect(conf.Spec).To(gomega.Equal(message3))
		}
	}
}

func TestListSpecificNameSpace(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	messages := convertToEnvelope(g, model.Gateway.MessageName, []proto.Message{gateway, gateway2, gateway3})
	message, message2, message3 := messages[0], messages[1], messages[2]

	change := convert(
		[]proto.Message{message, message2, message3},
		[]string{"namespace1/some-gateway1", "default/some-other-gateway", "namespace1/some-other-gateway3"},
		model.Gateway.MessageName)

	err := controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	c, err := controller.List("gateway", "namespace1")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(c)).To(gomega.Equal(2))

	for _, conf := range c {
		g.Expect(conf.Type).To(gomega.Equal(model.Gateway.Type))
		g.Expect(conf.Namespace).To(gomega.Equal("namespace1"))
		if conf.Name == "some-gateway1" {
			g.Expect(conf.Spec).To(gomega.Equal(message))
		} else {
			g.Expect(conf.Name).To(gomega.Equal("some-other-gateway3"))
			g.Expect(conf.Spec).To(gomega.Equal(message3))
		}
	}
}

func TestApplyInvalidType(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	message := convertToEnvelope(g, model.Gateway.MessageName, []proto.Message{gateway})
	change := convert([]proto.Message{message[0]}, []string{"some-gateway"}, "bad-type")

	err := controller.Apply(change)
	g.Expect(err).To(gomega.HaveOccurred())
}

func TestApplyValidTypeWithNoBaseURL(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	var createAndCheckGateway = func(g *gomega.GomegaWithT, controller coredatamodel.CoreDataModel, port uint32) {
		gateway := &networking.Gateway{
			Servers: []*networking.Server{
				{
					Port: &networking.Port{
						Number:   port,
						Name:     "http",
						Protocol: "HTTP",
					},
					Hosts: []string{"*.example.com"},
				},
			},
		}
		marshaledGateway, err := proto.Marshal(gateway)
		g.Expect(err).ToNot(gomega.HaveOccurred())

		message, err := makeMessage(marshaledGateway, model.Gateway.MessageName)
		g.Expect(err).ToNot(gomega.HaveOccurred())

		change := convert([]proto.Message{message}, []string{"some-gateway"}, model.Gateway.MessageName)
		err = controller.Apply(change)
		g.Expect(err).ToNot(gomega.HaveOccurred())

		c, err := controller.List("gateway", "")
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(len(c)).To(gomega.Equal(1))
		g.Expect(c[0].Name).To(gomega.Equal("some-gateway"))
		g.Expect(c[0].Type).To(gomega.Equal(model.Gateway.Type))
		g.Expect(c[0].Spec).To(gomega.Equal(message))
		g.Expect(c[0].Spec).To(gomega.ContainSubstring(fmt.Sprintf("number:%d", port)))
	}
	createAndCheckGateway(g, controller, 80)
	createAndCheckGateway(g, controller, 9999)
}

func TestApplyMetadataNameIncludesNamespace(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	message := convertToEnvelope(g, model.Gateway.MessageName, []proto.Message{gateway})

	change := convert([]proto.Message{message[0]}, []string{"istio-namespace/some-gateway"}, model.Gateway.MessageName)
	err := controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	c, err := controller.List("gateway", "istio-namespace")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(c)).To(gomega.Equal(1))
	g.Expect(c[0].Name).To(gomega.Equal("some-gateway"))
	g.Expect(c[0].Type).To(gomega.Equal(model.Gateway.Type))
	g.Expect(c[0].Spec).To(gomega.Equal(message[0]))
}

func TestApplyMetadataNameWithoutNamespace(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	message := convertToEnvelope(g, model.Gateway.MessageName, []proto.Message{gateway})

	change := convert([]proto.Message{message[0]}, []string{"some-gateway"}, model.Gateway.MessageName)
	err := controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	c, err := controller.List("gateway", "")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(c)).To(gomega.Equal(1))
	g.Expect(c[0].Name).To(gomega.Equal("some-gateway"))
	g.Expect(c[0].Type).To(gomega.Equal(model.Gateway.Type))
	g.Expect(c[0].Spec).To(gomega.Equal(message[0]))
}

func TestApplyChangeNoObjects(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	message := convertToEnvelope(g, model.Gateway.MessageName, []proto.Message{gateway})
	change := convert([]proto.Message{message[0]}, []string{"some-gateway"}, model.Gateway.MessageName)

	err := controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	c, err := controller.List("gateway", "")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(c)).To(gomega.Equal(1))
	g.Expect(c[0].Name).To(gomega.Equal("some-gateway"))
	g.Expect(c[0].Type).To(gomega.Equal(model.Gateway.Type))
	g.Expect(c[0].Spec).To(gomega.Equal(message[0]))

	change = convert([]proto.Message{}, []string{"some-gateway"}, model.Gateway.MessageName)

	err = controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	c, err = controller.List("gateway", "")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(c)).To(gomega.Equal(0))
}

func convert(resources []proto.Message, names []string, responseMessageName string) *mcpclient.Change {
	out := new(mcpclient.Change)
	out.TypeURL = responseMessageName
	for i, res := range resources {
		out.Objects = append(out.Objects,
			&mcpclient.Object{
				TypeURL: responseMessageName,
				Metadata: &mcpapi.Metadata{
					Name: names[i],
				},
				Resource: res,
			},
		)
	}
	return out
}

func convertToEnvelope(g *gomega.GomegaWithT, messageName string, resources []proto.Message) (messages []proto.Message) {
	for _, resource := range resources {
		marshaled, err := proto.Marshal(resource)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		message, err := makeMessage(marshaled, messageName)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		messages = append(messages, message)
	}
	return messages
}

func makeMessage(value []byte, responseMessageName string) (proto.Message, error) {
	resource := &types.Any{
		TypeUrl: fmt.Sprintf("type.googleapis.com/%s", responseMessageName),
		Value:   value,
	}

	var dynamicAny types.DynamicAny
	err := types.UnmarshalAny(resource, &dynamicAny)
	if err == nil {
		return dynamicAny.Message, nil
	}

	return nil, err
}

func TestApplyClusterScopedAuthPolicy(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	controller := coredatamodel.NewController(testControllerOptions)

	message0 := convertToEnvelope(g, model.AuthenticationPolicy.MessageName, []proto.Message{authnPolicy0})
	message1 := convertToEnvelope(g, model.AuthenticationMeshPolicy.MessageName, []proto.Message{authnPolicy1})

	change := convert(
		[]proto.Message{message0[0], message1[0]},
		[]string{"bar-namespace/foo", "default"},
		model.AuthenticationPolicy.MessageName)
	err := controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	c, err := controller.List(model.AuthenticationPolicy.Type, "bar-namespace")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(c)).To(gomega.Equal(1))
	g.Expect(c[0].Name).To(gomega.Equal("foo"))
	g.Expect(c[0].Namespace).To(gomega.Equal("bar-namespace"))
	g.Expect(c[0].Type).To(gomega.Equal(model.AuthenticationPolicy.Type))
	g.Expect(c[0].Spec).To(gomega.Equal(message0[0]))

	c, err = controller.List(model.AuthenticationMeshPolicy.Type, "")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(c)).To(gomega.Equal(1))
	g.Expect(c[0].Name).To(gomega.Equal("default"))
	g.Expect(c[0].Namespace).To(gomega.Equal(""))
	g.Expect(c[0].Type).To(gomega.Equal(model.AuthenticationMeshPolicy.Type))
	g.Expect(c[0].Spec).To(gomega.Equal(message1[0]))

	// verify the namespace scoped resource can be deleted
	change = convert(
		[]proto.Message{message1[0]},
		[]string{"default"},
		model.AuthenticationPolicy.MessageName)
	err = controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	c, err = controller.List(model.AuthenticationMeshPolicy.Type, "")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(c)).To(gomega.Equal(1))
	g.Expect(c[0].Name).To(gomega.Equal("default"))
	g.Expect(c[0].Namespace).To(gomega.Equal(""))
	g.Expect(c[0].Type).To(gomega.Equal(model.AuthenticationMeshPolicy.Type))
	g.Expect(c[0].Spec).To(gomega.Equal(message1[0]))

	// verify the namespace scoped resource can be added and mesh-scoped resource removed in the same batch
	change = convert(
		[]proto.Message{message0[0]},
		[]string{"bar-namespace/foo"},
		model.AuthenticationPolicy.MessageName)
	err = controller.Apply(change)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	c, err = controller.List(model.AuthenticationPolicy.Type, "bar-namespace")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(len(c)).To(gomega.Equal(1))
	g.Expect(c[0].Name).To(gomega.Equal("foo"))
	g.Expect(c[0].Namespace).To(gomega.Equal("bar-namespace"))
	g.Expect(c[0].Type).To(gomega.Equal(model.AuthenticationPolicy.Type))
	g.Expect(c[0].Spec).To(gomega.Equal(message0[0]))
}

func TestEventHandler(t *testing.T) {
	controller := coredatamodel.NewController(testControllerOptions)

	makeName := func(namespace, name string) string {
		return namespace + "/" + name
	}

	gotEvents := map[model.Event]map[string]model.Config{
		model.EventAdd:    map[string]model.Config{},
		model.EventUpdate: map[string]model.Config{},
		model.EventDelete: map[string]model.Config{},
	}
	controller.RegisterEventHandler(model.ServiceEntry.Type, func(m model.Config, e model.Event) {
		gotEvents[e][makeName(m.Namespace, m.Name)] = m
	})

	typeURL := "type.googleapis.com/istio.networking.v1alpha3.ServiceEntry"

	fakeCreateTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	fakeCreateTimeProto, err := types.TimestampProto(fakeCreateTime)
	if err != nil {
		t.Fatalf("Failed to parse create fake create time %v: %v", fakeCreateTime, err)
	}

	makeServiceEntry := func(name, host, version string) *mcpclient.Object {
		return &mcpclient.Object{
			TypeURL: typeURL,
			Metadata: &mcpapi.Metadata{
				Name:       fmt.Sprintf("default/%s", name),
				CreateTime: fakeCreateTimeProto,
				Version:    version,
			},
			Resource: &networking.ServiceEntry{
				Hosts: []string{host},
			},
		}
	}

	makeServiceEntryModel := func(name, host, version string) model.Config {
		return model.Config{
			ConfigMeta: model.ConfigMeta{
				Type:              model.ServiceEntry.Type,
				Group:             model.ServiceEntry.Group,
				Version:           model.ServiceEntry.Version,
				Name:              name,
				Namespace:         "default",
				Domain:            "cluster.local",
				ResourceVersion:   version,
				CreationTimestamp: fakeCreateTime,
			},
			Spec: &networking.ServiceEntry{Hosts: []string{host}},
		}
	}

	// Note: these tests steps are cumulative
	steps := []struct {
		name   string
		change *mcpclient.Change
		want   map[model.Event]map[string]model.Config
	}{
		{
			name: "initial add",
			change: &mcpclient.Change{
				TypeURL: typeURL,
				Objects: []*mcpclient.Object{
					makeServiceEntry("foo", "foo.com", "v0"),
				},
			},
			want: map[model.Event]map[string]model.Config{
				model.EventAdd: map[string]model.Config{
					"default/foo": makeServiceEntryModel("foo", "foo.com", "v0"),
				},
			},
		},
		{
			name: "update initial item",
			change: &mcpclient.Change{
				TypeURL: typeURL,
				Objects: []*mcpclient.Object{
					makeServiceEntry("foo", "foo.com", "v1"),
				},
			},
			want: map[model.Event]map[string]model.Config{
				model.EventUpdate: map[string]model.Config{
					"default/foo": makeServiceEntryModel("foo", "foo.com", "v1"),
				},
			},
		},
		{
			name: "subsequent add",
			change: &mcpclient.Change{
				TypeURL: typeURL,
				Objects: []*mcpclient.Object{
					makeServiceEntry("foo", "foo.com", "v1"),
					makeServiceEntry("foo1", "foo1.com", "v0"),
				},
			},
			want: map[model.Event]map[string]model.Config{
				model.EventAdd: map[string]model.Config{
					"default/foo1": makeServiceEntryModel("foo1", "foo1.com", "v0"),
				},
			},
		},
		{
			name: "single delete",
			change: &mcpclient.Change{
				TypeURL: typeURL,
				Objects: []*mcpclient.Object{
					makeServiceEntry("foo1", "foo1.com", "v0"),
				},
			},
			want: map[model.Event]map[string]model.Config{
				model.EventDelete: map[string]model.Config{
					"default/foo": makeServiceEntryModel("foo", "foo.com", "v1"),
				},
			},
		},
		{
			name: "multiple update and add",
			change: &mcpclient.Change{
				TypeURL: typeURL,
				Objects: []*mcpclient.Object{
					makeServiceEntry("foo1", "foo1.com", "v1"),
					makeServiceEntry("foo2", "foo2.com", "v0"),
					makeServiceEntry("foo3", "foo3.com", "v0"),
				},
			},
			want: map[model.Event]map[string]model.Config{
				model.EventAdd: map[string]model.Config{
					"default/foo2": makeServiceEntryModel("foo2", "foo2.com", "v0"),
					"default/foo3": makeServiceEntryModel("foo3", "foo3.com", "v0"),
				},
				model.EventUpdate: map[string]model.Config{
					"default/foo1": makeServiceEntryModel("foo1", "foo1.com", "v1"),
				},
			},
		},
		{
			name: "multiple deletes, updates, and adds ",
			change: &mcpclient.Change{
				TypeURL: typeURL,
				Objects: []*mcpclient.Object{
					makeServiceEntry("foo2", "foo2.com", "v1"),
					makeServiceEntry("foo3", "foo3.com", "v0"),
					makeServiceEntry("foo4", "foo4.com", "v0"),
					makeServiceEntry("foo5", "foo5.com", "v0"),
				},
			},
			want: map[model.Event]map[string]model.Config{
				model.EventAdd: map[string]model.Config{
					"default/foo4": makeServiceEntryModel("foo4", "foo4.com", "v0"),
					"default/foo5": makeServiceEntryModel("foo5", "foo5.com", "v0"),
				},
				model.EventUpdate: map[string]model.Config{
					"default/foo2": makeServiceEntryModel("foo2", "foo2.com", "v1"),
				},
				model.EventDelete: map[string]model.Config{
					"default/foo1": makeServiceEntryModel("foo1", "foo1.com", "v1"),
				},
			},
		},
	}

	for i, s := range steps {
		t.Run(fmt.Sprintf("[%v] %s", i, s.name), func(tt *testing.T) {
			if err := controller.Apply(s.change); err != nil {
				tt.Fatalf("Apply() failed: %v", err)
			}

			for eventType, wantConfigs := range s.want {
				gotConfigs := gotEvents[eventType]
				if !reflect.DeepEqual(gotConfigs, wantConfigs) {
					tt.Fatalf("wrong %v event: \n got %+v \nwant %+v", eventType, gotConfigs, wantConfigs)
				}
			}
			// clear saved events after every step
			gotEvents = map[model.Event]map[string]model.Config{
				model.EventAdd:    map[string]model.Config{},
				model.EventUpdate: map[string]model.Config{},
				model.EventDelete: map[string]model.Config{},
			}
		})
	}
}
