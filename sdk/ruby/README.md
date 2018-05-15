# Ruby SDK

## Notes

- All commands below from the root of this repo
- There is no need to run `generate-protos` directly (use the script / docker image)

### How to generate services and messages from the `.proto` file
```
export COPILOT_ROOT=$(pwd)
pushd sdk/ruby
  bin/dockerized-rebuild-proto
popd
```

### Run all of the associated ruby specs
```
pushd sdk/ruby
  bin/test
popd
```

### How to build and install the `cf-copilot` ruby gem
```
export COPILOT_ROOT=$(pwd)
pushd sdk/ruby
  bin/build-and-install-copilot-gem
popd
```
