# --- Expecting 403 as default-deny should be in place
GET https://127.0.0.1:8443/something/random
Host: foo.bar
HTTP 403

# --- Expecting 403 as default-deny should be in place
GET https://127.0.0.1:8443/public
Host: foo.bar
HTTP 403

# --- Expecting 403 as default-deny should be in place
GET https://127.0.0.1:8443/public
Host: foo.bar
Authorization: Bearer {{token}}
HTTP 403