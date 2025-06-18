# --- Expecting 200
GET https://127.0.0.1:8443/public
Host: foo.bar
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/public
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/public
Host: foo.bar
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/public
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/public/123
Host: foo.bar
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/public/123
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/public/123
Host: foo.bar
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/public/123
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 302
GET https://127.0.0.1:8443/secure
Host: foo.bar
HTTP 302

# --- Expecting 200
GET https://127.0.0.1:8443/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 302
POST https://127.0.0.1:8443/secure
Host: foo.bar
HTTP 302
[Asserts]
Header Location == {{location_header}}

# --- Expecting 200
POST https://127.0.0.1:8443/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

