# Codex-Only Rewrite Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove every non-Codex runtime, config, auth, and management path so this repository becomes truly Codex-only.

**Architecture:** First lock the intended behavior with failing tests at the config, synthesizer, and OAuth normalization boundaries. Then cut non-Codex provider support from config and runtime, delete orphaned provider/auth/management code, and finally update tests and docs to match the reduced surface area.

**Tech Stack:** Go, Gin, YAML config loading, watcher/synthesizer pipeline, Codex auth/token store, Docker build.

---

### Task 1: Lock Codex-only boundaries with failing tests

**Files:**
- Modify: `internal/watcher/synthesizer/config_test.go`
- Modify: `internal/watcher/synthesizer/file_test.go`
- Modify: `internal/api/handlers/management/oauth_sessions_test.go` or create it if missing

**Step 1: Write the failing tests**

- Add a test asserting config synthesizer emits only `codex` auth entries even when YAML contains `gemini-api-key`, `claude-api-key`, `vertex-api-key`, and `openai-compatibility`.
- Add a test asserting auth-file synthesizer ignores non-Codex auth JSON.
- Add a test asserting `NormalizeOAuthProvider` rejects any provider except `codex` / `openai`.

**Step 2: Run targeted tests to verify they fail**

Run:

```bash
go test ./internal/watcher/synthesizer ./internal/api/handlers/management -count=1
```

Expected: failures showing the repository still accepts non-Codex providers.

**Step 3: Commit after green later**

Commit message:

```bash
git commit -m "test: lock codex-only boundaries"
```

### Task 2: Make config synthesizer and auth-file synthesizer Codex-only

**Files:**
- Modify: `internal/watcher/synthesizer/config.go`
- Modify: `internal/watcher/synthesizer/file.go`
- Modify: `internal/watcher/clients.go`
- Modify: `sdk/cliproxy/providers.go`

**Step 1: Remove non-Codex synthesis paths**

- Delete Gemini / Claude / OpenAI-compat / Vertex synthesis branches from `config.go`
- Restrict `file.go` to Codex auth files only
- Reduce client counting and log strings to Codex-only counts

**Step 2: Run targeted tests**

Run:

```bash
go test ./internal/watcher/synthesizer ./internal/watcher ./sdk/cliproxy -count=1
```

Expected: pass after implementation.

**Step 3: Commit**

```bash
git commit -m "refactor: restrict watcher synthesis to codex"
```

### Task 3: Remove non-Codex runtime/provider branches

**Files:**
- Modify: `sdk/cliproxy/service.go`
- Modify: `sdk/cliproxy/auth/conductor.go`
- Modify: `internal/config/config.go`
- Modify: `config.example.yaml`

**Step 1: Cut provider branches**

- Keep only `codex` provider handling in `service.go`
- Keep only Codex upstream model resolution in `conductor.go`
- Remove non-Codex provider fields from config
- Rewrite example config to Codex-only

**Step 2: Run targeted tests**

Run:

```bash
go test ./sdk/cliproxy ./sdk/cliproxy/auth ./internal/config -count=1
```

Expected: pass with only Codex config/runtime support.

**Step 3: Commit**

```bash
git commit -m "refactor: remove non-codex runtime providers"
```

### Task 4: Replace generic management backend with Codex-only backend

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/handlers/management/handler.go`
- Modify: `internal/api/handlers/management/codex_management.go`
- Modify or delete: `internal/api/handlers/management/{api_tools.go,auth_files.go,config_lists.go,oauth_callback.go,oauth_sessions.go,vertex_import.go}`
- Modify: `internal/api/server_test.go`

**Step 1: Cut management surface**

- Keep only usage and Codex account/import/cleanup routes
- Remove generic auth/OAuth/API key management code
- Ensure handler package compiles without importing other provider auth packages

**Step 2: Run targeted tests**

Run:

```bash
go test ./internal/api ./internal/api/handlers/management -count=1
```

Expected: pass with a smaller Codex-only management backend.

**Step 3: Commit**

```bash
git commit -m "refactor: reduce management backend to codex only"
```

### Task 5: Delete orphaned provider packages and stale tests

**Files:**
- Delete: `internal/auth/antigravity/**`
- Delete: `internal/auth/claude/**`
- Delete: `internal/auth/gemini/**`
- Delete: `internal/auth/iflow/**`
- Delete: `internal/auth/kimi/**`
- Delete: `internal/auth/qwen/**`
- Delete: `internal/auth/vertex/**`
- Delete or update: stale tests under `internal/**` and `sdk/**`

**Step 1: Remove directories only after imports are gone**

- Delete non-Codex provider directories
- Delete tests that no longer make sense in a Codex-only product

**Step 2: Run full test suite**

Run:

```bash
go test ./... -count=1
```

Expected: all green.

**Step 3: Build binary and Docker image**

Run:

```bash
go build -o ./bin/codex-proxy ./cmd/server
docker build -t codex-proxy:codex-only .
```

Expected: both succeed.

**Step 4: Commit**

```bash
git commit -m "refactor: delete remaining non-codex code"
```

### Task 6: Final verification and push

**Files:**
- No code changes expected

**Step 1: Verify repository state**

Run:

```bash
git status --short --branch
git branch -vv
```

Expected: clean worktree on `alpen-rewrite`.

**Step 2: Push**

```bash
git push
```

Expected: remote updated.
