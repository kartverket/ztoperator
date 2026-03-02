# Copilot Documentation Report for Ztoperator

This report describes what documentation is ideal for GitHub Copilot to work optimally with the Ztoperator repository, covering all relevant documentation types.

---

## 1. `.github/copilot-instructions.md` (Implemented)

**Status**: ✅ Created at `.github/copilot-instructions.md`

This is the most impactful documentation type for Copilot. GitHub Copilot reads this file automatically in VS Code and other supported IDEs, injecting it as context into every chat and inline completion request scoped to this repository.

The file covers:

- **Project overview**: What Ztoperator is, what problem it solves, and why it exists.
- **Architecture**: How the operator maps an `AuthPolicy` to Istio resources (`RequestAuthentication`, `AuthorizationPolicy`, `EnvoyFilter`, `Secret`) and in what order Envoy filters execute.
- **Technology stack**: Go version, controller-runtime/kubebuilder, Istio 1.26–1.28, testing frameworks (Ginkgo, Gomega, envtest, Chainsaw), linting (golangci-lint), local cluster tooling (kind, Flox), and release tooling (goreleaser).
- **Repository structure**: Annotated directory tree so Copilot knows where to find controllers, resource generators, CRD types, tests, scripts, and config.
- **Key domain concepts**: All `AuthPolicy` spec fields and their semantics, status phases, and the step-by-step reconciliation flow.
- **Development conventions**: Naming conventions for generated resources, Go package conventions, owner-reference patterns, status condition conventions, error-handling patterns, logging conventions, and resource generator patterns.
- **Common `make` targets**: Quick reference so Copilot can suggest the correct command for any task.
- **Supported identity providers**: Special validation rules for ID-Porten and Ansattporten.
- **Testing approach**: Unit/integration tests with Ginkgo + envtest, end-to-end tests with Chainsaw, and the mock OAuth2 server setup.
- **CI/CD**: Overview of all GitHub Actions workflows.
- **External references**: URLs to Istio, Envoy, Kubebuilder, RFC 8707, and operator-sdk documentation.

---

## 2. Additional Markdown Files

Beyond `copilot-instructions.md`, the following markdown files already exist or are recommended to keep up to date:

| File | Purpose | Status |
|------|---------|--------|
| `README.md` | Public-facing overview, example `AuthPolicy`, architecture diagram, Istio compatibility | ✅ Exists |
| `CONTRIBUTING.md` | Local development setup (Flox, kind, make targets), test instructions | ✅ Exists |

### Recommendations for Additional Markdown Files

The following markdown documents would further improve Copilot's ability to assist with domain-specific tasks:

- **`docs/architecture.md`**: A detailed written description of the reconciliation pipeline, how Envoy filters interact, and the OIDC flow. This supplements the architecture diagrams in `README.md` with machine-readable text.
- **`docs/api-reference.md`**: A full reference for all `AuthPolicy` fields with examples, constraints, and defaults. Useful when Copilot generates or validates `AuthPolicy` manifests.
- **`docs/adr/`** (Architecture Decision Records): Short records of key design decisions (e.g., why Lua scripts are used in EnvoyFilters, why controller-runtime was chosen). ADRs help Copilot understand *why* code is structured as it is, reducing incorrect refactoring suggestions.

---

## 3. MCP Servers (Model Context Protocol)

MCP servers allow GitHub Copilot to query live, structured data sources at suggestion time. The following MCP servers would benefit Copilot when working in this repository:

### 3.1 Kubernetes API Schema MCP Server

- **Purpose**: Provides Copilot with live Kubernetes API schemas so it can generate accurate YAML manifests and Go struct definitions that match the installed API versions.
- **Value for this repo**: The operator generates `RequestAuthentication`, `AuthorizationPolicy`, and `EnvoyFilter` resources. Accurate schema awareness prevents suggestions that use deprecated or incorrect fields.
- **Reference**: [kubernetes-mcp-server](https://github.com/manusa/kubernetes-mcp-server) provides cluster-aware schema and resource access.

### 3.2 GitHub MCP Server

- **Purpose**: Gives Copilot access to issues, pull requests, and repository metadata directly from GitHub.
- **Value for this repo**: Helps Copilot understand open issues, recent changes in PRs, and code review context when suggesting fixes or new features.
- **Reference**: [github-mcp-server](https://github.com/github/github-mcp-server).

### 3.3 Filesystem / Documentation MCP Server

- **Purpose**: Allows Copilot to read the repository's own documentation files (this report, ADRs, API reference) as structured context.
- **Value for this repo**: Ensures documentation is always in scope even in large repositories where file context is limited.
- **Reference**: [filesystem MCP server](https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem).

### Configuring MCP Servers for the Repository

MCP servers can be configured at the user or organization level in GitHub Copilot settings, or in VS Code via `.vscode/mcp.json`. An example VS Code configuration:

```json
{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/"
    },
    "kubernetes": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@manusa/kubernetes-mcp-server"]
    }
  }
}
```

---

## 4. Allowlisted Domains

GitHub Copilot can be configured to fetch real-time documentation from specific domains when generating suggestions. The following domains are directly relevant to this repository and should be allowlisted in the organization's GitHub Copilot policy settings:

| Domain | Reason |
|--------|--------|
| `istio.io` | Istio API reference for `RequestAuthentication`, `AuthorizationPolicy`, and `EnvoyFilter` |
| `www.envoyproxy.io` | Envoy proxy OAuth2 filter, JWT filter, and RBAC filter documentation |
| `book.kubebuilder.io` | Kubebuilder patterns: controllers, webhooks, CRD markers, envtest |
| `sdk.operatorframework.io` | Operator SDK patterns and metrics |
| `pkg.go.dev` | Go standard library and dependency documentation |
| `kubernetes.io` | Kubernetes API reference (Pods, Secrets, ConfigMaps, RBAC) |
| `datatracker.ietf.org` | RFC 8707 (Resource Indicators for OAuth 2.0) and OAuth 2.0 RFCs |
| `openid.net` | OpenID Connect Core specification |
| `onsi.github.io` | Ginkgo and Gomega test framework documentation |
| `kyverno.github.io` | Chainsaw end-to-end testing documentation |

---

## 5. `.github/CODEOWNERS`

**Status**: ✅ Exists (covers `.security/`)

Copilot is aware of `CODEOWNERS` and uses it to understand code ownership. Extending it to cover more paths (e.g., `api/`, `pkg/`, `internal/`) would help Copilot assign appropriate reviewers in PR suggestions and improve context about who to ask for reviews.

---

## 6. Code Comments and Godoc

Well-written Go doc comments are among the highest-value documentation for Copilot because they are directly adjacent to the code being suggested. Recommendations:

- **All exported types and functions** in `api/v1alpha1/` and `pkg/` should have Godoc comments describing purpose, constraints, and usage.
- **Kubebuilder markers** (already heavily used in `authpolicy_types.go`) serve as machine-readable documentation for CRD field constraints and are correctly maintained.
- **Complex internal functions** in `pkg/resourcegenerators/` and `pkg/reconciliation/` benefit from inline comments that explain *why* a particular EnvoyFilter patch or Lua script is structured the way it is.

---

## 7. `catalog-info.yaml` (Backstage)

**Status**: ✅ Exists

The `catalog-info.yaml` registers the service in Backstage (internal developer portal). Enriching it with more metadata (e.g., `dependsOn`, `system`, links to runbooks) makes the Backstage catalog a useful context source if a Backstage MCP server is configured.

---

## 8. OpenAPI / CRD Schema

**Status**: Generated at `config/crd/bases/ztoperator.kartverket.no_authpolicies.yaml`

The CRD YAML contains an embedded OpenAPI v3 schema generated from kubebuilder markers. This schema is already comprehensive and serves as machine-readable documentation for all `AuthPolicy` fields. It is automatically consumed by `kubectl explain` and IDE YAML validation plugins, which in turn help Copilot generate correct manifests.

No additional action is needed here beyond keeping the `make generate` step part of the standard workflow (already the case).

---

## Summary

| Documentation Type | Status | Impact |
|-------------------|--------|--------|
| `.github/copilot-instructions.md` | ✅ Created | Very High — directly consumed by Copilot in all interactions |
| `README.md` | ✅ Exists | High — project overview and example |
| `CONTRIBUTING.md` | ✅ Exists | High — development workflow |
| `docs/architecture.md` | 💡 Recommended | Medium — deeper architectural context |
| `docs/api-reference.md` | 💡 Recommended | Medium — field-level detail for AuthPolicy |
| `docs/adr/` | 💡 Recommended | Medium — design rationale |
| MCP Servers (Kubernetes, GitHub) | 💡 Recommended | High — live schema and repo context |
| Allowlisted domains | 💡 Recommended | Medium — real-time external docs |
| Code comments / Godoc | 💡 Recommended | High — inline context for completions |
| CRD schema (OpenAPI) | ✅ Generated | High — machine-readable field spec |
| `catalog-info.yaml` | ✅ Exists | Low — enrichment possible |
