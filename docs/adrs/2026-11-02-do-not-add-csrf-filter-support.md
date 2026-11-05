# Do not add CSRF filter support for now

## Status

Accepted

## Context

We considered adding support for Envoy's `envoy.filters.http.csrf` filter, generated through the existing
`EnvoyFilter`. It rejects state-changing requests (POST, PUT, PATCH, DELETE) whose `Origin` (or `Referer`) does not match
the destination host, and it has a shadow mode that only counts violations. It would give CSRF protection that is
independent of cookie `SameSite` settings (see `2026-11-02-keep-samesite-lax-for-oauth-cookies.md`).

CSRF is only relevant when authentication is cookie-based, i.e. `autoLogin.enabled`. Risks include rejecting legitimate
traffic (SPAs on a different origin, clients that send no `Origin`/`Referer`, host mismatches behind the ingress
gateway), extra filter-ordering concerns relative to the login, `jwt-auth` and `rbac` filters, and further coupling to the
alpha `EnvoyFilter` API. No team has requested it, and `SameSite=Lax` already blocks cross-site POSTs.

## Decision

We do not add CSRF filter support now. In particular, we do not add a separate `CsrfFilter` CRD, since `AuthPolicy`
remains the only user-facing resource.

## Consequences

- The CRD and the generated `EnvoyFilter` stay small, and no extra e2e matrix has to be maintained.
- Apps rely on `SameSite=Lax` and their own CSRF protection for cookie-authenticated, state-changing endpoints.
- The decision is cheap to reverse. If a concrete need appears, add a field under `autoLogin`, for example
  `autoLogin.csrf: {mode: Off|Shadow|Enforce, additionalOrigins: []}`, defaulting to `Off`. Start with `Shadow`, and add
  e2e tests for same-origin pass, cross-origin block and missing `Origin`. That would be a new ADR.

## References

- `pkg/resourcegenerators/envoyfilter/auto_login_envoy_filter.go`
- Envoy CSRF filter: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/csrf_filter
