SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export OS   := $(shell if [ "$(shell uname)" = "Darwin" ]; then echo "darwin"; else echo "linux"; fi)
export ARCH := $(shell if [ "$(shell uname -m)" = "x86_64" ]; then echo "amd64"; else echo "arm64"; fi)

# Extracts the version number for a given dependency found in go.mod.
# Makes the test setup be in sync with what the operator itself uses.
extract-version = $(shell cat go.mod | grep $(1) | awk '{$$1=$$1};1' | cut -d' ' -f2 | sed 's/^v//')

CONTAINER_TOOL             ?= docker
OPERATOR_SDK_VERSION       ?= v1.38.0
IMG                        ?= ztoperator:latest
KIND_CLUSTER_NAME          ?= ztoperator
KUBECONTEXT                ?= kind-$(KIND_CLUSTER_NAME)
KUBERNETES_VERSION          = 1.31.4
ENVTEST_K8S_VERSION         = $(KUBERNETES_VERSION)
KIND_IMAGE                 ?= kindest/node:v$(KUBERNETES_VERSION)
CHAINSAW_VERSION           := 0.2.12
CONTROLLER_GEN_VERSION     := $(call extract-version,sigs.k8s.io/controller-tools)
ISTIO_VERSION              := $(call extract-version,istio.io/client-go)

## Location to install dependencies to
VENV := venv
PYTHON := $(VENV)/bin/python
PIP := $(VENV)/bin/pip
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL        ?= kubectl
KUSTOMIZE      ?= $(LOCALBIN)/kustomize
CHAINSAW       ?= $(LOCALBIN)/chainsaw
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
GOLANGCI_LINT   = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.4.2
CONTROLLER_TOOLS_VERSION ?= v0.15.0
GOLANGCI_LINT_VERSION ?= v1.59.1


.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=ztoperator crd paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/ztoperator cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name ztoperator-builder
	$(CONTAINER_TOOL) buildx use ztoperator-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm ztoperator-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) --context $(KUBECONTEXT) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) --context $(KUBECONTEXT) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: chainsaw
chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(LOCALBIN)
	$(call go-install-tool,$(CHAINSAW),github.com/kyverno/chainsaw,v$(CHAINSAW_VERSION))

.PHONY: helm
helm:
	# Check if istio helm repo is installed and add if not
	@helm repo list | grep istio || (echo "Adding istio helm repo..." && helm repo add istio https://istio-release.storage.googleapis.com/charts && helm repo update)

.PHONY: virtualenv
virtualenv:
	@which python3 >/dev/null || (echo "Python3 not installed, please install it to proceed"; exit 1)
	# Set up virtualenv, activate it and install required packages
	@python3 -m venv $(VENV)
	@$(PIP) install -r scripts/requirements.txt

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef


### CUSTOM TARGETS ###
.PHONY: run-local
run-local: build install
	./bin/ztoperator

.PHONY: setup-local
setup-local: kind-cluster install-istio-gateways install
	@echo "Cluster $(KUBECONTEXT) is setup"

#### KIND ####
.PHONY: check-kind
check-kind:
	@which kind >/dev/null || (echo "kind not installed, please install it to proceed"; exit 1)

.PHONY: kind-cluster
kind-cluster: check-kind
	@echo Create kind cluster... >&2
	@kind create cluster --image $(KIND_IMAGE) --name ${KIND_CLUSTER_NAME}

.PHONY: install-skiperator
install-skiperator:
	@kubectl create namespace skiperator-system --context $(KUBECONTEXT) || true
	@KUBECONTEXT=$(KUBECONTEXT) ./scripts/install-skiperator.sh

.PHONY: install-mock-oauth2
install-mock-oauth2:
	@KUBECONTEXT=$(KUBECONTEXT) ./scripts/install-mock-oauth2.sh --config ./scripts/mock-oauth2-server-config.json

.PHONY: setup-local-test
setup-local-test: install-skiperator install-mock-oauth2 expose-ingress virtualenv

#### ZTOPERATOR DEPENDENCIES ####

.PHONY: install-istio
install-istio:
	@echo "Downloading Istio..."
	@curl -L https://istio.io/downloadIstio | ISTIO_VERSION=$(ISTIO_VERSION) TARGET_ARCH=$(ARCH) sh -
	@echo "Installing Istio on Kubernetes cluster..."
	@./istio-$(ISTIO_VERSION)/bin/istioctl install -y --context $(KUBECONTEXT) --set meshConfig.accessLogFile=/dev/stdout --set profile=minimal
	@echo "Istio installation complete."

.PHONY: install-istio-gateways
install-istio-gateways: helm install-istio
	@echo "Creating istio-gateways namespace..."
	@kubectl create namespace istio-gateways --context $(KUBECONTEXT) || true
	@echo "Installing istio-gateways"
	@helm --kube-context $(KUBECONTEXT) install istio-ingressgateway istio/gateway -n istio-gateways --set labels.app=istio-ingress-external --set labels.istio=ingressgateway
	@echo "Istio gateways installed."

.PHONY: install-sample
install-sample:
	@kubectl apply -f samples/ --recursive --context $(KUBECONTEXT)

#### TESTS ####
.PHONY: expose-ingress
expose-ingress:
	@lsof -ni :8443 | grep LISTEN && (echo "Port 8443 is already in use. Trying to kill kubectl" && killall kubectl) || true
	@echo "Exposing istio ingress gateway on localhost 8443"
	@KUBECONTEXT=$(KUBECONTEXT) kubectl port-forward --context $(KUBECONTEXT) -n istio-gateways svc/istio-ingressgateway 8443:443 2>&1 & \

.PHONY: test-single
test-single: chainsaw install
	@./bin/chainsaw test --kube-context $(KUBECONTEXT) --config test/chainsaw/config.yaml --test-dir $(dir) && \
    echo "Test succeeded" || (echo "Test failed" && exit 1)

.PHONY: test
test:
	@echo "Checking if ztoperator is running..."
	@lsof -i :8081 | grep ___Ztoper  > /dev/null || (echo "ztoperator is not running. Please start it first." && exit 1)
	@echo "ztoperator is running. Proceeding with tests..."
	@bash -ec ' \
		for dir in test/chainsaw/authpolicy/*/ ; do \
			echo "Running test in $$dir"; \
			if ! $(MAKE) test-single dir=$$dir; then \
				echo "Test in $$dir failed."; \
				exit 1; \
			fi; \
		done; \
		echo "running unit tests..."; \
		$(MAKE) run-unit-tests; \
	' || (echo "Test(s) failed." && exit 1)

.PHONY: run-unit-tests
run-unit-tests:
	@if [ -z "$$(find . -name '*_test.go')" ]; then \
		echo "No unit tests found. Skipping."; \
	else \
		echo "Running unit tests..."; \
		go test ./... -v || (echo "Unit tests failed" && exit 1); \
	fi

.PHONY: run-test
export IMAGE_PULL_0_REGISTRY := ghcr.io
export IMAGE_PULL_1_REGISTRY := https://index.docker.io/v1/
export IMAGE_PULL_0_TOKEN :=
export IMAGE_PULL_1_TOKEN :=
run-test: build
	@echo "Starting ztoperator in background..."
	@LOG_FILE=$$(mktemp -t ztoperator-test.XXXXXXX); \
	./bin/ztoperator > "$$LOG_FILE" 2>&1 & \
	PID=$$!; \
	echo "ztoperator PID: $$PID"; \
	echo "Log redirected to file: $$LOG_FILE"; \
	( \
		for dir in test/chainsaw/authpolicy/*/ ; do \
			echo "Running test in $$dir"; \
			if ! $(MAKE) test-single dir=$$dir; then \
				echo "Test in $$dir failed. Stopping ztoperator (PID $$PID)"; \
				kill $$PID; \
				exit 1; \
			fi; \
		done; \
		echo "Stopping ztoperator (PID $$PID)..."; \
		kill $$PID; \
		echo "running unit tests..."; \
		$(MAKE) run-unit-tests; \
	) || (echo "Test or ztoperator failed." && exit 1)
