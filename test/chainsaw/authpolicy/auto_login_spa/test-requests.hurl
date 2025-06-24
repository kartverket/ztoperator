# --- Expecting 200
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/
Host: foo.bar
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/public
Host: foo.bar
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/public
Host: foo.bar
HTTP 200

# --- Expecting 401
GET https://127.0.0.1:8443/api
Host: foo.bar
HTTP 401

# --- Expecting 401
POST https://127.0.0.1:8443/api
Host: foo.bar
HTTP 401

# --- Expecting 401
GET https://127.0.0.1:8443/api/something
Host: foo.bar
HTTP 401

# --- Expecting 401
POST https://127.0.0.1:8443/api/something
Host: foo.bar
HTTP 401

# --- Expecting 200
GET https://127.0.0.1:8443/api
Host: foo.bar
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/api
Host: foo.bar
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/api/something
Host: foo.bar
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 200


# --- Expecting 200
POST https://127.0.0.1:8443/api/something
Host: foo.bar
[Cookies]
RefreshToken: {{entraid_refresh_token}}
HTTP 200