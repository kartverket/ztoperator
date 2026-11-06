# Use a separate `ztoperator.kartverket.no/inject` annotation to opt pods into mutation

## Status

Accepted

Deciders: Team tilgangsstyring

## Context

The pod webhooks are triggered by the `ztoperator.kartverket.no/verify-authpolicy: "true"` annotation. It was
introduced as an opt-in check for teams that want pod creation to fail unless the application is protected by an
AuthPolicy, so it describes validation only.

Ztoperator is gaining a mutating pod webhook that mounts the Envoy secret in the `istio-proxy` sidecar, replacing
the manual `sidecar.istio.io/userVolume` and `userVolumeMount` annotations required for `autoLogin`. Mutating pods is
work users otherwise do themselves, not a check, so `verify-authpolicy` is the wrong name for the trigger. We also
expect to inject our own Envoy sidecar and configuration if Istio ambient mode is introduced.

## Decision

Add the annotation `ztoperator.kartverket.no/inject: "true"` as the trigger for the mutating webhook. It follows the
`sidecar.istio.io/inject` convention and stays accurate if we inject more than a secret mount later.

`verify-authpolicy` keeps its current meaning and is not an alias of `inject`:

- `inject: "true"`: the mutating webhook looks up the pod's AuthPolicy and mounts the Envoy secret when `autoLogin` is
  enabled.
- `verify-authpolicy: "true"`: the validating webhook looks up the pod's AuthPolicy and validates the final pod spec,
  for example that `istio-proxy` mounts the Envoy secret.

Both annotations make the webhook require a ready AuthPolicy for the application, so a pod with either annotation is
rejected if there is none. Pods with neither annotation are untouched.

## Consequences

- Teams must add `inject: "true"` to get the mount from the webhook, and can remove the Istio annotations. Teams that
  keep the Istio annotations are unaffected, since the webhook does nothing when they are present.
- Setting `autoLogin.enabled: false` no longer breaks pod startup because of a missing Envoy secret, as long as the
  Istio annotations are removed: the webhook only mounts the secret when `autoLogin` is enabled.
- Teams using only `verify-authpolicy` see no change in behaviour.
- Teams should usually set both annotations: `inject` to mount the secret and `verify-authpolicy` to guarantee the
  final pod is protected.
