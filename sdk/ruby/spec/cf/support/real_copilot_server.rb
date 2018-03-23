class RealCopilotServer
  attr_reader :port, :host

  def initialize
    @port = 51002
    @host = "127.0.0.1"

    @copilotServer = fork do
      exec "cd spec/cf/fixtures && exec copilot-server -config copilot-config.json"
    end

    Process.detach(@copilotServer)
  end

  def stop
    Process.kill("TERM", @copilotServer)
  end
end
