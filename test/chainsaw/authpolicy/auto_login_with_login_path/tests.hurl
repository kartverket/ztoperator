# --- Expecting 403
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 302
POST https://127.0.0.1:8443/login
Host: foo.bar
HTTP 302