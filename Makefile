
# Image URL to use all building/pushing image targets
IMG_TAG ?= $(shell git rev-parse --short HEAD)
IMG_NAME ?= cloudtrace-exporter
DOCKER_HUB_NAME ?= $(shell docker info | sed '/Username:/!d;s/.* //')
IMG ?= $(DOCKER_HUB_NAME)/$(IMG_NAME):$(IMG_TAG)
KO_DOCKER_REPO ?= $(DOCKER_HUB_NAME)/$(IMG_NAME)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary.
	go build -o cmd/ctsexp.go

.PHONY: run
run: fmt vet ## Run a controller from your host.
	go run .cmd/ctsexp.go


ifndef ignore-not-found
  ignore-not-found = false
endif
#
#.PHONY: deploy
#deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
#	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
#	$(KUSTOMIZE) build config/default | kubectl apply -f -
#
#.PHONY: undeploy
#undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
#	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

KO ?= $(LOCALBIN)/ko

.PHONY: ko
ko: $(KO) ## Download ko.build locally if necessary.
$(KO): $(LOCALBIN)
	test -s $(LOCALBIN)/ko || GOBIN=$(LOCALBIN) go install github.com/google/ko@latest

ko-build: ko ## Use ko build to build locally.
	$(KO) build --local --bare github.com/akyriako/cloudtrace-exporter/cmd

ko-push: ko $(KO_DOCKER_REPO) ## Use ko build to build and push to remote hub.
	echo $(KO_DOCKER_REPO)

ko-deploy: ko ## Build image locally and deploy Deployment to Kubernetes.
	$(KO) apply --local --bare -f deploy/manifests/cloudtrace-exporter-deployment.yaml

##@ Deployment

CLOUDS_YAML := $(shell base64 -i clouds.yaml)
define encode_clouds
	export CLOUDS_YAML=$(CLOUDS_YAML)
	envsubst < deploy/manifests/cloudtrace-exporter-clouds-secret.yaml | kubectl apply -f -
endef

.PHONY: install
secret: ## Build Secret from clouds.yaml and deploy to Kubernetes.
	$(encode_clouds)

event-display: ## Deploy event-display Sink to Kubernetes.
	kubectl apply -f deploy/manifests/event-display-sink.yaml

manifests: ## Deploy rest of manifests to Kubernetes.
	kubectl apply -f deploy/manifests/cloudtrace-exporter-clouds-secret.yaml || kubectl apply -f deploy/manifests/cloudtrace-exporter-configmap.yaml

deploy-sb: event-display manifests ko-deploy ## Deploy Source using SinkBinding.
	kubectl apply -f deploy/manifests/cloudtrace-exporter-sinkbinding.yaml

deploy-cs: event-display manifests ko-deploy ## Deploy Source using ContainerSource.
	kubectl apply -f deploy/manifests/cloudtrace-exporter-containersource.yaml