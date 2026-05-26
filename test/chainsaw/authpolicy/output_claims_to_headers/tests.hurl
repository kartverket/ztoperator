# --- Expecting 403
GET https://127.0.0.1:8443/random/path
Host: foo.bar
HTTP 403

# --- Expecting 200
GET https://127.0.0.1:8443/random/path
Host: foo.bar
x-token-sub: should-be-replaced-by-token-sub
x-token-role: should-be-replaced-by-token-role
x-token-aud: should-be-replaced-by-token-aud
x-not-in-token-but-in-output-claims: should-be-stripped
x-not-in-token-but-also-not-output-claim: should-be-passed-as-is
Authorization: Bearer {{token}}
HTTP 200
Content-Type: application/json; charset=utf-8
[Asserts]
jsonpath "$.headers.x-token-sub" == "maskinporten_client"
jsonpath "$.headers.x-token-role" == "maskinporten_role"
jsonpath "$.headers.x-token-aud" == "WyJtYXNraW5wb3J0ZW5fc2VydmVyIiwibWFza2lucG9ydGVuX2NsaWVudCJd" # base64 encoded as aud-claim is of type array(string)
jsonpath "$.headers.x-not-in-token-but-in-output-claims" not exists
jsonpath "$.headers.x-not-in-token-but-also-not-output-claim" == "should-be-passed-as-is"
