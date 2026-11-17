# Validating pod webhook also validates pods with the inject annotation

## Status

Accepted. Partly supersedes [2026-11-05-inject-annotation-for-pod-mutation](2026-11-05-inject-annotation-for-pod-mutation.md),
which described `verify-authpolicy` as the only trigger for the validating webhook.

Deciders: Team tilgangsstyring

## Context

The mutating webhook mounts the Envoy secret in `istio-proxy` when `ztoperator.kartverket.no/inject: "true"`, and the
validating webhook checked the final pod spec only when `ztoperator.kartverket.no/verify-authpolicy: "true"`. A pod with
only `inject` was therefore mutated but never verified. If the mount did not happen, for example because Istio did not
inject the sidecar, the pod was admitted without a working auto-login, and if it opted out of sidecar injection, the
AuthPolicy was not enforced.

## Decision

The validating webhook runs `validation.ValidatePod` when either `inject` or `verify-authpolicy` is `"true"`. The
checks are the same in both cases: the pod must not opt out of sidecar injection while the AuthPolicy is enabled, and
when auto-login is in effect `istio-proxy` must mount the `<authpolicy-name>-envoy-secret` at `/etc/istio/config`. The
mutating webhook is unchanged and still acts only on `inject`.

## Consequences

- Using `inject` alone is safe: the result of the mutation is always verified after all mutating webhooks have run.
- `verify-authpolicy` is still useful on its own, to verify pods that are configured by hand or through the legacy
  `sidecar.istio.io/userVolume` annotations.
- Pods with `inject` that previously were admitted with a missing mount or disabled sidecar injection are now rejected
  with a clear message.
