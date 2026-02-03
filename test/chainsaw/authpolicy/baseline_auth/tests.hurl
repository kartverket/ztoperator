# --- ignoreAuthRules /public*

# --- Expecting 200 because public* GET is in ignore_auth_rules
GET https://127.0.0.1:8443/public
Host: foo.bar
HTTP 200

# --- Expecting 200 because public* GET is in ignore_auth_rules
GET https://127.0.1:8443/public/123
Host: foo.bar
HTTP 200

# --- Expecting 200 because public* is in ignore_auth_rules
GET https://127.0.1:8443/public123
Host: foo.bar
HTTP 200

# --- Expecting 403 because public* POST is not in ignore_auth_rules
POST https://127.0.0.1:8443/public
Host: foo.bar
HTTP 403

# --- baselineAuth role=entraid_role

# --- Expecting 403 because auth is required
GET https://127.0.0.1:8443/
Host: foo.bar
HTTP 403

# --- Expecting 403 because entraid_token_other_client doesn't have role entraid_role
GET https://127.0.0.1:8443/
Host: foo.bar
Authorization: Bearer {{entraid_token_other_client}}
HTTP 403

# --- Expecting 200 because entraid_token_client has role entraid_role
GET https://127.0.0.1:8443/
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 200 because entraid_token_user has role entraid_role
GET https://127.0.0.1:8443/
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200

# --- Expecting 403 because entraid_token_other_client doesn't have role entraid_role
GET https://127.0.0.1:8443/api
Host: foo.bar
Authorization: Bearer {{entraid_token_other_client}}
HTTP 403

# --- Expecting 200 because entraid_token_client has role entraid_role
GET https://127.0.0.1:8443/api
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 200 because entraid_token_user has role entraid_role
GET https://127.0.0.1:8443/api
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200

# --- authRules role=entra_user_role for /api with verb POST

# --- Expecting 403 because entraid_token_other_client doesn't have role entra_user_role
POST https://127.0.0.1:8443/api
Host: foo.bar
Authorization: Bearer {{entraid_token_other_client}}
HTTP 403

# --- Expecting 403 because entraid_token_client doesn't have role entra_user_role
POST https://127.0.0.1:8443/api
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 403

# --- Expecting 200 because entraid_token_user has role entra_user_role
POST https://127.0.0.1:8443/api
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200

# --- Expecting 403 because entraid_token_user has role entra_user_role but lacks sub entraid_client
POST https://127.0.0.1:8443/unreachable
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 403