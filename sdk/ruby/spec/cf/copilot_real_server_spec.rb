require_relative './support/test_client'
require_relative './support/real_copilot_server'

RSpec.describe Cloudfoundry::Copilot do
  before(:all) do
    @real_copilot_server = RealCopilotServer.new
    @client = TestClient.new(
       @real_copilot_server.host,
       @real_copilot_server.port
    )
  end

  after(:all) do
    @real_copilot_server.stop
  end

  it "can upsert a route" do
    @client.upsert_route(
       guid: "some-route-guid",
       host: "some-route-url"
    )
  end

  it "can delete a route" do
    @client.delete_route(
      guid: "some-route-guid"
    )
  end

  it "can map a route" do
    @client.map_route(
      capi_process_guid: "some-capi-process-guid",
      route_guid: "some-route-guid"
    )
  end

  it "can unmap a route" do
    @client.unmap_route(
      capi_process_guid: "some-capi-process-guid",
      route_guid: "some-route-guid"
    )
  end
end
