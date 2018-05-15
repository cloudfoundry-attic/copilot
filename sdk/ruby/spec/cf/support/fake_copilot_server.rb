class FakeCopilotServer
  attr_reader :port, :host, :handlers

  def initialize(handlers)
    @port = 51_002
    @host = '127.0.0.1'

    private_key_content = File.read('spec/cf/fixtures/copilot-server.key')
    cert_content = File.read('spec/cf/fixtures/copilot-server.crt')
    server_creds = GRPC::Core::ServerCredentials.new(
      nil, [{ private_key: private_key_content, cert_chain: cert_content }], true
    )

    @server = GRPC::RpcServer.new
    @server.add_http2_port("#{@host}:#{@port}", server_creds)

    @server.handle(handlers)

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
