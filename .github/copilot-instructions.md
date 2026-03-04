# Copilot Instructions for Ztoperator

## Project Overview

Ztoperator is a Kubernetes Operator built with **Kubebuilder v4** that enforces Zero Trust security for workloads by integrating with **Istio** and **OAuth 2.0**. It is part of Kartverket's **SKIP platform** (`skip.kartverket.no`) and works alongside **Skiperator** (`github.com/kartverket/skiperator`), which manages the lifecycle of workloads.

The operator is owned by team **tilgangsstyring** (access management) and is registered in Backstage (`catalog-info.yaml`).

## Domain Model

### Custom Resource: `AuthPolicy`

The single CRD `AuthPolicy` (`ztoperator.kartverket.no/v1alpha1`) is the only user-facing resource. It provides a high-level abstraction for configuring authentication and authorization using OAuth 2.0 / OIDC.

An `AuthPolicy` targets workloads via `spec.selector.matchLabels` and generates the following child resources (owned via `ownerReference`):

| Generated Resource | Naming Convention | Purpose |
|---|---|---|
| `RequestAuthentication` | `<authpolicy-name>` | JWT validation rules (issuer, JWKS, audiences, claim-to-header mapping) |
| `AuthorizationPolicy` (DENY) | `<authpolicy-name>-deny-auth-rules` | Denies requests that fail claim-based conditions (DENY takes precedence over ALLOW) |
| `AuthorizationPolicy` (ALLOW/ignore) | `<authpolicy-name>-ignore-auth` | Allows unauthenticated requests matching `ignoreAuthRules` |
| `AuthorizationPolicy` (ALLOW/require) | `<authpolicy-name>-require-auth` | Allows authenticated requests matching `authRules` / `baselineAuth` |
| `EnvoyFilter` | `<authpolicy-name>-login` | OAuth2 Authorization Code Flow via Envoy's OAuth2 + Lua filters (only when `autoLogin.enabled: true`) |
| `Secret` | `<authpolicy-name>-envoy-secret` | HMAC + OAuth client secret for Envoy (only when `autoLogin.enabled: true`) |

### Status Phases

`AuthPolicy.status.phase` transitions through: `Pending` → `Ready` | `Failed` | `Invalid`.

- **Invalid**: CRD validation (paths, pod annotations) fails; results in default-deny on all paths.
- **Failed**: Reconciliation of child resources errors out.
- **Ready**: All child resources reconciled successfully.

## Architecture & Code Structure

### Reconciliation Flow

```
Controller.Reconcile()
  ├─ Fetch AuthPolicy
  ├─ resolveAuthPolicy()         ← Resolvers (internal/resolver/)
  │   ├─ ResolveOAuthCredentials   (reads Secret for clientID/clientSecret)
  │   ├─ ResolveDiscoveryDocument  (fetches .well-known OIDC endpoint → issuer, jwks, token, auth, endsession URIs)
  │   ├─ ResolveAutoLoginConfig    (builds Lua script config, sane defaults for redirect/logout paths)
  │   └─ ResolveAudiences          (static values or from ConfigMap/Secret references)
  ├─ validateAuthPolicy()        ← Validators (pkg/validation/)
  │   ├─ Path validation           (RFC 3986 pchar, template syntax {*}, {**})
  │   └─ Pod annotation validation (sidecar.istio.io/userVolume + userVolumeMount must reference the envoy secret)
  ├─ ReconcileActions()           ← Builds list of 6 reconcile actions (internal/reconciler/actions.go)
  │   ├─ Secret
  │   ├─ EnvoyFilter
  │   ├─ RequestAuthentication
  │   ├─ AuthorizationPolicy (deny)
  │   ├─ AuthorizationPolicy (ignore)
  │   └─ AuthorizationPolicy (require)
  ├─ doReconcile()                ← Iterates and executes all actions
  └─ UpdateAuthPolicyStatus()     ← Status manager (internal/statusmanager/)
```

### Key Packages

| Package | Responsibility |
|---|---|
| `api/v1alpha1/` | CRD type definitions with kubebuilder markers; `AuthPolicy`, `AuthPolicySpec`, status, conditions |
| `cmd/main.go` | Entrypoint; scheme registration (Istio + K8s + ztoperator), manager setup |
| `internal/controller/` | Reconciler loop; resolves, validates, reconciles, updates status |
| `internal/reconciler/` | Generic adapter pattern for reconciling any `client.Object`. `actions.go` defines the 6 reconcile actions |
| `internal/resolver/` | Resolvers: audience, OAuth credentials, discovery document, auto-login config |
| `internal/state/` | `Scope` struct — the resolved state bag passed through reconciliation (AuthPolicy + resolved values + descendants) |
| `internal/statusmanager/` | Condition building, phase/readiness determination, status updates |
| `internal/eventhandler/pod/` | Watches Pods → enqueues all AuthPolicies in the same namespace for re-reconciliation |
| `pkg/resourcegenerators/` | Desired-state generators for each child resource type |
| `pkg/resourcegenerators/envoyfilter/` | EnvoyFilter generation (OAuth2 filter + Lua filter config patches) |
| `pkg/resourcegenerators/authorizationpolicy/` | Split into `deny/`, `ignore/`, `require/` sub-packages |
| `pkg/resourcegenerators/requestauthentication/` | RequestAuthentication generation |
| `pkg/resourcegenerators/secret/` | Envoy Secret generation (HMAC + token secret) |
| `pkg/luascript/` | Lua script templating (`ztoperator.lua` embedded via `//go:embed`); handles login/logout/redirect/deny-redirect logic |
| `pkg/validation/` | Path validation (RFC 3986, template patterns `{*}` / `{**}`), pod annotation validation; `path_classifier.go` distinguishes exact/prefix/template paths, `path_transformation.go` normalises them before matching |
| `pkg/reconciliation/` | `ReconcileAction` interface and generic `ReconcileFunc` / `ReconcileFuncAdapter` types |
| `pkg/metrics/` | Prometheus gauge `ztoperator_authpolicy_info` with labels: name, namespace, state, owner, issuer, enabled, auto_login_enabled, protected_pod |
| `pkg/rest/` | OIDC discovery document HTTP client (uses resty); `DiscoveryDocumentResolver` interface in `client.go` is the main test seam injected into the controller; pre-seeded static map of known providers in `dto.go` |
| `pkg/config/` | Env-based config via `envconfig` (currently just `ZTOPERATOR_GIT_REF`) |
| `pkg/log/` | Thin wrapper around `logr.Logger` with Debug/Info/Warning/Error levels |
| `pkg/helperfunctions/` | Shared utilities (ObjectMeta builder, URL parsing, pod lookup, generic `Ptr()`, etc.) |

### Adapter Pattern for Reconciliation

The reconciler uses a **generic adapter pattern** with Go generics:

```go
AuthPolicyAdapter[T client.Object]  →  ReconcileFuncAdapter[T]  →  ReconcileFunc[T]
```

Each `ReconcileFunc[T]` specifies:
- `DesiredResource`: the desired state (or nil to trigger deletion)
- `ShouldUpdate(current, desired T) bool`: comparison function
- `UpdateFields(current, desired T)`: field-level update function

The adapter handles the full lifecycle: **create if not found**, **update if changed**, **delete if desired is nil**.

### Scope (State Bag)

`state.Scope` is the central state object created during resolution and threaded through the entire reconciliation:

```go
type Scope struct {
    AuthPolicy             AuthPolicy
    Audiences              []string
    AutoLoginConfig        AutoLoginConfig
    OAuthCredentials       OAuthCredentials
    IdentityProviderUris   IdentityProviderUris
    Descendants            []Descendant[client.Object]  // tracks reconciled child resources + their status
    InvalidConfig          bool
    ValidationErrorMessage *string
}
```

## Coding Conventions

### Go Style

- **Go version**: 1.25.7+
- **Linter**: golangci-lint v2 with config in `.golangci.yml`. Key linters: `revive`, `gocyclo`, `govet`, `staticcheck`, `errcheck`, `ginkgolinter`.
- **Formatting**: `gofmt` + `goimports` enforced.
- **Import shadowing**: Disallowed (`revive` rule `import-shadowing`).
- **Comment spacing**: Enforced (`revive` rule `comment-spacings`).
- **Line length**: `lll` linter enabled (relaxed for `api/` and `internal/` paths).
- Use structured key-value pairs for log messages: `rLog.Info("msg", "key1", value, "key2", value, ...)`. The `Logger` type in `pkg/log/` accepts `keysAndValues ...interface{}` for all levels.
- Use `helperfunctions.Ptr(value)` to create pointers from literal values.
- Use `helperfunctions.BuildObjectMeta(name, namespace)` to create ObjectMeta for child resources.

### Naming Conventions

- Child resource names derive from AuthPolicy name: `<authpolicy-name>`, `<authpolicy-name>-login`, `<authpolicy-name>-deny-auth-rules`, `<authpolicy-name>-ignore-auth`, `<authpolicy-name>-require-auth`, `<authpolicy-name>-envoy-secret`.
- Package naming follows Go convention (single lowercase word).
- Resource generator packages export a single `GetDesired(scope, objectMeta)` function.

### CRD / API Changes

When modifying `api/v1alpha1/authpolicy_types.go`:
1. Add appropriate `+kubebuilder:validation:*` markers for validation.
2. Run `make generate` to regenerate CRD manifests (`config/crd/bases/`) and `zz_generated.deepcopy.go`.
3. Update `examples/example.yaml` if the change affects user-facing fields.
4. Add Chainsaw e2e tests for new behavior.

### Kubebuilder Markers

RBAC permissions are declared via `+kubebuilder:rbac` comments on the `Reconcile` method in `authpolicy_controller.go`. When adding new resource types to watch/manage, update both the RBAC markers and `SetupWithManager()`.

## Testing

### Unit/Integration Tests (envtest + Ginkgo)

- Framework: envtest + Ginkgo/Gomega (BDD-style).
- Suite setup in `internal/controller/suite_test.go`: bootstraps envtest with CRDs from `config/crd/bases/`, registers Istio + ztoperator schemes.
- Run: `make test`
- Tests use a real API server (envtest) but no real cluster.

### End-to-End Tests (Chainsaw + Hurl)

- Framework: Kyverno Chainsaw v0.2.14 for test orchestration, Hurl for HTTP assertions.
- Config: `test/chainsaw/config.yaml` (parallel: 40, timeouts configured).
- Test location: `test/chainsaw/authpolicy/<test-name>/`
- Each test folder contains:
  - `chainsaw-test.yaml` — test steps (create resources, wait, run hurl)
  - `authpolicy.yaml` — the AuthPolicy under test
  - `tests.hurl` — HTTP request/response assertions
- Shared resources: `test/resources/` (Skiperator Application, ingress configs).
- Mock OAuth2 server provides tokens for test assertions.
- Run all: `make chainsaw-test-host` (operator on host) or `make chainsaw-test-remote` (operator in cluster).
- Run single: `make chainsaw-test-host-single dir=test/chainsaw/authpolicy/<test-name>/`

### Test Naming

Chainsaw test directories use descriptive snake_case names that describe the scenario being tested (e.g., `auto_login_sane_defaults`, `baseline_auth_with_multiple_claims_same_key`, `pod_annotation_validation`).

## Technology Stack & Compatibility

### Istio

- **Compatible versions**: Istio 1.26 – 1.28 (any patch).
- Istio API types used: `RequestAuthentication`, `AuthorizationPolicy` (from `security.istio.io/v1`), `EnvoyFilter` (from `networking.istio.io/v1alpha3`).
- The Istio client-go version in `go.mod` determines the Istio version used locally (`istio.io/client-go`).

### Envoy Filters (Execution Order)

The operator generates **one** `EnvoyFilter` resource (`<authpolicy-name>-login`, only when `autoLogin.enabled: true`). It inserts config patches into Envoy's built-in filter chain in strict order:

1. **`login` (Lua + OAuth2)** ← _generated by ztoperator_: Handles auto-login. If login succeeds, injects `Authorization: Bearer <token>` header.
2. **`jwt-auth` (JWT Authentication)** ← _Istio built-in_: Validates JWT token.
3. **`rbac` (RBAC)** ← _Istio built-in_: Enforces authorization rules based on validated JWT claims.

The Lua script (`pkg/luascript/ztoperator.lua`) is embedded at compile time via `//go:embed` and handles:
- OAuth2 redirect detection and bypass
- Logout (RP-initiated logout with `end_session_endpoint`)
- Deny-redirect behavior for API endpoints
- Cookie-based session management

### OAuth 2.0 / OIDC Standards

- **OIDC Discovery**: `wellKnownURI` fetches the `.well-known/openid-configuration` document.
- **RFC 8707**: `acceptedResources` implements Resource Indicators for audience-restricted access tokens.
- **RP-Initiated Logout**: `autoLogin.logoutPath` triggers redirect to the IdP's `end_session_endpoint`.
- **Known Norwegian IdPs**: ID-porten, Ansattporten (hardcoded well-known URIs in CRD CEL validation to enforce `acceptedResources`).
- **Pre-seeded Discovery Documents**: `pkg/rest/dto.go` contains a hardcoded static map of well-known URIs to discovery documents for mock-oauth2 (entraid, smapi, maskinporten), Microsoft Entra ID, ID-porten, and Maskinporten. This avoids live HTTP lookups for known providers.

### Skiperator Integration

Ztoperator works alongside Skiperator (`skiperator.kartverket.no/v1alpha1`):
- Skiperator's `Application` CRD manages pod lifecycle (deployments, services, ingress).
- AuthPolicy targets Skiperator-managed pods via label selectors.
- The `examples/example.yaml` shows both an `Application` and an `AuthPolicy` working together.
- Skiperator is installed in the local dev environment via `make skiperator` / `scripts/install-skiperator.sh`.

### Pod Annotation Requirements

When `autoLogin` is enabled, pods must mount the generated envoy secret into the istio-proxy sidecar via annotations:
- `sidecar.istio.io/userVolume`: JSON array with secret volume referencing `<authpolicy-name>-envoy-secret`
- `sidecar.istio.io/userVolumeMount`: JSON array mounting at `/etc/istio/config`

The operator validates these annotations and sets the AuthPolicy to `Invalid` phase if they are missing or malformed.

## Local Development

### Environment Setup

- **Flox**: Development environment manager (`.flox/env/manifest.toml`). `flox activate` sets up everything.
- **Kind**: Local Kubernetes cluster (`kind-ztoperator` context).
- Components installed locally: Istio, cert-manager, Skiperator, mock-oauth2-server.
- Env vars: `.env` file (currently just `ZTOPERATOR_GIT_REF`).
- IDE run configs: `.run/Ztoperator.run.xml`, `.run/Setup.run.xml` (JetBrains GoLand/IntelliJ).

### Key Make Targets

| Target | What it does |
|---|---|
| `make local` | Full local environment setup (cluster + all dependencies) |
| `make run-local` | Run operator from host machine |
| `make deploy` | Build image, deploy operator to kind cluster |
| `make test` | Run envtest/Ginkgo unit+integration tests |
| `make chainsaw-test-host` | Run all Chainsaw e2e tests (operator on host) |
| `make generate` | Regenerate CRDs, RBAC, DeepCopy code |
| `make lint` | Run golangci-lint |
| `make clean` | Delete kind cluster |

## CI/CD

- **Build & Deploy**: `.github/workflows/build-and-deploy.yaml` — builds container image, pushes to `ghcr.io`.
- **Tests**: `.github/workflows/test-and-compare-code-coverage.yml`, `.github/workflows/test-chainsaw.yml`.
- **Lint**: `.github/workflows/golangci-lint.yml`.
- **Releases**: `.github/workflows/release-version.yaml` with GoReleaser (`.goreleaser.yaml`).
- **Dependency updates**: Dependabot for `gomod` and `github-actions` (weekly on Monday 08:00 Europe/Oslo).

## Important Constraints

1. **Never manually edit** `config/crd/bases/` or `zz_generated.deepcopy.go` — these are generated by `make generate`.
2. **EnvoyFilter is alpha API** (`networking.istio.io/v1alpha3`) — it may change across Istio versions.
3. **DENY AuthorizationPolicies take precedence** over ALLOW — ordering matters in Istio's policy evaluation.
4. **The Lua script is embedded at compile time** — changes to `pkg/luascript/ztoperator.lua` require recompilation.
5. **Pod watch triggers full namespace reconciliation** — every Pod change in a namespace re-reconciles all AuthPolicies in that namespace (see `internal/eventhandler/pod/`).
6. **CEL validation rules** on the CRD enforce constraints at admission time (e.g., `acceptedResources` required for ID-porten/Ansattporten).

## Keeping Documentation Up-to-Date
- Update this document when making significant changes to architecture, code structure, conventions etc.
- Ensure that specific references to packages, patterns, versions etc. is updated as the code evolves.