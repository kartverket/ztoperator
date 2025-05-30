# --- Expecting 200
GET https://127.0.0.1:8443/idporten/public
Host: foo.bar
HTTP 200

# --- Expecting 403
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 403
POST https://127.0.0.1:8443/idporten/secure
Host: foo.bar
HTTP 403

# --- Expecting 200
POST https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Expecting 403
POST https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Expecting 403
POST https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Expecting 200
PUT https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Expecting 403
PUT https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Expecting 403
PUT https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Expecting 200
DELETE https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Expecting 403
DELETE https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Expecting 403
DELETE https://127.0.0.1:8443/idporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
HTTP 200

# --- Expecting 403
POST https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Expecting 200
PUT https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Expecting 200
DELETE https://127.0.0.1:8443/maskinporten/public
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 200

# --- Expecting 403
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 403

# --- Expecting 403
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Expecting 403
GET https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
HTTP 403

# --- Expecting 200
POST https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 200

# --- Expecting 403
POST https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 403

# --- Expecting 403
POST https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 403

# --- Expecting 403
POST https://127.0.0.1:8443/maskinporten/secure
Host: foo.bar
HTTP 403

# --- Expecting 403
GET https://127.0.0.1:8443/entraid/public
Host: foo.bar
HTTP 403

# --- Expecting 403
GET https://127.0.0.1:8443/entraid/public
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 403

# --- Expecting 403
GET https://127.0.0.1:8443/entraid/public
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/entraid/public
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 403
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 403

# --- Expecting 403
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 403

# --- Expecting 403
GET https://127.0.0.1:8443/entraid/secure
Host: foo.bar
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{maskinporten_token}}
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/other/path
Host: foo.bar
Authorization: Bearer {{entraid_token}}
HTTP 200

# --- Expecting 403
GET https://127.0.0.1:8443/other/path
Host: foo.bar
HTTP 403