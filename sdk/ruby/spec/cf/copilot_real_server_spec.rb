require_relative './support/test_client'
require_relative './support/real_copilot_server'

RSpec.describe Cloudfoundry::Copilot do
  before(:all) do
    @server = RealCopilotServer.new
    @client = TestClient.new(
      @server.host,
      @server.port
    )
  end

  after(:all) do
    @server.stop
  end

  it 'can upsert a route' do
    expect(@client.upsert_route(
      guid: 'some-route-guid',
      host: 'some-route-url',
      path: '/some/path'
    )).to be_a(::Api::UpsertRouteResponse)
  end

  it 'can upsert a route with an internal flag' do
    expect(
      @client.upsert_route(
        guid: 'some-route-guid',
        host: 'some-route-url',
        path: '/some/path',
        internal: true,
        vip: '1.2.3.4',
      )
    ).to be_a(::Api::UpsertRouteResponse)
  end

  it 'raises an error if internal is true but vip is nil' do
    expect do
      @client.upsert_route(
        guid: 'some-route-guid',
        host: 'some-route-url',
        path: '/some/path',
        internal: true,
      )
    end.to raise_error(
      Cloudfoundry::Copilot::Client::PilotError,
      'vip required for internal routes'
    )
  end

  it 'can delete a route' do
    expect(@client.delete_route(
      guid: 'some-route-guid'
    )).to be_a(::Api::DeleteRouteResponse)
  end

  it 'can map a route' do
    expect(@client.map_route(
      capi_process_guid: 'some-capi-process-guid',
      route_guid: 'some-route-guid',
      route_weight: 128
    )).to be_a(::Api::MapRouteResponse)

  end

  it 'can unmap a route' do
    expect(@client.unmap_route(
             capi_process_guid: 'some-capi-process-guid-to-unmap',
             route_guid: 'some-route-guid-to-unmap',
             route_weight: 128
    )).to be_a(::Api::UnmapRouteResponse)
  end

  it 'can upsert a capi-diego-process-association' do
    expect(@client.upsert_capi_diego_process_association(
             capi_process_guid: 'some-capi-process-guid',
             diego_process_guids: ['some-diego-process-guid']
    )).to be_a(::Api::UpsertCapiDiegoProcessAssociationResponse)
  end

  it 'can delete a capi-diego-process-association' do
    expect(@client.delete_capi_diego_process_association(
             capi_process_guid: 'some-capi-process-guid'
    )).to be_a(::Api::DeleteCapiDiegoProcessAssociationResponse)
  end

  it 'can chunk requests during bulk sync' do
    # by sending only a single route, route_mapping, capi_diego_process_association group
    x = create_and_bulk_sync(1)
    expect(x[:response].total_bytes_received).to equal(x[:request].to_proto.length)

    # by sending multiple route, route_mapping, capi_diego_process_association groups
    x = create_and_bulk_sync(3)
    expect(x[:response].total_bytes_received).to equal(x[:request].to_proto.length)
  end

  context 'when GRPC raises a PilotError' do
    let(:service) { instance_double(Api::CloudControllerCopilot::Stub) }

    before do
      allow(@client).to receive(:service).and_return(service)
    end

    context 'upsert route' do
      before do
        allow(service).to receive(:upsert_route)
          .and_raise(GRPC::Unknown.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect do
          @client.upsert_route(
            guid: 'some-route-guid',
            host: 'some-route-url'
          )
        end.to raise_error(
          Cloudfoundry::Copilot::Client::PilotError,
          'some cause - {:data=>"metadata"}'
        )
      end
    end

    context 'delete route' do
      before do
        allow(service).to receive(:delete_route)
          .and_raise(GRPC::Unknown.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect do
          @client.delete_route(
            guid: 'some-route-guid'
          )
        end.to raise_error(
          Cloudfoundry::Copilot::Client::PilotError,
          'some cause - {:data=>"metadata"}'
        )
      end
    end

    context 'map route' do
      before do
        allow(service).to receive(:map_route)
          .and_raise(GRPC::Unknown.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect do
          @client.map_route(
            capi_process_guid: 'some-capi-process-guid',
            route_guid: 'some-route-guid',
            route_weight: 128
          )
        end.to raise_error(
          Cloudfoundry::Copilot::Client::PilotError,
          'some cause - {:data=>"metadata"}'
        )
      end
    end

    context 'unmap route' do
      before do
        allow(service).to receive(:unmap_route)
          .and_raise(GRPC::Unknown.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect do
          @client.unmap_route(
            capi_process_guid: 'some-capi-process-guid',
            route_guid: 'some-route-guid',
            route_weight: 128
          )
        end.to raise_error(
          Cloudfoundry::Copilot::Client::PilotError,
          'some cause - {:data=>"metadata"}'
        )
      end
    end

    context 'upsert capi diego process association' do
      before do
        allow(service).to receive(:upsert_capi_diego_process_association)
          .and_raise(GRPC::Cancelled.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect do
          @client.upsert_capi_diego_process_association(
            capi_process_guid: 'some-capi-process-guid',
            diego_process_guids: ['some-diego-guid']
          )
        end.to raise_error(Cloudfoundry::Copilot::Client::PilotError, 'some cause - {:data=>"metadata"}')
      end
    end

    context 'delete capi diego process association' do
      before do
        allow(service).to receive(:delete_capi_diego_process_association)
          .and_raise(GRPC::Cancelled.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect do
          @client.delete_capi_diego_process_association(capi_process_guid: 'some-capi-process-guid')
        end.to raise_error(Cloudfoundry::Copilot::Client::PilotError, 'some cause - {:data=>"metadata"}')
      end
    end

    context 'bulk sync' do
      before do
        allow(service).to receive(:bulk_sync)
          .and_raise(GRPC::DeadlineExceeded.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect do
          @client.bulk_sync(routes: [],
                            route_mappings: [],
                            capi_diego_process_associations: [])
        end.to raise_error(Cloudfoundry::Copilot::Client::PilotError, 'some cause - {:data=>"metadata"}')
      end
    end
  end
end

def create_and_bulk_sync(n_messages)
  n = 0
  routes = []
  route_mappings = []
  capi_diego_process_associations = []

  until n == n_messages
    routes << Api::Route.new(
      guid:  "some-route-guid-%d" % n,
      host: 'example.host.com',
      path: '/some/path'
    )
    route_mappings << Api::RouteMapping.new(
      route_guid: "some-route-guid-%d" % n,
      capi_process_guid: "some-capi-process-guid-%d" % n,
      route_weight: 128
    )
    capi_diego_process_associations << Api::CapiDiegoProcessAssociation.new(
      capi_process_guid: "some-capi-process-guid-%d" % n,
      diego_process_guids: ["some-diego-process-guid-%d" % n]
    )
    n += 1
  end
  resp = @client.bulk_sync(routes: routes,
                           route_mappings: route_mappings,
                           capi_diego_process_associations: capi_diego_process_associations
                          )
  req = Api::BulkSyncRequest.new(
    routes: routes,
    route_mappings: route_mappings,
    capi_diego_process_associations: capi_diego_process_associations
  )
  { response: resp, request: req }
end
