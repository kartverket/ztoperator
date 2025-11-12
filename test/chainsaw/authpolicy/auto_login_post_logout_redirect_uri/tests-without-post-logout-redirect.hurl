POST https://127.0.0.1:8443/logout
Host: foo.bar
HTTP 302
[Asserts]
header "Location" not contains "post_logout_redirect_uri="