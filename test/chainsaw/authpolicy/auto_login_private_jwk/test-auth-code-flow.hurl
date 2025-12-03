# Access protected path to trigger auth code flow
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains "http://mock-oauth2.auth:8080/entraid/authorize?client_id=entraid_server&code_challenge="
[Captures]
auth_redirect: header "Location"

# Follows redirection to auth server
GET {{auth_redirect}}
HTTP 200

# "Simulates user login" by posting to the mock auth server
POST {{auth_redirect}}
[Form]
username: testuser
claims:
HTTP 302
[Asserts]
header "Location" contains "https://foo.bar/oauth2/callback?code="
[Captures]
callback_redirect_path: header "Location" regex "https://[^/]+(.*)"

# Follows redirection back to the application callback endpoint
GET https://127.0.0.1:8443{{callback_redirect_path}}
Host: foo.bar
HTTP 302
[Asserts]
cookie "RefreshToken" exists
cookie "BearerToken" exists
cookie "OauthHMAC" exists
cookie "OauthExpires" exists
header "Location" matches "https://foo.bar/"
[Captures]
final_redirect_path: header "Location" regex "https://[^/]+(.*)"

# Final redirection to the originally requested resource
GET http://127.0.0.1:8443{{final_redirect_path}}
Host: foo.bar
HTTP 200
