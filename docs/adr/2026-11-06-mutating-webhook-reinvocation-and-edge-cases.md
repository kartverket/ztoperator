# Mutating pod webhook relies on reinvocation and ignores unrealistic edge cases

## Status

Accepted

Deciders: Team tilgangsstyring

## Context

The mutating pod webhook mounts the Envoy secret (`<authpolicy-name>-envoy-secret`) at `/etc/istio/config` in the
`istio-proxy` sidecar, replacing the manual `sidecar.istio.io/userVolume` and `userVolumeMount` annotations. Istio's
injector is another mutating webhook, so `istio-proxy` may or may not exist when our webhook runs. Several edge cases
were considered.

## Decision

- **Reinvocation:** the webhook sets `reinvocationPolicy: IfNeeded`. If `istio-proxy` is not yet present, the webhook
  does nothing. If Istio then mutates the pod after our webhook, the API server calls our webhook once more and it
  patches the injected container, in `containers` or in `initContainers` for native sidecars. Each webhook runs at most
  twice per request, so this cannot loop, and the mutation is idempotent because it is skipped when the secret is
  already mounted.
- **No placeholder container:** we do not add a placeholder `istio-proxy` container with `image: auto`. It would only
  be needed if our webhook ran before Istio's, and it leaves a container with an unpullable image if Istio never
  injects.
- **No handling of foreign volumes:** we do not check for an existing volume named `ztoperator-envoy-secret`, or for
  another volume mounted at `/etc/istio/config` in `istio-proxy`. Neither happens in practice. Istio does not use that
  path, and the API server rejects duplicate mount paths. `verify-authpolicy` also rejects a pod that does not mount
  the right secret.
- **Skipped cases:** the webhook does nothing when auto-login is not in effect, when the pod has
  `sidecar.istio.io/inject: "false"`, when the Istio annotations are set, or when the secret is already mounted.

## Consequences

- The webhook is small and has no error paths of its own.
- If Istio never injects the sidecar, the pod is left unchanged. With `verify-authpolicy` it is rejected with a clear
  message that the secret is not mounted.
- The mutation depends on Istio injecting the sidecar before or after our webhook. If a cluster setup changes this,
  for example with a different injector, the end-to-end test `auto_login_webhook_mount` should catch it.
- If one of the unhandled edge cases turns out to occur, it should be handled with a clear error in the webhook, and
  this decision updated by a new ADR.
