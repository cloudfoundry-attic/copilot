### How to generate services and messages from the `.proto` file

Starting from the repo root:
```
cd copilot/api/protos
protoc --ruby_out=../../sdk/ruby/lib/copilot/protos
  \ --grpc_out=../../sdk/ruby/lib/copilot/protos
  \ --plugin="$(which grpc_tools_ruby_protoc_plugin"
  \ ./cloud_controller_future.proto
```

### How to build and install the `cf-copilot` ruby gem

Starting from the repo root:
```
cd sdk/ruby
gem build ./cf-copilot.gemspec && gem install cf-copilot
```
