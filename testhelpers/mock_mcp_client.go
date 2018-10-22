package testhelpers

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	copilotsnapshot "code.cloudfoundry.org/copilot/snapshot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	mcp "istio.io/api/mcp/v1alpha1"
	"istio.io/api/networking/v1alpha3"
	mcpclient "istio.io/istio/pkg/mcp/client"
)

type MockMCPUpdater struct {
	changesMux sync.Mutex
	objects    map[string][]*mcpclient.Object
}

func (m *MockMCPUpdater) Apply(c *mcpclient.Change) error {
	m.changesMux.Lock()
	defer m.changesMux.Unlock()
	m.objects[c.TypeURL] = c.Objects
	return nil
}

func (m *MockMCPUpdater) GetAllObjectNames() map[string][]string {
	m.changesMux.Lock()
	defer m.changesMux.Unlock()
	allObjectNames := map[string][]string{}
	for obType, objects := range m.objects {
		for _, object := range objects {
			allObjectNames[obType] = append(allObjectNames[obType], object.Metadata.Name)
		}
	}
	return allObjectNames
}

func (m *MockMCPUpdater) GetAllVirtualServices() []*v1alpha3.VirtualService {
	m.changesMux.Lock()
	defer m.changesMux.Unlock()
	allVirtualServices := []*v1alpha3.VirtualService{}
	for _, vs := range m.objects[copilotsnapshot.VirtualServiceTypeURL] {
		r := vs.Resource.(interface{}).(*v1alpha3.VirtualService)
		allVirtualServices = append(allVirtualServices, r)
	}

	return allVirtualServices
}

func (m *MockMCPUpdater) GetAllDestinationRules() []*v1alpha3.DestinationRule {
	m.changesMux.Lock()
	defer m.changesMux.Unlock()
	allDestinationRules := []*v1alpha3.DestinationRule{}
	for _, o := range m.objects[copilotsnapshot.DestinationRuleTypeURL] {
		r := o.Resource.(interface{}).(*v1alpha3.DestinationRule)
		allDestinationRules = append(allDestinationRules, r)
	}

	return allDestinationRules
}

func (m *MockMCPUpdater) GetAllGateways() []*v1alpha3.Gateway {
	m.changesMux.Lock()
	defer m.changesMux.Unlock()
	allGateways := []*v1alpha3.Gateway{}
	for _, o := range m.objects[copilotsnapshot.GatewayTypeURL] {
		r := o.Resource.(interface{}).(*v1alpha3.Gateway)
		allGateways = append(allGateways, r)
	}

	return allGateways
}

func (m *MockMCPUpdater) GetAllServiceEntries() []*v1alpha3.ServiceEntry {
	m.changesMux.Lock()
	defer m.changesMux.Unlock()
	allServiceEntries := []*v1alpha3.ServiceEntry{}
	for _, o := range m.objects[copilotsnapshot.ServiceEntryTypeURL] {
		r := o.Resource.(interface{}).(*v1alpha3.ServiceEntry)
		allServiceEntries = append(allServiceEntries, r)
	}

	return allServiceEntries
}

func (m *MockMCPUpdater) GetAllMessageNames() []string {
	m.changesMux.Lock()
	defer m.changesMux.Unlock()
	typeURLs := []string{}
	for k, _ := range m.objects {
		typeURLs = append(typeURLs, k)
	}
	return typeURLs
}

type MockPilotMCPClient struct {
	ctx        context.Context
	client     *mcpclient.Client
	cancelFunc func()
	conn       *grpc.ClientConn
	*MockMCPUpdater
}

func (m *MockPilotMCPClient) Close() error {
	m.cancelFunc()
	err := m.conn.Close()
	return err
}

func NewMockPilotMCPClient(tlsConfig *tls.Config, serverAddr string) (*MockPilotMCPClient, error) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithTimeout(5 * time.Second),
	}

	conn, err := grpc.DialContext(context.Background(), serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	svcClient := mcp.NewAggregatedMeshConfigServiceClient(conn)
	mockUpdater := &MockMCPUpdater{objects: make(map[string][]*mcpclient.Object)}

	typeURLs := []string{
		copilotsnapshot.GatewayTypeURL,
		copilotsnapshot.VirtualServiceTypeURL,
		copilotsnapshot.DestinationRuleTypeURL,
		copilotsnapshot.ServiceEntryTypeURL,
		copilotsnapshot.EnvoyFilterTypeURL,
		copilotsnapshot.HTTPAPISpecTypeURL,
		copilotsnapshot.HTTPAPISpecBindingTypeURL,
		copilotsnapshot.QuotaSpecTypeURL,
		copilotsnapshot.QuotaSpecBindingTypeURL,
		copilotsnapshot.PolicyTypeURL,
		copilotsnapshot.MeshPolicyTypeURL,
		copilotsnapshot.ServiceRoleTypeURL,
		copilotsnapshot.ServiceRoleBindingTypeURL,
		copilotsnapshot.RbacConfigTypeURL,
	}
	cl := mcpclient.New(svcClient, typeURLs, mockUpdater, "", nil)
	ctx, cancelFunc := context.WithCancel(context.Background())
	go cl.Run(ctx)

	return &MockPilotMCPClient{
		ctx:            ctx,
		client:         cl,
		cancelFunc:     cancelFunc,
		conn:           conn,
		MockMCPUpdater: mockUpdater,
	}, nil
}
