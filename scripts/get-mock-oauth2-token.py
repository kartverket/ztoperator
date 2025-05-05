#!/usr/bin/env python3
import httpx

hostname = "fake.auth"
local_server = "127.0.0.1:8443"
token_endpoint = f"https://{local_server}/default/token"

client_id = "my-client"
client_secret = "my-secret"

# Request token (client_credentials flow)
data = {
    "grant_type": "client_credentials",
    "client_id": client_id,
    "client_secret": client_secret,
    "scope": "my-scope"  # Optional depending on mock server setup
}

client = httpx.Client(verify=False)
headers = {"Host": hostname}
extensions = {"sni_hostname": hostname}

token_resp = client.post(
    token_endpoint,
    headers=headers,
    extensions=extensions,
    data=data
)

token_resp.raise_for_status()

access_token = token_resp.json()["access_token"]
print(access_token)
