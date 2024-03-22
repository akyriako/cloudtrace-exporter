#!/bin/bash

# Install the required tools and dependencies
sudo apt-get update

# Provision a KinD cluster
kind create cluster --name neo4j --config .devcontainer/cluster.yaml

# Provision a Neo4j cluster on Kubernetes
helm repo add neo4j https://helm.neo4j.com/neo4j
helm repo update

helm upgrade --install n4j-cluster neo4j/neo4j -f .devcontainer/overrides.yaml -n n4j --create-namespace
