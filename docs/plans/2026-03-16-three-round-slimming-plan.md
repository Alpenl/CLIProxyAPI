# Three-Round Slimming Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove the remaining non-essential repository and runtime surfaces so the current product is a minimal Codex-only proxy with zero-config bootstrap and a Chinese management UI.

**Architecture:** Perform slimming in three ordered passes. First remove dead config fields, dead packages, and stale helper entrypoints that do not affect the live flow. Then remove the `codex-api-key` runtime chain so imported auth files are the only credential source. Finally replace remote model refresh and trim the remaining request API surface to the currently required Codex path.

**Tech Stack:** Go, Gin, embedded HTML UI, Go tests, Docker build

---

### Task 1: Record The Approved Slimming Scope

**Files:**
- Create: `docs/plans/2026-03-16-three-round-slimming-design.md`
- Create: `docs/plans/2026-03-16-three-round-slimming-plan.md`

**Step 1: Write the design document**

Capture the approved three-round scope and the compatibility boundary that must survive the refactor.

**Step 2: Confirm the files exist**

Run: `test -f docs/plans/2026-03-16-three-round-slimming-design.md && test -f docs/plans/2026-03-16-three-round-slimming-plan.md`
Expected: exit code `0`

### Task 2: Round 1 Safe Prune

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/bootstrap.go`
- Modify: `internal/watcher/diff/config_diff.go`
- Modify: `internal/api/server.go`
- Modify: `internal/cmd/run.go`
- Modify: `sdk/config/config.go`
- Modify: `sdk/cliproxy/service.go`
- Modify: `README.md`
- Modify: `README_CN.md`
- Modify: `config.example.yaml`
- Delete: `internal/managementasset/updater.go`
- Delete: `internal/browser/browser.go`
- Delete: `internal/store/gitstore.go`
- Delete: `internal/store/objectstore.go`
- Delete: `internal/store/postgresstore.go`

**Step 1: Write failing tests or tighten existing ones**

Update config and server tests so removed fields and removed packages are no longer referenced.

**Step 2: Remove the dead config surface**

Delete:
- `quota-exceeded`
- `ws-auth`
- `remote-management.disable-control-panel`
- `remote-management.panel-github-repository`

Also delete change-diff output for those fields.

**Step 3: Remove dead helper entrypoints**

Delete `StartServiceBackground`, `WaitForCloudDeploy`, and `RegisterUsagePlugin` if nothing in the repo uses them.

**Step 4: Remove dead packages and clean module deps**

Delete `internal/store`, `internal/browser`, and `internal/managementasset`, then run `go mod tidy` and `go mod vendor`.

**Step 5: Run focused verification**

Run: `go test ./internal/config ./internal/api ./internal/watcher/... ./sdk/... -count=1`
Expected: PASS

### Task 3: Round 2 Remove `codex-api-key`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/bootstrap.go`
- Modify: `internal/watcher/synthesizer/config.go`
- Modify: `internal/watcher/clients.go`
- Modify: `internal/watcher/diff/config_diff.go`
- Modify: `sdk/cliproxy/service.go`
- Modify: `internal/runtime/executor/codex_executor.go`
- Modify: `sdk/config/config.go`
- Modify: `README.md`
- Modify: `README_CN.md`
- Modify: `config.example.yaml`
- Delete or rewrite: tests that only exist for `codex-api-key`

**Step 1: Write the failing test**

Add or update tests so config loading no longer expects `codex-api-key` support, and watcher synthesis produces zero auth entries from config alone.

**Step 2: Remove config schema and sanitization**

Delete the `CodexKey` types and related sanitization logic from config.

**Step 3: Remove runtime synthesis**

Delete the watcher logic that converts `codex-api-key` config entries into runtime auth records.

**Step 4: Remove executor and service back-links**

Delete runtime helpers that re-resolve imported auths back into `codex-api-key` entries for models, headers, or proxy behavior.

**Step 5: Run focused verification**

Run: `go test ./internal/config ./internal/watcher/... ./internal/runtime/executor ./sdk/cliproxy/... -count=1`
Expected: PASS

### Task 4: Round 3 Runtime Surface Reduction

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/registry/model_updater.go`
- Modify: `sdk/cliproxy/service.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/server_test.go`
- Modify: `sdk/api/handlers/openai/...`
- Modify: `internal/translator/...` only if route reduction leaves dead imports

**Step 1: Write the failing test**

Adjust route tests to reflect the intended minimal request surface after this round.

**Step 2: Remove remote model updater**

Stop starting the remote refresh loop and keep only embedded static Codex model metadata.

**Step 3: Remove the corresponding refresh callback plumbing**

Delete service logic that re-registers auths in response to periodic external model catalog refresh.

**Step 4: Reduce request routes**

Remove handlers and route wiring that are not needed by the current Codex client path while preserving the routes still exercised by the management UI and active Codex usage flow.

**Step 5: Run focused verification**

Run: `go test ./internal/api ./internal/registry ./sdk/api/handlers/... ./sdk/cliproxy/... -count=1`
Expected: PASS

### Task 5: Final Verification And Packaging

**Files:**
- Verify only: `cmd/server`
- Verify only: `Dockerfile`

**Step 1: Run full tests**

Run: `go test ./... -count=1`
Expected: PASS

**Step 2: Build the binary**

Run: `go build -o ./bin/codex-proxy ./cmd/server`
Expected: PASS

**Step 3: Build Docker image**

Run: `docker build -t codex-proxy:slim .`
Expected: PASS

**Step 4: Smoke test runtime**

Run the binary with an empty config location and verify:
- bootstrap config is auto-generated
- management page loads
- bootstrap setup works
- login works
- account list and config page still load

**Step 5: Commit and push**

Commit the verified slimming pass with a focused refactor message and push `alpen-rewrite`.
