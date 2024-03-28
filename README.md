# cloudtrace-exporter

A custom exporter that collects traces from [Open Telekom Cloud](https://www.open-telekom-cloud.com/en) CloudTrace service and loads them as graph in a
[Neo4j](https://neo4j.com/) database.

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
Cloud Events is provided, which loads these events in a Neo4j database as graphs. You could positively bind the cts_exporter
to **any** other sink that conforms to Knative specifications. You can find an example in the repo that uses
_gcr.io/knative-releases/knative.dev/eventing/cmd/event_display_ as a target sink. That is a demo Knative Eventing Service that
simply logs the events in the `os.Stdout`. 

![graph.png](assets%2Fimg%2Fgraph.png)

[Neo4j](https://neo4j.com/) is a highly acclaimed graph database management system developed by Neo4j, Inc. Unlike 
traditional relational databases that store data in tables, Neo4j is designed around the concept of storing and managing 
data as nodes and relationships. This structure is particularly well-suited for handling complex and interconnected data, 
making it easier to model, store, and query relationships directly.

Graph databases like Neo4j are based on graph theory and use graph structures with nodes, edges, and properties to 
represent and store data. In this context:

- **Nodes** represent entities (such as subjects, actions, resources, tenants & regions in the context of CloudTrace domain).
- **Relationships** provide directed, named connections between nodes. These relationships can also have properties that 
provide more context about the connection (such as who performed an action, on which resource this action was performed, 
in which tenant is this resource is member, in which region is this tenant located)
- **Properties** are key-value pairs attached to nodes and relationships, allowing for the storage of additional 
information about those elements (such as unique identifiers for nodes, tenant and domain identifiers, subjects name etc)

The graph generated for every CloudTrace record can be summarized by the following domain object:

![graph-mock.png](assets%2Fimg%2Fgraph-mock.png)

An **ACTION** (login, logout, start an ECS instance etc) is _PERFORMED_BY_ a **SUBJECT** (user, agent etc) and is _APPLIED_ON_ 
a **RESOURCE** (ECS instance, CCE cluster etc) resulting _WITH_STATUS_ either **NORMAL**, **WARNING** or **INCIDENT** depending on 
the outcome of this **ACTION**. The **RESOURCE** is _MEMBER_OF_ a **TENANT** which is _LOCATED_AT_ a specific **REGION**. 
The central element of this domain model is the **ACTION**.

Terms in **BOLD** signify a **Node** and those in _ITALICS_ signify a **Relationship**.

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
      user_domain_name: 'OTC0000000000xxxxxxxxxx'
      auth_url: 'https://iam.eu-de.otc.t-systems.com:443/v3'
    interface: 'public'
    identity_api_version: 3

```

> [!IMPORTANT]
> **clouds.yaml** is already added to **.gitignore**, so there is no danger leaking its sensitive contents in public!

Additionally, you need to set the following environment variables for **cts_exporter**: 

- `OS_CLOUD` the cloud profile you want to choose from your **cloud.yaml** file
- `OS_DEBUG` whether you want to swap to debug mode, defaults to `false`
- `CTS_TRACKER` the CTS tracker you want to hook on, default to `system`
- `CTS_FROM` an integer value in minutes, that signifies how long in the past to look for traces and the interval between two consecutive queries, defaults to `5`
- `CTS_X_PNP` whether you want to push the collected traces to a sink, defaults to `true` 

> [!IMPORTANT]
> There are two additional environment variables, that need to be addressed separately, and those are: 
>
> - `K_SINK` the URL of the resolved sink
> - `K_CE_OVERRIDES` a JSON object that specifies overrides to the outbound event
> 
> If you choose to deploy **cts_exporter** as a plain Kubernetes `Deployment`, for test reasons,  
> using `deploy/manifests/cloudtrace-exporter-deployment.yaml` you need to explicitly set the value of `K_SINK` yourself.
> This will not unfold the whole functionality, because the resource will be deployed outside of the realm of responsibility 
> of Knative reconcilers. As mentioned again, this is **exclusively** for quick test purposes.
> 
> If you deploy **cts_exporter** as a `ContainerSource` or `SinkBinding`, Knative will take care of the rest and inject in 
> your container an environment variable named `K_SINK` by itself.

For **neo4j_sink** you need to set the following environment variables:

- `NEO4J_URI` the Neo4j connection uri for your instance, defaults to `neo4j://localhost:7687`
- `NEO4J_USER` the username to use for authentication
- `NEO4J_PASSWORD` the password to use for authentication

> [!NOTE]
> At the moment, the client wrapper around Neo4j driver, built in `neo4j_sink`, is supporting only Basic Auth.

## Deployment

The project is coming with a `Makefile` that takes care of everything for you, from building (using [ko](https://ko.build/);
neither a `Dockerfile` is needed nor docker registries to push the generated container images) to deployment on a 
Kubernetes cluster. Only thing you need, is to have a Kubernetes cluster in place, already employed with 
**Knative Serving & Eventing** artifacts.

Before installing you need to define the values of **cts_exporter** environment variables in `deploy/manifests/cloudtrace-exporter-configmap.yaml` e.g:

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

As mentioned earlier, you are given two options as how to deploy **cts_exporter** as a Knative workload; either as a 
`ContainerSource`:

```shell
make install-containersource
```

or as a `SinkBinding`:

```shell
make install-sinkbinding
```

### Uninstall

```shell
make uninstall
```

## Development

Development comes as well with "batteries included". You can either go ahead and start debugging straight on your local
machine, or take advantage of the `.devcontainer.json` file that can be found in the repo, that instructs any IDE that 
supports Dev Containers, to set up an isolated containerized environment for you with a Neo4j database included.

### Local

Working on your local machine, requires the following:

- Assign values to the environment variables for both binaries, as mentioned earlier in this document
- Provide a Neo4j database instance. You can choose among a simple container, a Kubernetes workload or even the new [Neo4j Desktop](https://neo4j.com/docs/desktop-manual/current/)

### Dev Container

Dev Container will create a container with all the necessary prerequisites to get you started developing immediately. An
Ubuntu Jammy container will be spawned with the following features pre-installed:

- Git
- Docker in Docker
- Kubectl, Helm, Helmfile, K9s, KinD, Dive
- [Bridge to Kubernetes](https://learn.microsoft.com/en-us/visualstudio/bridge/overview-bridge-to-kubernetes) Visual Studio Code Extension
- Latest version of Golang

A `postCreateCommand` (**.devcontainer/setup.sh**) will provision:

- A containerized **Kubernetes cluster** with 1 control and 3 worker nodes **and** a private registry, using KinD (cluster manifest is in **.devcontainer/cluster.yaml**) 
- A standalone **Neo4j cluster** (you can change that and get a HA cluster by increasing the value of `minimumClusterSize` in **.devcontainer/overrides.yaml**)
- the necessary resources for the **Knative Serving & Eventing infrastructure**

Only thing left to you is, as long as you are working with Visual Studio Code,  
to forward the 3 ports (`7473`, `7474` and `7687`) exposed from the **n4j-cluster-lb-neo4j** Service, so your Neo4j 
database is accessible from your Dev Container environment. 

> [!NOTE]
> You can just port-forward the Kubernetes Service ports straight from K9s, in an integrated Visual Studio Code terminal,
> and Visual Studio Code will pick up automatically those ports and forward them to your local machine.

![devcontainer.png](assets%2Fimg%2Fdevcontainer.png)
