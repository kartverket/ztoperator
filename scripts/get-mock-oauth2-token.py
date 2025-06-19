#!/usr/bin/env python3
import argparse

import httpx

parser = argparse.ArgumentParser(description="Mock OAuth2 token fetcher")
parser.add_argument("--issuer", required=True, help="Issuer (e.g., idporten)")
parser.add_argument("--code", required=True, help="Authorization code to exchange for token (e.g., idporten_code)")
parser.add_argument("--token_name", required=True, help="Which token in the token response should be returned by the script?")
args = parser.parse_args()

hostname = "fake.auth"
local_server = "127.0.0.1:8443"
token_endpoint = f"https://{local_server}/{args.issuer}/token"

client_id = "my-client"
client_secret = "my-secret"

# Request token (client_credentials flow)
data = {
    "grant_type": "authorization_code",
    "code": args.code,
    "client_id": client_id,
    "client_secret": client_secret
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

token = token_resp.json()[args.token_name]

print(token)
