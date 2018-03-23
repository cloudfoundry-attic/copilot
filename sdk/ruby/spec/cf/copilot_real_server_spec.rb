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

end
