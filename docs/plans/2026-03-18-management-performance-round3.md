# Management Performance Round 3 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce repeated management-plane polling and overview aggregation work in `CLIProxyAPI` and `cdx-rt` without changing user-visible behavior.

**Architecture:** Add a short-lived overview payload cache on the `CLIProxyAPI` side and invalidate it conservatively after management-side writes. On the `cdx-rt` side, add `ETag`-based conditional fetch for the jobs list so the UI can reuse the current state when no job metadata changed.

**Tech Stack:** Go 1.26, Gin, TypeScript, Node.js 20, embedded HTML admin UIs

---

### Task 1: Cache CPA overview payload briefly

**Files:**
- Modify: `internal/api/handlers/management/handler.go`
- Modify: `internal/api/handlers/management/overview.go`
- Test: `internal/api/handlers/management/replenishment_ui_test.go`

- [ ] **Step 1: Write the failing test**

Add a test that calls `buildOverviewPayload` twice within the TTL window and verifies the replenishment status callback runs only once, then advances time and verifies the cache expires.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/handlers/management -run TestBuildOverviewPayloadUsesShortTTLCache -count=1`

- [ ] **Step 3: Write minimal implementation**

Add a handler-owned overview cache with a short TTL, reuse it from `buildOverviewPayload`, and invalidate it together with existing Codex account cache invalidations and on replenishment-trigger responses.

- [ ] **Step 4: Run tests to verify it passes**

Run: `go test ./internal/api/handlers/management -run TestBuildOverviewPayloadUsesShortTTLCache -count=1`

### Task 2: Add RT jobs list conditional fetch support

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/src/service/http-api.ts`
- Modify: `/home/alpen/DEV/cdx-rt/src/app-paths.ts`
- Test: `/home/alpen/DEV/cdx-rt/test/http-api.test.ts`

- [ ] **Step 1: Write the failing test**

Add a test that fetches `GET /api/v1/jobs`, captures the response `ETag`, repeats the request with `If-None-Match`, and expects `304`. Then mutate the job and verify the next request returns `200` with a different `ETag`.

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- test/http-api.test.ts`

- [ ] **Step 3: Write minimal implementation**

Derive a jobs list version token from the persisted job store file signature, emit `ETag`, and short-circuit to `304` when the client already has the current version.

- [ ] **Step 4: Run tests to verify it passes**

Run: `npm test -- test/http-api.test.ts`

### Task 3: Teach RT UI to reuse unchanged jobs state

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/ui/index.html`
- Test: `/home/alpen/DEV/cdx-rt/test/ui-html.test.ts`

- [ ] **Step 1: Write the failing test**

Add a UI HTML assertion that the management console sends `If-None-Match` for subsequent jobs refreshes and tracks the jobs list `ETag`.

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- test/ui-html.test.ts`

- [ ] **Step 3: Write minimal implementation**

Store the last jobs `ETag`, attach it to follow-up `/api/v1/jobs` requests, and keep the current `remoteJobs` list when the API returns `304`.

- [ ] **Step 4: Run tests to verify it passes**

Run: `npm test -- test/ui-html.test.ts`

### Task 4: Full verification

**Files:**
- None

- [ ] **Step 1: Run CPA targeted verification**

Run: `go test ./internal/api/... ./internal/replenishment/... -count=1`

- [ ] **Step 2: Run RT test suite**

Run: `npm test`

- [ ] **Step 3: Run RT build**

Run: `npm run build`
