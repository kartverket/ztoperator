POST https://127.0.0.1:8443/logout
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains "post_logout_redirect_uri=https%3A%2F%2Fsome-random-uri.com"