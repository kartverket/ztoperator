# --- Test 1: GET /entraid/public without token (expect 200)
GET https://127.0.0.1:8443/entraid/public
Host: foo.bar
HTTP 200

# --- Test 2: GET /entraid/secure with correct token (expect 200)
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Test 13: GET /entraid/secure with idporten token (expect 401)
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 401

# --- Test 16: GET /other/path with idporten token (expect 401)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 401

# --- Test 18: GET /other/path with entraid token (expect 200)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Test 19: GET /other/path without token (expect 403)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
HTTP 403