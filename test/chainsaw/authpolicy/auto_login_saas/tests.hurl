# --- Expecting 200
GET https://127.0.0.1:8443/public
Host: foo.bar
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/public
Host: foo.bar
[Cookies]
BearerToken: {{entraid_token}}
RefreshToken: {{entraid_refresh_token}}
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/public
Host: foo.bar
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/public
Host: foo.bar
[Cookies]
BearerToken: {{entraid_token}}
RefreshToken: {{entraid_refresh_token}}
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/public/123
Host: foo.bar
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/public/123
Host: foo.bar
[Cookies]
BearerToken: {{entraid_token}}
RefreshToken: {{entraid_refresh_token}}
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/public/123
Host: foo.bar
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/public/123
Host: foo.bar
[Cookies]
BearerToken: {{entraid_token}}
RefreshToken: {{entraid_refresh_token}}
HTTP 200

# --- Expecting 302
GET https://127.0.0.1:8443/secure
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

# --- Expecting 200
GET https://127.0.0.1:8443/secure
Host: foo.bar
[Cookies]
BearerToken: {{entraid_token}}
RefreshToken: {{entraid_refresh_token}}
HTTP 200

# --- Expecting 302
POST https://127.0.0.1:8443/secure
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

# --- Expecting 200
POST https://127.0.0.1:8443/secure
Host: foo.bar
[Cookies]
BearerToken: {{entraid_token}}
RefreshToken: {{entraid_refresh_token}}
HTTP 200

