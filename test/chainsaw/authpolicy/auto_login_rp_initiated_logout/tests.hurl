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