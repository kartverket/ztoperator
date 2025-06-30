<p>
  <img src="ztoperator_logo.png" alt="Architecture Diagram" width="600"/>
</p>

Ztoperator is a Kubernetes operator that simplifies and enforces zero trust security for workloads using Istio and OAuth 2.0. 
At the core of Ztoperator is the custom resource definition (CRD) AuthPolicy, which provides an abstraction layer for specifying authentication and authorization rules based on OAuth 2.0.


# Core functionality
Ztoperator provides one CRD, AuthPolicy, which configures authentication and authorization rules towards an arbitrary identity provider supporting OAuth 2.0 and Open ID Connect (OIDC). 
Ztoperator uses Istio RequestAuthentication and AuthorizationPolicy to validate the authenticity of requetss, as well as authorize requests based on JWT claims. 
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
  wellKnownURI: https://example.com/.well-known/openid-configuration
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
  oAuthCredentials:
    clientIDKey: CLIENT_ID
    clientSecretKey: CLIENT_SECRET
    secretRef: oauth-secret
  authRules:
    - paths:
      - /api*
      denyRedirect: true
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

## How Ztoperator works

Ztoperator enforces authentication and authorization of incoming requests towards one of more workloads by utilizing Istio and Envoy's 
CRD's to enrich the capabilities of the Istio sidecar proxy. Under the hood, [`EnvoyFilters`](https://istio.io/latest/docs/reference/config/networking/envoy-filter/) 
is used to enforce OAuth 2.0 authorization code flow, validation of JWT authenticity and enforcement of allow and deny rules based on JWT claims. 
The following figure shows how Ztoperator sets up multiple `EnvoyFilters` in the istio sidecar proxy of a Kubernetes pod.

