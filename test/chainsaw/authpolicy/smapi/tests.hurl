POST {{ baseUrl }}/api/oppdateringer
Authorization: Bearer {{admin}}
Host: foo.bar
HTTP 200

POST {{ baseUrl }}/api/oppdateringer
Host: foo.bar
HTTP 403

POST {{ baseUrl }}/api/oppdateringer
Authorization: Bearer {{user}}
Host: foo.bar
HTTP 403

POST {{ baseUrl }}/api/oppdateringer
Authorization: Bearer {{backstage}}
Host: foo.bar
HTTP 403

POST {{ baseUrl }}/api/oppdateringer/trends/vulnerabilities
Authorization: Bearer {{admin}}
Host: foo.bar
HTTP 200

POST {{ baseUrl }}/api/oppdateringer/trends/vulnerabilities
Host: foo.bar
HTTP 403

POST {{ baseUrl }}/api/oppdateringer/trends/vulnerabilities
Authorization: Bearer {{user}}
Host: foo.bar
HTTP 403

POST {{ baseUrl }}/api/oppdateringer/trends/vulnerabilities
Authorization: Bearer {{backstage}}
Host: foo.bar
HTTP 403

GET {{ baseUrl }}/api/oppdateringer
Authorization: Bearer {{admin}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/oppdateringer
Authorization: Bearer {{user}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/oppdateringer
Authorization: Bearer {{backstage}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/oppdateringer
Host: foo.bar
HTTP 403

GET {{ baseUrl }}/api/metrikker
Authorization: Bearer {{admin}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/metrikker
Authorization: Bearer {{user}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/metrikker
Authorization: Bearer {{backstage}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/metrikker
Host: foo.bar
HTTP 403

GET {{ baseUrl }}/api/metrikker/utskrift-server
Authorization: Bearer {{admin}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/metrikker/utskrift-server
Authorization: Bearer {{user}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/metrikker/utskrift-server
Authorization: Bearer {{backstage}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/metrikker/utskrift-server
Host: foo.bar
HTTP 403

GET {{ baseUrl }}/dummy/vulnerabilities/shipwreck
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/dummy/vulnerabilities/shipwreck
Authorization: Bearer {{backstage}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/securityChampion
Authorization: Bearer {{admin}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/securityChampion
Authorization: Bearer {{user}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/securityChampion
Authorization: Bearer {{backstage}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/securityChampion
Host: foo.bar
HTTP 403

GET {{ baseUrl }}/api/securityChampion/workMail
Authorization: Bearer {{admin}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/securityChampion/workMail
Authorization: Bearer {{backstage}}
Host: foo.bar
HTTP 200

GET {{ baseUrl }}/api/securityChampion/workMail
Authorization: Bearer {{user}}
Host: foo.bar
HTTP 403

GET {{ baseUrl }}/api/securityChampion/workMail
Host: foo.bar
HTTP 403

POST {{ baseUrl }}/api/securityChampion/update
Authorization: Bearer {{admin}}
Host: foo.bar
HTTP 200

POST {{ baseUrl }}/api/securityChampion/update
Authorization: Bearer {{user}}
Host: foo.bar
HTTP 403

POST {{ baseUrl }}/api/securityChampion/update
Authorization: Bearer {{backstage}}
Host: foo.bar
HTTP 403

POST {{ baseUrl }}/api/securityChampion/update
Host: foo.bar
HTTP 403
