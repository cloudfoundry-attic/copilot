require 'json'
require 'tempfile'

class RealCopilotServer
  attr_reader :resolver_port, :port, :host

  def fixture(name)
    File.expand_path("#{File.dirname(__FILE__)}/../fixtures/#{name}")
  end

  def initialize
    @port = 51_002
    @resolver_port = 52_003
    @host = '127.0.0.1'

    config = {
      'ListenAddressForCloudController' => "#{host}:#{port}",
      'ListenAddressForVIPResolver' => "#{host}:#{resolver_port}",
      'ListenAddressForMCP' => "#{host}:51003",
      'ListenAddressForPilot' => "#{host}:1234",
      'PilotClientCAPath' => fixture('fakeCA.crt'),
      'CloudControllerClientCAPath' => fixture('fakeCA.crt'),
      'ServerCertPath' => fixture('copilot-server.crt'),
      'ServerKeyPath' => fixture('copilot-server.key'),
      'VIPCIDR' => "127.128.0.0/9",
      'BBS' => { 'Disable' => true },
      'LogLevel' => 'fatal',
      'PolicyServerDisabled' => true,
      'PolicyServerAddress' => 'https://policy-server.service.cf.internal:4003',
      'PolicyServerClientCertPath' => fixture('copilot-server.crt'),
      'PolicyServerClientKeyPath' => fixture('copilot-server.key'),
      'PolicyServerCAPath' => fixture('fakeCA.crt')
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
