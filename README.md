# cron-scale-operator

Kubernetes operator for **scheduled scaling**: a `CronScale` custom resource
opens named time windows on a cron schedule, and the controller patches the
replica count of the referenced Deployment for as long as a window is open.

## Why

HorizontalPodAutoscalers react to load but have no notion of the calendar.
Teams that know their traffic shape (weekday business hours, nightly batch,
regional handover) end up with `kubectl scale` in CI jobs or a hand-rolled
CronJob calling the API. Both drift:

- cron jobs in cluster timezones silently shift on DST changes;
- nothing records *which* schedule set the replica count;
- a failed scale leaves no status behind.

`CronScale` keeps the schedule in the API server, evaluates it in an explicit
IANA timezone, clamps the result to declared bounds, and publishes the decision
on the object status. The controller requeues exactly at the next window
boundary instead of polling.

## Architecture

```
CronScale (platform.milan.dev/v1alpha1)
   |  reconcile: compile windows -> select open window -> clamp
   v
CronScaleReconciler --- merge patch spec.replicas ---> Deployment
   |
   +--> status: activeWindow, desiredReplicas, Ready/Degraded conditions
```

```
[todo: architecture diagram placeholder]
```

Ownership is one-directional: the operator only ever writes `spec.replicas`
and its own status. It never scales anything it was not pointed at, and a
suspended object is left untouched.

## CRD

```yaml
apiVersion: platform.milan.dev/v1alpha1
kind: CronScale
metadata:
  name: checkout-api
spec:
  target:
    kind: Deployment      # Deployment or StatefulSet
    name: checkout-api
  timezone: Europe/Prague # IANA name, defaults to UTC
  minReplicas: 2          # floor for every window
  maxReplicas: 40         # ceiling for every window
  windows:
    - name: business
      start: "0 8 * * 1-5"
      end: "0 18 * * 1-5"
      replicas: 24
    - name: night
      start: "0 22 * * *"
      end: "0 6 * * *"
      replicas: 4
```

| Field | Type | Notes |
| --- | --- | --- |
| `spec.target.kind` | string | `Deployment` (default) or `StatefulSet` |
| `spec.target.name` | string | workload in the CronScale namespace |
| `spec.timezone` | string | IANA zone; cron expressions are evaluated in it |
| `spec.minReplicas` / `spec.maxReplicas` | int32 | clamp applied to every window |
| `spec.windows[].start` / `.end` | string | five-field cron, minute resolution |
| `spec.windows[].replicas` | int32 | replica count while the window is open |
| `spec.suspend` | bool | freeze reconciliation |

Semantics: a window is open when its most recent `start` occurrence is later
than its most recent `end` occurrence. When several windows are open the one
that started most recently wins. With no open window the target is held at
`spec.minReplicas`.

Status fields: `activeWindow`, `desiredReplicas`, `observedGeneration`, and
`Ready`/`Degraded` conditions carrying the reason for the last decision.

## Install

Requires a cluster you can install CRDs into and Go 1.22+.

```sh
kubectl apply -f config/crd/bases/platform_v1alpha1_cronscale.yaml
go build -o bin/manager ./cmd
./bin/manager --leader-elect
```

Then apply a sample policy:

```sh
kubectl apply -f config/samples/platform_v1alpha1_cronscale.yaml
kubectl get cronscale checkout-api -o jsonpath='{.status}'
```

Run the evaluator tests with `go test ./...`.

## Layout

```
api/v1alpha1/           CronScale types, scheme registration, deep copies
internal/scale/         cron compilation, window selection, clamping, tests
internal/controller/    reconciler against the scale decision
cmd/                    manager entrypoint
config/crd, config/samples
```

## Limits

- minute-resolution cron only; no sub-minute windows
- no HPA interplay: a CronScale owns `spec.replicas` while it is installed
- windows are evaluated on reconcile plus a requeue at the next boundary, so a
  dropped event can delay a transition until the following boundary

## License

MIT, see [LICENSE](LICENSE).
