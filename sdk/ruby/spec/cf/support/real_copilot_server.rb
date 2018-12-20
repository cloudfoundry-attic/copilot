require 'json'
require 'tempfile'

class RealCopilotServer
  attr_reader :port, :host

  def fixture(name)
    File.expand_path("#{File.dirname(__FILE__)}/../fixtures/#{name}")
  end

  def initialize
    @port = 51_002
    @host = '127.0.0.1'

    config = {
      'ListenAddressForCloudController' => "#{host}:#{port}",
      'ListenAddressForMCP' => "#{host}:51003",
      'PilotClientCAPath' => fixture('fakeCA.crt'),
      'CloudControllerClientCAPath' => fixture('fakeCA.crt'),
      'ServerCertPath' => fixture('copilot-server.crt'),
      'ServerKeyPath' => fixture('copilot-server.key'),
      'VIPCIDR' => "127.128.0.0/9",
      'BBS' => { 'Disable' => true },
      'PilotLogLevel' => 'fatal',
    }

    config_file = Tempfile.new('copilot-config')
    config_file.write(config.to_json)
    config_file.close

    @copilot_server = fork do
      exec "copilot-server -config #{config_file.path}"
    end

    Process.detach(@copilot_server)
  end

  def stop
    Process.kill('TERM', @copilot_server)
  end
end
