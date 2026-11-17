# Reject unsupported Ztoperator pod annotations and non-boolean values

## Status

Accepted

Deciders: Team tilgangsstyring

## Context

Both pod webhooks only act when `ztoperator.kartverket.no/inject` or `ztoperator.kartverket.no/verify-authpolicy` is
exactly `"true"`. Previously any other value, such as `"True"`, `"yes"` or `"1"`, was silently treated as disabled. A
misspelled key under the `ztoperator.kartverket.no/` prefix, such as `verfy-authpolicy`, made the pod eligible for
webhook processing but had no effect. In both cases the user believes the AuthPolicy is injected or verified when it
is not, which is a silent security failure.

## Decision

Both the mutating and the validating webhook reject a pod, after the eligibility check, if:

- it has an annotation under the `ztoperator.kartverket.no/` prefix other than `inject` and `verify-authpolicy`, or
- the value of `inject` or `verify-authpolicy` is anything other than `"true"` or `"false"`.

The error names the offending annotation and lists the supported keys or values. Annotations outside the prefix are
ignored. The check is `ValidateAnnotations` in `internal/webhook/v1/pod_webhook.go`.

## Consequences

- Typos and invalid values fail loudly at admission instead of silently disabling enforcement.
- Adding a new annotation under the prefix requires adding it to the supported list in `validateAnnotations`, otherwise
  pods using it are rejected.
- Existing workloads with an invalid value or an unknown key are rejected the next time their pods are created. This
  only affects Skiperator-managed pods in `skip.kartverket.no/skip-managed` namespaces, since other pods are never
  inspected.
