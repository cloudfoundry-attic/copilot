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

## Using the Server

We are using a generic grpc client to interact with cloud controller grpc service (installation instructions below)

### Setup GRPC Client

If you are developing locally, you can install `grpcurl`
```sh
go get -u github.com/fullstorydev/grpcurl
```

If you are using a cloudfoundry
- bosh ssh to the istio vm and `sudo su`
- grpcurl is at `/var/vcap/packages/grpcurl/bin/grpcurl`
- the certs you need are in `/var/vcap/jobs/pilot-discovery/config/certs/`


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

(running from `/var/vcap/jobs/pilot-discovery/config/certs`)
```sh
grpcurl -cacert ./ca.crt \
  -key ./client.key \
  -cert ./client.crt \
  -d '{"route": {"host": "example.com", "guid": "route-guid-a"}}'
  127.0.0.1:9000 \
  api.CloudControllerCopilot/UpsertRoute
```

### Map a Route

(running from `/var/vcap/jobs/pilot-discovery/config/certs`)
```sh
/var/vcap/packages/grpcurl/bin/grpcurl -cacert ./ca.crt \
  -key ./client.key \
  -cert ./client.crt \
  -d '{"route_mapping": {"route_guid": "route-guid-a", "capi_process": {"diego_process_guid": "diego_guid_1", "guid": "capi_guid_1"}}}'
  127.0.0.1:9000 \
  api.CloudControllerCopilot/MapRoute
```

### List Routes

(running from `/var/vcap/jobs/pilot-discovery/config/certs`)
```sh
/var/vcap/packages/grpcurl/bin/grpcurl -cacert ./ca.crt \
  -key ./client.key \
  -cert ./client.crt \
  127.0.0.1:9000 \
  api.IstioCopilot/Routes
```

### Unmap a Route

(running from `/var/vcap/jobs/pilot-discovery/config/certs`)
```sh
/var/vcap/packages/grpcurl/bin/grpcurl -cacert ./ca.crt \
  -key ./client.key \
  -cert ./client.crt \
  -d '{"capi_process_guid": "capi_guid_1", "route_guid": "route-guid-a"}'
  127.0.0.1:9000 \
  api.CloudControllerCopilot/UnmapRoute
```

### Delete a Route

(running from `/var/vcap/jobs/pilot-discovery/config/certs`)
```sh
/var/vcap/packages/grpcurl/bin/grpcurl -cacert ./ca.crt \
  -key ./client.key \
  -cert ./client.crt \
  -d '{"guid": "route-guid-a"}'
  127.0.0.1:9000 \
  api.CloudControllerCopilot/DeleteRoute
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

