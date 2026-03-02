# GitHub Copilot Instructions for Ztoperator

## Project Overview

**Ztoperator** is a Kubernetes Operator written in Go that enforces Zero Trust security for workloads. It integrates with **Istio** (service mesh) and **OAuth 2.0 / OpenID Connect (OIDC)** to provide authentication and authorization for Kubernetes workloads without requiring application-level changes.

The operator introduces a single Custom Resource Definition (CRD): **`AuthPolicy`** (API group: `ztoperator.kartverket.no/v1alpha1`). An `AuthPolicy` defines how incoming traffic to a set of pods should be authenticated and authorized.

## Architecture and How It Works

When an `AuthPolicy` is created or updated, the operator reconciles the following Kubernetes/Istio resources:

1. **`RequestAuthentication`** (Istio) — Validates incoming JWT tokens against the configured OIDC issuer (fetched from `wellKnownURI`).
2. **`AuthorizationPolicy`** (Istio) — Enforces access control rules based on JWT claims.
3. **`EnvoyFilter`** (Istio) — Implements custom logic via Lua scripts and the Envoy OAuth2 filter:
   - **`login` filter**: Handles the OAuth 2.0 Authorization Code Flow (auto-login), injects an `Authorization` header with a bearer token on success.
   - **`jwt-auth` filter**: Validates JWT tokens.
   - **`rbac` filter**: Enforces RBAC rules based on JWT claims.
4. **`Secret`** — Named `<authpolicy-name>-envoy-secret`, holds OAuth credentials for the Envoy OAuth2 filter. Must be mounted into the `istio-proxy` sidecar.

Filter execution order in Istio sidecar: `login` → `jwt-auth` → `rbac`.

## Technology Stack

- **Language**: Go (≥ 1.25.7)
- **Operator Framework**: [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) / [kubebuilder](https://book.kubebuilder.io/)
- **Service Mesh**: Istio 1.26–1.28 (uses `istio.io/client-go` and `istio.io/api`)
- **Testing**: [Ginkgo v2](https://onsi.github.io/ginkgo/) + [Gomega](https://onsi.github.io/gomega/) for unit/integration tests; [envtest](https://book.kubebuilder.io/reference/envtest) for controller tests; [Chainsaw](https://kyverno.github.io/chainsaw/) for end-to-end tests
- **Linting**: [golangci-lint](https://golangci-lint.run/) with config in `.golangci.yml`
- **Local dev cluster**: [kind](https://kind.sigs.k8s.io/) via [Flox](https://flox.dev/) environment
- **Dependency management**: Go modules (`go.mod` / `go.sum`)
- **Release**: [goreleaser](https://goreleaser.com/)
- **Container registry**: GitHub Container Registry (`ghcr.io`)

## Repository Structure

```
ztoperator/
├── api/v1alpha1/              # CRD types (AuthPolicy and related types)
│   ├── authpolicy_types.go    # All CRD type definitions
│   └── groupversion_info.go   # API group/version registration
├── cmd/main.go                # Operator entrypoint
├── config/                    # Kubebuilder/kustomize manifests
│   ├── crd/                   # Generated CRD manifests
│   ├── manager/               # Manager deployment manifests
│   └── rbac/                  # RBAC role manifests
├── internal/
│   ├── controller/            # Kubernetes controller (reconcile loop)
│   ├── eventhandler/          # Pod event handler for watch triggers
│   ├── reconciler/            # High-level reconciliation actions (apply/delete)
│   ├── resolver/              # Resolves dynamic values (audiences from ConfigMap/Secret)
│   ├── state/                 # Builds reconciliation state from AuthPolicy
│   └── statusmanager/         # Updates AuthPolicy status conditions
├── pkg/
│   ├── config/                # Operator configuration (env vars via envconfig)
│   ├── helperfunctions/       # Utility functions
│   ├── log/                   # Logging setup (zap)
│   ├── luascript/             # Lua scripts used in EnvoyFilters
│   ├── metrics/               # Prometheus metrics (ztoperator_authpolicy_info)
│   ├── reconciliation/        # Core reconciliation logic
│   ├── resourcegenerators/    # Generates Istio/k8s resource specs
│   │   ├── authorizationpolicy/  # AuthorizationPolicy generators
│   │   ├── envoyfilter/          # EnvoyFilter generators (auto-login, jwt-auth, rbac)
│   │   ├── requestauthentication/ # RequestAuthentication generators
│   │   └── secret/               # Secret generator for OAuth credentials
│   ├── rest/                  # HTTP client for fetching OIDC discovery documents
│   └── validation/            # AuthPolicy validation logic
├── test/chainsaw/             # End-to-end Chainsaw test scenarios
├── test/resources/            # Shared test resources
├── examples/                  # Example AuthPolicy + Application manifests
├── scripts/                   # Setup scripts for local cluster
├── Makefile                   # All development commands
├── Dockerfile                 # Container image build
├── CONTRIBUTING.md            # Development setup guide
└── README.md                  # Project documentation
```

## Key Domain Concepts

### AuthPolicy CRD Fields

- **`selector.matchLabels`**: Selects which pods this policy applies to.
- **`enabled`**: When `false`, the operator skips generating auth resources.
- **`wellKnownURI`**: URL to the OIDC provider's discovery document (e.g., `https://example.com/.well-known/openid-configuration`). The operator fetches this at reconcile time to obtain `issuer` and `jwks_uri`.
- **`allowedAudiences`**: JWT `aud` claim values that are accepted. Supports static values or references to ConfigMap/Secret keys (`valueFrom`).
- **`acceptedResources`**: RFC 8707 resource indicators added to the authorize request; the resulting `aud` values must be present in the JWT.
- **`autoLogin`**: Configures the OAuth 2.0 Authorization Code Flow (redirect-based login). Requires `oAuthCredentials`.
- **`oAuthCredentials`**: Reference to a Kubernetes Secret containing `clientID` and `clientSecret`.
- **`authRules`**: Paths/methods that require authentication, with optional JWT claim conditions (`when`). `denyRedirect: true` disables auto-login redirect for that rule.
- **`ignoreAuthRules`**: Paths/methods that are publicly accessible (no auth required).
- **`baselineAuth`**: JWT claim conditions applied to all authenticated paths.
- **`outputClaimToHeaders`**: Copy JWT claims to HTTP headers.
- **`forwardJwt`**: Whether to pass the original token upstream (default: `true`).

### AuthPolicy Status Phases

- **`Pending`**: Reconciliation in progress.
- **`Ready`**: All resources successfully reconciled.
- **`Failed`**: Reconciliation failed (e.g., network error fetching well-known URI).
- **`Invalid`**: AuthPolicy spec is invalid (e.g., `wellKnownURI` is unreachable or response is malformed).

### Reconciliation Flow

1. Fetch `AuthPolicy`.
2. If `enabled: false`, delete all owned resources and set status to `Ready`.
3. Resolve dynamic values (audiences from ConfigMap/Secret refs).
4. Fetch OIDC discovery document from `wellKnownURI`.
5. Build reconciliation state.
6. Generate and apply/update all owned resources (`RequestAuthentication`, `AuthorizationPolicy`, `EnvoyFilter`s, `Secret`).
7. Update `AuthPolicy` status.

## Development Conventions

### Naming

- Generated resource names follow the pattern: `<authpolicy-name>-<resource-suffix>` (e.g., `auth-policy-envoy-secret`).
- Go package names match directory names (lowercase, no underscores).
- Test files use `_test.go` suffix; test suites use Ginkgo `Describe`/`It` structure.

### Patterns

- **Owner references**: All generated resources have the `AuthPolicy` as owner; deletion cascades automatically.
- **Status conditions**: Use `metav1.Condition` with types like `"Reconciled"`, following Kubernetes conventions.
- **Error handling**: Use `k8s.io/apimachinery/pkg/util/errors` for aggregating multiple errors.
- **Logging**: Use the `pkg/log` wrapper around `go.uber.org/zap`.
- **Resource generation**: Each resource type has its own package under `pkg/resourcegenerators/`.

### Common Make Targets

```bash
make run-local        # Run operator locally against kind cluster
make deploy           # Build and deploy to kind cluster
make test             # Run Ginkgo unit/integration tests
make chainsaw-test-host   # Run end-to-end tests (operator on host)
make chainsaw-test-remote # Run end-to-end tests (operator in cluster)
make generate         # Regenerate CRD manifests and DeepCopy methods
make lint             # Run golangci-lint
make fmt              # Run go fmt
make vet              # Run go vet
make local            # Set up full local dev environment
make virtualenv       # Set up Python virtualenv for e2e tests
make expose-ingress   # Expose Istio ingress for local testing
```

## Supported Identity Providers

The `wellKnownURI` field accepts any OIDC-compliant provider. Known providers with special validation rules:

- **ID-Porten** (`idporten.no`): Requires `acceptedResources` to be non-empty.
- **Ansattporten** (`ansattporten.no`): Requires `acceptedResources` to be non-empty.

## Testing Approach

- **Unit/Integration tests**: Ginkgo v2 + Gomega + envtest. Run with `make test`. Coverage tracked and must not regress.
- **End-to-end tests**: Chainsaw scenarios in `test/chainsaw/`. Uses a real kind cluster with Istio, Cert-Manager, Skiperator, and a mock OAuth2 server.
- **Mock OAuth2 server**: `mock-oauth2-server` (deployed in `auth` namespace) simulates an OIDC provider for local testing.

## CI/CD

- **Build and push**: `.github/workflows/build-and-deploy.yaml` — builds and pushes Docker image to `ghcr.io/kartverket/ztoperator`.
- **Tests**: `.github/workflows/test-and-compare-code-coverage.yml` — runs Ginkgo tests and fails if coverage regresses.
- **Chainsaw**: `.github/workflows/test-chainsaw.yml` — runs full e2e test suite.
- **Lint**: `.github/workflows/golangci-lint.yml`.
- **Release**: `.github/workflows/release-version.yaml` — goreleaser on tags matching `v*`.
- **Security scan**: Pharos security scan on PRs.
- **Dependency review**: `actions/dependency-review-action` on PRs to `main`.

## External References

- Istio `RequestAuthentication`: https://istio.io/latest/docs/reference/config/security/request_authentication/
- Istio `AuthorizationPolicy`: https://istio.io/latest/docs/reference/config/security/authorization-policy/
- Envoy OAuth2 Filter: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/oauth2_filter
- Kubebuilder book: https://book.kubebuilder.io/
- RFC 8707 (Resource Indicators): https://datatracker.ietf.org/doc/html/rfc8707
- operator-sdk metrics: https://sdk.operatorframework.io/docs/building-operators/golang/advanced-topics/metrics/
