# --- Expecting 302 as only /public is open
GET https://127.0.0.1:8443/something/random
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

# --- Expecting 200 as /public is open
GET https://127.0.0.1:8443/public
Host: foo.bar
HTTP 200

# --- Expecting 200 as I have a valid Bearer Token
GET https://127.0.0.1:8443/public
Host: foo.bar
Authorization: Bearer {{token}}
HTTP 200