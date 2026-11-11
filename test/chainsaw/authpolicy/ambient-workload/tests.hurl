# --- Expecting 302
GET https://127.0.0.1:8443/something
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

# --- Expecting 401
POST https://127.0.0.1:8443/something
Host: foo.bar
Authorization: Bearer dummy-value
HTTP 401

# --- Expecting 200
POST https://127.0.0.1:8443/something
Host: foo.bar
Authorization: Bearer {{entraid_access_token}}
HTTP 200