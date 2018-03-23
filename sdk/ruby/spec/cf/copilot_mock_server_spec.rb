require_relative './support/test_client'
require_relative './support/fake_copilot_server'

class FakeCopilotHandlers < Api::CloudControllerCopilot::Service
  attr_reader :upsert_route_got_request

  def health(_healthRequest, _call)
    ::Api::HealthResponse.new(healthy: true)
  end

  def upsert_route(upsertRouteRequest, _call)
    @upsert_route_got_request = upsertRouteRequest
    ::Api::UpsertRouteResponse.new
  end
end


RSpec.describe Cloudfoundry::Copilot do
  before(:all) do
    @handlers = FakeCopilotHandlers.new
    @fake_copilot_server = FakeCopilotServer.new(@handlers)

    @client = TestClient.new(
      @fake_copilot_server.host,
      @fake_copilot_server.port,
    )
  end

  after(:all) do
    @fake_copilot_server.stop
  end

  it 'can upsert a route' do
    expect(@client.upsert_route(
             guid: 'some-route-guid',
             host: 'some-route-url'
    )).to be_a(::Api::UpsertRouteResponse)

    expect(@handlers.upsert_route_got_request).to eq(
      Api::UpsertRouteRequest.new(
        route: Api::Route.new(guid: 'some-route-guid', host: 'some-route-url')
      )
    )
  end
end
