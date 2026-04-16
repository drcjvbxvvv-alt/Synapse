# M19a Implementation Record — StreamLogs (Tekton & Argo)

## Summary

Implemented live Pod log streaming for the Tekton and Argo Workflows CI engine
adapters. Both adapters previously returned `ErrUnsupported` for `StreamLogs`;
they now stream container logs directly from the Kubernetes API.

**Status: ✅ Complete**

---

## What Changed

### Stage 1 — ClusterResolver interface extension

Both `tekton.ClusterResolver` and `argo.ClusterResolver` now include:

```go
Kubernetes(clusterID uint) (kubernetes.Interface, error)
```

The shared `router.k8sClusterResolver` struct gained the matching
implementation (returns the cluster's existing typed clientset). Because Go
uses structural interface satisfaction, the single router-layer struct
satisfies both adapter interfaces without code duplication.

Files modified:
- `internal/services/pipeline/engine/tekton/cluster.go`
- `internal/services/pipeline/engine/argo/cluster.go`
- `internal/router/k8s_cluster_resolver.go`
- `internal/services/pipeline/engine/tekton/adapter_test.go` (fakeResolver)
- `internal/services/pipeline/engine/argo/adapter_test.go` (fakeResolver)

### Stage 2 — Tekton StreamLogs

**`internal/services/pipeline/engine/tekton/logs.go`** — replaced stub.

Flow:
1. Validate `runID`, `resolver`, `namespace`.
2. `resolveStepID(ctx, dyn, ns, runID, stepID)`:
   - Non-empty stepID must be `"<taskrun-name>/<container-name>"`.
   - Empty stepID lists TaskRuns by `tekton.dev/pipelineRun=<runID>` label
     and picks the TaskRun with the smallest name (alphabetical), using
     `spec.steps[0].name` → `"step-<name>"` as the container.
3. GET the TaskRun to read `status.podName`.
4. `resolver.Kubernetes()` → stream `CoreV1().Pods().GetLogs()` with
   `container=<containerName>, follow=false`.

Key helpers:
- `resolveStepID` — stepID parsing + auto-select logic
- `readPodName(obj)` — reads `status.podName` from unstructured
- `firstStepContainer(obj)` — reads `spec.steps[0].name` → `"step-<name>"`

**`internal/services/pipeline/engine/tekton/logs_test.go`** — replaced stub (2 tests → 19 tests).

Testing approach: `httptest.NewServer` serves the real Kubernetes pod log API
path (`/api/v1/namespaces/<ns>/pods/<pod>/log`). A real `kubernetes.Clientset`
built from `rest.Config{Host: testServer.URL}` issues the actual HTTP GET — no
mocking of the streaming code path.

### Stage 3 — Argo StreamLogs

**`internal/services/pipeline/engine/argo/logs.go`** — replaced stub.

Flow:
1. Validate `runID`, `resolver`, `namespace`.
2. GET the Workflow object via `dyn.Resource(gvrWorkflow)`.
3. `resolveArgoNodeID(dyn, ns, runID, stepID, wfObject)`:
   - Non-empty stepID matched against `node.displayName` first, then `node.id`
     (the map key). Both are valid step references.
   - Empty stepID selects the first `type=Pod` node (alphabetical by id) for
     stable auto-selection.
   - The node id equals the Kubernetes pod name Argo creates.
4. `resolver.Kubernetes()` → stream `CoreV1().Pods().GetLogs()` with
   `container="main", follow=false`.

Key helpers:
- `resolveArgoNodeID` — stepID resolution within Workflow status.nodes
- `argoNodes(obj)` — extracts `status.nodes` map from unstructured

**`internal/services/pipeline/engine/argo/logs_test.go`** — replaced stub (2 tests → 20 tests).

Same `httptest.Server` pattern as Tekton.

### Stage 4 — Capabilities + cleanup

Updated `SupportsLiveLog: true` in:
- `internal/services/pipeline/engine/tekton/adapter.go`
- `internal/services/pipeline/engine/argo/adapter.go`

Updated matching capability contract tests in:
- `internal/services/pipeline/engine/tekton/adapter_test.go`
- `internal/services/pipeline/engine/argo/adapter_test.go`

---

## Architecture Decisions

### Node id = Pod name in Argo

Argo Workflows creates a Kubernetes pod whose name equals the workflow node id.
This means no additional GET is needed to map from node → pod, unlike Tekton
where `TaskRun.status.podName` must be fetched explicitly.

### Container name conventions

| Adapter  | Container name          | Rationale                                                            |
|----------|-------------------------|----------------------------------------------------------------------|
| Tekton   | `step-<step-name>`      | Tekton prefixes all step containers with `step-`                     |
| Argo     | `main`                  | Argo user workload always runs in `main`; `wait`/`init` are sidecars |

### Stable auto-selection

Both adapters use alphabetical-by-name ordering when `stepID` is empty.
This is deterministic across calls and requires no additional state.

### No mock for pod log streaming

The Kubernetes `GetLogs().Stream()` call goes through the real HTTP client
stack. `httptest.Server` + `rest.Config{Host: srv.URL}` is the correct testing
pattern — mocking the streaming interface would test the mock, not the code.

---

## Test Results

```
ok  internal/services/pipeline/engine/tekton   5.961s
ok  internal/services/pipeline/engine/argo     4.004s
ok  internal/services/pipeline/engine/github   3.339s
ok  internal/services/pipeline/engine/gitlab   3.564s
ok  internal/services/pipeline/engine/jenkins  4.083s
ok  internal/services/pipeline/engine          2.005s
```

All tests pass with `-race`.
