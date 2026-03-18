# Management Performance Round 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reduce management-plane CPU, disk IO, and request fan-out in `CLIProxyAPI` and `cdx-rt` without changing protocol or scheduling behavior.

**Architecture:** Add server-side aggregation and short-lived account list caching in `CLIProxyAPI`, then add in-process config/job caches and efficient log tail reads in `cdx-rt`. Keep all changes behind existing APIs where possible so UI behavior stays stable.

**Tech Stack:** Go 1.26, Gin, TypeScript, Node.js 20, embedded HTML admin UIs

---

### Task 1: Add CLIProxyAPI overview aggregation endpoint

**Files:**
- Modify: `internal/api/handlers/management/handler.go`
- Modify: `internal/api/handlers/management/codex_management.go`
- Modify: `internal/api/server.go`
- Test: `internal/api/handlers/management/replenishment_ui_test.go`
- Modify: `internal/managementui/assets/index.html`

**Step 1: Write failing test**

Add a handler test that calls a new `GET /v0/management/overview` route and asserts the response includes:
- `accounts`
- `usage`
- `replenishment`

**Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handlers/management -run TestGetOverview -count=1`

**Step 3: Implement minimal endpoint**

- Add an overview response builder in the management handler.
- Reuse existing list accounts, usage snapshot, and replenishment status methods.
- Register the new route in `server.go`.
- Switch the UI overview refresh path to call the aggregated endpoint once.

**Step 4: Run tests**

Run:
- `go test ./internal/api/handlers/management -count=1`

### Task 2: Add CLIProxyAPI account list caching

**Files:**
- Modify: `internal/api/handlers/management/handler.go`
- Modify: `internal/api/handlers/management/codex_management.go`
- Modify: `internal/api/handlers/management/codex_auth_helpers.go`
- Test: `internal/api/handlers/management/codex_management_test.go`

**Step 1: Write failing test**

Add a test that calls the account list twice within a short interval and verifies expensive entry construction runs only once.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handlers/management -run TestListCodexAccountsUsesShortTTLCache -count=1`

**Step 3: Implement minimal cache**

- Add handler-owned short TTL cache for `[]gin.H`.
- Invalidate on config/auth manager changes conservatively.
- Reuse cached results for overview and account listing.
- Rework `id_token` claim extraction to prefer already-derived values such as `plan_type`.

**Step 4: Run tests**

Run:
- `go test ./internal/api/handlers/management -count=1`

### Task 3: Add cdx-rt config and job state caches

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/src/service/config-store.ts`
- Modify: `/home/alpen/DEV/cdx-rt/src/service/job-store.ts`
- Test: `/home/alpen/DEV/cdx-rt/test/config-store.test.ts`
- Test: `/home/alpen/DEV/cdx-rt/test/job-store.test.ts`

**Step 1: Write failing tests**

Add tests that:
- load config twice without touching disk and expect cache hits to preserve values
- update the store and expect subsequent reads in the same process to use the updated cached state

**Step 2: Run tests to verify they fail**

Run:
- `npm test -- test/config-store.test.ts`
- `npm test -- test/job-store.test.ts`

**Step 3: Implement minimal caches**

- Cache sanitized config and job state in memory.
- Use file metadata (`mtime` / `size`) to detect external changes.
- Ensure `save*` and in-process mutations refresh cache immediately.

**Step 4: Run tests**

Run:
- `npm test -- test/config-store.test.ts`
- `npm test -- test/job-store.test.ts`

### Task 4: Replace cdx-rt full-file log reads with tail reads

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/src/service/http-api.ts`
- Test: `/home/alpen/DEV/cdx-rt/test/http-api.test.ts`

**Step 1: Write failing test**

Add a test with a larger runtime log and verify `GET /api/v1/jobs/:id/logs` still returns the last N lines correctly.

**Step 2: Run test to verify it fails**

Run: `npm test -- test/http-api.test.ts`

**Step 3: Implement minimal tail reader**

- Read from the file end in fixed-size chunks.
- Stop once enough newline-delimited lines are collected.
- Preserve current API shape.

**Step 4: Run tests**

Run: `npm test -- test/http-api.test.ts`

### Task 5: Full verification

**Files:**
- None

**Step 1: Run CLIProxyAPI full tests**

Run: `go test ./... -count=1`

**Step 2: Run cdx-rt full tests**

Run:
- `npm test`
- `npm run build`

**Step 3: Summarize perf wins**

Record:
- endpoints reduced per overview refresh
- major read-path caches added
- log tail IO behavior changed
