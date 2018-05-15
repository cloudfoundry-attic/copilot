require_relative './support/test_client'
require_relative './support/fake_copilot_server'

RSpec.describe Cloudfoundry::Copilot do
  before(:all) do
    @handlers = FakeCopilotHandlers.new
    @server = FakeCopilotServer.new(@handlers)

    @client = TestClient.new(
      @server.host,
      @server.port
    )
  end

  after(:all) do
    @server.stop
  end

  it 'can upsert a route' do
    expect(
      @client.upsert_route(
        guid: 'some-route-guid',
        host: 'some-route-url',
        path: '/some/path'
      )
    ).to be_a(::Api::UpsertRouteResponse)

    expect(@handlers.upsert_route_got_request).to eq(
      Api::UpsertRouteRequest.new(
        route: Api::Route.new(
          guid: 'some-route-guid',
          host: 'some-route-url',
          path: '/some/path'
        )
      )
    )
  end

  it 'can delete a route' do
    expect(@client.delete_route(
             guid: 'some-route-guid'
    )).to be_a(::Api::DeleteRouteResponse)

    expect(@handlers.delete_route_got_request).to eq(
      Api::DeleteRouteRequest.new(
        guid: 'some-route-guid'
      )
    )
  end

  it 'can map a route' do
    expect(@client.map_route(
             capi_process_guid: 'some-capi-process-guid-to-map',
             route_guid: 'some-route-guid-to-map'
    )).to be_a(::Api::MapRouteResponse)

    expect(@handlers.map_route_got_request).to eq(
      Api::MapRouteRequest.new(route_mapping: Api::RouteMapping.new(
        capi_process_guid: 'some-capi-process-guid-to-map',
        route_guid: 'some-route-guid-to-map'
      ))
    )
  end

  it 'can unmap a route' do
    expect(@client.unmap_route(
             capi_process_guid: 'some-capi-process-guid-to-unmap',
             route_guid: 'some-route-guid-to-unmap'
    )).to be_a(::Api::UnmapRouteResponse)

    expect(@handlers.unmap_route_got_request).to eq(
      Api::UnmapRouteRequest.new(route_mapping: Api::RouteMapping.new(
        capi_process_guid: 'some-capi-process-guid-to-unmap',
        route_guid: 'some-route-guid-to-unmap'
      ))
    )
  end

  it 'can upsert a capi diego process association' do
    expect(@client.upsert_capi_diego_process_association(
             capi_process_guid: 'some-capi-process-guid',
             diego_process_guids: ['some-diego-process-guid']
    )).to be_a(::Api::UpsertCapiDiegoProcessAssociationResponse)

    expect(@handlers.upsert_capi_diego_process_association_got_request).to eq(Api::UpsertCapiDiegoProcessAssociationRequest.new(
                                                                                capi_diego_process_association: {
                                                                                  capi_process_guid: 'some-capi-process-guid',
                                                                                  diego_process_guids: ['some-diego-process-guid']
                                                                                }
    ))
  end

  it 'can delete a capi diego process association' do
    expect(@client.delete_capi_diego_process_association(
             capi_process_guid: 'some-capi-process-guid'
    )).to be_a(::Api::DeleteCapiDiegoProcessAssociationResponse)

    expect(@handlers.delete_capi_diego_process_association_got_request).to eq(Api::DeleteCapiDiegoProcessAssociationRequest.new(
                                                                                capi_process_guid: 'some-capi-process-guid'
    ))
  end

  it 'can sync' do
    expect(@client.bulk_sync(
             routes: [{ guid: 'some-route-guid', host: 'example.host.com', path: '/some/path' }],
             route_mappings: [{ route_guid: 'some-route-guid', capi_process_guid: 'some-capi-process-guid' }],
             capi_diego_process_associations: [{ capi_process_guid: 'some-capi-process-guid', diego_process_guids: ['some-diego-process-guid'] }]
    )).to be_a(::Api::BulkSyncResponse)

    expect(@handlers.bulk_sync_got_request).to eq(Api::BulkSyncRequest.new(
                                                    routes: [{ guid: 'some-route-guid', host: 'example.host.com', path: '/some/path' }],
                                                    route_mappings: [{ route_guid: 'some-route-guid', capi_process_guid: 'some-capi-process-guid' }],
                                                    capi_diego_process_associations: [{ capi_process_guid: 'some-capi-process-guid', diego_process_guids: ['some-diego-process-guid'] }]
    ))
  end
end

class FakeCopilotHandlers < Api::CloudControllerCopilot::Service
  attr_reader :upsert_route_got_request, :delete_route_got_request, :map_route_got_request,
              :unmap_route_got_request, :upsert_capi_diego_process_association_got_request,
              :delete_capi_diego_process_association_got_request, :bulk_sync_got_request

  def health(_healthRequest, _call)
    ::Api::HealthResponse.new(healthy: true)
  end

  def upsert_route(upsertRouteRequest, _call)
    @upsert_route_got_request = upsertRouteRequest
    ::Api::UpsertRouteResponse.new
  end

  def delete_route(request, _call)
    @delete_route_got_request = request
    ::Api::DeleteRouteResponse.new
  end

  def map_route(request, _call)
    @map_route_got_request = request
    ::Api::MapRouteResponse.new
  end

  def unmap_route(request, _call)
    @unmap_route_got_request = request
    ::Api::UnmapRouteResponse.new
  end

  def upsert_capi_diego_process_association(request, _call)
    @upsert_capi_diego_process_association_got_request = request
    ::Api::UpsertCapiDiegoProcessAssociationResponse.new
    end

  def delete_capi_diego_process_association(request, _call)
    @delete_capi_diego_process_association_got_request = request
    ::Api::DeleteCapiDiegoProcessAssociationResponse.new
  end

  def bulk_sync(request, _call)
    @bulk_sync_got_request = request
    ::Api::BulkSyncResponse.new
  end
end
