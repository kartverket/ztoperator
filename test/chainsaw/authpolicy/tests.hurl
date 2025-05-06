# --- Test 1: GET /public without token (expect 200)
GET https://127.0.0.1:5678/public
Host: foo.bar

HTTP 200


# --- Test 2: GET /deny without token (expect 403)
GET https://127.0.0.1:5678/secure
Host: foo.bar

HTTP 403
