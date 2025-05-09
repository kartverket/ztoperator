# --- Test 1: GET /idporten/public without token (expect 200)
GET https://127.0.0.1:8443/idporten/public
Host: foo.bar
HTTP 200

# --- Test 2: GET /idporten/secure without token (expect 403)
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
HTTP 403

# --- Test 3: GET /idporten/secure with idporten token (expect 200)
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Test 4: GET /idporten/secure with maskinporten token (expect 200)
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 200

# --- Test 5: GET /idporten/secure with entraid token (expect 200)
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Test 6: POST /idporten/secure without token (expect 403)
POST https://127.0.0.1:8443/idporten/secure
Host: foo.bar
HTTP 403

# --- Test 7: POST /idporten/secure with idporten token (expect 200)
POST https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Test 8: POST /idporten/secure with maskinporten token (expect 403)
POST https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Test 9: POST /idporten/secure with entraid token (expect 403)
POST https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Test 10: PUT /idporten/secure with idporten token (expect 200)
PUT https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Test 11: PUT /idporten/secure with maskinporten token (expect 403)
PUT https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Test 12: PUT /idporten/secure with entraid token (expect 403)
PUT https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Test 13: DELETE /idporten/secure with idporten token (expect 200)
DELETE https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Test 14: DELETE /idporten/secure with maskinporten token (expect 403)
DELETE https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Test 15: DELETE /idporten/secure with entraid token (expect 403)
DELETE https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Test 16: GET /maskinporten/public without token (expect 200)
GET https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
HTTP 200

# --- Test 17: POST /maskinporten/public without token (expect 403)
POST https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
HTTP 403

# --- Test 18: POST /maskinporten/public with idporten token (expect 200)
GET https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Test 19: PUT /maskinporten/public with maskinporten token (expect 200)
PUT https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Test 20: DELETE /maskinporten/public with entraid token (expect 200)
DELETE https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Test 21: GET /maskinporten/secure with correct token (expect 200)
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 200

# --- Test 22: GET /maskinporten/secure with idporten token (expect 403)
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 403

# --- Test 23: GET /maskinporten/secure with entraid token (expect 403)
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Test 24: GET /maskinporten/secure without token (expect 403)
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
HTTP 403

# --- Test 25: POST /maskinporten/secure with correct token (expect 200)
POST https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 200

# --- Test 26: POST /maskinporten/secure with idporten token (expect 403)
POST https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 403

# --- Test 27: POST /maskinporten/secure with entraid token (expect 403)
POST https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Test 28: POST /maskinporten/secure without token (expect 403)
POST https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
HTTP 403

# --- Test 29: GET /entraid/public without token (expect 403)
GET https://127.0.0.1:8443/entraid/public
Host: foo.bar
HTTP 403

# --- Test 30: GET /entraid/public with idporten token (expect 403)
GET https://127.0.0.1:8443/entraid/public
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 403

# --- Test 31: GET /entraid/public with maskinporten token (expect 403)
GET https://127.0.0.1:8443/entraid/public
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Test 32: GET /entraid/public with entraid token (expect 200)
GET https://127.0.0.1:8443/entraid/public
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Test 33: GET /entraid/secure with entraid token (expect 200)
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Test 34: GET /entraid/secure with idporten token (expect 403)
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 403

# --- Test 35: GET /entraid/secure with maskinporten token (expect 403)
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Test 36: GET /entraid/secure without token (expect 403)
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
HTTP 403

# --- Test 37: GET /other/path with idporten token (expect 200)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Test 38: GET /other/path with maskinporten token (expect 200)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 200

# --- Test 39: GET /other/path with entraid token (expect 200)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Test 40: GET /other/path without token (expect 403)
GET https://127.0.0.1:8443/other/path
Host: foo.bar
HTTP 403