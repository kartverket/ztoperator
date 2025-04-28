#!/usr/bin/env python3
import requests

# Discover OIDC configuration
well_known_url = "http://localhost:8080/default/.well-known/openid-configuration"
resp = requests.get(well_known_url)
resp.raise_for_status()
oidc_config = resp.json()

token_endpoint = oidc_config["token_endpoint"]

# Define client credentials
client_id = "my-client"
client_secret = "my-secret"

# Request token (client_credentials flow)
data = {
    "grant_type": "client_credentials",
    "client_id": client_id,
    "client_secret": client_secret,
    "scope": "my-scope"  # Optional depending on mock server setup
}

token_resp = requests.post(token_endpoint, data=data)
token_resp.raise_for_status()

access_token = token_resp.json()["access_token"]
print(access_token)
