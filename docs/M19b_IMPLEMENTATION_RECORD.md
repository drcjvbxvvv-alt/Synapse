# M19b Implementation Record — CI Engine Run Operations HTTP Layer

## Summary

Added the HTTP layer for CI engine run operations. Adapters (`Trigger`,
`GetRun`, `Cancel`, `StreamLogs`, `GetArtifacts`) were already implemented in
M18b–e and M19a; M19b wires them to service methods and HTTP endpoints.

**Status: ✅ Complete**

---

## Routes Added

```
POST   /api/v1/ci-engines/:id/runs                    → TriggerRun
GET    /api/v1/ci-engines/:id/runs/:runId             → GetRun
DELETE /api/v1/ci-engines/:id/runs/:runId             → CancelRun
GET    /api/v1/ci-engines/:id/runs/:runId/logs        → StreamLogs  (?step=<stepID>)
GET    /api/v1/ci-engines/:id/runs/:runId/artifacts   → GetArtifacts
```

All routes sit under the existing `PlatformAdminRequired` group — the same
credential trust level required to configure an engine connection.

---

## What Changed

### Stage 1 — Service run methods (`internal/services/ci_engine_service.go`)

Added five methods that share one internal helper:

**`buildAdapter(ctx, id)`** — loads config from DB via `s.Get(ctx, id)`, then
calls `s.factory.Build(cfg)`. Single source of truth for config lookup + adapter
construction errors.

**`mapEngineError(err, op)`** — translates engine sentinel errors to structured
`*apierrors.AppError` values:

| Engine sentinel      | HTTP status | AppError code                |
|----------------------|-------------|------------------------------|
| `ErrNotFound`        | 404         | `CI_ENGINE_RUN_NOT_FOUND`    |
| `ErrInvalidInput`    | 400         | `CI_ENGINE_RUN_INVALID_INPUT`|
| `ErrUnavailable`     | 503         | `CI_ENGINE_UNAVAILABLE`      |
| `ErrUnsupported`     | 501         | `CI_ENGINE_NOT_SUPPORTED`    |

**`TriggerRun`** / **`GetRun`** / **`CancelRun`** / **`StreamLogs`** /
**`GetArtifacts`** — each validates the minimal required input (non-nil
request, non-empty runID), calls `buildAdapter`, delegates to the adapter, and
maps errors.

Also fixed pre-existing `ci_engine_service_argo_test.go` and
`ci_engine_service_tekton_test.go`: both `fakeArgoResolver` / `fakeTektonResolver`
were missing `Kubernetes()` after the M19a interface extension; added the
method + `k8sfake` import. Updated stale `SupportsLiveLog: false` assertions to
`true` (changed in M19a).

New test file: `internal/services/ci_engine_service_run_test.go` — 18 tests
covering `mapEngineError` and all five service methods (error and happy paths).

### Stage 2 — Handler run methods (`internal/handlers/ci_engine_handler.go`)

Five handler methods added following the standard 5-step handler flow
(parse → ctx → call service → map error → respond):

- **`TriggerRun`**: binds `engine.TriggerRequest` from JSON body, injects
  `user_id` from auth context as `TriggeredByUser`, returns 201 Created.
- **`GetRun`**: reads `:runId` path param, returns 200 with `RunStatus`.
- **`CancelRun`**: reads `:runId`, returns 204 No Content on success.
- **`StreamLogs`**: reads `?step=` query param (empty = auto-select),
  streams log bytes via `io.Copy(c.Writer, rc)` with `Content-Type: text/plain`.
  Timeout: 5 minutes (longer than normal to accommodate slow pods).
- **`GetArtifacts`**: returns artifact list via `response.List`.

New test file: `internal/handlers/ci_engine_handler_run_test.go` — 18 tests.

### Stage 3 — Routes (`internal/router/routes_ci_engine.go`)

Run routes registered as a sub-group of the existing admin group:

```go
runs := admin.Group("/:id/runs")
runs.POST("", h.TriggerRun)
runs.GET("/:runId", h.GetRun)
runs.DELETE("/:runId", h.CancelRun)
runs.GET("/:runId/logs", h.StreamLogs)
runs.GET("/:runId/artifacts", h.GetArtifacts)
```

---

## Design Decisions

### Admin-only run operations

Run operations inherit `PlatformAdminRequired` from the parent group. Triggering
or cancelling a run on an external engine uses stored credentials (token/password)
so it requires the same trust level as configuring the engine.

### StreamLogs as plain chunked response

`StreamLogs` pipes the adapter's `io.ReadCloser` directly to the HTTP response
with `io.Copy`. This works correctly for all current adapters (all use
`Follow: false` / snapshot mode). A future SSE/chunked upgrade can extend this
endpoint without breaking the contract.

### `stepID` via query param `?step=`

The `stepID` for `StreamLogs` is passed as a query parameter rather than a path
segment. Path segments imply stable resource identity; step IDs can contain
slashes (Tekton: `<taskrun>/<container>`) which makes path encoding fragile.

---

## Test Results

```
ok  internal/services   17.5s
ok  internal/handlers   17.6s
```

All tests pass with `-race`.
