# Keep SameSite=Lax for OAuth cookies and do not add a Strict option

## Status

Accepted

## Context

The Envoy OAuth2 filter generated for `autoLogin` sets all its cookies (`bearer_token`, `id_token`, `refresh_token`,
`oauth_hmac`, `oauth_expires`, `oauth_nonce`, `code_verifier`) with `SameSite=Lax`. We considered letting users opt in
to `SameSite=Strict` for the session cookies.

Lax already blocks cookies on cross-site POSTs and subresource requests, so Strict only additionally protects against
state-changing GET requests triggered by cross-site top-level navigation. In return, the Envoy filter validates the
session cookies as a group, so a cross-site navigation arriving without them is treated as unauthenticated. This causes
a redirect to the IdP, loss of cross-site POST bodies, no silent token refresh, and a possible full login prompt.
The flow cookies (`oauth_nonce`, `code_verifier`) must stay Lax, since the IdP callback is a cross-site navigation.
No team has requested Strict.

## Decision

Keep `SameSite=Lax` hardcoded for all OAuth cookies. We do not add a SameSite option to the `AuthPolicy` CRD.

## Consequences

- The CRD stays small, and no extra e2e matrix (browsers, IdPs, refresh) has to be maintained.
- Apps with state-changing GET endpoints are not protected by Strict and must fix this in the application.
- The decision is cheap to reverse. If a concrete compliance requirement appears, add a single enum, for example
  `autoLogin.cookieSameSite: Lax|Strict` (default `Lax`). It should apply only to the session cookie group
  (`bearer_token`, `id_token`, `refresh_token`, `oauth_hmac`, `oauth_expires`), leave the flow cookies Lax, document the
  deep-link and POST trade-offs, and have e2e coverage. That would supersede this ADR.

## References

- `pkg/resourcegenerators/envoyfilter/configpatch/oauth_sidecar_config_patch.go` (`cookie_configs`)
