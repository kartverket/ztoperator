# --- Expecting 403
GET https://127.0.0.1:8443/random/path
Host: foo.bar
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 403
POST https://127.0.0.1:8443/random/path
Host: foo.bar
HTTP 403

# --- Expecting 200
POST https://127.0.0.1:8443/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 403
GET https://127.0.0.1:8443/other/random/path
Host: foo.bar
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/other/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 403
POST https://127.0.0.1:8443/other/random/path
Host: foo.bar
HTTP 403

# --- Expecting 200
POST https://127.0.0.1:8443/other/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200