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

## Adding a Route

We are using a generic grpc client to interact with cloud controller grpc service (installation instructions below)

### Setup Generic GRPC Client
- bosh ssh to the istio vm and `sudo su`
- install latest nodejs and npm:

```sh
curl -sL https://deb.nodesource.com/setup_6.x | sudo -E bash -
apt install nodejs
npm install -g grpcc
```

### Grab Latest Proto

```sh
cd /var/vcap/jobs/pilot-discovery/config/certs
curl -s -L -o cloud_controller.proto https://raw.githubusercontent.com/cloudfoundry/copilot/master/api/protos/cloud_controller.proto
```

### Push an App

```sh
cf push ...
```

### Find Diego Process GUID
The process guid is under the `service-key` as the prefix *before* `.cfapps.internal`.

```sh
curl localhost:8080/v1/registration
```

### Add a Route
Do not try to use absolute paths for the certs / key - they do not work

```sh
grpcc --root_cert ./ca.crt \
--private_key ./client.key \
--cert_chain ./client.crt \
--proto ./cloud_controller.proto \
--address 127.0.0.1:9000 --eval 'client.addRoute({hostname: "some.hostname.you.choose", processGuid: "the-process-guid"}, pr)'
```

### View pilot API results
```sh
curl localhost:8080/v1/routes/http_proxy/x/router~x~x~x
```

and

```sh
curl localhost:8080/v1/clusters/x/router~x~x~x
```

and

```sh
curl localhost:8080/v1/registration
```

or

```sh
curl localhost:8080/v1/registration/some.hostname.you.choose
```

you can scale your app up

```sh
cf scale -i 3 your-app
```

and then re-run the above `curl` commands.

## Debugging

To open an ssh against a copilot running in a cloud foundry:

- `ssh -f -L 9000:$COPILOT_IP:9000 jumpbox@$(bbl jumpbox-address) -i $JUMPBOX_PRIVATE_KEY sleep 600` this will open a tunnel for 10 minutes
- make sure that `copilot.listen_address` is `0.0.0.0:9000` and not `127.0.0.1:9000`
- open a hole in the jumpbox firewall rule (envname-jumpbox-to-all) to allow traffic on port 9000

Now you are ready to start your own pilot:

- `bosh scp -r istio:/var/vcap/jobs/pilot-discovery/config /tmp/config`
- check that the `/tmp/config/cf_config.yml` so the IP address matches your tunnel and the cert file paths point to /tmp/config
- install dlv on your machine `go get -u github.com/derekparker/delve/cmd/dlv`
- from istio: `dlv debug ./pilot/cmd/pilot-discovery/main.go -- discovery --configDir=/dev/null --registries=CloudFoundry --cfConfig=/users/pivotal/downloads/config/cf_config.yml --meshConfig=/dev/null`

## CLIs

The server uses gRPC to communicate, so a cli is required for a developer to communicate with the server.
There are two clis, one for communicating with endpoints used by cloud controller, and another one for endpoints used by istio.

To compile the clis:

```sh
go build code.cloudfoundry.org/copilot/cmd/copilot-clients/cloud-controller
go build code.cloudfoundry.org/copilot/cmd/copilot-clients/istio
```

