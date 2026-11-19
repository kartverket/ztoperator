# --- Expecting 200 because /public* is open
GET https://127.0.0.1:8443/public
Host: foo.bar
HTTP 200

# --- Expecting 302 to the authorize endpoint
GET https://127.0.0.1:8443/secure
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}
