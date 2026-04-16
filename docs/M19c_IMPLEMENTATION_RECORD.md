# M19c Implementation Record — CI Engine Log Viewer (Frontend)

## Summary

Added the frontend Log Viewer for CI engine run operations. Three stages:
- Stage 1: Fix service layer and add run types/methods
- Stage 2: `CIEngineRunViewer` drawer with logs, steps, artifacts tabs
- Stage 3: `TriggerRunModal` + integration into `CIEngineSettings`

**Status: ✅ Complete**

---

## What Changed

### Stage 1 — `ciEngineService.ts`

**Fixed incorrect `request(url)` calls** — the old service called `request('/api/v1/...')` as a function, but `api.ts` exports `request` as an object `{ get, post, put, delete, patch }`. All existing calls were rewritten:

- `request('/api/v1/ci-engines')` → `request.get('/ci-engines')`
- `request('/api/v1/ci-engines', { method: 'POST', body: ... })` → `request.post('/ci-engines', req)`
- etc.

Also removed `/api/v1` prefix from all paths since `api.ts` baseURL is already `/api/v1`.

**Added run operation types:**

```typescript
export type RunPhase = 'pending' | 'running' | 'success' | 'failed' | 'cancelled' | 'unknown';
export const RUN_PHASE_TERMINAL: ReadonlySet<RunPhase>  // {success, failed, cancelled}

export interface TriggerRunRequest { ref?, variables?, pipeline_id? }
export interface TriggerRunResult  { run_id, external_id?, url?, queued_at }
export interface StepStatus        { name, phase, raw?, started_at?, finished_at? }
export interface RunStatus         { run_id, external_id?, phase, raw?, message?, started_at?, finished_at?, steps? }
export interface Artifact          { name, kind, url?, size_bytes?, digest?, created_at }
```

**Added run API methods:**

```typescript
triggerRun(id, req)           → POST /ci-engines/:id/runs
getRun(id, runId)             → GET  /ci-engines/:id/runs/:runId
cancelRun(id, runId)          → DELETE /ci-engines/:id/runs/:runId
fetchLogs(id, runId, step?)   → GET  /ci-engines/:id/runs/:runId/logs?step=
getArtifacts(id, runId)       → GET  /ci-engines/:id/runs/:runId/artifacts
```

`fetchLogs` uses `responseType: 'text'` since the backend returns `text/plain`.
`runId` is URI-encoded via `encodeURIComponent` in all run methods to handle Tekton-style IDs with slashes.

### Stage 2 — `CIEngineRunViewer.tsx`

Drawer (`width=800`) showing:

- **Header**: run ID + phase Tag + animated Badge while running; refresh + cancel buttons in `extra`
- **Cancel**: `Popconfirm` → `DELETE` call; disabled once terminal
- **Logs tab**: `text/plain` snapshot rendered in terminal dark theme (`#1e1e1e` bg per CLAUDE.md §1.4 exception); polls every 3 s while run is active, stops when terminal
- **Steps tab**: table with step name, phase Tag, start/end times
- **Artifacts tab**: table with name, kind Tag, size (formatted), created_at

**Polling via React Query `refetchInterval`:**

```typescript
refetchInterval: (query) => {
  const phase = query.state.data?.phase;
  return phase && RUN_PHASE_TERMINAL.has(phase) ? false : 3_000;
},
```

Both run status and logs use this pattern — polling stops automatically when the run phase becomes terminal.

### Stage 3 — `TriggerRunModal.tsx` + `CIEngineSettings.tsx`

**`TriggerRunModal`**: Modal (`width=480`) with:
- `ref` field (Branch / Tag / Commit, optional)
- `variables` textarea (KEY=VALUE per line, parsed to `Record<string,string>`)
- On success: `message.success` + calls `onTriggered(runId)` → parent opens RunViewer

**`CIEngineSettings.tsx` changes:**
- Added `PlayCircleOutlined` icon import
- Added `triggerTarget` and `runViewer` state
- Added trigger button (▶) to the table actions column (before edit)
- Added `<TriggerRunModal>` and `<CIEngineRunViewer>` to the render section
- `onTriggered` callback chains trigger → viewer open automatically

### i18n keys added

New `ciEngine.runViewer.*` and `ciEngine.triggerRun.*` keys added to all three locales (`zh-TW`, `zh-CN`, `en-US`).

---

## Design Decisions

### Polling not SSE

Backend `StreamLogs` returns a `text/plain` snapshot (not a streaming response). Using React Query `refetchInterval` to poll every 3 s while the run is active is the correct approach. No SSE/EventSource needed at this layer.

### `encodeURIComponent` on runId

Tekton run IDs contain slashes (`<taskrun>/<container>`). Encoding prevents URL misrouting.

### `fetchLogs` with `responseType: 'text'`

Axios by default parses `application/json`; without `responseType: 'text'`, the plain-text log body would be returned as-is but could be misinterpreted. Explicit `responseType: 'text'` ensures the string is returned correctly.

### Terminal log colours hardcoded

Per CLAUDE.md §1.4: terminal components are allowed to hardcode dark theme colours (`#1e1e1e`, `#d4d4d4`). `theme.useToken()` tokens are used everywhere else.

---

## Test Results

```
TypeScript: npx tsc --noEmit → 0 errors
```
