class TestClient < Cloudfoundry::Copilot::Client
  def initialize(host, port)
    super(
        host: host,
        port: port,
        client_ca_file: 'spec/cf/fixtures/fakeCA.crt',
        client_key_file: 'spec/cf/fixtures/cloud-controller-client.key',
        client_chain_file: 'spec/cf/fixtures/cloud-controller-client.crt'
    )
    healthy = false
    num_tries = 0
    until healthy
      begin
        healthy = health
      rescue
        sleep 1
        num_tries += 1
        raise "copilot didn't become healthy" if num_tries > 5
      end
    end
  end
end
