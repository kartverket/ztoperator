# --- Expecting 200
GET https://127.0.0.1:8443/secure
Host: foo.bar
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/secure
Host: foo.bar
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 200