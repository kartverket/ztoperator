GET http://localhost:8181/metrics
HTTP/1.1 200
[Asserts]
body contains "ztoperator_authpolicy_info{auto_login_enabled=\"true\",enabled=\"true\",issuer=\"http://mock-oauth2.auth\",name=\"auth-policy\",namespace=\"chainsaw-custom-metrics\",owner=\"tilgangsstyring\",protected_skiperator_application=\"application\",state=\"Failed\"} 1"