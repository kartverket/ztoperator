import json
import os
import re
import sys
import urllib.request
from urllib.parse import urlparse

import yaml

ap = yaml.safe_load(sys.stdin)

metadata = ap.get("metadata") or {}
spec = ap.get("spec") or {}

ap_name = metadata.get("name", "auth-policy")

# Helpers ----------------------------------------------------------


def lua_rules_from_matchers(matchers):
    """Convert a list of request matchers into a Lua rules table.

    Each matcher is expected to look like:
      {"paths": ["/foo"], "methods": ["GET", "POST"]}
    """
    if not matchers:
        return "{}"

    out_parts = []
    for m in matchers:
        if not isinstance(m, dict):
            continue
        paths = m.get("paths") or []
        methods = m.get("methods") or []

        for p in paths:
            regex = str(p)
            # If the path has a wildcard, convert "*" to ".*" and anchor at start.
            if "*" in regex:
                regex = "^" + regex.replace("*", ".*")
            else:
                # no wildcard: exact match
                if not regex.startswith("^"):
                    regex = "^" + regex + "$"

            if methods:
                methods_table = "{" + ",".join(f'["{mm}"]=true' for mm in methods) + "}"
            else:
                methods_table = "{}"  # empty == all methods

            out_parts.append(f'{{regex="{regex}",methods={methods_table}}}')

    return "{" + ",".join(out_parts) + "}"


def lua_table_from_map(mapping):
    if not mapping:
        return "{}"
    parts = []
    for k, v in mapping.items():
        parts.append(f'["{k}"]="{v}"')
    return "{" + ",".join(parts) + "}"


# Extract rules ----------------------------------------------------

ignore_matchers = spec.get("ignoreAuthRules") or []

auto = spec.get("autoLogin") or {}
login_path = auto.get("loginPath")
redirect_path = auto.get("redirectPath") or "/oauth2/callback"
logout_path = auto.get("logoutPath") or "/logout"

require_matchers = []
if redirect_path:
    require_matchers.append({"paths": [redirect_path], "methods": []})
if logout_path:
    require_matchers.append({"paths": [logout_path], "methods": []})
if login_path:
    require_matchers.append({"paths": [login_path], "methods": []})

deny_matchers = []  # no explicit deny rules in this AuthPolicy schema

ignore_lua = lua_rules_from_matchers(ignore_matchers)
require_lua = lua_rules_from_matchers(require_matchers)
deny_lua = lua_rules_from_matchers(deny_matchers)

# Identity provider / auto-login config ---------------------------

well_known = spec.get("wellKnownURI", "")
if not well_known:
    raise SystemExit("spec.wellKnownURI must be set on AuthPolicy to generate EnvoyFilter")

# Special-case mock-oauth2.auth:8080 so the script doesn't try to fetch it over HTTP.
# Pattern: http://mock-oauth2.auth:8080/<issuer>/.well-known/openid-configuration
mock_pattern = r"^http://mock-oauth2\.auth:8080/([^/]+)/\.well-known/openid-configuration$"
m = re.match(mock_pattern, well_known)

if m:
    issuer = m.group(1)
    base = f"http://mock-oauth2.auth:8080/{issuer}"
    authorize_endpoint = f"{base}/authorize"
    token_endpoint = f"{base}/token"
    end_session_endpoint = f"{base}/endsession"
else:
    # Call the real well-known endpoint and parse the discovery document.
    try:
        with urllib.request.urlopen(well_known) as resp:
            discovery = json.load(resp)
    except Exception as e:
        raise SystemExit(f"Failed to fetch discovery document from {well_known}: {e}")

    authorize_endpoint = discovery.get("authorization_endpoint")
    end_session_endpoint = discovery.get("end_session_endpoint")
    token_endpoint = discovery.get("token_endpoint")

    if not authorize_endpoint or not token_endpoint:
        raise SystemExit(
            f"Discovery document at {well_known} is missing required fields: "
            f"authorization_endpoint or token_endpoint"
        )

# Prefer token_endpoint URI for cluster host/port/scheme, fall back to wellKnownURI.
parsed_token = urlparse(token_endpoint)
base_uri = parsed_token if parsed_token.scheme and parsed_token.hostname else urlparse(well_known)

oauth_scheme = base_uri.scheme or "https"
oauth_host = base_uri.hostname
oauth_port = base_uri.port

if not oauth_host:
    raise SystemExit(
        f"Could not determine OAuth host from token_endpoint={token_endpoint} "
        f"or wellKnownURI={well_known}"
    )

use_tls = oauth_scheme == "https"

if oauth_port is None:
    oauth_port = 443 if use_tls else 80

login_params = auto.get("loginParams") or {}

from urllib.parse import quote
# URL‑encode each login param value
login_params = {k: quote(str(v), safe="") for k, v in login_params.items()}
raw_post_logout = auto.get("postLogoutRedirectUri", "")
post_logout = quote(raw_post_logout, safe="")

login_params_lua = lua_table_from_map(login_params)

scopes = auto.get("scopes") or ["openid", "offline_access", "User.Read"]
match_labels = (spec.get("selector") or {}).get("matchLabels") or {}

# Load external Lua file template with %s placeholders
lua_file_path = os.path.join(os.path.dirname(__file__), "..", "pkg", "luascript", "ztoperator.lua")
with open(lua_file_path, "r") as f:
    lua_template = f.read()

# Fill in the %s placeholders in this order:
#   1) ignore_rules
#   2) require_rules
#   3) deny_redirect_rules
#   4) authorize_endpoint
#   5) login_params
#   6) end_session_endpoint
#   7) post_logout_redirect_uri
#   8) bypass-header-name
#   9) deny-redirect-header-name
lua_filled = lua_template % (
    ignore_lua,
    require_lua,
    deny_lua,
    authorize_endpoint,
    login_params_lua,
    end_session_endpoint or "",
    post_logout,
    "x-bypass-login",
    "x-deny-redirect",
)


def indent(text: str, prefix: str) -> str:
    return "\n".join(prefix + line if line else prefix for line in text.splitlines())


lua_indented = indent(lua_filled, "                ")

# Emit EnvoyFilter YAML -------------------------------------------

print("apiVersion: networking.istio.io/v1alpha3")
print("kind: EnvoyFilter")
print("metadata:")
print(f"  name: {ap_name}-login")
print("spec:")
print("  configPatches:")
print("    - applyTo: HTTP_FILTER")
print("      match:")
print("        context: SIDECAR_INBOUND")
print("        listener:")
print("          filterChain:")
print("            filter:")
print("              name: envoy.filters.network.http_connection_manager")
print("      patch:")
print("        operation: INSERT_BEFORE")
print("        value:")
print("          name: envoy.filters.http.lua")
print("          typed_config:")
print("            '@type': type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua")
print("            default_source_code:")
print("              inline_string: |-")
print(lua_indented)
print("    - applyTo: CLUSTER")
print("      match:")
print("        cluster:")
print("          service: oauth")
print("      patch:")
print("        operation: ADD")
print("        value:")
print("          connect_timeout: 10s")
print("          dns_lookup_family: V4_ONLY")
print("          lb_policy: ROUND_ROBIN")
print("          load_assignment:")
print("            cluster_name: oauth")
print("            endpoints:")
print("              - lb_endpoints:")
print("                  - endpoint:")
print("                      address:")
print("                        socket_address:")
print(f"                          address: {oauth_host}")
print(f"                          port_value: {oauth_port}")
print("          name: oauth")
if use_tls:
    print("          transport_socket:")
    print("            name: envoy.transport_sockets.tls")
    print("            typed_config:")
    print("              '@type': type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext")
    print(f"              sni: {oauth_host}")
print("          type: LOGICAL_DNS")
print("    - applyTo: HTTP_FILTER")
print("      match:")
print("        context: SIDECAR_INBOUND")
print("        listener:")
print("          filterChain:")
print("            filter:")
print("              name: envoy.filters.network.http_connection_manager")
print("              subFilter:")
print("                name: envoy.filters.http.jwt_authn")
print("      patch:")
print("        operation: INSERT_BEFORE")
print("        value:")
print("          name: envoy.filters.http.oauth2")
print("          typed_config:")
print("            '@type': type.googleapis.com/envoy.extensions.filters.http.oauth2.v3.OAuth2")
print("            config:")
print("              auth_scopes:")
for scope in scopes:
    print(f"                - {scope}")
print(f"              authorization_endpoint: {authorize_endpoint}")
print("              credentials:")
print("                client_id: entraid_server")
print("                hmac_secret:")
print("                  name: hmac")
print("                  sds_config:")
print("                    path_config_source:")
print("                      path: /etc/istio/config/hmac-secret.yaml")
print("                      watched_directory:")
print("                        path: /etc/istio/config")
print("                token_secret:")
print("                  name: token")
print("                  sds_config:")
print("                    path_config_source:")
print("                      path: /etc/istio/config/token-secret.yaml")
print("                      watched_directory:")
print("                        path: /etc/istio/config")
print("              deny_redirect_matcher:")
print("                - name: x-deny-redirect")
print("                  string_match:")
print('                    exact: "true"')
print(f"              end_session_endpoint: {end_session_endpoint or ''}")
print("              forward_bearer_token: true")
print("              pass_through_matcher:")
print("                - name: authorization")
print("                  string_match:")
print("                    prefix: 'Bearer '")
print("                - name: x-bypass-login")
print("                  string_match:")
print('                    exact: "true"')
print("              redirect_path_matcher:")
print("                path:")
print(f"                  exact: {redirect_path}")
print(f"              redirect_uri: https://%REQ(:authority)%{redirect_path}")
print("              retry_policy: {}")
print("              signout_path:")
print("                path:")
print(f"                  exact: {logout_path}")
print("              token_endpoint:")
print("                cluster: oauth")
print("                timeout: 5s")
print(f"                uri: {token_endpoint}")
print("              use_refresh_token: true")
print("  workloadSelector:")
print("    labels:")
for k, v in match_labels.items():
    print(f"      {k}: {v}")