# --- Initial unauthenticated request should trigger the OAuth Authorization Code
# --- Flow. Envoy responds with 302 -> IdP authorize endpoint and sets the two
# --- "flow" cookies (OauthNonce for CSRF/state, CodeVerifier for PKCE) that the
# --- callback handler will later validate. Both must be SameSite=Lax so they
# --- are included on the top-level cross-site GET back from the IdP.
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains "http://mock-oauth2.auth:8080/entraid/authorize"
cookie "OauthNonce[SameSite]"   == "Lax"
cookie "CodeVerifier[SameSite]" == "Lax"
# --- Sanity: the cookies must also be HttpOnly to prevent JS access.
cookie "OauthNonce[HttpOnly]"   exists
cookie "CodeVerifier[HttpOnly]" exists

# --- Authenticated request with only a RefreshToken cookie forces Envoy to use
# --- the refresh_token grant to mint a new access token. On success it sets
# --- new session cookies (BearerToken, IdToken, OauthHMAC, OauthExpires, and
# --- a rotated RefreshToken). Each of these must also be SameSite=Lax.
GET https://127.0.0.1:8443/
Host: foo.bar
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 200
[Asserts]
cookie "BearerToken[SameSite]"   == "Lax"
cookie "IdToken[SameSite]"       == "Lax"
cookie "OauthHMAC[SameSite]"     == "Lax"
cookie "OauthExpires[SameSite]"  == "Lax"
cookie "RefreshToken[SameSite]"  == "Lax"
