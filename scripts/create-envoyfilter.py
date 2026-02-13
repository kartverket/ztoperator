import json
import re
import sys
import urllib.request
from pathlib import Path
from urllib.parse import quote_plus, urlparse

import yaml
from jinja2 import Environment, FileSystemLoader

ap = yaml.safe_load(sys.stdin)

metadata = ap.get("metadata") or {}
spec = ap.get("spec") or {}

ap_name = metadata.get("name", "auth-policy")
script_dir = Path(__file__).resolve().parent

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
            # we need to escape '-' because it is a special character in Lua pattern matching
            regex = regex.replace("-", "%-")
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
    for k, v in sorted(mapping.items()):
        parts.append(f'["{k}"]="{v}"')
    return "{" + ",".join(parts) + "}"


# Extract rules ----------------------------------------------------

ignore_matchers = spec.get("ignoreAuthRules") or []
auth_matcher_raw = spec.get("authRules") or []
auth_matcher = []
for m in auth_matcher_raw:
    if isinstance(m, dict):
        # filter out optional 'when' clauses
        mm = {k: v for k, v in m.items() if k != "when"}
        auth_matcher.append(mm)

auto = spec.get("autoLogin") or {}
login_path = auto.get("loginPath")
redirect_path = auto.get("redirectPath") or "/oauth2/callback"
logout_path = auto.get("logoutPath") or "/logout"

require_matchers = auth_matcher  # authRules are the "require" rules in the Lua script
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

login_params_raw = auto.get("loginParams") or {}
# URL-encode each login param value using + for spaces
login_params_encoded = {k: quote_plus(str(v), safe="") for k, v in login_params_raw.items()}
# Sort login params alphabetically on key for deterministic Lua output
login_params = {k: login_params_encoded[k] for k in sorted(login_params_encoded.keys())}

raw_post_logout = auto.get("postLogoutRedirectUri", "")
post_logout = quote_plus(raw_post_logout, safe="")

login_params_lua = lua_table_from_map(login_params)

scopes = auto.get("scopes") or ["openid", "offline_access", "User.Read"]
match_labels = (spec.get("selector") or {}).get("matchLabels") or {}
accepted_resources_raw = spec.get("acceptedResources")

if isinstance(accepted_resources_raw, list):
    resources = [str(r) for r in accepted_resources_raw]
elif accepted_resources_raw is not None:
    raise SystemExit("spec.acceptedResources must be a list of strings")
else:
    resources = []

# Load external Lua file template with %s placeholders
lua_file_path = script_dir.parent / "pkg" / "luascript" / "ztoperator.lua"
with lua_file_path.open("r") as f:
    lua_template = f.read()

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

template_dir = script_dir / "templates"
env = Environment(
    loader=FileSystemLoader(str(template_dir)),
    autoescape=False,
    trim_blocks=True,
    lstrip_blocks=True,
)
template = env.get_template("envoyfilter.yaml.j2")

rendered = template.render(
    ap_name=ap_name,
    lua_indented=lua_indented,
    oauth_host=oauth_host,
    oauth_port=oauth_port,
    use_tls=use_tls,
    scopes=scopes,
    authorize_endpoint=authorize_endpoint,
    end_session_endpoint=end_session_endpoint or "",
    redirect_path=redirect_path,
    logout_path=logout_path,
    token_endpoint=token_endpoint,
    match_labels=match_labels,
    resources=resources,
)

print(rendered, end="")
