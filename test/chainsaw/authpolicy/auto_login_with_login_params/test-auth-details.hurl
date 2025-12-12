# --- Expecting 302 to authorize with the acr_values=entraid and auth_details specified as a query parameters
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 302
[Asserts]
header "Location" matches /^https?:\/\/mock-oauth2\.auth:8080\/entraid\/authorize\?/
header "Location" matches /(?:[?&])client_id=entraid_server(?:&|$)/
header "Location" matches /(?:[?&])acr_values=entraid(?:&|$)/
header "Location" matches /(?:[?&])authorization_details=%5B%0A(?:\+|%20)*%7B%0A(?:\+|%20)*%22type%22%3A(?:\+|%20)*%22ansattporten%3Aaltinn%3Aservice%22%2C%0A(?:\+|%20)*%22resource%22%3A(?:\+|%20)*%22urn%3Aaltinn%3Aresource%3A2480%3A40%22%0A(?:\+|%20)*%7D%2C%0A(?:\+|%20)*%7B%0A(?:\+|%20)*%22type%22%3A(?:\+|%20)*%22ansattporten%3Aaltinn%3Aservice%22%2C%0A(?:\+|%20)*%22resource%22%3A(?:\+|%20)*%22urn%3Aaltinn%3Aresource%3A5900%3A1%22%2C%0A(?:\+|%20)*%22allow_multiple_organizations%22%3A(?:\+|%20)*true%0A(?:\+|%20)*%7D%0A(?:\+|%20)*%5D%0A(?:&|$)/