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

package model_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"

	mpb "istio.io/api/mixer/v1"
	mccpb "istio.io/api/mixer/v1/config/client"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
)

func TestGogoProtoSchemaConversions(t *testing.T) {
	msg := &mccpb.HTTPAPISpec{
		Attributes: &mpb.Attributes{
			Attributes: map[string]*mpb.Attributes_AttributeValue{
				"api.service": {
					Value: &mpb.Attributes_AttributeValue_StringValue{
						StringValue: "my-service",
					},
				},
				"api.version": {
					Value: &mpb.Attributes_AttributeValue_StringValue{
						StringValue: "1.0.0",
					},
				},
			},
		},
		Patterns: []*mccpb.HTTPAPISpecPattern{
			{
				Attributes: &mpb.Attributes{
					Attributes: map[string]*mpb.Attributes_AttributeValue{
						"api.operation": {
							Value: &mpb.Attributes_AttributeValue_StringValue{
								StringValue: "createPet",
							},
						},
					},
				},
				HttpMethod: "POST",
				Pattern: &mccpb.HTTPAPISpecPattern_UriTemplate{
					UriTemplate: "/pet/{id}",
				},
			},
		},
		ApiKeys: []*mccpb.APIKey{
			{
				Key: &mccpb.APIKey_Query{
					Query: "api_key",
				},
			},
		},
	}

	wantYAML := `apiKeys:
- query: api_key
attributes:
  attributes:
    api.service:
      stringValue: my-service
    api.version:
      stringValue: 1.0.0
patterns:
- attributes:
    attributes:
      api.operation:
        stringValue: createPet
  httpMethod: POST
  uriTemplate: /pet/{id}
`

	wantJSON := `
{
   "attributes": {
      "attributes": {
         "api.service": {
            "stringValue": "my-service"
         },
         "api.version": {
            "stringValue": "1.0.0"
         }
      }
   },
   "patterns": [
      {
         "attributes": {
            "attributes": {
               "api.operation": {
                  "stringValue": "createPet"
               }
            }
         },
         "httpMethod": "POST",
         "uriTemplate": "/pet/{id}"
      }
   ],
   "apiKeys": [
      {
         "query": "api_key"
      }
   ]
}
`
	wantJSONMap := map[string]interface{}{
		"attributes": map[string]interface{}{
			"attributes": map[string]interface{}{
				"api.service": map[string]interface{}{
					"stringValue": "my-service",
				},
				"api.version": map[string]interface{}{
					"stringValue": "1.0.0",
				},
			},
		},
		"patterns": []interface{}{
			map[string]interface{}{
				"httpMethod":  "POST",
				"uriTemplate": "/pet/{id}",
				"attributes": map[string]interface{}{
					"attributes": map[string]interface{}{
						"api.operation": map[string]interface{}{
							"stringValue": "createPet",
						},
					},
				},
			},
		},
		"apiKeys": []interface{}{
			map[string]interface{}{
				"query": "api_key",
			},
		},
	}

	gotJSON, err := model.ToJSON(msg)
	if err != nil {
		t.Errorf("ToJSON failed: %v", err)
	}
	if gotJSON != strings.Join(strings.Fields(wantJSON), "") {
		t.Errorf("ToJSON failed: \ngot %s, \nwant %s", gotJSON, strings.Join(strings.Fields(wantJSON), ""))
	}

	if _, err = model.ToJSON(nil); err == nil {
		t.Error("should produce an error")
	}

	gotFromJSON, err := model.HTTPAPISpec.FromJSON(wantJSON)
	if err != nil {
		t.Errorf("FromJSON failed: %v", err)
	}
	if !reflect.DeepEqual(gotFromJSON, msg) {
		t.Errorf("FromYAML failed: got %+v want %+v", spew.Sdump(gotFromJSON), spew.Sdump(msg))
	}

	gotYAML, err := model.ToYAML(msg)
	if err != nil {
		t.Errorf("ToYAML failed: %v", err)
	}
	if !reflect.DeepEqual(gotYAML, wantYAML) {
		t.Errorf("ToYAML failed: \ngot %+v \nwant %+v", spew.Sdump(gotYAML), spew.Sdump(wantYAML))
	}

	if _, err = model.ToYAML(nil); err == nil {
		t.Error("should produce an error")
	}

	gotFromYAML, err := model.HTTPAPISpec.FromYAML(wantYAML)
	if err != nil {
		t.Errorf("FromYAML failed: %v", err)
	}
	if !reflect.DeepEqual(gotFromYAML, msg) {
		t.Errorf("FromYAML failed: got %+v want %+v", spew.Sdump(gotFromYAML), spew.Sdump(msg))
	}

	if _, err = model.HTTPAPISpec.FromYAML(":"); err == nil {
		t.Errorf("should produce an error")
	}

	gotJSONMap, err := model.ToJSONMap(msg)
	if err != nil {
		t.Errorf("ToJSONMap failed: %v", err)
	}
	if !reflect.DeepEqual(gotJSONMap, wantJSONMap) {
		t.Errorf("ToJSONMap failed: \ngot %vwant %v", spew.Sdump(gotJSONMap), spew.Sdump(wantJSONMap))
	}

	if _, err = model.ToJSONMap(nil); err == nil {
		t.Error("should produce an error")
	}

	gotFromJSONMap, err := model.HTTPAPISpec.FromJSONMap(wantJSONMap)
	if err != nil {
		t.Errorf("FromJSONMap failed: %v", err)
	}
	if !reflect.DeepEqual(gotFromJSONMap, msg) {
		t.Errorf("FromJSONMap failed: got %+v want %+v", spew.Sdump(gotFromJSONMap), spew.Sdump(msg))
	}

	if _, err = model.HTTPAPISpec.FromJSONMap(1); err == nil {
		t.Error("should produce an error")
	}
	if _, err = model.HTTPAPISpec.FromJSON(":"); err == nil {
		t.Errorf("should produce an error")
	}
}

func TestProtoSchemaConversions(t *testing.T) {
	destinationRuleSchema := &model.ProtoSchema{MessageName: model.DestinationRule.MessageName}

	msg := &networking.DestinationRule{
		Host: "something.svc.local",
		TrafficPolicy: &networking.TrafficPolicy{
			LoadBalancer: &networking.LoadBalancerSettings{
				LbPolicy: &networking.LoadBalancerSettings_Simple{},
			},
		},
		Subsets: []*networking.Subset{
			{
				Name: "foo",
				Labels: map[string]string{
					"test": "label",
				},
			},
		},
	}

	wantJSON := `
		{
      "host":"something.svc.local",
      "trafficPolicy": {
        "loadBalancer":{"simple":"ROUND_ROBIN"}
       },
       "subsets": [
         {"name":"foo","labels":{"test":"label"}}
       ]
		}`

	wantYAML := `host: something.svc.local
subsets:
- labels:
    test: label
  name: foo
trafficPolicy:
  loadBalancer:
    simple: ROUND_ROBIN
`

	wantJSONMap := map[string]interface{}{
		"host": "something.svc.local",
		"trafficPolicy": map[string]interface{}{
			"loadBalancer": map[string]interface{}{
				"simple": "ROUND_ROBIN",
			},
		},
		"subsets": []interface{}{
			map[string]interface{}{
				"name": "foo",
				"labels": map[string]interface{}{
					"test": "label",
				},
			},
		},
	}

	badSchema := &model.ProtoSchema{MessageName: "bad-name"}
	if _, err := badSchema.FromYAML(wantYAML); err == nil {
		t.Errorf("FromYAML should have failed using ProtoSchema with bad MessageName")
	}

	gotJSON, err := model.ToJSON(msg)
	if err != nil {
		t.Errorf("ToJSON failed: %v", err)
	}
	if gotJSON != strings.Join(strings.Fields(wantJSON), "") {
		t.Errorf("ToJSON failed: got %s, want %s", gotJSON, wantJSON)
	}

	if _, err = model.ToJSON(nil); err == nil {
		t.Error("should produce an error")
	}

	gotFromJSON, err := destinationRuleSchema.FromJSON(wantJSON)
	if err != nil {
		t.Errorf("FromJSON failed: %v", err)
	}
	if !reflect.DeepEqual(gotFromJSON, msg) {
		t.Errorf("FromYAML failed: got %+v want %+v", spew.Sdump(gotFromJSON), spew.Sdump(msg))
	}

	gotYAML, err := model.ToYAML(msg)
	if err != nil {
		t.Errorf("ToYAML failed: %v", err)
	}
	if !reflect.DeepEqual(gotYAML, wantYAML) {
		t.Errorf("ToYAML failed: got %+v want %+v", spew.Sdump(gotYAML), spew.Sdump(wantYAML))
	}

	if _, err = model.ToYAML(nil); err == nil {
		t.Error("should produce an error")
	}

	gotFromYAML, err := destinationRuleSchema.FromYAML(wantYAML)
	if err != nil {
		t.Errorf("FromYAML failed: %v", err)
	}
	if !reflect.DeepEqual(gotFromYAML, msg) {
		t.Errorf("FromYAML failed: got %+v want %+v", spew.Sdump(gotFromYAML), spew.Sdump(msg))
	}

	if _, err = destinationRuleSchema.FromYAML(":"); err == nil {
		t.Errorf("should produce an error")
	}

	gotJSONMap, err := model.ToJSONMap(msg)
	if err != nil {
		t.Errorf("ToJSONMap failed: %v", err)
	}
	if !reflect.DeepEqual(gotJSONMap, wantJSONMap) {
		t.Errorf("ToJSONMap failed: \ngot %vwant %v", spew.Sdump(gotJSONMap), spew.Sdump(wantJSONMap))
	}

	if _, err = model.ToJSONMap(nil); err == nil {
		t.Error("should produce an error")
	}

	gotFromJSONMap, err := destinationRuleSchema.FromJSONMap(wantJSONMap)
	if err != nil {
		t.Errorf("FromJSONMap failed: %v", err)
	}
	if !reflect.DeepEqual(gotFromJSONMap, msg) {
		t.Errorf("FromJSONMap failed: got %+v want %+v", spew.Sdump(gotFromJSONMap), spew.Sdump(msg))
	}

	if _, err = destinationRuleSchema.FromJSONMap(1); err == nil {
		t.Error("should produce an error")
	}
	if _, err = destinationRuleSchema.FromJSON(":"); err == nil {
		t.Errorf("should produce an error")
	}
}
