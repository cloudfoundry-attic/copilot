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
    @client.upsert_route(
       guid: 'some-route-guid',
       host: 'some-route-url'
    )
  end

  it 'can delete a route' do
    @client.delete_route(
      guid: 'some-route-guid'
    )
  end

  it 'can map a route' do
    @client.map_route(
      capi_process_guid: 'some-capi-process-guid',
      route_guid: 'some-route-guid'
    )
  end

  it 'can unmap a route' do
    @client.unmap_route(
      capi_process_guid: 'some-capi-process-guid',
      route_guid: 'some-route-guid'
    )
  end

  it 'can upsert a capi-diego-process-association' do
    @client.upsert_capi_diego_process_association(
      capi_process_guid: 'some-capi-process-guid',
      diego_process_guids: ['some-diego-guid']
    )
  end

  it 'can delete a capi-diego-process-association' do
    @client.delete_capi_diego_process_association(
      capi_process_guid: 'some-capi-process-guid'
    )
  end

  it 'can bulk sync' do
    @client.bulk_sync(
      routes: [{guid: 'some-route-guid', host: 'example.host.com'}],
      route_mappings: [{route_guid: 'some-route-guid', capi_process_guid: 'some-capi-process-guid'}],
      capi_diego_process_associations: [{capi_process_guid: 'some-capi-process-guid', diego_process_guids: ['some-diego-process-guid']}]
    )
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
        expect {
          @client.upsert_route(
            guid: 'some-route-guid',
            host: 'some-route-url'
          )
        }.to raise_error(Cloudfoundry::Copilot::Client::PilotError, 'some cause - {:data=>"metadata"}')
      end
    end

    context 'delete route' do
      before do
        allow(service).to receive(:delete_route)
        .and_raise(GRPC::Unknown.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect {
          @client.delete_route(
            guid: 'some-route-guid'
          )
        }.to raise_error(Cloudfoundry::Copilot::Client::PilotError, 'some cause - {:data=>"metadata"}')
      end
    end

    context 'map route' do
      before do
        allow(service).to receive(:map_route)
        .and_raise(GRPC::Unknown.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect {
          @client.map_route(
            capi_process_guid: 'some-capi-process-guid',
            route_guid: 'some-route-guid'
          )
        }.to raise_error(Cloudfoundry::Copilot::Client::PilotError, 'some cause - {:data=>"metadata"}')
      end
    end

    context 'unmap route' do
      before do
        allow(service).to receive(:unmap_route)
        .and_raise(GRPC::Unknown.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect {
          @client.unmap_route(
            capi_process_guid: 'some-capi-process-guid',
            route_guid: 'some-route-guid'
          )
        }.to raise_error(Cloudfoundry::Copilot::Client::PilotError, 'some cause - {:data=>"metadata"}')
      end
    end

    context 'upsert cdpas' do
      before do
        allow(service).to receive(:upsert_capi_diego_process_association)
        .and_raise(GRPC::Cancelled.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect {
          @client.upsert_capi_diego_process_association(
            capi_process_guid: 'some-capi-process-guid',
            diego_process_guids: ['some-diego-guid']
          )
        }.to raise_error(Cloudfoundry::Copilot::Client::PilotError, 'some cause - {:data=>"metadata"}')
      end
    end

    context 'delete cdpas' do
      before do
        allow(service).to receive(:delete_capi_diego_process_association)
        .and_raise(GRPC::Cancelled.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect {
          @client.delete_capi_diego_process_association( capi_process_guid: 'some-capi-process-guid')
        }.to raise_error(Cloudfoundry::Copilot::Client::PilotError, 'some cause - {:data=>"metadata"}')
      end
    end

    context 'bulk sync' do
      before do
        allow(service).to receive(:bulk_sync)
        .and_raise(GRPC::DeadlineExceeded.new('some cause', data: 'metadata'))
      end

      it 'raises a PilotError' do
        expect {
          @client.bulk_sync(routes: [],
                            route_mappings: [],
                            capi_diego_process_associations: [])
        }.to raise_error(Cloudfoundry::Copilot::Client::PilotError, 'some cause - {:data=>"metadata"}')
      end
    end
  end
end
