<p>
  <img src="ztoperator_logo.png" alt="Architecture Diagram" width="600"/>
</p>

ZToperator is a Kubernetes operator that simplifies and enforces zero trust security for workloads using Istio and OAuth 2.0. 
At the core of ZToperator is the custom resource definition (CRD) AuthPolicy, which provides an abstraction layer for specifying authentication and authorization rules based on OAuth 2 tokens.


# Core functionality
ZToperator provides one CRD, AuthPolicy, which configures valid JWT issuers and authorization rules using Istio RequestAuthentication and AuthorizationPolicy.
Optionally, an [OAuth EnvoyFilter](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/oauth2_filter) 
can also be created to support login using the authorization code flow. 

Example AuthPolicy:
```yaml
apiVersion: ztoperator.kartverket.no/v1alpha1
kind: AuthPolicy
metadata:
  name: auth-policy
spec:
  selector:
    matchLabels:
      app: some-app
  enabled: true
  wellKnownUri: https://example.com/.well-known/openid-configuration
  audience:
    - example-audience
  acceptedResources:
   - https://some-app.com
  autoLogin:
    enabled: true
    logoutPath: /logout
    redirectPath: /oauth2/callback
    scopes:
      - openid
      - profile
  authRules:
    - paths:
      - /api
      methods:
      - GET
      when:
        - claim: acr
          values:
            - Level4
    - paths:
      - /admin
      methods:
      - GET
      - POST
      - PUT
      when:
      - claim: role
        values:
          - "admin"
  ignoreAuthRules:
    - paths:
      - /public
      methods:
      - GET
```

## Local development
Please refer to [CONTRIBUTING.md](CONTRIBUTING.md) on how to run and test ztoperator locally.