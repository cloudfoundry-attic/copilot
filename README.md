# Copilot

To help Pilot work with Cloud Foundry

To get started:

```sh
git clone https://github.com/cloudfoundry/copilot.git
cd copilot
go get github.com/onsi/ginkgo/ginkgo
go get github.com/golang/dep/cmd/dep
dep ensure
```

To run the tests:

```sh
ginkgo -r -p
```

To compile the server:

```sh
go build code.cloudfoundry.org/copilot/cmd/copilot-server
```

## CLIs

The server uses gRPC to communicate, so a cli is required for a developer to communicate with the server.
There are two clis, one for communicating with endpoints used by cloud controller, and another one for endpoints used by istio.

To compile the clis:

```sh
go build code.cloudfoundry.org/copilot/cmd/copilot-clients/cloud-controller
go build code.cloudfoundry.org/copilot/cmd/copilot-clients/istio
```


