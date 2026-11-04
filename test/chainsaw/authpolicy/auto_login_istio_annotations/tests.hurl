# --- Expecting 200 because /* is open
GET https://127.0.0.1:8443/random
Host: foo.bar
HTTP 200

# --- Expecting 401 as this endpoint should return "OAuth flow failed"
GET https://127.0.0.1:8443/oauth2/callback
Host: foo.bar
[Cookies]
BearerToken: dummy-value
RefreshToken: dummy-value
HTTP 401

# --- Expecting 401 as this endpoint should return "OAuth flow failed"
POST https://127.0.0.1:8443/oauth2/callback?a=1&b=2
Host: foo.bar
HTTP 401