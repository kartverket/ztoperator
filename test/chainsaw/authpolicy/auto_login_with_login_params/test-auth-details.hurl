# --- Expecting 302 to authorize with the acr_values=entraid and auth_details specified as a query parameters
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 302
[Asserts]
header "Location" matches /^https?:\/\/mock-oauth2\.auth:8080\/entraid\/authorize\?/
header "Location" matches /(?:[?&])client_id=entraid_server(?:&|$)/
header "Location" matches /(?:[?&])acr_values=entraid(?:&|$)/
header "Location" matches /(?:[?&])authorization_details=%5B\+%7B\+%22type%22%3A\+%22ansattporten%3Aaltinn%3Aservice%22%2C\+%22resource%22%3A\+%22urn%3Aaltinn%3Aresource%3A2480%3A40%22\+\%7D%2C\+%7B\+%22type%22%3A\+%22ansattporten%3Aaltinn%3Aservice%22%2C\+%22resource%22%3A\+%22urn%3Aaltinn%3Aresource%3A5900%3A1%22%2C\+%22allow_multiple_organizations%22%3A\+true\+%7D\+%5D(?:&|$)/