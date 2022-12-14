

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.24.2
GITCOMMIT ?= $(shell git rev-parse --short HEAD)
# Image URL to use all building/pushing image targets
# IMG ?= www.cliufreever.com/library/resourcelimiter-controller:v0.0.1-$(GITCOMMIT)
IMG ?= www.cliufreever.com/library/resourcelimiter-controller:v0.0.2-${GITCOMMIT}
CHECKERIMG ?= www.cliufreever.com/library/resourcelimiter-checker:v0.0.2-${GITCOMMIT}
CONVERTERIMG ?= www.cliufreever.com/library/resourcelimiter-converter:v0.0.2-${GITCOMMIT}
HELM_REGISTRY ?= cliufreever
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

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=resourceadminrole crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run all tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)"  ACK_GINKGO_DEPRECATIONS=1.16.5 go test -v ./... -coverprofile cover.out

.PHONY: e2e-test
e2e-test:   controller-e2e-test ## Run all e2e tests.

.PHONY: unit-test
unit-test: webhook-unit-test conversion-unit-test ## Run all unit tests

.PHONY: controller-e2e-test
controller-e2e-test:  manifests generate fmt vet envtest ## Run controller e2e tests.
	cd ./controllers && \
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)"  ACK_GINKGO_DEPRECATIONS=1.16.5 go test -v ./... -coverprofile cover.out

.PHONY: webhook-unit-test
webhook-unit-test: envtest ## Run mutate && validate tests
	cd ./pkg/cmd/ && \
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)"  ACK_GINKGO_DEPRECATIONS=1.16.5 go test -v ./... -coverprofile cover.out

.PHONY: conversion-unit-test
conversion-unit-test: envtest ## Run mutate && validate tests
	cd ./pkg/conversion/ && \
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)"  ACK_GINKGO_DEPRECATIONS=1.16.5 go test -v ./... -coverprofile cover.out



##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${IMG} .


.PHONY: docker-checker-build
docker-checker-build:  ## Build docker image with the checker.
	docker build -t ${CHECKERIMG} -f Dockerfile.checker .

.PHONY: docker-converter-build
docker-converter-build:  ## Build docker image with the checker.
	docker build -t ${CONVERTERIMG} -f Dockerfile.converter .

.PHONY: docker-all-build
docker-all-build: docker-build docker-checker-build docker-converter-build ## build all components

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: docker-checker-push
docker-checker-push: ## Push docker image with the manager.
	docker push ${CHECKERIMG}

.PHONY: docker-converter-push
docker-converter-push: ## Push docker image with the manager.
	docker push ${CONVERTERIMG}

.PHONY: docker-all-push
docker-all-push: docker-push docker-checker-push docker-converter-push ## push all components

.PHONY: helm-all-push
helm-all-push: helm-crd-push helm-conversion-push helm-controller-push ## helm push all charts

.PHONY: helm-crd-push
helm-crd-push: ## helm push crd helm
	cd charts/resourcelimiter-crd && helm cm-push . ${HELM_REGISTRY}

.PHONY: helm-conversion-push
helm-conversion-push: ## push conversion webhook chart
	cd charts/resourcelimiter-conversion && helm cm-push . ${HELM_REGISTRY}

.PHONY: helm-controller-push
helm-controller-push: ## push controller chart
	cd charts/resourcelimiter && helm dependency update && helm cm-push . ${HELM_REGISTRY}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.9.2

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	test -s $(LOCALBIN)/kustomize || { curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
