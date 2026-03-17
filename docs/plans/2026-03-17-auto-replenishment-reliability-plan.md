# Auto Replenishment Reliability Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make automatic replenishment recover cleanly from restarts, file corruption, hanging network calls, and atomic config writes.

**Architecture:** Harden `cdx-rt` first because it is the execution backend and queue owner. Then harden `CLIProxyAPI` watcher semantics so config updates reliably reach the already-running replenishment manager without manual restarts.

**Tech Stack:** Go 1.26, Node.js 20, TypeScript, node:test, Go testing, fsnotify.

---

### Task 1: Add failing `cdx-rt` persistence tests

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/test/job-store.test.ts`
- Modify: `/home/alpen/DEV/cdx-rt/test/config-store.test.ts`

**Step 1: Write the failing tests**

Cover:

- corrupted `jobs.json` falls back to backup
- corrupted `jobs.json` with no backup becomes empty state
- corrupted `service-config.json` falls back to backup

**Step 2: Run test to verify it fails**

Run: `cd /home/alpen/DEV/cdx-rt && npm test -- test/job-store.test.ts test/config-store.test.ts`

Expected: FAIL because current store reads `JSON.parse` directly and has no backup recovery.

**Step 3: Write minimal implementation**

- add atomic state persistence helper
- add backup file management
- make load paths tolerant to malformed JSON

**Step 4: Run test to verify it passes**

Run: `cd /home/alpen/DEV/cdx-rt && npm test -- test/job-store.test.ts test/config-store.test.ts`

Expected: PASS.

### Task 2: Add failing `cdx-rt` runner recovery tests

**Files:**
- Modify: `/home/alpen/DEV/cdx-rt/test/job-runner.test.ts`
- Create or Modify: `/home/alpen/DEV/cdx-rt/src/service/job-runner.ts`

**Step 1: Write the failing tests**

Cover:

- startup reconciles stale `running` jobs to terminal state
- timeout marks the active job as failed with a timeout error

**Step 2: Run test to verify it fails**

Run: `cd /home/alpen/DEV/cdx-rt && npm test -- test/job-runner.test.ts`

Expected: FAIL because runner neither reconciles stale jobs nor enforces a batch timeout.

**Step 3: Write minimal implementation**

- add store API for reconciling stale jobs
- run recovery during `start()`
- wrap job execution with timeout handling

**Step 4: Run test to verify it passes**

Run: `cd /home/alpen/DEV/cdx-rt && npm test -- test/job-runner.test.ts`

Expected: PASS.

### Task 3: Add failing `cdx-rt` transport timeout tests

**Files:**
- Create: `/home/alpen/DEV/cdx-rt/test/transport.test.ts`
- Modify: `/home/alpen/DEV/cdx-rt/src/protocol/transport.ts`

**Step 1: Write the failing test**

Cover:

- fetch transport aborts when request exceeds configured timeout

**Step 2: Run test to verify it fails**

Run: `cd /home/alpen/DEV/cdx-rt && npm test -- test/transport.test.ts`

Expected: FAIL because fetch transport currently has no timeout.

**Step 3: Write minimal implementation**

- add timeout signal composition for fetch transport
- return a stable timeout error message

**Step 4: Run test to verify it passes**

Run: `cd /home/alpen/DEV/cdx-rt && npm test -- test/transport.test.ts`

Expected: PASS.

### Task 4: Add failing `CLIProxyAPI` watcher tests

**Files:**
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/watcher/watcher_test.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/watcher/events.go`
- Modify: `/home/alpen/DEV/CLIProxyAPI/internal/watcher/watcher.go`

**Step 1: Write the failing test**

Cover:

- atomic replace of `config.yaml` still triggers exactly one reload callback

**Step 2: Run test to verify it fails**

Run: `cd /home/alpen/DEV/CLIProxyAPI && go test ./internal/watcher -count=1`

Expected: FAIL because watcher only adds a watch on the config file inode.

**Step 3: Write minimal implementation**

- watch config parent directory
- treat matching parent-dir events as config events
- re-register file watch after rename/remove

**Step 4: Run test to verify it passes**

Run: `cd /home/alpen/DEV/CLIProxyAPI && go test ./internal/watcher -count=1`

Expected: PASS.

### Task 5: Full verification

**Files:**
- Verify both repositories

**Step 1: Run focused test suites**

Run:

- `cd /home/alpen/DEV/cdx-rt && npm test -- test/job-store.test.ts test/config-store.test.ts test/job-runner.test.ts test/transport.test.ts`
- `cd /home/alpen/DEV/CLIProxyAPI && go test ./internal/watcher ./internal/replenishment ./internal/api/handlers/management -count=1`

**Step 2: Run broader repo tests if focused suites pass**

Run:

- `cd /home/alpen/DEV/cdx-rt && npm test`
- `cd /home/alpen/DEV/CLIProxyAPI && go test ./... -count=1`

**Step 3: Record remaining gaps**

If any full-suite failures are unrelated, document them explicitly with evidence.
