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

# Pinned by digest so local runs and the api-docs workflow render identically.
CRDOC_IMAGE                ?= ghcr.io/fybrik/crdoc@sha256:355ef777a45021ee864e613b2234b4f2c6193762e3e0de94a26b66d06cec81c3
CRDOC_RESOURCES_DIR        := crdoc-resources

KUBERNETES_VERSION			= 1.35.1
CERT_MANAGER_VERSION		= 1.20.2
ISTIO_VERSION 				= $(call extract-version,istio.io/client-go)

KIND_IMAGE					= kindest/node:v$(KUBERNETES_VERSION)
KIND_CLUSTER_NAME          ?= ztoperator
KUBECONTEXT                ?= kind-$(KIND_CLUSTER_NAME)
LOCAL_WEBHOOK_CERTS_DIR	   ?= webhook-certs
HOSTS_FILE				   ?= /etc/hosts
GATEWAY					   ?= istio-external
GATEWAY_NS 				   ?= istio-gateways
GATEWAY_PORTS 			   ?= 443:443 80:80
GATEWAY_FORWARD_PID        ?= /tmp/ztoperator-gateway-forward.pid
GATEWAY_FORWARD_LOG        ?= /tmp/ztoperator-gateway-forward.log
KUBEFWD_PID                ?= /tmp/ztoperator-kubefwd.pid
KUBEFWD_LOG                ?= /tmp/ztoperator-kubefwd.log

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
KUBEFWD ?= $(LOCALBIN)/kubefwd
JQ ?= $(LOCALBIN)/jq

## Tool Versions
KUSTOMIZE_VERSION ?= v5.8.1
# Temporary pin until Chainsaw publishes a release with Go 1.27 support.
CHAINSAW_VERSION ?= b7d4a4f5dd45e333add856a5ae9057fc5a57e93a
CONTROLLER_TOOLS_VERSION ?= v0.21.0
KUBECTL_VERSION ?= v$(KUBERNETES_VERSION)
KIND_VERSION ?= v0.31.0
GOLANGCI_LINT_VERSION ?= v2.15.0
HELM_VERSION ?= v4.0.0
SKIPERATOR_VERSION ?= v2.19.0
GATEWAY_API_VERSION ?= v1.6.2
KUBEFWD_VERSION ?= 1.25.15
JQ_VERSION ?= 1.8.1

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: run-local
run-local: ensurelocal ensureztoperatornotdeployed generate install webhooks ## Run ztoperator from your host.
	set -a; . config/manager/base/.env; set +a; go run ./cmd/main.go -webhook-cert-path=./webhook-certs

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

.PHONY: local
local: cluster ztoperator-namespace cert-manager istio-gateways skiperator mock-oauth2 generate install ## Set up entire local development environment with external dependencies

EXAMPLE_MANIFEST ?= examples/example.yaml
EXAMPLE_HOST     ?= demo.localdev

.PHONY: example
example: kubectl ensurelocal ensurerunningordeployed ## Deploy examples/example.yaml and expose it on https://demo.localdev (gateway forward runs in the background)
	@echo -e "🤞  Applying $(EXAMPLE_MANIFEST)..."
	@"$(KUBECTL)" apply -f "$(EXAMPLE_MANIFEST)" --context $(KUBECONTEXT)
	@echo -e "✅  Applied $(EXAMPLE_MANIFEST)"
	@$(MAKE) --no-print-directory mock-oauth2-ingress
	@$(MAKE) --no-print-directory host-entry host=$(EXAMPLE_HOST)
	@$(MAKE) --no-print-directory gateway-forward GATEWAY_PORTS="443:443 80:80"
	@echo -e "🌐  Open https://$(EXAMPLE_HOST)  (stop the forward with 'make gateway-forward-stop')"

.PHONY: clean
clean: kind ## Clean up local environment by deleting kind cluster
	"$(KIND)" delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" object rbac:roleName=ztoperator crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases output:webhook:artifacts:config=config/webhook/bases

# Renders api-docs.md from the generated CRDs. crdoc only reads the directory it
# is pointed at, so the CRDs are staged in one and removed again afterwards.
.PHONY: apidocs
apidocs:
	@if ! command -v docker >/dev/null 2>&1; then \
		echo "docker not found on PATH, skipping apidocs"; \
		exit 0; \
	fi; \
	mkdir -p $(CRDOC_RESOURCES_DIR) || exit 1; \
	cp config/crd/bases/ztoperator.kartverket.no_*.yaml $(CRDOC_RESOURCES_DIR)/ || exit 1; \
	docker run -u $$(id -u):$$(id -g) --rm -v "$(PWD)":/workdir $(CRDOC_IMAGE) \
		--resources /workdir/$(CRDOC_RESOURCES_DIR) \
		--output /workdir/api-docs.md \
		--template /workdir/.github/crdoc.tmpl; \
	status=$$?; \
	rm -rf $(CRDOC_RESOURCES_DIR); \
	exit $$status

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

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: deploy
deploy: ensurelocal isnotrunning ztoperator-namespace generate install kustomize docker-build ## Deploy ztoperator and all the required resources for ztoperator to run properly to the kind cluster
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KIND)" load docker-image ${IMG} --name $(KIND_CLUSTER_NAME)
	"$(KUSTOMIZE)" build config/webhook | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -
	"$(KUSTOMIZE)" build config/manager | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -
	"$(KUBECTL)" wait pod --for=condition=ready --timeout=60s -n ztoperator-system -l app=ztoperator --context $(KUBECONTEXT) || (echo -e "❌  Error deploying ztoperator." && exit 1)
	@echo -e "✅  ztoperator installed in namespace 'ztoperator-system'!"

.PHONY: undeploy
undeploy: kustomize ## Undeploy ztoperator and all the resources deployed by ztoperator to the kind cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/webhook 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No webhook resources to delete; skipping."; fi
	@out="$$( "$(KUSTOMIZE)" build config/manager 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No manager resources to delete; skipping."; fi

.PHONY: webhooks
webhooks: kustomize ## Extract webhook certificate details
	@/bin/bash ./scripts/get-webhook-certs.sh

.PHONY: install
install: kustomize generate ## Install CRDs, Webhook configurations and ClusterRoles into the local kind cluster.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No CRDs to install; skipping."; fi
	@out="$$( "$(KUSTOMIZE)" build config/rbac 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No ClusterRoles to install; skipping."; fi
	@if $(MAKE) ensureztoperatornotdeployed >/dev/null 2>&1; then \
		echo "Ztoperator is not deployed; installing local webhook config."; \
		out="$$( "$(KUSTOMIZE)" build config/webhook-local 2>/dev/null || true )"; \
		if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No Webhook configurations to install; skipping."; fi; \
	else \
		echo "Ztoperator is deployed; installing cluster webhook config."; \
		out="$$( "$(KUSTOMIZE)" build config/webhook 2>/dev/null || true )"; \
		if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply --context $(KUBECONTEXT) -f -; else echo "No Webhook configurations to install; skipping."; fi; \
	fi


.PHONY: uninstall
uninstall: generate kustomize kubectl ## Uninstall CRDs, Webhook configurations and ClusterRoles from the local kind cluster. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi
	@out="$$( "$(KUSTOMIZE)" build config/rbac 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No ClusterRoles to delete; skipping."; fi
	@if $(MAKE) ensureztoperatornotdeployed >/dev/null 2>&1; then \
		echo "Ztoperator is not deployed; uninstalling local webhook config."; \
		out="$$( "$(KUSTOMIZE)" build config/webhook-local 2>/dev/null || true )"; \
		if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No Webhook configurations to delete; skipping."; fi; \
	else \
		echo "Ztoperator is deployed; uninstalling cluster webhook config."; \
		out="$$( "$(KUSTOMIZE)" build config/webhook 2>/dev/null || true )"; \
		if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --context $(KUBECONTEXT) --ignore-not-found=$(ignore-not-found) -f -; else echo "No Webhook configurations to delete; skipping."; fi; \
	fi

##@ Cluster

.PHONY: cluster
cluster: kind ## Create Kind cluster with kube context kind-ztoperator
	@echo Create kind cluster... >&2
	"$(KIND)" create cluster --image $(KIND_IMAGE) --name ${KIND_CLUSTER_NAME}

##@ Namespace

.PHONY: ztoperator-namespace
ztoperator-namespace: kubectl ## Create ztoperator-system namespace in the cluster
	@/bin/bash ./scripts/create-ztoperator-namespace.sh

##@ Operators

.PHONY: skiperator
skiperator: ## Install Skiperator on k8s cluster
	@echo -e "🤞  Installing Skiperator..."
	@KUBECONTEXT=$(KUBECONTEXT) SKIPERATOR_VERSION=$(SKIPERATOR_VERSION) GATEWAY_API_VERSION=$(GATEWAY_API_VERSION) \
	  /bin/bash ./scripts/install-skiperator.sh
	"$(KUBECTL)" wait pod --for=condition=ready --timeout=30s -n skiperator-system -l app=skiperator --context $(KUBECONTEXT) || (echo -e "❌  Error deploying Skiperator." && exit 1)
	@echo -e "✅  Skiperator installed in namespace 'skiperator-system'!"

.PHONY: gateway-api-crds
gateway-api-crds: kubectl ## Install Gateway API CRDs (standard channel — includes ListenerSet)
	@echo "⬇️  Installing Gateway API $(GATEWAY_API_VERSION) CRDs..."
	"$(KUBECTL)" apply --context $(KUBECONTEXT) --server-side --force-conflicts \
	  -f https://github.com/kubernetes-sigs/gateway-api/releases/download/$(GATEWAY_API_VERSION)/standard-install.yaml
	"$(KUBECTL)" wait --for=condition=Established --timeout=60s --context $(KUBECONTEXT) \
	  crd/gateways.gateway.networking.k8s.io \
	  crd/httproutes.gateway.networking.k8s.io \
	  crd/listenersets.gateway.networking.k8s.io

.PHONY: install-istio
install-istio: gateway-api-crds ## Install istio
	@echo "⬇️ Downloading Istio..."
	@curl -L https://istio.io/downloadIstio | ISTIO_VERSION=$(ISTIO_VERSION) TARGET_ARCH=$(ARCH) sh -
	@echo "⛵️  Installing Istio on Kubernetes cluster..."
	@# Strict gateway merging is disabled so Skiperator's Legacy (Istio Gateway) and
	@# Standard (Gateway API) routing can both be served by the Gateway API managed gateways.
	@./istio-$(ISTIO_VERSION)/bin/istioctl install --context $(KUBECONTEXT) -y \
	  --set meshConfig.accessLogFile=/dev/stdout \
	  --set profile=ambient \
	  --set values.pilot.env.PILOT_ENABLE_STRICT_GATEWAY_MERGING=false
	@rm -rf istio-$(ISTIO_VERSION)
	@echo "✅  Istio installation complete."

.PHONY: istio-gateways
istio-gateways: kubectl istiohelm install-istio ## Install istio ingress gateways (Legacy + Gateway API)
	@echo "⛵️ Creating istio-gateways namespace..."
	@"$(KUBECTL)" create namespace istio-gateways --context $(KUBECONTEXT) &> /dev/null || true
	@echo "⬇️  Installing legacy istio-ingressgateway (routingProvider=Legacy)"
	"$(HELM)" upgrade --install istio-ingressgateway istio/gateway --version v$(ISTIO_VERSION) \
	  -n istio-gateways --kube-context $(KUBECONTEXT) \
	  --set labels.app=istio-ingress-external --set labels.istio=ingressgateway
	@echo "⬇️  Installing Gateway API gateways (routingProvider=Standard)"
	"$(KUBECTL)" apply --context $(KUBECONTEXT) -f config/istio/gateways.yaml
	"$(KUBECTL)" wait gateway istio-external istio-internal -n istio-gateways \
	  --for=condition=Programmed --timeout=120s --context $(KUBECONTEXT)
	@echo "✅  Istio gateways installed."

.PHONY: cert-manager
cert-manager: kustomize kubectl ## Install cert-manager to the cluster
	@echo -e "🤞  Installing cert-manager..."
	"$(KUBECTL)" apply -f https://github.com/cert-manager/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml
	@echo "🕑  Waiting for cert-manager to be ready..."
	"$(KUBECTL)" -n cert-manager wait deploy --all --for=condition=Available --timeout=120s
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
mock-token: ## Retrieves a JWT issued by mock-oauth2
	@JQ_OUTPUT=$$($(MAKE) jq 2>&1); \
	if [ $$? -ne 0 ]; then echo "$$JQ_OUTPUT"; exit 1; fi
	@ENSURE_OUTPUT=$$($(MAKE) ensuremockoauth2isreachable 2>&1); \
	if [ $$? -ne 0 ]; then echo "$$ENSURE_OUTPUT"; exit 1; fi
	@token=$$(curl -s -X POST "http://mock-oauth2.auth:8080/ztoperator/token" \
		-d "grant_type=authorization_code" \
		-d "code=code" \
		-d "client_id=something" | "$(JQ)" -r '.access_token // empty'); \
	if [ -z "$$token" ]; then \
		echo -e "❌  No access_token found in response" >&2; \
		exit 1; \
	fi; \
	echo "$$token"

.PHONY: mock-oauth2-ingress
mock-oauth2-ingress: kubefwd ## Ensure mock-oauth2 is reachable via kubefwd running in the background (stop with 'make mock-oauth2-ingress-stop')
	@KUBEFWD=$$(command -v kubefwd 2>/dev/null || echo "$(KUBEFWD)"); \
	if [ ! -x "$$KUBEFWD" ]; then \
		echo -e "❌  kubefwd not found. Install it or run 'make kubefwd'."; \
		exit 1; \
	fi; \
	echo -e "🔍  Checking if mock-oauth2 is running in the cluster..."; \
	"$(KUBECTL)" wait pod --for=condition=Ready --timeout=10s -n auth -l app=mock-oauth2 --context $(KUBECONTEXT) 2>/dev/null || { \
		echo -e "❌  mock-oauth2 is not ready. Deploy it first with 'make mock-oauth2'."; \
		exit 1; \
	}; \
	echo -e "✅  mock-oauth2 is ready"; \
	STATUS=$$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 http://mock-oauth2.auth:8080/ztoperator/.well-known/openid-configuration 2>/dev/null || echo "000"); \
	if [ "$$STATUS" = "200" ]; then \
		echo -e "✅  mock-oauth2 is reachable on http://mock-oauth2.auth:8080"; \
		exit 0; \
	fi; \
	echo -e "⚠️   mock-oauth2 is not reachable (HTTP $$STATUS). Starting kubefwd in the background (requires sudo)..."; \
	sudo -v || { echo -e "❌  sudo authentication failed"; exit 1; }; \
	if pgrep -x "kubefwd" >/dev/null 2>&1; then \
		echo -e "⏳  Stopping existing kubefwd..."; \
		sudo pkill -x "kubefwd" || true; \
		sleep 1; \
	fi; \
	rm -f "$(KUBEFWD_PID)"; \
	nohup sudo -E "$$KUBEFWD" svc -n auth --context $(KUBECONTEXT) \
		> "$(KUBEFWD_LOG)" 2>&1 & \
	echo $$! > "$(KUBEFWD_PID)"; \
	echo -e "⏳  Waiting for kubefwd to establish connections..."; \
	for i in $$(seq 1 15); do \
		sleep 2; \
		if ! ps -p "$$(cat "$(KUBEFWD_PID)")" > /dev/null 2>&1; then \
			echo -e "❌  kubefwd exited. See $(KUBEFWD_LOG):"; \
			tail -n 20 "$(KUBEFWD_LOG)"; \
			rm -f "$(KUBEFWD_PID)"; \
			exit 1; \
		fi; \
		STATUS=$$(curl -s -o /dev/null -w "%{http_code}" --max-time 2 http://mock-oauth2.auth:8080/ztoperator/.well-known/openid-configuration 2>/dev/null || echo "000"); \
		if [ "$$STATUS" = "200" ]; then \
			echo -e "✅  mock-oauth2 is now reachable on http://mock-oauth2.auth:8080 (kubefwd PID $$(cat "$(KUBEFWD_PID)"), log: $(KUBEFWD_LOG))"; \
			echo -e "ℹ️   Stop it with: make mock-oauth2-ingress-stop"; \
			exit 0; \
		fi; \
	done; \
	echo -e "❌  mock-oauth2 is still not reachable. Check $(KUBEFWD_LOG) for details."; \
	exit 1

.PHONY: mock-oauth2-ingress-stop
mock-oauth2-ingress-stop: ## Stop the background kubefwd started by 'make mock-oauth2-ingress'
	@if [ -f "$(KUBEFWD_PID)" ] && ps -p "$$(cat "$(KUBEFWD_PID)")" > /dev/null 2>&1; then \
		echo -e "🛑  Stopping kubefwd (PID $$(cat "$(KUBEFWD_PID)"), requires sudo)"; \
		sudo kill "$$(cat "$(KUBEFWD_PID)")" || true; \
	elif pgrep -x "kubefwd" >/dev/null 2>&1; then \
		echo -e "🛑  Stopping kubefwd (no PID file, requires sudo)"; \
		sudo pkill -x "kubefwd" || true; \
	else \
		echo -e "✅  kubefwd is not running, nothing to stop"; \
	fi; \
	rm -f "$(KUBEFWD_PID)"

.PHONY: host-entry
host-entry: ## Point a hostname at 127.0.0.1 in /etc/hosts. Usage: make host-entry host=foo.bar
	$(if $(strip $(host)),,$(error host is not set. Usage: make host-entry host=<HOSTNAME>))
	@marker="# ztoperator-managed"; \
	if awk -v h="$(host)" '!/^[[:space:]]*#/ { for (i = 2; i <= NF; i++) if ($$i == h) { found = 1; exit } } END { exit(found ? 0 : 1) }' "$(HOSTS_FILE)"; then \
	   echo "✅  '$(host)' is already in $(HOSTS_FILE), nothing to do"; \
	   exit 0; \
	fi; \
	echo "🤞  Adding '$(host)' to $(HOSTS_FILE) (requires sudo)"; \
	printf '127.0.0.1\t%s\t%s\n' "$(host)" "$$marker" | sudo tee -a "$(HOSTS_FILE)" > /dev/null; \
	echo "✅  Added: 127.0.0.1 $(host)  $$marker"; \
	case "$(host)" in \
	   *.skip.statkart.no|*.kartverket-intern.cloud) \
	      echo "ℹ️   Skiperator routes this hostname through the internal gateway. Reach it with:"; \
	      echo "    make gateway-forward GATEWAY=istio-internal" ;; \
	   *) \
	      echo "ℹ️   Reach it with:"; \
	      echo "    make gateway-forward" ;; \
	esac

.PHONY: host-entry-clean
host-entry-clean: ## Remove /etc/hosts entries added by 'make host-entry'. All of them, or one: make host-entry-clean host=foo.bar
	@marker="# ztoperator-managed"; \
	matches=$$(awk -v m="$$marker" -v h="$(strip $(host))" \
	   'function hostmatch() { for (i = 2; i <= NF; i++) if ($$i == h) return 1; return 0 } \
	    index($$0, m) > 0 && (h == "" || hostmatch())' "$(HOSTS_FILE)"); \
	if [ -z "$$matches" ]; then \
	   if [ -n "$(strip $(host))" ]; then \
	      echo "✅  No entry for '$(host)' managed by ztoperator in $(HOSTS_FILE), nothing to do"; \
	   else \
	      echo "✅  No entries managed by ztoperator in $(HOSTS_FILE), nothing to do"; \
	   fi; \
	   exit 0; \
	fi; \
	echo "🧹  Removing from $(HOSTS_FILE) (requires sudo):"; \
	echo "$$matches" | sed 's/^/      /'; \
	tmp=$$(mktemp); \
	awk -v m="$$marker" -v h="$(strip $(host))" \
	   'function hostmatch() { for (i = 2; i <= NF; i++) if ($$i == h) return 1; return 0 } \
	    !(index($$0, m) > 0 && (h == "" || hostmatch()))' "$(HOSTS_FILE)" > "$$tmp"; \
	if [ ! -s "$$tmp" ]; then \
	   echo "❌  Refusing to write an empty $(HOSTS_FILE) - aborting, nothing changed"; \
	   rm -f "$$tmp"; \
	   exit 1; \
	fi; \
	sudo tee "$(HOSTS_FILE)" < "$$tmp" > /dev/null; \
	rm -f "$$tmp"; \
	echo "✅  Removed $$(printf '%s\n' "$$matches" | wc -l | tr -d ' ') entry/entries from $(HOSTS_FILE)"

.PHONY: gateway-forward
gateway-forward: kubectl ## Forward the istio gateway to localhost in the background (stop with 'make gateway-forward-stop'). Override: GATEWAY=istio-internal GATEWAY_PORTS="8443:443 80:80"
	@if [ -f "$(GATEWAY_FORWARD_PID)" ] && ps -p "$$(cat "$(GATEWAY_FORWARD_PID)")" > /dev/null 2>&1; then \
	   echo "✅  Gateway forward already running (PID $$(cat "$(GATEWAY_FORWARD_PID)"), log: $(GATEWAY_FORWARD_LOG))"; \
	   echo "ℹ️   Restart with: make gateway-forward-stop gateway-forward"; \
	   exit 0; \
	fi; \
	rm -f "$(GATEWAY_FORWARD_PID)"; \
	echo "🔍  Checking gateway '$(GATEWAY)' in namespace '$(GATEWAY_NS)'..."; \
	"$(KUBECTL)" wait gateway $(GATEWAY) -n $(GATEWAY_NS) --for=condition=Programmed \
	   --timeout=30s --context $(KUBECONTEXT) > /dev/null 2>&1 || { \
	   echo "❌  Gateway '$(GATEWAY)' is not Programmed. Install it with 'make istio-gateways'."; \
	   exit 1; \
	}; \
	svc="$(GATEWAY)-istio"; \
	"$(KUBECTL)" get svc "$$svc" -n $(GATEWAY_NS) --context $(KUBECONTEXT) > /dev/null 2>&1 || { \
	   echo "❌  Service '$$svc' not found in $(GATEWAY_NS)."; \
	   exit 1; \
	}; \
	for mapping in $(GATEWAY_PORTS); do \
	   port="$${mapping%%:*}"; \
	   if lsof -nP -iTCP:"$$port" -sTCP:LISTEN > /dev/null 2>&1; then \
	      echo "❌  Local port $$port is already in use. Stop whatever holds it, or pass GATEWAY_PORTS."; \
	      exit 1; \
	   fi; \
	done; \
	hosts=$$("$(KUBECTL)" get httproutes -A --context $(KUBECONTEXT) \
	   -o jsonpath='{range .items[*]}{range .spec.hostnames[*]}{@}{"\n"}{end}{end}' 2>/dev/null | sort -u); \
	if [ -n "$$hosts" ]; then \
	   echo "🌐  Hostnames routed by this cluster:"; \
	   for h in $$hosts; do \
	      if awk -v h="$$h" '!/^[[:space:]]*#/ { for (i = 2; i <= NF; i++) if ($$i == h) { found = 1; exit } } END { exit(found ? 0 : 1) }' $(HOSTS_FILE); then \
	         echo "      https://$$h"; \
	      else \
	         echo "      https://$$h   ⚠️  not in $(HOSTS_FILE) - run 'make host-entry host=$$h'"; \
	      fi; \
	   done; \
	fi; \
	echo "🔌  Forwarding $(GATEWAY_PORTS) from $$svc in the background (sudo needed for ports below 1024)"; \
	sudo -v || { echo "❌  sudo authentication failed"; exit 1; }; \
	nohup sudo KUBECONFIG="$$HOME/.kube/config" "$(KUBECTL)" port-forward \
	   -n $(GATEWAY_NS) "svc/$$svc" $(GATEWAY_PORTS) --context $(KUBECONTEXT) \
	   > "$(GATEWAY_FORWARD_LOG)" 2>&1 & \
	echo $$! > "$(GATEWAY_FORWARD_PID)"; \
	for mapping in $(GATEWAY_PORTS); do \
	   port="$${mapping%%:*}"; \
	   for i in $$(seq 1 20); do \
	      lsof -nP -iTCP:"$$port" -sTCP:LISTEN > /dev/null 2>&1 && break; \
	      if ! ps -p "$$(cat "$(GATEWAY_FORWARD_PID)")" > /dev/null 2>&1; then \
	         echo "❌  Port-forward exited. See $(GATEWAY_FORWARD_LOG):"; \
	         tail -n 20 "$(GATEWAY_FORWARD_LOG)"; \
	         rm -f "$(GATEWAY_FORWARD_PID)"; \
	         exit 1; \
	      fi; \
	      sleep 0.5; \
	   done; \
	done; \
	echo "✅  Gateway forward running (PID $$(cat "$(GATEWAY_FORWARD_PID)"), log: $(GATEWAY_FORWARD_LOG))"; \
	echo "ℹ️   Stop it with: make gateway-forward-stop"

.PHONY: gateway-forward-stop
gateway-forward-stop: ## Stop the background gateway port-forward started by 'make gateway-forward'
	@if [ ! -f "$(GATEWAY_FORWARD_PID)" ]; then \
	   echo "✅  No gateway forward PID file found, nothing to stop"; \
	   exit 0; \
	fi; \
	pid=$$(cat "$(GATEWAY_FORWARD_PID)"); \
	if ps -p "$$pid" > /dev/null 2>&1; then \
	   echo "🛑  Stopping gateway forward (PID $$pid, requires sudo)"; \
	   sudo kill "$$pid" || true; \
	fi; \
	rm -f "$(GATEWAY_FORWARD_PID)"; \
	echo "✅  Gateway forward stopped"

.PHONY: ensurelocal
ensurelocal: kind kubectl ## Ensure local environment is set up with necessary tools and kind cluster is running
	@/bin/bash ./scripts/ensure-local-setup.sh

.PHONY: ensureztoperatornotdeployed
ensureztoperatornotdeployed: kubectl ## Ensure ztoperator is NOT deployed in the kind cluster
	@if "$(KUBECTL)" -n ztoperator-system get deployment ztoperator >/dev/null 2>&1; then \
		echo "❌ Ztoperator IS deployed to the cluster"; \
		exit 1; \
	else \
		echo "✅ Ztoperator IS NOT deployed to the cluster"; \
	fi

.PHONY: ensureztoperatordeployed
ensureztoperatordeployed: kubectl ensurelocal isnotrunning ## Ensure ztoperator is deployed in the kind cluster
	@/bin/bash ./scripts/ensure-ztoperator-deployed.sh || (echo "❌ Ztoperator resources are not deployed correctly to the cluster. To fix it, run 'make deploy'." && exit 1)

.PHONY: ensurerunningordeployed
ensurerunningordeployed: ## Ensure ztoperator is running on host OR deployed in cluster, but not both
	@$(MAKE) isrunning >/dev/null 2>&1 && running=1 || running=0; \
	$(MAKE) ensureztoperatordeployed >/dev/null 2>&1 && deployed=1 || deployed=0; \
	if [ "$$running" = "1" ] && [ "$$deployed" = "1" ]; then \
		echo "❌ Ztoperator is both running on the host AND deployed in the cluster. Stop one before continuing."; \
		exit 1; \
	fi; \
	if [ "$$running" = "0" ] && [ "$$deployed" = "0" ]; then \
		echo "❌ Ztoperator is neither running on the host nor deployed in the cluster."; \
		echo "   Start it in your IDE / with 'make run-local', or deploy it with 'make deploy'."; \
		exit 1; \
	fi; \
	if [ "$$running" = "1" ]; then echo "✅ Ztoperator is running on the host."; fi; \
	if [ "$$deployed" = "1" ]; then echo "✅ Ztoperator is deployed an running in the local cluster."; fi

.PHONY: webhook-test-manifests
webhook-test-manifests: kustomize ## Build webhook manifests for envtest into webhook-tests/
	@mkdir -p webhook-tests
	@"$(KUSTOMIZE)" build config/webhook > webhook-tests/webhook-manifests.yaml

##@ Dependencies

.PHONY: istiohelm
istiohelm: helm ## Fetch helm charts for Istio
	# Ensure istio helm repo exists
	"$(HELM)" repo list | grep -q '^istio\s' || (echo "Adding istio helm repo..." && $(HELM) repo add istio https://istio-release.r2.istio.io/charts)
	# Make sure the requested ISTIO_VERSION is available; update index if not
	"$(HELM)" search repo istio/gateway --versions | grep -q "$(ISTIO_VERSION)" || (echo "Updating Helm repos to fetch Istio charts..." && $(HELM) repo update)
	"$(HELM)" search repo istio/gateway --versions | grep -q "$(ISTIO_VERSION)" || (echo "❌ Istio Helm chart version $(ISTIO_VERSION) not found in repo index." && echo "   Tip: check available versions with: helm search repo istio/gateway --versions" && exit 1)

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

.PHONY: jq
jq: $(JQ) ## Download jq locally if necessary.
$(JQ): $(LOCALBIN)
	@set -e; \
	if [ -x "$(JQ)" ]; then \
		echo "✅ jq already exists at $(JQ)"; \
		exit 0; \
	fi; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in \
		x86_64|amd64) arch=amd64 ;; \
		aarch64|arm64) arch=arm64 ;; \
		*) echo "❌ Unsupported architecture: $$arch" >&2; exit 1 ;; \
	esac; \
	case "$$os" in \
		linux) binary="jq-linux-$${arch}" ;; \
		darwin) binary="jq-macos-$${arch}" ;; \
		*) echo "❌ Unsupported OS: $$os" >&2; exit 1 ;; \
	esac; \
	url="https://github.com/jqlang/jq/releases/download/jq-$(JQ_VERSION)/$${binary}"; \
	echo "Downloading jq $(JQ_VERSION) from $$url"; \
	curl -L -o "$(JQ)" "$$url"; \
	chmod +x "$(JQ)"; \
	echo "✅ jq installed at $(JQ)"

.PHONY: kubefwd
kubefwd: $(KUBEFWD) ## Download kubefwd locally if necessary.
$(KUBEFWD): $(LOCALBIN)
	@set -e; \
	if [ -x "$(KUBEFWD)" ]; then \
		echo "✅ kubefwd already exists at $(KUBEFWD)"; \
		exit 0; \
	fi; \
	os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case "$$arch" in \
		x86_64|amd64) arch=x86_64 ;; \
		aarch64|arm64) arch=arm64 ;; \
		*) echo "❌ Unsupported architecture: $$arch" >&2; exit 1 ;; \
	esac; \
	case "$$os" in \
		linux) os_name=Linux ;; \
		darwin) os_name=Darwin ;; \
		*) echo "❌ Unsupported OS: $$os" >&2; exit 1 ;; \
	esac; \
	url="https://github.com/txn2/kubefwd/releases/download/v$(KUBEFWD_VERSION)/kubefwd_$${os_name}_$${arch}.tar.gz"; \
	echo "Downloading kubefwd $(KUBEFWD_VERSION) from $$url"; \
	curl -L -o kubefwd.tar.gz "$$url"; \
	tar -xzf kubefwd.tar.gz -C "$(LOCALBIN)" kubefwd; \
	chmod +x "$(KUBEFWD)"; \
	rm kubefwd.tar.gz; \
	echo "✅ kubefwd installed at $(KUBEFWD)"

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
	@if [ ! -d "$(VENV)" ]; then \
		echo "❌ Python venv does not exist. Please create it with 'make virtualenv'."; \
		exit 1; \
	else \
		echo "✅ Python venv exists."; \
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

.PHONY: chainsaw-test-all
chainsaw-test-all: ## Run chainsaw tests against local kind cluster with ztoperator running in the cluster
	@bash -ec ' \
			for dir in test/chainsaw/authpolicy/*/ ; do \
				echo "Running test in $$dir"; \
				if ! $(MAKE) chainsaw-test-single dir=$$dir; then \
					echo "Test in $$dir failed."; \
					exit 1; \
				fi; \
			done; \
	' || (echo "Test(s) failed." && exit 1)

.PHONY: chainsaw-test-single
chainsaw-test-single: chainsaw-ensure-dir chainsaw install ensurelocal ensurerunningordeployed isingressready ## Run a specific chainsaw test. Example usage: make chainsaw-test-single dir=<CHAINSAW_TEST_DIR>
	"$(CHAINSAW)" test --kube-context $(KUBECONTEXT) --config test/chainsaw/config.yaml --test-dir $(dir) && \
    	echo "✅ Test succeeded" || (echo "❌ Test failed" && exit 1)

.PHONY: chainsaw-ensure-dir
chainsaw-ensure-dir: ## Ensure that the 'dir' variable is set when running 'make chainsaw-test-single' and that the dir exists
	$(if $(strip $(dir)),,$(error dir is not set. Usage: make chainsaw-test-single dir=<CHAINSAW_TEST_DIR>))
	@test -d $(dir) || { echo "❌ dir $(dir) is not a directory"; exit 1; }

##@ Custom targets

# Extracts the version number for a given dependency found in go.mod.
# Makes the test setup be in sync with what the operator itself uses.
extract-version = $(shell cat go.mod | grep $(1) | awk '{$$1=$$1};1' | cut -d' ' -f2 | sed 's/^v//')

ensuremockoauth2isreachable: kubefwd ## Ensure kubefwd is installed and running and that mock-oauth2 is reachable
	@KUBEFWD_PIDS=$$(pgrep -f "kubefwd( |$$)" 2>/dev/null); \
	if [ -z "$$KUBEFWD_PIDS" ]; then \
		echo -e "❌  mock-oauth2 ingress is not configured."; \
		echo -e "    Configure it with 'make mock-oauth2-ingress'"; \
		exit 1; \
	fi
	@echo "Verifying mock-oauth2 is reachable via kubefwd..."
	@HTTP_STATUS=$$(curl -s -o /dev/null -w "%{http_code}" http://mock-oauth2:8080/ztoperator/.well-known/openid-configuration); \
	if [ "$$HTTP_STATUS" = "200" ]; then \
		echo -e "✅  mock-oauth2 is reachable (HTTP 200)"; \
	else \
		echo -e "❌  mock-oauth2 returned HTTP $$HTTP_STATUS (expected 200)."; \
		echo -e "    Make sure mock-oauth2 is running and that forwarding is configured."; \
		echo -e "    This can be done with 'make mock-oauth2' and 'make mock-oauth2-ingress' respectively."; \
		exit 1; \
	fi
