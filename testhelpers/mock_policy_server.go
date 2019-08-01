package testhelpers

import (
	"code.cloudfoundry.org/policy_client"
	"encoding/json"
	"github.com/onsi/gomega/ghttp"
	"net/http"
	"strconv"
)

type MockPolicyServer struct {
	Server *ghttp.Server
}

func NewMockPolicyServer() *MockPolicyServer {
	policyServer := ghttp.NewUnstartedServer()
	return &MockPolicyServer{Server: policyServer}
}

func (m *MockPolicyServer) SetGetPoliciesResponse(c2cPolicies []*policy_client.Policy) {
	m.Server.RouteToHandler("GET", "/networking/v1/internal/policies", func(w http.ResponseWriter, req *http.Request) {
		var policies struct {
			Policies       []*policy_client.Policy       `json:"policies"`
			EgressPolicies []*policy_client.EgressPolicy `json:"egress_policies"`
		}
		policies.Policies = c2cPolicies
		data, _ := json.Marshal(&policies)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}
