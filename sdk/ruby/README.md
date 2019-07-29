# Ruby SDK

## Notes

- All commands below from the root of this repo
- There is no need to run `generate-protos` directly (use the script / docker image)

### How to generate services and messages from the `.proto` file
```
pushd sdk/ruby
  bin/dockerized-rebuild-proto
popd
```

### How to build and install the `cf-copilot` ruby gem
```
pushd sdk/ruby
  bin/build-and-install-copilot-gem
popd
```

### How to run tests
```
pushd sdk/ruby
  bin/run-tests-in-docker
popd
```
