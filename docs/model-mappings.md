# Ideas for mapping Istio and Cloud Foundry domain models
This doc is meant for two audiences:
- Istio contributors
- Cloud Foundry contributors

Istio and CF use some of the same words (Service, Route) to mean very different things.
So we will try to prefix every jargon word with "Istio" or "Cloud Foundry" (CF) in order to disambiguate.

This doc is meant to illustrate **one possible mapping** from CF to Istio.  There are other possible mappings.
A full discussion of the tradeoffs for all possible mappings is beyond the scope of this document.

## Cloud Foundry Platform Adapter

Istio Pilot has a extension model called the [platform adapter](https://istio.io/docs/concepts/traffic-management/pilot.html).

Platforms like CF or Kubernetes have a specialized adapter to translate data about platform-specific resources, like CF App Instances,
into Istio-specific resources, like Istio Service Instances.

Our Cloud Foundry Platform Adapter ([code here](https://github.com/istio/istio/tree/6ab44cdfd3401a1ae2cd3b4dd9f42f823e6d02d7/pilot/pkg/serviceregistry/cloudfoundry))
works with [CF Copilot](https://github.com/cloudfoundry/copilot) to
create an [Istio Service](https://godoc.org/istio.io/istio/pilot/pkg/model#Service) for each
[Cloud Foundry Route (CAPI v2)](https://apidocs.cloudfoundry.org/280/routes/creating_a_route.html).

There are two kinds of CF Routes: *external* and *internal*. (Internal routes are a brand-new thing, [here are some rough docs](https://github.com/cloudfoundry/cf-app-sd-release#example-usage)).

Here's an Istio Service representation of a **CF External Route**:

```go
// Abstract representation of a CF external route as an Istio Service.
// Istio config (like RouteRules) may reference this service by Hostname
someExternalRoute := Service{
  Hostname: "myapp.example.com",  // external host name

  Ports: []*Port{
    &Port{  // only matters when accessed without a gateway
      Name: "default"
      Port: 8080,
      Protocol: "http",
    },
  },
}
```

The Istio Service representation of a **CF Internal Route** might look like:
```go
// Abstract representation of a CF internal route as an Istio Service.
// Istio config (like RouteRules) may reference this service by Hostname
someInternalRoute := Service{
  Hostname: "myapp.apps.internal",  // internal host name

  Address: "172.16.30.3",  // virtual IP, assigned by CF, used for intra-mesh TCP/UDP routing

  Ports: []*Port{
    &Port{  // must match the port on the backing application
      Name: "default"
      Port: 8080,
      Protocol: "http",
    },
  },
}
```



## Gateway
Istio has a notion of a Gateway, which defines ingress configuration for the mesh.

For Cloud Foundry ingress routing, we will configure a Gateway that exposes port 80 and/or 443.
Each exposed port (a `server` in Gateway-speak) is configured with a list of `hosts`.  In Cloud Foundry,
each external [CF Domain](https://docs.cloudfoundry.org/devguide/deploy-apps/routes-domains.html) will appear as a host in that list.

In particular, the internal domain (`*.apps.internal`) will not be in the `hosts` list.


```yaml
apiVersion: config.istio.io/v1alpha2
kind: Gateway
metadata:
  name: my-gateway
spec:
  servers:
  - port:
      number: 80
      name: http
    hosts:
    - "*.example.com"
    - "*.example.net"
    - "example.com"
    - "*.other-shared-domain.com"
    - "*.some-private-domain.org"
    ...
  - port:
      number: 443
      name: https
    hosts:
    - "*.example.com"
    - "*.example.net"
    - "example.com"
    - "*.other-shared-domain.com"
    - "*.some-private-domain.org"
    ...
    tls:
      mode: simple
      serverCertificate: /var/vcap/jobs/ingress-envoy/certs/cert.pem
      privateKey: /vcap/jobs/ingress-envoy/certs/key.pem
```


## References
- Cloud Controller
  - [v3 Processes](http://v3-apidocs.cloudfoundry.org/version/3.38.0/index.html#processes)
  - [v2 Routes](https://apidocs.cloudfoundry.org/280/#routes)
- Istio
  - [Service Godoc](https://godoc.org/istio.io/istio/pilot/pkg/model#Service) and [Service code](https://github.com/istio/istio/blob/448436a5acd72b77206eb0d61be084b572710022/pilot/pkg/model/service.go#L34-L65)
  - [Service Instance](https://github.com/istio/istio/blob/448436a5acd72b77206eb0d61be084b572710022/pilot/pkg/model/service.go#L197-L221)
  - [Route Rule (v1alpha2)](https://godoc.org/istio.io/api/routing/v1alpha2#RouteRule)
