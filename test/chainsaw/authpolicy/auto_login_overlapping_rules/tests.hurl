GET https://127.0.0.1:8443/secure/specific/path1
Host: foo.bar
HTTP 200

GET https://127.0.0.1:8443/secure/specific/path12
Host: foo.bar
HTTP 200

POST https://127.0.0.1:8443/secure/specific/path12
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

GET https://127.0.0.1:8443/secure/specific/path2
Host: foo.bar
HTTP 200

GET https://127.0.0.1:8443/secure/specific/path21
Host: foo.bar
HTTP 200

POST https://127.0.0.1:8443/secure/specific/path21
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

POST https://127.0.0.1:8443/something/random
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

PUT https://127.0.0.1:8443/something/random
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

DELETE https://127.0.0.1:8443/something/random
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

POST https://127.0.0.1:8443/something/random
Host: foo.bar
Authorization: Bearer {{token_with_claim}}
HTTP 200

PUT https://127.0.0.1:8443/something/random
Host: foo.bar
Authorization: Bearer {{token_with_claim}}
HTTP 200

DELETE https://127.0.0.1:8443/something/random
Host: foo.bar
Authorization: Bearer {{token_with_claim}}
HTTP 200

POST https://127.0.0.1:8443/something/random
Host: foo.bar
Authorization: Bearer {{token_without_claim}}
HTTP 403

PUT https://127.0.0.1:8443/something/random
Host: foo.bar
Authorization: Bearer {{token_without_claim}}
HTTP 403

DELETE https://127.0.0.1:8443/something/random
Host: foo.bar
Authorization: Bearer {{token_without_claim}}
HTTP 403

GET https://127.0.0.1:8443/something/random
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

GET https://127.0.0.1:8443/something/random
Host: foo.bar
Authorization: Bearer {{token_with_claim}}
HTTP 200

GET https://127.0.0.1:8443/something/random
Host: foo.bar
Authorization: Bearer {{token_without_claim}}
HTTP 200

GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

GET https://127.0.0.1:8443/
Host: foo.bar
Authorization: Bearer {{token_with_claim}}
HTTP 200

GET https://127.0.0.1:8443/
Host: foo.bar
Authorization: Bearer {{token_without_claim}}
HTTP 403

GET https://127.0.0.1:8443/secure
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

GET https://127.0.0.1:8443/secure
Host: foo.bar
Authorization: Bearer {{token_with_claim}}
HTTP 200

GET https://127.0.0.1:8443/secure
Host: foo.bar
Authorization: Bearer {{token_without_claim}}
HTTP 403

GET https://127.0.0.1:8443/secure1
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

GET https://127.0.0.1:8443/secure1
Host: foo.bar
Authorization: Bearer {{token_with_claim}}
HTTP 200

GET https://127.0.0.1:8443/secure1
Host: foo.bar
Authorization: Bearer {{token_without_claim}}
HTTP 403

GET https://127.0.0.1:8443/secure2
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

GET https://127.0.0.1:8443/secure2
Host: foo.bar
Authorization: Bearer {{token_with_claim}}
HTTP 200

GET https://127.0.0.1:8443/secure2
Host: foo.bar
Authorization: Bearer {{token_without_claim}}
HTTP 403

GET https://127.0.0.1:8443/other/path
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{token_with_claim}}
HTTP 200

GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{token_without_claim}}
HTTP 403

GET https://127.0.0.1:8443/other/path1
Host: foo.bar
HTTP 302
[Asserts]
header "Location" contains {{location_header}}

GET https://127.0.0.1:8443/other/path1
Host: foo.bar
Authorization: Bearer {{token_with_claim}}
HTTP 200

GET https://127.0.0.1:8443/other/path1
Host: foo.bar
Authorization: Bearer {{token_without_claim}}
HTTP 403

