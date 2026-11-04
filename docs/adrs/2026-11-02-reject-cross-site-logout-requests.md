# Reject cross-site requests to the logout path in the Lua filter

## Status

Accepted

## Context

The Envoy OAuth2 filter signs the user out (clears the session cookies and redirects to the IdP's `end_session_endpoint`)
on any request whose path equals `signout_path`, regardless of HTTP method and cookies. A third-party site can therefore
log users out with a link or `<meta http-equiv="refresh">` (logout CSRF). `SameSite=Lax` does not help, since it permits
cookies on top-level GET navigations, and `SameSite=Strict` does not help either, since the browser still applies the
`Set-Cookie` headers that clear the cookies. Envoy's CSRF filter only evaluates unsafe methods, so it ignores GET.
The impact is low (forced logout, possibly also of the IdP session), but the fix is cheap.
See `2026-11-02-keep-samesite-lax-for-oauth-cookies.md` and `2026-11-02-do-not-add-csrf-filter-support.md`.

## Decision

The generated Lua filter, which runs before the OAuth2 filter, rejects cross-site requests to the logout path with 403.
It uses `Sec-Fetch-Site` (`cross-site` is rejected, `same-origin`, `same-site` and `none` are allowed). If the header is
absent or has any other value, it falls back to comparing `Origin`, then `Referer`, with `:authority` using the scheme,
lowercase host and effective port. The external scheme is always https (the OAuth2 filter's `redirect_uri` is built as
`https://<authority>`), so an omitted port means 443 (`app.example.com:443` matches `https://app.example.com`) and an
`http://` origin never matches. A request with none of them is allowed.
`Origin`/`Referer` alone is not used as the primary signal, since `Referrer-Policy: no-referrer` suppresses `Referer`.
The check is always on and not configurable, since a legitimate logout is always same-origin.

### Why trusting `Sec-Fetch-Site` is safe against CSRF

`Sec-Fetch-*` are forbidden request header names in the Fetch specification. Browsers set them, and JavaScript on a
page (including an attacker's page) cannot set or override them, so a cross-site page cannot make the victim's browser
send a spoofed `same-origin` or an unknown value. They are only absent in browsers that do not implement Fetch
Metadata, where the `Origin`/`Referer` fallback applies.

Non-browser clients such as `curl` can send any value, for example `Sec-Fetch-Site: same-origin`, and thereby bypass the
check. This is not a vulnerability, since CSRF requires the victim's browser to attach the victim's cookies. A client
that crafts its own requests has no access to the victim's session cookies and can only log out its own session.
The check is therefore not an access control mechanism, and does not require authentication to log out.

Unknown `Sec-Fetch-Site` values are ignored rather than trusted, so they are handled like a missing header.

## Consequences

- Links to the logout path from other origins (for example an external portal) stop working. This is intended.
- Clients that send no `Sec-Fetch-Site`, `Origin` or `Referer` (old browsers, non-browser clients) are not protected,
  but are not blocked either.
- `same-site` requests (for example from a sibling subdomain) are allowed.
- No new Envoy filter or CRD field is needed. The Lua script gains a `logout_path` template argument.

## References

- `pkg/luascript/ztoperator.lua`
- `pkg/luascript/lua_script_test.go`
- `test/chainsaw/authpolicy/auto_login_rp_initiated_logout/tests.hurl`
