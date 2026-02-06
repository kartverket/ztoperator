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

# --- Expecting 200 because entraid_token_client has role entraid_role without entra_user_role
GET https://127.0.0.1:8443/
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 200 because entraid_token_user has role entraid_role and entra_user_role
GET https://127.0.0.1:8443/
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200