# Docker Data Layout Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Force Docker/Sealos runtime data into `/data` so config, auth files, imports, and logs all live under one mounted directory.

**Architecture:** Keep local non-Docker development flexible, but make the packaged image always boot from `/data/config.yaml`. Anchor auth and log defaults to the config directory, and update the management UI and docs to advertise `/data/import` and `/data/auths`.

**Tech Stack:** Go, Gin, Docker, node:test, apply_patch

---

### Task 1: Lock the new `/data` behavior with tests

**Files:**
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/config/bootstrap_config_test.go`
- Create: `/home/alpen/DEV/CLIProxyAPI/internal/logging/global_logger_test.go`
- Create: `/home/alpen/DEV/CLIProxyAPI/internal/managementui/assets/index.data-layout.test.mjs`

**Step 1: Write failing tests**

- Assert bootstrap config created at `<tmp>/data/config.yaml` uses `<tmp>/data/auths`.
- Assert log directory resolution with a config file path under `/data` resolves to `<config-dir>/logs`.
- Assert management UI defaults reference `/data/import` and `/data/auths`.

**Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/config ./internal/logging -count=1
node --test internal/managementui/assets/index.data-layout.test.mjs
```

### Task 2: Implement runtime path unification

**Files:**
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/logging/global_logger.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/cmd/server/main.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/api/server.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/managementui/assets/index.html`
- Modify: `/home/alpen/DEV/CLIProxyAPI/config.example.yaml`

**Step 1: Make log resolution config-root aware**

- Add a config-path-aware log directory helper so both main logs and request logs resolve to `<config-dir>/logs`.

**Step 2: Update Docker-oriented defaults**

- Change management UI default import path to `/data/import`.
- Change config sample auth directory to `/data/auths`.

### Task 3: Update image packaging

**Files:**
- Modify: `/home/alpen/DEV/CLIProxyAPI/Dockerfile`
- Modify: `/home/alpen/DEV/CLIProxyAPI/docker-compose.yml`

**Step 1: Move immutable assets under `/app`**

- Copy binary and sample config into `/app`.

**Step 2: Make the image boot from `/data/config.yaml`**

- Change the container command and data mount target to `/data`.

### Task 4: Sync docs and verify

**Files:**
- Modify: `/home/alpen/DEV/CLIProxyAPI/README.md`
- Modify: `/home/alpen/DEV/CLIProxyAPI/README_CN.md`

**Step 1: Document the `/data` tree**

- Show `/data/config.yaml`, `/data/auths`, `/data/import`, `/data/logs`.

**Step 2: Run verification**

Run:

```bash
go test ./internal/config ./internal/logging ./internal/api -count=1
node --test internal/managementui/assets/index.data-layout.test.mjs internal/managementui/assets/index.quota-refresh.test.mjs
docker build -t cliproxyapi:data-layout .
docker run --rm cliproxyapi:data-layout /app/CLIProxyAPI -config /data/config.yaml
```
