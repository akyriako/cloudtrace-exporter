#!/bin/bash

# Install the required tools and dependencies
# sudo apt-get update

# Provision a KinD cluster
if kind get clusters | grep -q "neo4j"; then
    echo "A Kubernetes cluster with the name 'neo4j' already exists."
else
    kind create cluster --name=neo4j --config .devcontainer/cluster.yaml
fi

mkdir ~/.kube && kind --name neo4j export kubeconfig >> ~/.kube/config
chmod 400 ~/.kube/config
kubectl config use-context kind-neo4j

# Provision a Neo4j cluster on Kubernetes
if helm list -A | grep -q "n4j-cluster"; then
    echo "A chart with the name 'n4j-cluster' already exists."
else
    helm repo add neo4j https://helm.neo4j.com/neo4j
    helm repo update

    helm upgrade --install n4j-cluster neo4j/neo4j -f .devcontainer/overrides.yaml -n n4j --create-namespace
fi

