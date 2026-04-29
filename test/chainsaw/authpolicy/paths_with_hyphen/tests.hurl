# --- Returns 200 OK because GET on /* is open
GET https://127.0.0.1:8443/something
Host: foo.bar
HTTP 200

# --- Returns 302 to login because POST on /something is not covered --> requires auth
POST https://127.0.0.1:8443/something
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

# --- Expecting 302 to login because POST to /with-hyphen is not covered --> requires auth
POST https://127.0.0.1:8443/with-hyphen
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

# --- Expecting 200 OK because POST to /with-hyphen with access-token is OK
POST https://127.0.0.1:8443/with-hyphen
Host: foo.bar
Authorization: Bearer {{token}}
HTTP 200

# --- Expecting 302 to login because GET to /with-hyphen requires auth
GET https://127.0.0.1:8443/with-hyphen
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

# --- Expecting 200 OK because GET to /with-hyphen with access-token is OK
GET https://127.0.0.1:8443/with-hyphen
Host: foo.bar
Authorization: Bearer {{token}}
HTTP 200