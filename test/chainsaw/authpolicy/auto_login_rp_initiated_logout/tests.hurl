# --- Expecting 200
GET https://127.0.0.1:8443/secure
Host: foo.bar
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 200

# --- Expecting 302
POST https://127.0.0.1:8443/logout
Host: foo.bar
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

# --- Expecting 403, cross-site logout (logout CSRF) must not clear cookies or redirect to the IdP
GET https://127.0.0.1:8443/logout
Host: foo.bar
Sec-Fetch-Site: cross-site
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 403
[Asserts]
header "Set-Cookie" not exists
header "Location" not exists

# --- Expecting 403, foreign Origin without fetch metadata
GET https://127.0.0.1:8443/logout
Host: foo.bar
Origin: https://evil.example.com
HTTP 403
[Asserts]
header "Set-Cookie" not exists
header "Location" not exists

# --- Expecting 302, same-origin logout is still allowed
GET https://127.0.0.1:8443/logout
Host: foo.bar
Sec-Fetch-Site: same-origin
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 302
[Asserts]
header "Location" contains {{location_header}}
