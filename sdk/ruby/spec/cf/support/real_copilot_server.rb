require 'json'
require 'tempfile'

class RealCopilotServer
  attr_reader :port, :host

  def fixture(name)
    File.expand_path("#{File.dirname(__FILE__)}/../fixtures/#{name}")
  end

  def initialize
    @port = 51002
    @host = "127.0.0.1"

    config = {
      "ListenAddressForPilot" => "#{host}:51001",
      "ListenAddressForCloudController" => "#{host}:#{port}",
      "PilotClientCAPath" => fixture("fakeCA.crt"),
      "CloudControllerClientCAPath" => fixture("fakeCA.crt"),
      "ServerCertPath" => fixture("copilot-server.crt"),
      "ServerKeyPath" => fixture("copilot-server.key"),
      "BBS" => { "Disable" => true  }
    }

    config_file = Tempfile.new("copilot-config")
    config_file.write(config.to_json)
    config_file.close

    @copilotServer = fork do
      exec "copilot-server -config #{config_file.path}"
    end

    Process.detach(@copilotServer)
  end

  def stop
    Process.kill("TERM", @copilotServer)
  end
end
