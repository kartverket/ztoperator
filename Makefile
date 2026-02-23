# Image URL to use all building/pushing image targets
IMG ?= ztoperator:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

##@ Variables

# Extracts the version number for a given dependency found in go.mod.
# Makes the test setup be in sync with what the operator itself uses.
extract-version = $(shell cat go.mod | grep $(1) | awk '{$$1=$$1};1' | cut -d' ' -f2 | sed 's/^v//')

KUBERNETES_VERSION			= 1.35.0
KIND_IMAGE					= kindest/node:v$(KUBERNETES_VERSION)
KIND_CLUSTER_NAME          ?= ztoperator
KUBECONTEXT                ?= kind-$(KIND_CLUSTER_NAME)
ISTIO_VERSION 				= $(call extract-version,istio.io/client-go)
CERT_MANAGER_VERSION		= 1.19.2

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: run-local
run-local: ensurelocal ensureztoperatornotdeployed generate install sourceenv ## Run ztoperator from your host.
	go run ./cmd/main.go

.PHONY: isrunning
isrunning: ## Check if ztoperator is running on your host machine (i.e. from IDE or with 'make run-local')
	@echo "Checking if ztoperator is running..."
	@lsof -i :8081 > /dev/null || (echo "❌ ztoperator is not running. Please start it first either in your IDE or with 'make run-local'." && exit 1)
	@echo "✅ ztoperator is running."

.PHONY: isnotrunning
isnotrunning: ## Check if ztoperator is NOT running on your host machine (i.e. from IDE or with 'make run-local')
	@echo "Checking if ztoperator is not running..."
	@lsof -i :8081 > /dev/null || (echo "✅ ztoperator is not running on your host. Ready to deploy." && exit 0 || echo "❌ ztoperator is running on your host. Please stop it first." && exit 1)
	@echo "✅ ztoperator is not running."

.PHONY: sourceenv
sourceenv: ## Source environment variables from .env file
	@set -a; [ -f .env ] && . .env; set +a

.PHONY: local
local: cluster accesserator-namespace cert-manager istio-gateways skiperator mock-oauth2 generate install ## Set up entire local development environment with external dependencies

.PHONY: clean
clean: kind ## Clean up local environment by deleting kind cluster
	"$(KIND)" delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" rbac:roleName=ztoperator crd paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run --config .golangci.yml

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	"$(GOLANGCI_LINT)" run --fix --config .golangci.yml

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -o bin/ztoperator cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

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

.PHONY: deploy
deploy: ensurelocal isnotrunning accesserator-namespace generate install kustomize docker-build ## Deploy ztoperator and all the required resources for ztoperator to run properly to the kind cluster
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KIND)" load docker-image ${IMG} --name $(KIND_CLUSTER_NAME)
	"$(KUBECTL)" create secret generic ztoperator-env --from-env-file=.env -n ztoperator-system --context $(KUBECONTEXT)
	"$(KUSTOMIZE)" build config/manager | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy ztoperator and all the resources deployed by ztoperator to the kind cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/manager 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No manager resources to delete; skipping."; fi

.PHONY: install
install: kustomize generate ## Install CRDs and ClusterRoles into the K8s cluster specified in ~/.kube/config.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No CRDs to install; skipping."; fi
	@out="$$( "$(KUSTOMIZE)" build config/rbac 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No ClusterRoles to install; skipping."; fi

.PHONY: uninstall
uninstall: generate kustomize kubectl ## Uninstall CRDs and ClusterRoles from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi
	@out="$$( "$(KUSTOMIZE)" build config/rbac 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No ClusterRoles to delete; skipping."; fi

##@ Cluster

.PHONY: cluster
cluster: kind ## Create Kind cluster with kube context kind-ztoperator
	@echo Create kind cluster... >&2
	"$(KIND)" create cluster --image $(KIND_IMAGE) --name ${KIND_CLUSTER_NAME}

##@ Namespace

.PHONY: ztoperator-namespace
accesserator-namespace: kubectl ## Create ztoperator-system namespace in the cluster
	@/bin/bash ./scripts/create-ztoperator-namespace.sh

##@ Operators

.PHONY: skiperator
skiperator: ## Install Skiperator on k8s cluster
	@echo -e "🤞  Installing Skiperator..."
	@KUBECONTEXT=$(KUBECONTEXT) /bin/bash ./scripts/install-skiperator.sh
	"$(KUBECTL)" wait pod --for=condition=ready --timeout=30s -n skiperator-system -l app=skiperator --context $(KUBECONTEXT) || (echo -e "❌  Error deploying Skiperator." && exit 1)
	@echo -e "✅  Skiperator installed in namespace 'skiperator-system'!"

.PHONY: install-istio
install-istio: ## Install istio
	@echo "⬇️ Downloading Istio..."
	@curl -L https://istio.io/downloadIstio | ISTIO_VERSION=$(ISTIO_VERSION) TARGET_ARCH=$(ARCH) sh -
	@echo "⛵️  Installing Istio on Kubernetes cluster..."
	@./istio-$(ISTIO_VERSION)/bin/istioctl install --context $(KUBECONTEXT) -y --set meshConfig.accessLogFile=/dev/stdout --set profile=minimal &> /dev/null
	@rm -rf istio-$(ISTIO_VERSION)
	@echo "✅  Istio installation complete."

.PHONY: istio-gateways
istio-gateways: istiohelm install-istio ## Install istio gateways
	@echo "⛵️ Creating istio-gateways namespace..."
	@kubectl create namespace istio-gateways --context $(KUBECONTEXT) &> /dev/null || true
	@echo "⬇️  Installing istio-gateways"
	"$(HELM)" install istio-ingressgateway istio/gateway --version v$(ISTIO_VERSION) -n istio-gateways --kube-context $(KUBECONTEXT) --set labels.app=istio-ingress-external --set labels.istio=ingressgateway
	@echo "✅  Istio gateways installed."

.PHONY: cert-manager
cert-manager: kustomize kubectl ## Install cert-manager to the cluster
	@echo -e "🤞  Installing cert-manager..."
	"$(KUBECTL)" apply -f https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml
	@echo "🕑  Waiting for cert-manager to be ready..."
	"$(KUBECTL)" -n cert-manager wait deploy --all --for=condition=Available --timeout=60s
	@echo -e "✅  Cert-manager installed!"
	@out="$$( "$(KUSTOMIZE)" build config/cert-manager 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No cert manager resources to install; skipping."; fi

##@ Helper services

.PHONY: mock-oauth2
mock-oauth2: ## Deployinh Mock-OAuth service in auth namespace
	@echo -e "🤞  Deploying 'mock-oauth2'..."
	@KUBECONTEXT=$(KUBECONTEXT) MOCK_OAUTH2_CONFIG=scripts/mock-oauth2-server-config.json /bin/bash ./scripts/install-mock-oauth2.sh
	@echo -e "✅  'mock-oauth2' is ready and running"

##@ Helpers

.PHONY: mock-token
mock-token: ensureflox ensurekubefwd ## Retrieves a JWT issued by mock-oauth2
	@command -v jq >/dev/null 2>&1 || { echo -e "❌  jq is required (used to parse JSON). Please install jq and try again."; exit 1; }
	@token=$$(curl -s -X POST "http://mock-oauth2.auth:8080/entraid/token" \
		-d "grant_type=authorization_code" \
		-d "code=entraid_user" \
		-d "client_id=something" | jq -r '.access_token // empty'); \
	if [ -z "$$token" ]; then \
		echo -e "❌  No access_token found in response"; \
		exit 1; \
	fi; \
	echo "$$token"

.PHONY: ensurelocal
ensurelocal: kind kubectl ## Ensure local environment is set up with necessary tools and kind cluster is running
	@/bin/bash ./scripts/ensure-local-setup.sh

.PHONY: ensureztoperatornotdeployed
ensureztoperatornotdeployed: kubectl ## Ensure ztoperator is NOT deployed in the kind cluster
	"$(KUBECTL)" -n ztoperator-system get deployment ztoperator >/dev/null 2>&1 && (echo "❌ Ztoperator IS deployed to the cluster" && exit 1) || (echo "✅ Ztoperator IS NOT deployed to the cluster" && exit 0)

.PHONY: ensureztoperatordeployed
ensureztoperatordeployed: kubectl ensurelocal isnotrunning ## Ensure ztoperator is deployed in the kind cluster
	@/bin/bash ./scripts/ensure-ztoperator-deployed.sh || (echo "❌ Ztoperator resources are not deployed correctly to the cluster. To fix it, run 'make deploy'." && exit 1)

##@ Dependencies

.PHONY: istiohelm
istiohelm: helm ## Fetch helm charts for Istio
	# Ensure istio helm repo exists
	"$(HELM)" repo list | grep -q '^istio\s' || (echo "Adding istio helm repo..." && "$(HELM)" repo add istio https://istio-release.storage.googleapis.com/charts)
	# Make sure the requested ISTIO_VERSION is available; update index if not
	"$(HELM)" search repo istio/gateway --versions | grep -q "$(ISTIO_VERSION)" || (echo "Updating Helm repos to fetch Istio charts..." && "$(HELM)" repo update)
	"$(HELM)" search repo istio/gateway --versions | grep -q "$(ISTIO_VERSION)" || (echo "❌ Istio Helm chart version $(ISTIO_VERSION) not found in repo index." && echo "   Tip: check available versions with: helm search repo istio/gateway --versions" && exit 1)

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

## Tool Binaries
KUBECTL ?= $(LOCALBIN)/kubectl
KIND ?= $(LOCALBIN)/kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CHAINSAW ?= $(LOCALBIN)/chainsaw
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
HELM ?= $(LOCALBIN)/helm

## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.1
CHAINSAW_VERSION ?= v0.2.14
CONTROLLER_TOOLS_VERSION ?= v0.19.0
KUBECTL_VERSION ?= v1.34.2
KIND_VERSION ?= v0.31.0
GOLANGCI_LINT_VERSION ?= v2.10.1
HELM_VERSION ?= v4.0.0

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

.PHONY: helm
helm: $(HELM) ## Download helm locally if necessary.
$(HELM): $(LOCALBIN)
	@set -e; \
	if [ -x "$(HELM)" ]; then \
		echo "✅ helm already exists at $(HELM)"; \
		exit 0; \
	fi; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in \
		x86_64|amd64) arch=amd64 ;; \
		aarch64|arm64) arch=arm64 ;; \
		armv7l) arch=arm ;; \
		*) echo "❌ Unsupported architecture: $$arch" >&2; exit 1 ;; \
	esac; \
	url="https://get.helm.sh/helm-$(HELM_VERSION)-$${os}-$${arch}.tar.gz"; \
	echo "Downloading helm $(HELM_VERSION) from $$url"; \
	curl -L -o helm.tar.gz "$$url"; \
	tar -xzf helm.tar.gz -C "$(LOCALBIN)" --strip-components=1 --no-same-owner "$${os}-$${arch}/helm"; \
	chmod +x "$(HELM)"; \
	rm helm.tar.gz; \
	echo "✅ helm installed at $(HELM)"

.PHONY: chainsaw
chainsaw: $(CHAINSAW) ## Download chainsaw locally if necessary.
$(CHAINSAW): $(LOCALBIN)
	$(call go-install-tool,$(CHAINSAW),github.com/kyverno/chainsaw,$(CHAINSAW_VERSION))

.PHONY: kubectl
kubectl: $(KUBECTL) ## Download kubectl locally if necessary.
$(KUBECTL): $(LOCALBIN)
	@set -e; \
	if [ -x "$(KUBECTL)" ]; then \
		echo "✅ kubectl already exists at $(KUBECTL)"; \
		exit 0; \
	fi; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in \
		x86_64|amd64) arch=amd64 ;; \
		aarch64|arm64) arch=arm64 ;; \
		armv7l) arch=arm ;; \
		*) echo "❌ Unsupported architecture: $$arch" >&2; exit 1 ;; \
	esac; \
	url="https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$${os}/$${arch}/kubectl"; \
	echo "Downloading kubectl $(KUBECTL_VERSION) from $$url"; \
	curl -L -o "$(KUBECTL)" "$$url"; \
	chmod +x "$(KUBECTL)"; \
	echo "✅ kubectl installed at $(KUBECTL)"

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef

##@ Testing

## Location to install dependencies to
VENV := venv
PYTHON := $(VENV)/bin/python
PIP := $(VENV)/bin/pip

.PHONY: virtualenv
virtualenv: ## Set up Python virtual environment and install required packages
	@which python3 >/dev/null || (echo "Python3 not installed, please install it to proceed"; exit 1)
	# Set up virtualenv, activate it and install required packages
	@python3 -m venv $(VENV)
	@$(PIP) install -r scripts/requirements.txt

.PHONY: expose-ingress
expose-ingress: ## Expose istio ingress gateway on localhost:8443
	@lsof -ni :8443 | grep LISTEN && (echo "Port 8443 is already in use. Trying to kill kubectl" && killall kubectl) || true
	@echo "Exposing istio ingress gateway on localhost 8443"
	@KUBECONTEXT=$(KUBECONTEXT) kubectl port-forward --context $(KUBECONTEXT) -n istio-gateways svc/istio-ingressgateway 8443:443 2>&1 & \

.PHONY: isingressready
isingressready: ## Check if venv is activated and Istio ingress gateway is exposed on localhost:8443
	@if [ -z "$$VIRTUAL_ENV" ]; then \
		echo "❌ Python venv is not activated. Please activate your virtual environment with 'make virtualenv'."; \
		exit 1; \
	else \
		echo "✅ Python venv is activated."; \
	fi
	@if nc -z localhost 8443; then \
		echo "✅ Istio ingress gateway is exposed on localhost:8443."; \
	else \
		echo "❌ Istio ingress gateway is NOT exposed on localhost:8443. This can be done with 'make expose-ingress'"; \
		exit 1; \
	fi

.PHONY: test
test: generate fmt vet setup-envtest ## Run go tests.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./...) -coverprofile cover.out

.PHONY: chainsaw-test-remote
chainsaw-test-remote: chainsaw isnotrunning ensureztoperatordeployed isingressready ## Run chainsaw tests against local kind cluster with ztoperator running in the cluster
	@bash -ec ' \
			for dir in test/chainsaw/authpolicy/*/ ; do \
				echo "Running test in $$dir"; \
				if ! $(MAKE) chainsaw-test-remote-single dir=$$dir; then \
					echo "Test in $$dir failed."; \
					exit 1; \
				fi; \
			done; \
	' || (echo "Test(s) failed." && exit 1)

.PHONY: chainsaw-test-remote-single
chainsaw-test-remote-single: chainsaw isnotrunning ensureztoperatordeployed isingressready
	"$(CHAINSAW)" test --kube-context $(KUBECONTEXT) --config test/chainsaw/config.yaml --test-dir $(dir) && \
    	echo "✅ Test succeeded" || (echo "❌ Test failed" && exit 1)

.PHONY: chainsaw-test-host
chainsaw-test-host: chainsaw install ensurelocal ensureztoperatornotdeployed isrunning isingressready ## Run chainsaw tests against local kind cluster with ztoperator running on host
	@bash -ec ' \
    		for dir in test/chainsaw/authpolicy/*/ ; do \
    			echo "Running test in $$dir"; \
    			if ! $(MAKE) chainsaw-test-host-single dir=$$dir; then \
    				echo "Test in $$dir failed."; \
    				exit 1; \
    			fi; \
    		done; \
	' || (echo "Test(s) failed." && exit 1)

.PHONY: chainsaw-test-host-single
chainsaw-test-host-single: chainsaw install ensurelocal ensureztoperatornotdeployed isrunning isingressready ## Run a specific chainsaw test against local kind cluster with ztoperator running on host. Example usage: chainsaw-test-host-single dir=<CHAINSAW_TEST_DIR>
	"$(CHAINSAW)" test --kube-context $(KUBECONTEXT) --config test/chainsaw/config.yaml --test-dir $(dir) && \
    	echo "✅ Test succeeded" || (echo "❌ Test failed" && exit 1)

##@ Custom targets
ensureflox: ## Ensure Flox is installed and activated
	@if ! command -v "flox" >/dev/null 2>&1; then \
		echo -e "❌  Flox is not installed. Please install Flox (https://flox.dev/docs/install-flox/) and try again."; \
		exit 1; \
	fi
ifndef FLOX_ENV
	echo -e "❌  Flox is not activated. Please activate flox with 'flox activate' and try again." && exit 1
endif

ensurekubefwd: ensureflox ## Ensure kubefwd is installed and running
	@pgrep -f "kubefwd( |$$)" >/dev/null 2>&1 || { \
		echo -e "❌  kubefwd is not running."; \
		echo -e "    Start it in another terminal with:"; \
		echo -e "      sudo kubefwd svc -n <namespace> --context $(KUBECONTEXT)"; \
		exit 1; \
	}
