# Istio Stackdriver E2E

This test uses the E2E framework to:

1. Stand up an Istio cluster.
1. Configure a Stackdriver Mixer handler and Metric, Log, and Trace
   instances and rules.
1. Run traffic through the cluster.
1. Verify via Stackdriver APIs that metrics, logs, and traces were
   written in the unique test namespace.

## GCP Project

Normally E2E tests only use `kubectl` to interact with the cluster,
however this test needs to know the GCP project that the cluster is in
to read from the Stackdriver APIs. This is provided by setting the
`GCP_PROJ` envorinment variable. If `GCP_PROJ` env variable is not
set, the test code will try to use `gcloud` to find out the project
id.

On the cluster side, the Stackdriver adapter determines the project
via the GCE metadata server.

## Auth

There are two sides of the auth for the test. Cluster side which
writes using normal Istio behavior, and test side which reads from the
Stackdriver APIs.

### Cluster side

No special auth is setup in the test. Therefore the test must:

* Be run on GKE
* The compute service account must have roles assigned to allow
  writing metrics, logs, and traces.
* The cluster must be created with scopes that allow the above as
  well.

At this time, these are in the default permissions and scopes for GKE
clusters.

### Test side

The test uses Application Default Credentials to read from the
Stackdriver APIs. This usually means you either have `gcloud`
installed and have run `gcloud auth application-default login`, or you
have a Service Account json file, and have set the
`GOOGLE_APPLICATION_CREDENTIALS` to the path of that json file. You can
also set `sa_cred` param to the Service Account json file path, and
the test code will take care of setting environment variable. See
the [Authentication
Overview](https://cloud.google.com/docs/authentication/) for more
info.

## Run

1. Meet the above requirements of setting the GCP project and
   authentication.
1. Use the [general GKE setup](../../UsingGKE.md).
1. Run with:

    ```
    make e2e_stackdriver
    ```
