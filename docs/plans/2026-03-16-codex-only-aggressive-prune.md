# Codex-Only Aggressive Prune Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove the remaining multi-provider internal abstractions so the repository is structurally Codex-only, not just behaviorally Codex-only.

**Architecture:** Keep the external request flow intact, but collapse internal registry and scheduler state to a single Codex provider. Update tests and docs to stop referencing removed providers or legacy compatibility behavior that no longer exists.

**Tech Stack:** Go, Gin, Docker, ripgrep, go test, docker build

---

### Task 1: Simplify Model Registry To Codex-Only State

**Files:**
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/registry/model_registry.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/sdk/cliproxy/model_registry.go`
- Test: `/home/alpen/DEV/CLIProxyAPI/internal/registry/model_registry_cache_test.go`
- Test: `/home/alpen/DEV/CLIProxyAPI/internal/registry/model_registry_hook_test.go`
- Test: `/home/alpen/DEV/CLIProxyAPI/internal/registry/model_registry_safety_test.go`

**Step 1: Write the failing test**

Change registry tests so they only register `codex`, and add expectations that non-codex provider lookups return empty while Codex lookups still clone safely.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/registry -count=1`
Expected: FAIL if registry internals still assume arbitrary provider-specific maps or old provider names.

**Step 3: Write minimal implementation**

Refactor registry storage so:
- registrations track one Codex-facing model snapshot instead of per-provider metadata maps
- hooks always emit `codex`
- `RegisterClient` accepts the old signature for compatibility but normalizes to Codex
- `GetModelProviders` returns `["codex"]` only when the model exists
- `GetAvailableModelsByProvider` returns models only for `codex`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/registry -count=1`
Expected: PASS

**Step 5: Commit**

Commit after Task 3 if the diff is logically isolated.

### Task 2: Simplify Scheduler To A Single Codex Pool

**Files:**
- Modify: `/home/alpen/DEV/CLIProxyAPI/sdk/cliproxy/auth/scheduler.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/sdk/cliproxy/auth/conductor.go`
- Test: `/home/alpen/DEV/CLIProxyAPI/sdk/cliproxy/auth/scheduler_test.go`
- Test: `/home/alpen/DEV/CLIProxyAPI/sdk/cliproxy/auth/scheduler_benchmark_test.go`
- Test: `/home/alpen/DEV/CLIProxyAPI/sdk/cliproxy/auth/conductor_overrides_test.go`
- Test: `/home/alpen/DEV/CLIProxyAPI/sdk/cliproxy/auth/conductor_scheduler_refresh_test.go`

**Step 1: Write the failing test**

Update tests to stop registering `gemini`/`claude` placeholders and assert Codex-only scheduling semantics.

**Step 2: Run test to verify it fails**

Run: `go test ./sdk/cliproxy/auth -count=1`
Expected: FAIL until scheduler internals stop expecting multi-provider state.

**Step 3: Write minimal implementation**

Refactor scheduler so:
- it keeps one Codex scheduler state instead of a provider map
- auth/provider bookkeeping is normalized to Codex
- selection still honors priority, websocket preference, cooldown, disabled state, and pinned auth behavior

**Step 4: Run test to verify it passes**

Run: `go test ./sdk/cliproxy/auth -count=1`
Expected: PASS

**Step 5: Commit**

Commit after scheduler changes if stable.

### Task 3: Remove Stale Multi-Provider References From Docs And Integration Tests

**Files:**
- Modify: `/home/alpen/DEV/CLIProxyAPI/README.md`
- Modify: `/home/alpen/DEV/CLIProxyAPI/README_CN.md`
- Modify: `/home/alpen/DEV/CLIProxyAPI/sdk/cliproxy/service_excluded_models_test.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/sdk/auth/refresh_registry.go`

**Step 1: Write the failing test**

Update tests or assertions that still mention removed providers or removed config-stripping behavior.

**Step 2: Run test to verify it fails**

Run: `go test ./sdk/... ./internal/... -count=1`
Expected: FAIL if stale provider assumptions remain in tests.

**Step 3: Write minimal implementation**

Adjust docs and small glue code so the repository description matches reality:
- no claim that non-Codex provider blocks are still stripped on save
- no remaining test-only references to removed providers when Codex-only expectations are sufficient
- refresh-lead registration is direct Codex-only wiring

**Step 4: Run test to verify it passes**

Run: `go test ./... -count=1`
Expected: PASS

**Step 5: Commit**

Commit final verified prune pass.

### Task 4: Verify Build And Container Packaging

**Files:**
- Verify only: `/home/alpen/DEV/CLIProxyAPI/cmd/server`
- Verify only: `/home/alpen/DEV/CLIProxyAPI/Dockerfile`

**Step 1: Run targeted verification**

Run: `go build -o ./bin/codex-proxy ./cmd/server`
Expected: build succeeds

**Step 2: Run container verification**

Run: `docker build -t codex-proxy:codex-only .`
Expected: docker build succeeds

**Step 3: Commit**

If all prior tasks are complete and the tree is clean, commit the verified prune pass with a focused refactor message.
