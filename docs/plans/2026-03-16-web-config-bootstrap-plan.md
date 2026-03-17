# Bootstrap 配置与网页配置管理 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow the Codex proxy to boot with no pre-existing config file, complete first-time setup from the web UI, and manage the curated Codex-only config surface from the same console.

**Architecture:** First lock bootstrap and config-management behavior with failing tests. Then add bootstrap config generation and unauthenticated initialization endpoints, followed by the authenticated config API and the management UI state machine for first-run setup and later config edits.

**Tech Stack:** Go, Gin, YAML config persistence, watcher hot reload, embedded management HTML, Docker build.

---

### Task 1: Lock bootstrap startup behavior with failing tests

**Files:**
- Create: `internal/config/bootstrap_config_test.go`
- Modify: `cmd/server/main.go` or helper files discovered during implementation

**Step 1: Write the failing tests**

- Add a test asserting missing `config.yaml` causes a bootstrap config file to be created.
- Assert generated config keeps `usage-statistics-enabled: true`, empty management secret, empty API keys, and a non-empty auth dir.

**Step 2: Run targeted test to verify it fails**

Run:

```bash
go test ./internal/config -run 'TestLoadOrCreateConfig' -count=1
```

Expected: fail because the helper does not exist yet.

**Step 3: Write minimal implementation**

- Add a config loader/helper that creates the bootstrap file and returns parsed config.

**Step 4: Run targeted test to verify it passes**

Run:

```bash
go test ./internal/config -run 'TestLoadOrCreateConfig' -count=1
```

Expected: PASS.

### Task 2: Lock bootstrap management routes with failing tests

**Files:**
- Modify: `internal/api/server_test.go`
- Create: `internal/api/handlers/management/config_management_test.go`

**Step 1: Write the failing tests**

- Add a server test asserting `/v0/management/bootstrap/status` exists even when no management secret is configured.
- Add a handler test asserting bootstrap save writes a hashed secret and persists API keys/auth dir.
- Add a handler test asserting ordinary config reads/updates require authenticated mode once setup is complete.

**Step 2: Run targeted tests to verify they fail**

Run:

```bash
go test ./internal/api ./internal/api/handlers/management -run 'Bootstrap|Config' -count=1
```

Expected: fail because routes and handlers do not exist yet.

**Step 3: Write minimal implementation**

- Add bootstrap status and bootstrap save endpoints.
- Add authenticated config get/update endpoints with curated DTOs and validation.

**Step 4: Run targeted tests to verify they pass**

Run:

```bash
go test ./internal/api ./internal/api/handlers/management -run 'Bootstrap|Config' -count=1
```

Expected: PASS.

### Task 3: Add first-run setup and config page to the management UI

**Files:**
- Modify: `internal/managementui/assets/index.html`

**Step 1: Add failing UI-facing behavior checks indirectly**

- Use the backend contract from Task 2 as the behavior boundary:
  - first page load checks bootstrap status
  - unconfigured systems show setup UI instead of login
  - configured systems retain login flow

**Step 2: Write minimal implementation**

- Add bootstrap state to the front-end app state.
- Add a first-run setup form and a separate “配置管理” page.
- Save config through the new management endpoints.
- Show “需重启后生效” feedback for host/port/TLS changes.

**Step 3: Run build-oriented verification**

Run:

```bash
go test ./internal/api ./internal/api/handlers/management ./internal/config -count=1
go test ./... -count=1
```

Expected: PASS.

### Task 4: Verify binary and Docker startup with no config file

**Files:**
- Modify only if verification reveals gaps

**Step 1: Build binary**

Run:

```bash
go build -o ./bin/codex-proxy ./cmd/server
```

Expected: exit 0.

**Step 2: Exercise zero-config startup**

Run:

```bash
rm -f ./tmp/bootstrap-e2e/config.yaml
mkdir -p ./tmp/bootstrap-e2e
./bin/codex-proxy -config ./tmp/bootstrap-e2e/config.yaml
```

Expected: service starts, auto-generates config, and serves `/management.html` plus `/v0/management/bootstrap/status`.

**Step 3: Build Docker image**

Run:

```bash
docker build -t codex-proxy:bootstrap-ui .
```

Expected: exit 0.

### Task 5: Final verification, commit, and push

**Files:**
- No planned code changes

**Step 1: Verify**

Run:

```bash
go test ./... -count=1
go build -o ./bin/codex-proxy ./cmd/server
docker build -t codex-proxy:bootstrap-ui .
git status --short --branch
```

Expected: all verification passes and worktree is ready to commit.

**Step 2: Commit**

```bash
git add -A
git commit -m "feat: add bootstrap config management ui"
```

**Step 3: Push**

```bash
git push origin alpen-rewrite
```
