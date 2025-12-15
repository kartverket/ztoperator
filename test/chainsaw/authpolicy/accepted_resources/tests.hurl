# --- Should return 302 to authorize WITH resource indicators as query parameter
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 302
[Asserts]
header "Location" matches /(?:[?&])resource=https%3A%2F%2Fexample.com%2Fapi-1(?:&|$)/
header "Location" matches /(?:[?&])resource=https%3A%2F%2Fexample.com%2Fapi-2(?:&|$)/
header "Location" contains {{location_header}}