# Verify that request without cookies is redirected to the authorization endpoint
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains "http://mock-oauth2.auth:8080/entraid"

# Verify that EnvoyFilter rerieves an access_token when only RefreshToken is sent as cookie
GET https://127.0.0.1:8443/
Host: foo.bar
Cookie: RefreshToken=dummy-value
HTTP 200