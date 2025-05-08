# ztoperator
Ztoperator is a Kubernetes operator for managing zero trust through authentication and authorization in a Kubernetes cluster utilizing Istio as service mesh.

# Core functionality
Provides a CRD, AuthPolicy, for configuring valid JWT issuers and authorization rules using Istio RequestAuthentication and AuthorizationPolicy.

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
  rules:
    - enabled: true
      issuerURI: https://example.com
      jwksURI: https://example.com/jwks
      audience: example-audience
      authRules:
        - paths:
            - /api
          methods:
            - GET
          when:
            - claim: sub
              values:
                - "*"
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

## Running integration tests locally
Set up a local kind cluster and install dependencies:
```bash
make setup-local install-skiperator install-mock-oauth2 virtualenv expose-ingress
```
Then run all tests:
```bash
make run-test
```
You can also run specific tests. This will require you to run the ztoperator controller seperately in another terminal or in your IDE.
```bash
# Run ztoperator in IDE or on another terminal
make run-local

# Run specific test
dir=test/chainsaw/authpolicy make test-single
