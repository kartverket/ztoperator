local ignore_rules = %s
local require_rules = %s
local deny_redirect_rules = %s
local authorize_endpoint = "%s"
local login_params = %s
local end_session_endpoint = "%s"
local post_logout_redirect_uri = "%s"
local logout_path = "%s"

-- returns true when {p,m} matches any rule in the supplied table
local function match(rules, p, m)
    for _, rule in ipairs(rules) do
        if string.match(p, rule.regex) then
        -- empty "methods" table == all methods
            if next(rule.methods) == nil or rule.methods[m] then
                return true
            end
        end
    end
    return false
end

-- returns true if {p,m} is in ignore_rules *and* NOT in require_rules
local function should_bypass(p, m)
    local bypass = false
    if p ~= "" and m ~= "" then
        -- bypass only when it is in ignore_rules *and* NOT in require_rules
        if match(ignore_rules, p, m) and not match(require_rules, p, m) then
            bypass = true
        end
    end
    return bypass
end

-- returns true if {p,m} is in deny_redirect_rules
local function should_deny_redirect(p, m)
    local deny_redirect = false
    if p ~= "" and m ~= "" then
        -- deny redirect only when it is in deny_redirect_rules
        if match(deny_redirect_rules, p, m) then
            deny_redirect = true
        end
    end
    return deny_redirect
end

local function is_empty_table(t)
    return type(t) ~= "table" or next(t) == nil
end

-- splits "host[:port]" (including bracketed IPv6) into a lowercase host and a port, defaulting the port when omitted
local function split_host_port(host_port, default_port)
    local hp = string.lower((string.gsub(host_port, "^[^@]*@", "")))
    local host, port = string.match(hp, "^(%%[[^%%]]*%%]):(%%d+)$")
    if host == nil then
        host, port = string.match(hp, "^([^:]+):(%%d+)$")
    end
    if host == nil then
        return hp, default_port
    end
    return host, port
end

-- compares the origin (scheme, host, effective port) of an Origin/Referer value with the request authority
local function is_same_origin(source, authority)
    local scheme, source_host_port = string.match(string.lower(source), "^(%%a[%%w+.-]*)://([^/?#]*)")
    if scheme ~= "http" and scheme ~= "https" then
        return false
    end
    local default_port = scheme == "https" and "443" or "80"
    local source_host, source_port = split_host_port(source_host_port, default_port)
    local authority_host, authority_port = split_host_port(authority, default_port)
    return source_host == authority_host and source_port == authority_port
end

-- The Envoy OAuth2 filter signs the user out on any request to the logout path, regardless of method and cookies.
-- Reject cross-site requests so that a third-party site cannot log the user out (logout CSRF).
local function is_cross_site(headers)
    local site = headers:get("sec-fetch-site")
    if site ~= nil and site ~= "" then
        return site == "cross-site"
    end
    local source = headers:get("origin") or headers:get("referer")
    if source == nil or source == "" then
        return false
    end
    return not is_same_origin(source, headers:get(":authority") or "")
end

function envoy_on_request(request_handle)
    local raw_p = request_handle:headers():get(":path") or ""
    local m = request_handle:headers():get(":method") or ""
    local p = string.match(raw_p, "^[^?]*")

    if logout_path ~= "" and p == logout_path and is_cross_site(request_handle:headers()) then
        request_handle:respond({[":status"] = "403"}, "")
        return
    end

    local bypass = should_bypass(p, m)
    request_handle:headers():add("%s", tostring(bypass))

    local deny_redirect = should_deny_redirect(p, m)
    request_handle:headers():add("%s", tostring(deny_redirect))
end

function envoy_on_response(response_handle)
    local status = response_handle:headers():get(":status") or ""
    if status == "302" then
        local loc = response_handle:headers():get("location") or ""
        if loc ~= "" then
            if string.sub(loc, 1, #authorize_endpoint) == authorize_endpoint and not is_empty_table(login_params) then
                local base, qs = loc:match("^([^?]+)%%??(.*)$")
                local filtered = {}
                if qs ~= "" then
                    local params = {}
                    for key, val in string.gmatch(qs, "([^&=?]+)=([^&=?]+)") do
                        params[key] = val
                    end
                    for k, v in pairs(login_params) do
                        params[k] = v
                    end
                    for k, v in pairs(params) do
                        table.insert(filtered, k .. "=" .. v)
                    end
                end

                local new_qs = table.concat(filtered, "&")
                local new_url = base .. (new_qs ~= "" and ("?" .. new_qs) or "")
                response_handle:headers():replace("location", new_url)
            end

            if string.sub(loc, 1, #end_session_endpoint) == end_session_endpoint then
                local base, qs = loc:match("^([^?]+)%%??(.*)$")
                local filtered = {}
                if qs ~= "" then
                    local params = {}
                    for key, val in string.gmatch(qs, "([^&=?]+)=([^&=?]+)") do
                        params[key] = val
                    end
                    if post_logout_redirect_uri == "" then
                        params["post_logout_redirect_uri"] = nil
                    else
                        params["post_logout_redirect_uri"] = post_logout_redirect_uri
                    end
                    for k, v in pairs(params) do
                        table.insert(filtered, k .. "=" .. v)
                    end
                end

                local new_qs = table.concat(filtered, "&")
                local new_url = base .. (new_qs ~= "" and ("?" .. new_qs) or "")
                response_handle:headers():replace("location", new_url)
            end
        end
    end
end