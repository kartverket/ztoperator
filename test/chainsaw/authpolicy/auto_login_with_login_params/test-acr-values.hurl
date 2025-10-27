# --- Expecting 302 to authorize with the acr_values=entraid specified as a query parameter
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 302
[Asserts]
header "Location" matches /^https?:\/\/mock-oauth2\.auth:8080\/entraid\/authorize\?/
header "Location" matches /(?:[?&])client_id=entraid_server(?:&|$)/
header "Location" matches /(?:[?&])acr_values=entraid(?:&|$)/