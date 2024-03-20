# cloudtrace-exporter

A custom exporter that collects traces from [Open Telekom Cloud](https://www.open-telekom-cloud.com/en) CloudTrace service and loads them as graph in a 
neo4j database.

[Cloud Trace Service (CTS)](https://www.open-telekom-cloud.com/en/products-services/core-services/cloud-trace) is an 
effective monitoring tool that allows users to analyze their cloud resources using traces. A tracker is automatically 
generated when the service is started and monitors access to all the respective user’s cloud resources using the generated traces. 
The monitoring logs can be saved long-term and cost-effectively in the [Object Storage Service (OBS)](https://www.open-telekom-cloud.com/en/products-services/core-services/object-storage-service). 
The CTS can also be used in conjunction with [Simple Message Notification (SMN)](https://www.open-telekom-cloud.com/en/products-services/core-services/simple-message-notification), 
allowing the user to receive a message when certain events occur.

This custom exporter is taking a different route. It's utilizing [Knative Eventing](https://knative.dev/docs/eventing/) 
to create a custom source (**cts_exporter**) that collects traces from CTS and forwards them, as [Cloud Events](https://cloudevents.io/) 
to an agnostic _sink_, defined by an environment variable called `K_SINK`, as is required by Knative Eventing specifications 
for interconnecting microservices. In addition to cts_exporter, a custom sink (**neo4j_sink**) that listens for those 
Cloud Events is provided, which loads these events in a [Neo4j](https://neo4j.com/) database as graphs. You could positively bind the cts_exporter
to any other that sink that conforms to Knative specifications. You can find an example in the repo that uses
_gcr.io/knative-releases/knative.dev/eventing/cmd/event_display_ as a target. That is a simple Knative Eventing Service that
simply logs the events in the `os.Stdout`. 

![graph.png](assets%2Fimg%2Fgraph.png)

[Neo4j](https://neo4j.com/) is a highly acclaimed graph database management system developed by Neo4j, Inc. Unlike 
traditional relational databases that store data in tables, Neo4j is designed around the concept of storing and managing 
data as nodes and relationships. This structure is particularly well-suited for handling complex and interconnected data, 
making it easier to model, store, and query relationships directly.

Graph databases like Neo4j are based on graph theory and use graph structures with nodes, edges, and properties to 
represent and store data. In this context:

- **Nodes** represent entities (such as people, products, or accounts).
- **Relationships** provide directed, named connections between nodes. These relationships can also have properties that provide more context about the connection.
- **Properties** are key-value pairs attached to nodes and relationships, allowing for the storage of additional information about those elements.

Neo4j is widely used in various applications that require efficient analysis and querying of complex networks of data. 
Examples include social networks, recommendation engines, fraud detection, network and IT operations, and more. It 
offers a powerful query language called [Cypher](https://neo4j.com/product/cypher-graph-query-language/), specifically 
designed for working with graph data, enabling users to intuitively and efficiently retrieve and manipulate data within a graph structure.

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

> **_WARNING:_**  **clouds.yaml** is already added to **.gitignore**, so there is no danger leaking its sensitive 
> contents in public!

Additionally you need to set the following environment variables for **cts_exporter**: 

- `OS_CLOUD` the cloud profile you want to choose from your **cloud.yaml** file
- `OS_DEBUG` whether you want to swap to debug mode, defaults to `false`
- `CTS_TRACKER` the CTS tracker you want to hook on, default to `system`
- `CTS_FROM` an integer value in minutes, that signifies how long in the past to look for traces and the interval between two consecutive queries, defaults to `5`
- `CTS_X_PNP` whether you want to push the collected traces to a sink, defaults to `true` 
- `K_SINK` the URL of the resolved sink
- `K_CE_OVERRIDES` a JSON object that specifies overrides to the outbound event

For **neo4j_sink** you need to set the following environment variables:

- `NEO4J_URI` the Neo4j connection uri for your instance, defaults to `neo4j://localhost:7687`
- `NEO4J_USER` the username to use for authentication
- `NEO4J_PASSWORD` the password to use for authentication

> **_NOTE:_**  At the moment the client wrapper around Neo4j driver, built in `neo4j_sink`, is supporting only Basic Auth.

## Deployment

The project is coming with a `Makefile` that takes care of everything for you, from building (using [ko](https://ko.build/);
neither a `Dockerfile` is needed nor docker registries to push the generated container images) to deployment on a 
Kubernetes cluster. Only thing you need, is to have a Kubernetes cluster in place, already employed with **Knative Serving & Eventing** artifacts.

Before installing you need to define the values of cts_exporter environment variables in `deploy/manifests/cloudtrace-exporter-configmap.yaml` e.g:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cloudtrace-exporter-config
  namespace: default
data:
  OS_CLOUD: "otc"
  OS_DEBUG: "false"
  CTS_X_PNP: "true"
  CTS_FROM: "1"
```

### Install

```shell
make install
```

### Uninstall

```shell
make uninstall
```

## Development

Development comes as well with "batteries included". You can either go ahead and start debugging straight on your local 
machine, or take advantage of the `.devcontainer` file that can be found in the repo, that sets up an isolated 
containerized environment for you with a Neo4j database included.

