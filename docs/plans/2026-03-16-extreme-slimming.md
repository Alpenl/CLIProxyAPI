# Extreme Slimming Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove the remaining non-essential runtime surfaces from the Codex-only rewrite so the shipped binary gets smaller without breaking the existing Codex API and management panel flows.

**Architecture:** The current binary still pulls in three avoidable layers: request logging, the generic request access provider abstraction, and unused generic OpenAI registration paths. This plan removes them in that order because request logging is purely additive, API key validation can be inlined safely, and the translator/thinking cleanup depends on first confirming the live Codex request formats.

**Tech Stack:** Go, Gin, embedded management UI, Codex executor, Go tests, Docker-style Go build flags

---

### Task 1: Record the active slimming target in the repo

**Files:**
- Create: `docs/plans/2026-03-16-extreme-slimming.md`

**Step 1: Write the implementation plan**

Write this file so the remaining work has a fixed scope.

**Step 2: Confirm the plan is present**

Run: `test -f docs/plans/2026-03-16-extreme-slimming.md`
Expected: exit code `0`

### Task 2: Remove request logging from the runtime

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/sdk_config.go`
- Modify: `cmd/server/main.go`
- Modify: `config.example.yaml`
- Modify: `internal/watcher/diff/config_diff.go`
- Modify: `sdk/api/options.go`
- Delete: `internal/api/middleware/request_logging.go`
- Delete: `internal/api/middleware/request_logging_test.go`
- Delete: `internal/api/middleware/response_writer.go`
- Delete: `internal/logging/request_logger.go`
- Delete: `sdk/logging/request_logger.go`
- Modify: request-logging related tests so they stop referencing the removed API

**Step 1: Remove the failing references**

Delete request logger option plumbing, config toggles, and middleware wiring from the server so the code no longer references request log interfaces.

**Step 2: Remove the deleted files from build paths**

Delete the request logging implementation files and any direct compile-time dependencies from handlers or executors.

**Step 3: Keep usage statistics intact**

Do not remove usage aggregation or dashboard usage endpoints. Only remove request body/response log capture.

**Step 4: Run targeted tests**

Run: `go test ./internal/api ./sdk/api/handlers/...`
Expected: PASS

### Task 3: Inline API key request authentication

**Files:**
- Modify: `internal/api/server.go`
- Modify: `sdk/cliproxy/builder.go`
- Modify: `sdk/cliproxy/service.go` if needed for field cleanup
- Modify: `cmd/server/main.go`
- Delete: `internal/access/reconcile.go`
- Delete: `internal/access/config_access/provider.go`
- Delete: `sdk/access/manager.go`
- Delete: `sdk/access/registry.go`
- Delete: `sdk/access/types.go`
- Keep or trim tests that directly depended on the old manager abstraction

**Step 1: Replace access manager with a tiny validator**

Introduce a compact API key validator owned by the server path, backed only by normalized keys from config.

**Step 2: Preserve hot reload**

When config reloads, refresh the in-memory key set from `cfg.APIKeys`.

**Step 3: Preserve current auth behavior**

Continue accepting `Authorization: Bearer`, `X-Goog-Api-Key`, `X-Api-Key`, `?key=`, and `?auth_token=`.

**Step 4: Run targeted tests**

Run: `go test ./internal/api ./sdk/cliproxy/...`
Expected: PASS

### Task 4: Remove unused generic OpenAI registration paths

**Files:**
- Modify: `internal/translator/init.go`
- Modify: `sdk/api/handlers/openai/openai_handlers.go`
- Modify: `internal/runtime/executor/thinking_providers.go`
- Optionally delete now-unreferenced helper packages if nothing imports them

**Step 1: Keep only Codex translator registrations**

Stop importing the generic `internal/translator/openai/...` registration packages.

**Step 2: Inline the one remaining responses-to-chat helper if needed**

If `openai_handlers.go` only imports the generic OpenAI responses package for a helper, move the helper into the handler package and remove the heavy package import.

**Step 3: Remove unused openai thinking provider import**

Keep Codex thinking support only.

**Step 4: Run targeted tests**

Run: `go test ./internal/runtime/executor ./sdk/api/handlers/openai`
Expected: PASS

### Task 5: Verify the final binary size reduction

**Files:**
- No source changes required unless verification exposes a regression

**Step 1: Build the server binary**

Run: `go build ./cmd/server`
Expected: PASS

**Step 2: Build the Docker-like stripped binary**

Run: `CGO_ENABLED=0 GOOS=linux GOMAXPROCS=1 go build -trimpath -mod=vendor -p=1 -tags timetzdata -ldflags="-s -w -X main.Version=dev -X main.Commit=none -X main.BuildDate=unknown" -o /tmp/cliproxy-size-compare/alpen-rewrite/codex-proxy-dockerlike-trimpath ./cmd/server`
Expected: PASS and a smaller binary than the previous `12984482` byte baseline if the refactor was effective.

**Step 3: Report the new size and residual candidates**

Summarize the new binary size, the delta from the previous measurement, and whether any compiled non-Codex surfaces remain.
