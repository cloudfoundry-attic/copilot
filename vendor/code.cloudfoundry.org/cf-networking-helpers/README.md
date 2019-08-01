## cf-networking-helpers


### Running tests

To run tests use `./scripts/docker-test`.

This will use `dep` to get the dependencies necessary using the `Gopkg.toml` and `Gopkg.lock`.
Since this is a library those two files are only used to get dependencies for testing. DO NOT
vendor packages here.
