# --- Expecting 200
GET https://127.0.0.1:8443/api-docs
Host: foo.bar
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/api-docs/123
Host: foo.bar
HTTP 200

# --- Expecting 403
POST https://127.0.0.1:8443/api-docs/123
Host: foo.bar
HTTP 403

# --- Expecting 200
POST https://127.0.0.1:8443/api-docs/123
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 403
GET https://127.0.0.1:8443/api/update
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 403

# --- Expecting 403
POST https://127.0.0.1:8443/api/update
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 403

# --- Expecting 200
POST https://127.0.0.1:8443/api/update
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200

# --- Expecting 401
POST https://127.0.0.1:8443/api/update
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 401

# --- Expecting 403
GET https://127.0.0.1:8443/api/update/123
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 403

# --- Expecting 403
POST https://127.0.0.1:8443/api/update/123
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 403

# --- Expecting 403
POST https://127.0.0.1:8443/api/update/123
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 403

# --- Expecting 401
GET https://127.0.0.1:8443/api/update/123
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 401

# --- Expecting 403
GET https://127.0.0.1:8443
Host: foo.bar
HTTP 403

# --- Expecting 403
GET https://127.0.0.1:8443/random/path
Host: foo.bar
HTTP 403

# --- Expecting 401
GET https://127.0.0.1:8443/random/path
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 401

# --- Expecting 200
GET https://127.0.0.1:8443/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 200
GET https://127.0.0.1:8443/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200

# --- Expecting 401
POST https://127.0.0.1:8443/random/path
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 401

# --- Expecting 403
POST https://127.0.0.1:8443/random/path
Host: foo.bar
HTTP 403

# --- Expecting 200
POST https://127.0.0.1:8443/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200

# --- Expecting 403
GET https://127.0.0.1:8443/api/random/path
Host: foo.bar
HTTP 403

# --- Expecting 401
GET https://127.0.0.1:8443/api/random/path
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 401

# --- Expecting 403
GET https://127.0.0.1:8443/api/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/api/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200

# --- Expecting 401
POST https://127.0.0.1:8443/api/random/path
Host: foo.bar
Authorization: Bearer {{idporten_token}}
HTTP 401

# --- Expecting 403
POST https://127.0.0.1:8443/api/random/path
Host: foo.bar
HTTP 403

# --- Expecting 200
POST https://127.0.0.1:8443/api/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 200
POST https://127.0.0.1:8443/api/random/path
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200