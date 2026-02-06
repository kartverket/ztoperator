# --- With token client (client role and client sub)

# --- Expecting 200 because client token has the client role and client sub
GET https://127.0.0.1:8443/client-sub-with-client-role
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 200

# --- Expecting 403 because client token has the client role and client sub
GET https://127.0.0.1:8443/client-sub-with-user-role
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 403

# --- Expecting 403 because client token has the client role and client sub
GET https://127.0.0.1:8443/user-sub-with-client-role
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 403

# --- Expecting 403 because client token has the client role and client sub
GET https://127.0.0.1:8443/user-sub-with-user-role
Host: foo.bar
Authorization: Bearer {{entraid_token_client}}
HTTP 403

# --- With token user (user+client role and user sub)

# --- Expecting 403 because user token has user sub
GET https://127.0.0.1:8443/client-sub-with-client-role
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 403

# --- Expecting 403 because user token has user sub
GET https://127.0.0.1:8443/client-sub-with-user-role
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 403

# --- Expecting 200 because user token has the client role and user sub
GET https://127.0.0.1:8443/user-sub-with-client-role
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200

# --- Expecting 200 because user token has the user role and user sub
GET https://127.0.0.1:8443/user-sub-with-user-role
Host: foo.bar
Authorization: Bearer {{entraid_token_user}}
HTTP 200