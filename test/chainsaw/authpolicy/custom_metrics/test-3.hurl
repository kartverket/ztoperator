GET http://localhost:8181/metrics
HTTP/1.1 200
[Asserts]
body not matches /ztoperator_authpolicy_info\{auto_login_enabled="true",enabled="true",issuer="http:\/\/mock-oauth2\.auth",name="auth-policy",namespace="chainsaw-custom-metrics",owner="tilgangsstyring",protected_pod="application[a-z0-9-]+",state="Ready"\}\s*1/

GET http://localhost:8181/metrics
HTTP/1.1 200
[Asserts]
body matches /ztoperator_authpolicy_info\{auto_login_enabled="true",enabled="true",issuer="http:\/\/mock-oauth2\.auth",name="other-auth-policy",namespace="chainsaw-custom-metrics",owner="tilgangsstyring",protected_pod="application[a-z0-9-]+",state="Ready"\}\s*1/