# --- Test 1: GET /idporten/public without token (expect 200)
GET https://127.0.0.1:8443/idporten/public
Host: foo.bar
HTTP 200

# --- Test 2: GET /entraid/public without token (expect 200)
GET https://127.0.0.1:8443/entraid/public
Host: foo.bar
HTTP 200

# --- Test 3: GET /maskinporten/public without token (expect 200)
GET https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
HTTP 200

# --- Test 4: GET /idporten/secure with correct token (expect 200)
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Test 5: GET /idporten/secure with maskinporten token (expect 403)
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Test 6: GET /idporten/secure with entraid token (expect 403)
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Test 7: GET /idporten/secure without token (expect 403)
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
HTTP 403

# --- Test 8: GET /maskinporten/secure with correct token (expect 200)
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 200

# --- Test 9: GET /maskinporten/secure with idporten token (expect 403)
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 403

# --- Test 10: GET /maskinporten/secure with entraid token (expect 403)
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Test 11: GET /maskinporten/secure without token (expect 403)
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
HTTP 403

# --- Test 12: GET /entraid/secure with correct token (expect 200)
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Test 13: GET /entraid/secure with idporten token (expect 403)
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 403

# --- Test 14: GET /entraid/secure with maskinporten token (expect 403)
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Test 15: GET /entraid/secure without token (expect 403)
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
HTTP 403

# --- Test 16: GET /other/path with idporten token (expect 200)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Test 17: GET /other/path with maskinporten token (expect 200)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 200

# --- Test 18: GET /other/path with entraid token (expect 200)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Test 19: GET /other/path without token (expect 403)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
HTTP 403