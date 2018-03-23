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

class FakeCopilotServer
  attr_reader :port, :host, :handlers

  def initialize
    @port = 5002
    @host = '127.0.0.1'

    private_key_content = File.read('spec/cf/fixtures/copilot-server.key')
    cert_content = File.read('spec/cf/fixtures/copilot-server.crt')
    server_creds = GRPC::Core::ServerCredentials.new(
        nil, [{ private_key: private_key_content, cert_chain: cert_content }], true
    )

    @server = GRPC::RpcServer.new
    @server.add_http2_port("#{@host}:#{@port}", server_creds)

    @handlers = FakeCopilotHandlers.new
    @server.handle(@handlers)
  end

  def start
    @thread = Thread.new do
      begin
        @server.run
      ensure
        @server.stop
      end
    end
  end

  def stop
    @server.stop
    Thread.kill(@thread)
  end
end

RSpec.describe Cloudfoundry::Copilot do
  before(:all) do
    @fake_copilot_server = FakeCopilotServer.new
    @fake_copilot_server.start

    @client = Cloudfoundry::Copilot::Client.new(
      host: @fake_copilot_server.host,
      port: @fake_copilot_server.port,
      client_ca_file: 'spec/cf/fixtures/fakeCA.crt',
      client_key_file: 'spec/cf/fixtures/cloud-controller-client.key',
      client_chain_file: 'spec/cf/fixtures/cloud-controller-client.crt'
    )
    healthy = false
    num_tries = 0
    until healthy
      begin
        healthy = @client.health
      rescue
        sleep 1
        num_tries += 1
        raise "copilot didn't become healthy" if num_tries > 5
      end
    end
  end

  after(:all) do
    @fake_copilot_server.stop
  end

  it 'can upsert a route' do
    expect(@client.upsert_route(
             guid: 'some-route-guid',
             host: 'some-route-url'
    )).to be_a(::Api::UpsertRouteResponse)

    expect(@fake_copilot_server.handlers.upsert_route_got_request).to eq(
      Api::UpsertRouteRequest.new(
        route: Api::Route.new(guid: 'some-route-guid', host: 'some-route-url')
      )
    )
  end
end
