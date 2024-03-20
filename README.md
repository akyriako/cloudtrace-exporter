# cloudtrace-exporter

A custom exporter that collects traces from [Open Telekom Cloud](https://www.open-telekom-cloud.com/en) CloudTrace service and loads them as graph in a 
neo4j database.

Cloud Trace Service (CTS) is an effective monitoring tool that allows users to analyze their cloud resources using 
traces. A tracker is automatically generated when the service is started and monitors access to all the respective 
user’s cloud resources using the generated traces. The monitoring logs can be saved long-term and cost-effectively in 
the [Object Storage Service (OBS)](https://www.open-telekom-cloud.com/en/products-services/core-services/object-storage-service). 
The CTS can also be used in conjunction with Simple Message Notification (SMN), allowing the user to receive a message when certain events occur.

This custom exporter is taking a different route. It's utilizing [Knative Eventing](https://knative.dev/docs/eventing/) 
to create a custom source (**cts_exporter**) that collects traces from CTS and forwards them, as [Cloud Events](https://cloudevents.io/) 
to an agnostic _sink_, defined by an environment variable called `K_SINK`, as is required by Knative Eventing specifications 
for interconnecting microservices. In addition to cts_exporter, a custom sink (**neo4j_sink**) that listens for those 
Cloud Events is provided, which loads these events in a neo4j database as graphs. You could positively bind the cts_exporter
to any other that sink that conforms to Knative specifications. You can find an example in the repo that uses
gcr.io/knative-releases/knative.dev/eventing/cmd/event_display as a target. That is a simple Knative Eventing Service that
simply logs the events in the `os.Stdout`. 

## Usage

Use the `clouds.tpl` as a template, and fill in a `clouds.yaml` that contains all the relevant auth information for your connecting
to your Open Telekom Cloud Tenant. **cts_exporter** requires the presence of this file.

```yaml

clouds:
  otc:
    profile: otc
    auth:
      username: '<USER_NAME>'
      password: '<PASSWORD>'
      ak: '<ACCESS_KEY>'
      sk: '<SECRET_KEY>'
      project_name: 'eu-de_<PROJECT_NAME>
      user_domain_name: 'OTC00000000001000000xxx'
      auth_url: 'https://iam.eu-de.otc.t-systems.com:443/v3'
    interface: 'public'
    identity_api_version: 3

```

Additionally you need to set the following environment variables for **cts_exporter**: 

- `OS_CLOUD` the cloud profile you want to choose from your **cloud.yaml** file
- `OS_DEBUG` whether you want to swap to debug mode, defaults to `false`
- `CTS_TRACKER` the CTS tracker you want to hook on, default to `system`
- `CTS_FROM` an integer value in minutes, that signifies how long in the past to look for traces and the interval between two consecutive queries, defaults to `5`
- `CTS_X_PNP` whether you want to push the collected traces to a sink, defaults to `true` 
- `K_SINK` the URL of the resolved sink
- `K_CE_OVERRIDES` a JSON object that specifies overrides to the outbound event

For **neo4j_sink** you need to set the following environment variables:

- `NEO4J_URI` the neo4j connection uri for your instance, defaults to `neo4j://localhost:7687`
- `NEO4J_USER` the username to use for authentication
- `NEO4J_PASSWORD` the password to use for authentication

> **_NOTE:_**  At the moment the client wrapper around neo4j driver, built in `neo4j_sink`, is supporting only Basic Auth.

## Deployment

## Development

