# Account Pool Replenishment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a two-service Codex account replenishment system where `CLIProxyAPI` automatically maintains a healthy account target by calling a real `cdx-rt` job API.

**Architecture:** `CLIProxyAPI` remains the Codex-only proxy and control plane. `cdx-rt` is upgraded into a long-running service with bootstrap config, job queue, archive download, and a real management console. CPA computes account deficits, creates replenishment jobs, downloads produced archives, and imports account artifacts with dedupe and health tracking.

**Tech Stack:** Go 1.26, Gin, embedded HTML management UI, Node.js 20, TypeScript, built-in Node HTTP server, zip archive packaging, Docker, Sealos-friendly local persistence.

---

### Task 1: Add persistent bootstrap config and runtime state to cdx-rt

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/src/app-paths.ts`
- Modify: `/home/alpen/DEV/cdx-rt/src/config.ts`
- Modify: `/home/alpen/DEV/cdx-rt/src/types.ts`
- Create: `/home/alpen/DEV/cdx-rt/src/service/config-store.ts`
- Create: `/home/alpen/DEV/cdx-rt/test/config-store.test.ts`

**Step 1: Write the failing test**

Add tests that verify:
- first boot creates default bootstrap state without a pre-existing config file
- management secret is empty until explicitly set
- config writes are persisted and reloaded

**Step 2: Run test to verify it fails**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/config-store.test.ts
```

Expected: FAIL because the config store module and bootstrap persistence do not exist.

**Step 3: Write minimal implementation**

Implement:
- app data root resolution under `/data` when present, else project-local fallback
- JSON-backed config store for bootstrap and service settings
- default config values for UI-only first boot

**Step 4: Run test to verify it passes**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/config-store.test.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git -C /home/alpen/DEV/cdx-rt add src/app-paths.ts src/config.ts src/types.ts src/service/config-store.ts test/config-store.test.ts
git -C /home/alpen/DEV/cdx-rt commit -m "feat: add cdx-rt bootstrap config store"
```

### Task 2: Add cdx-rt job store and task state machine

**Files:**
- Create: `/home/alpen/DEV/cdx-rt/src/service/job-store.ts`
- Create: `/home/alpen/DEV/cdx-rt/src/service/job-types.ts`
- Modify: `/home/alpen/DEV/cdx-rt/src/types.ts`
- Create: `/home/alpen/DEV/cdx-rt/test/job-store.test.ts`

**Step 1: Write the failing test**

Add tests that verify:
- queued jobs are persisted
- jobs transition `queued -> running -> completed`
- partial and failed states retain attempt counters and timestamps

**Step 2: Run test to verify it fails**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/job-store.test.ts
```

Expected: FAIL because job store/state machine is missing.

**Step 3: Write minimal implementation**

Implement a JSON-backed job store with:
- create job
- start job
- update progress
- finish/cancel job
- list and fetch by id

**Step 4: Run test to verify it passes**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/job-store.test.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git -C /home/alpen/DEV/cdx-rt add src/service/job-store.ts src/service/job-types.ts src/types.ts test/job-store.test.ts
git -C /home/alpen/DEV/cdx-rt commit -m "feat: add cdx-rt job persistence"
```

### Task 3: Add archive packaging for cdx-rt output artifacts

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/src/output.ts`
- Create: `/home/alpen/DEV/cdx-rt/src/service/archive.ts`
- Create: `/home/alpen/DEV/cdx-rt/test/archive.test.ts`

**Step 1: Write the failing test**

Add tests that verify:
- account JSON files and summary files are packaged into one zip
- archive manifest contains expected relative paths
- empty jobs do not generate broken archives

**Step 2: Run test to verify it fails**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/archive.test.ts
```

Expected: FAIL because no archive packager exists.

**Step 3: Write minimal implementation**

Implement archive creation with:
- `accounts/` entries
- `summary.json`
- `report.md`
- `accounts.txt`
- `accounts-resumable.txt`

**Step 4: Run test to verify it passes**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/archive.test.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git -C /home/alpen/DEV/cdx-rt add src/output.ts src/service/archive.ts test/archive.test.ts
git -C /home/alpen/DEV/cdx-rt commit -m "feat: add cdx-rt archive packaging"
```

### Task 4: Add cdx-rt runner abstraction for queued jobs

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/src/index.ts`
- Create: `/home/alpen/DEV/cdx-rt/src/service/job-runner.ts`
- Create: `/home/alpen/DEV/cdx-rt/test/job-runner.test.ts`

**Step 1: Write the failing test**

Add tests that verify:
- a job stops when `requestedSuccesses` is reached
- attempt count scales from configured or estimated success rate
- runner records partial completion on timeout

**Step 2: Run test to verify it fails**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/job-runner.test.ts
```

Expected: FAIL because queued job execution is not abstracted.

**Step 3: Write minimal implementation**

Refactor the current CLI flow into a callable runner that:
- accepts a job spec
- writes artifacts into a per-job output directory
- reports progress back to the job store
- calculates `maxAttempts` from recent success rate

**Step 4: Run test to verify it passes**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/job-runner.test.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git -C /home/alpen/DEV/cdx-rt add src/index.ts src/service/job-runner.ts test/job-runner.test.ts
git -C /home/alpen/DEV/cdx-rt commit -m "refactor: add cdx-rt queued job runner"
```

### Task 5: Add cdx-rt HTTP API and health endpoints

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/src/ui-server.ts`
- Create: `/home/alpen/DEV/cdx-rt/src/service/http-api.ts`
- Create: `/home/alpen/DEV/cdx-rt/test/http-api.test.ts`

**Step 1: Write the failing test**

Add tests that verify:
- first boot bootstrap status is returned
- job creation requires auth after bootstrap
- archive download works for completed jobs
- health endpoint responds without auth

**Step 2: Run test to verify it fails**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/http-api.test.ts
```

Expected: FAIL because the server only serves static HTML and `/healthz`.

**Step 3: Write minimal implementation**

Extend the server to expose:
- bootstrap config API
- authenticated settings API
- job CRUD endpoints
- archive download
- health endpoint

**Step 4: Run test to verify it passes**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/http-api.test.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git -C /home/alpen/DEV/cdx-rt add src/ui-server.ts src/service/http-api.ts test/http-api.test.ts
git -C /home/alpen/DEV/cdx-rt commit -m "feat: add cdx-rt service api"
```

### Task 6: Replace cdx-rt simulated UI with a real management console

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/ui/index.html`
- Modify: `/home/alpen/DEV/cdx-rt/src/ui-server.ts`
- Create: `/home/alpen/DEV/cdx-rt/test/ui-console.test.ts`

**Step 1: Write the failing test**

Add tests that verify the HTML includes:
- bootstrap secret setup flow
- configuration form
- manual replenishment form
- job list and archive download actions

**Step 2: Run test to verify it fails**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/ui-console.test.ts
```

Expected: FAIL because the current UI is only a shell.

**Step 3: Write minimal implementation**

Rewrite the UI to:
- require only a management secret on first visit
- use the real API for config and jobs
- support manual job creation and zip download

**Step 4: Run test to verify it passes**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test -- test/ui-console.test.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git -C /home/alpen/DEV/cdx-rt add ui/index.html src/ui-server.ts test/ui-console.test.ts
git -C /home/alpen/DEV/cdx-rt commit -m "feat: add cdx-rt management console"
```

### Task 7: Add replenishment settings to CPA config and management API

**Files:**
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/config/config.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/config/bootstrap.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/api/handlers/management/config_management.go`
- Create: `/home/alpen/DEV/CLIProxyAPI/internal/config/replenishment_config_test.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/api/handlers/management/config_management_test.go`

**Step 1: Write the failing test**

Add tests that verify:
- new replenishment fields load with sane defaults
- invalid URLs and intervals are rejected
- bootstrap view includes replenishment settings

**Step 2: Run test to verify it fails**

Run:

```bash
cd /home/alpen/DEV/CLIProxyAPI && go test ./internal/config ./internal/api/handlers/management -count=1
```

Expected: FAIL because replenishment settings are absent.

**Step 3: Write minimal implementation**

Add config fields for:
- auto-replenishment enable flag
- target account count
- scheduler interval
- quota refresh interval
- cdx-rt service URL
- cdx-rt service token

**Step 4: Run test to verify it passes**

Run:

```bash
cd /home/alpen/DEV/CLIProxyAPI && go test ./internal/config ./internal/api/handlers/management -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git -C /home/alpen/DEV/CLIProxyAPI add internal/config/config.go internal/config/bootstrap.go internal/api/handlers/management/config_management.go internal/config/replenishment_config_test.go internal/api/handlers/management/config_management_test.go
git -C /home/alpen/DEV/CLIProxyAPI commit -m "feat: add replenishment config"
```

### Task 8: Add CPA replenishment client, scheduler, and deduping archive import

**Files:**
- Create: `/home/alpen/DEV/CLIProxyAPI/internal/replenishment/client.go`
- Create: `/home/alpen/DEV/CLIProxyAPI/internal/replenishment/manager.go`
- Create: `/home/alpen/DEV/CLIProxyAPI/internal/replenishment/types.go`
- Create: `/home/alpen/DEV/CLIProxyAPI/internal/replenishment/client_test.go`
- Create: `/home/alpen/DEV/CLIProxyAPI/internal/replenishment/manager_test.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/api/handlers/management/codex_management.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/api/server.go`

**Step 1: Write the failing test**

Add tests that verify:
- deficit calculation excludes reserved in-flight jobs
- scheduler creates at most one auto job
- archive import ignores duplicate accounts
- service outage triggers backoff instead of a job storm

**Step 2: Run test to verify it fails**

Run:

```bash
cd /home/alpen/DEV/CLIProxyAPI && go test ./internal/replenishment ./internal/api/handlers/management -count=1
```

Expected: FAIL because replenishment manager and import flow do not exist.

**Step 3: Write minimal implementation**

Implement:
- authenticated HTTP client to `cdx-rt`
- scheduler with deficit formula
- download and unzip import path
- dedupe by email, file hash, and account id
- in-memory or persisted job tracking

**Step 4: Run test to verify it passes**

Run:

```bash
cd /home/alpen/DEV/CLIProxyAPI && go test ./internal/replenishment ./internal/api/handlers/management -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git -C /home/alpen/DEV/CLIProxyAPI add internal/replenishment/client.go internal/replenishment/manager.go internal/replenishment/types.go internal/replenishment/client_test.go internal/replenishment/manager_test.go internal/api/handlers/management/codex_management.go internal/api/server.go
git -C /home/alpen/DEV/CLIProxyAPI commit -m "feat: add account replenishment manager"
```

### Task 9: Add CPA UI pages and manual replenishment controls

**Files:**
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/managementui/assets/index.html`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/api/handlers/management/handler.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/api/handlers/management/codex_management.go`
- Create: `/home/alpen/DEV/CLIProxyAPI/internal/api/handlers/management/replenishment_ui_test.go`

**Step 1: Write the failing test**

Add tests that verify:
- config payload includes replenishment fields
- UI shell exposes replenishment controls and history region
- manual `补到目标值` endpoint is wired

**Step 2: Run test to verify it fails**

Run:

```bash
cd /home/alpen/DEV/CLIProxyAPI && go test ./internal/api/handlers/management -count=1
```

Expected: FAIL because the UI and handler do not expose replenishment controls.

**Step 3: Write minimal implementation**

Update the Chinese management console to show:
- target account card
- replenishment status
- job history
- service health
- manual top-up action

**Step 4: Run test to verify it passes**

Run:

```bash
cd /home/alpen/DEV/CLIProxyAPI && go test ./internal/api/handlers/management -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git -C /home/alpen/DEV/CLIProxyAPI add internal/managementui/assets/index.html internal/api/handlers/management/handler.go internal/api/handlers/management/codex_management.go internal/api/handlers/management/replenishment_ui_test.go
git -C /home/alpen/DEV/CLIProxyAPI commit -m "feat: add replenishment controls to management ui"
```

### Task 10: Update container packaging and verify the two-service workflow

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/Dockerfile`
- Modify: `/home/alpen/DEV/cdx-rt/README.md`
- Modify: `/home/alpen/DEV/CLIProxyAPI/README.md`
- Modify: `/home/alpen/DEV/CLIProxyAPI/Dockerfile`

**Step 1: Write the failing test**

There is no unit test here. Define a verification script and expected manual assertions:
- `cdx-rt` boots without a config file
- `CLIProxyAPI` boots without a config file
- root page redirects to the management console
- replenishment flow works end-to-end locally

**Step 2: Run verification to capture current failure**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm run build
cd /home/alpen/DEV/CLIProxyAPI && go test ./... -count=1
```

Expected: the feature is not yet wired end-to-end.

**Step 3: Write minimal implementation**

Update Docker images and docs so both services:
- boot from generated runtime state
- expose expected ports
- persist under `/data`
- can be connected in Sealos without external config files

**Step 4: Run verification to confirm success**

Run:

```bash
cd /home/alpen/DEV/cdx-rt && npm test && npm run build
cd /home/alpen/DEV/CLIProxyAPI && go test ./... -count=1
cd /home/alpen/DEV/cdx-rt && docker build -t cdx-rt:replenishment .
cd /home/alpen/DEV/CLIProxyAPI && docker build -t codex-proxy:replenishment .
```

Expected:
- tests pass
- builds succeed
- Docker images build successfully

**Step 5: Commit**

```bash
git -C /home/alpen/DEV/cdx-rt add Dockerfile README.md
git -C /home/alpen/DEV/CLIProxyAPI add Dockerfile README.md
git -C /home/alpen/DEV/cdx-rt commit -m "docs: update cdx-rt container workflow"
git -C /home/alpen/DEV/CLIProxyAPI commit -m "docs: update proxy replenishment workflow"
```
