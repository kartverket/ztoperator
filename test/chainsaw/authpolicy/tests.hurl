# --- Test 1: GET /robots.txt without token (expect 200)
GET https://127.0.0.1:8443/robots.txt
Host: foo.bar

HTTP 200


# --- Test 2: GET /deny without token (expect 403)
GET https://127.0.0.1:8443/deny
Host: foo.bar

HTTP 403


# --- Test 3: GET /deny with token (expect 200)
GET https://127.0.0.1:8443/deny
Authorization: Bearer {{token}}
Host: foo.bar

HTTP 200


# --- Test 4: POST /post with token (expect 200)
POST https://127.0.0.1:8443/post
Host: foo.bar
Authorization: Bearer {{token}}
Content-Type: application/json

{
  "message": "hi"
}

HTTP 200

# --- Test 5: POST /post without token (expect 403)
POST https://127.0.0.1:8443/post
Host: foo.bar
Content-Type: application/json

{
  "message": "hi"
}

HTTP 403
