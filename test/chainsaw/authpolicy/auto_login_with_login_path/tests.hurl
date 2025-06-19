# --- Expecting 403
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/
Host: foo.bar
[Cookies]
BearerToken: {{entraid_token}}
RefreshToken: {{entraid_refresh_token}}
HTTP 200

# --- Expecting 302
POST https://127.0.0.1:8443/login
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}