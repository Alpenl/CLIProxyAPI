# Codex Account Pool Replenishment Design

**Date:** 2026-03-17

**Status:** Approved

## Goal

Build a two-service Codex account replenishment system where:

- `CLIProxyAPI` remains the Codex-only proxy and control plane
- `cdx-rt` becomes a long-running account factory service
- the proxy automatically keeps the healthy account pool at a configurable target size
- both services can boot without operator-supplied config files and be fully configured from their web consoles

## Context

`CLIProxyAPI` is already reduced to a Codex-only runtime with a built-in Chinese management console. It currently supports account import, quota refresh, invalid-account cleanup, and minimal proxy statistics.

`cdx-rt` can already generate CPA-compatible `codex-<email>-free.json` artifacts, but it is still shaped like a batch CLI with a lightweight UI shell. It does not yet expose a real task API, background job queue, or downloadable result archive flow.

The deployment target is Sealos with Docker images for both services.

## Requirements

### Functional

- Keep the proxy account pool at a configurable target count, default `10`
- Count only healthy and warming accounts toward the target
- Trigger replenishment automatically when the pool drops below target
- Allow manual replenishment from the proxy web console
- Allow manual replenishment from the `cdx-rt` web console
- `cdx-rt` manual exports must be downloadable as a zip archive
- All runtime configuration must be editable from the web UI
- First boot should not require a pre-created config file

### Operational

- Support Docker and Sealos deployment
- Prefer internal service-to-service HTTP instead of shared filesystem coupling
- Avoid duplicate import of the same account
- Preserve logs and job history
- Keep existing proxy behavior stable while adding replenishment

## Options Considered

### Option A: Shared volume handoff

`cdx-rt` writes JSON files into a shared directory and `CLIProxyAPI` periodically scans and imports them.

Pros:

- minimal initial implementation
- no API contract needed

Cons:

- weak traceability
- harder dedupe and retry behavior
- poor fit for distributed/containerized deployments
- awkward for manual zip export and job history

Decision: rejected.

### Option B: Service-oriented replenishment

`CLIProxyAPI` computes deficits and calls an internal `cdx-rt` API to request new accounts. `cdx-rt` runs queued jobs, produces artifacts, and exposes job history plus archive download.

Pros:

- clear separation of concerns
- best fit for Sealos and Docker
- supports both auto and manual replenishment
- clean audit trail and retry semantics

Cons:

- requires real API, queue, and artifact lifecycle in `cdx-rt`

Decision: selected.

### Option C: Merge replenishment into the proxy

Move account creation logic into `CLIProxyAPI`.

Pros:

- single service on paper

Cons:

- mixes Go proxy concerns with Node account-creation runtime
- larger blast radius
- harder maintenance and packaging

Decision: rejected.

## Target Architecture

### Service Roles

#### CLIProxyAPI

- Codex-compatible proxy surface
- account inventory and health model
- quota refresh and invalid-account cleanup
- auto-replenishment scheduler
- operator-facing control console

#### cdx-rt

- account creation worker runtime
- job queue and state machine
- artifact packaging
- operator-facing console for manual jobs
- internal API for proxy-triggered jobs

### Communication

- CPA calls `cdx-rt` over internal HTTP using a dedicated service token
- The primary handoff is an archive download or artifact fetch API, not a shared directory
- Both services persist their own state under their data directories

## Account Pool Model

Accounts in CPA are classified as:

- `healthy`: eligible for routing
- `warming`: newly imported, awaiting first verification cycle
- `cooldown`: credentials valid but quota currently exhausted
- `invalid`: unusable and eligible for cleanup
- `pending_job`: expected from in-flight replenishment jobs

Only `healthy` and `warming` count toward the target pool.

`cooldown` accounts are retained and rechecked during scheduled quota refresh. They are not routed while exhausted.

## Replenishment Algorithm

### Inputs

- `targetCount`
- `healthyCount`
- `warmingCount`
- `cooldownCount`
- `openJobs`
- scheduler interval
- quota refresh interval

### Core Formula

```text
usable = healthy + warming
reserved = sum(open_job.requested_successes - open_job.imported_successes)
deficit = target_count - usable - reserved
```

Behavior:

- if `deficit <= 0`, do nothing
- if `deficit > 0`, create a replenishment request to `cdx-rt`
- only one auto-created replenishment job may run at a time by default

### Efficiency Rules

- CPA requests a number of successful accounts, not a number of raw attempts
- `cdx-rt` calculates `maxAttempts` from recent effective success rate
- stop a job as soon as `requestedSuccesses` is met
- use exponential backoff when the replenishment service is unavailable
- do not delete quota-exhausted accounts unless they later become invalid

### Default Timers

- account pool check: every 5 minutes
- quota refresh: every 1 hour
- job timeout: 20 minutes
- service outage backoff: 1m, 3m, 10m, 30m

## cdx-rt Job Model

Each replenishment job should track:

- `jobId`
- `source` (`auto` or `manual`)
- `requestedSuccesses`
- `attemptedCount`
- `successCount`
- `failureCount`
- `status`
- `createdAt`, `startedAt`, `finishedAt`
- artifact paths
- summarized failure categories

Job states:

- `queued`
- `running`
- `completed`
- `partial`
- `failed`
- `cancelled`

## API Contract

### cdx-rt

- `POST /api/v1/jobs`
- `GET /api/v1/jobs`
- `GET /api/v1/jobs/:id`
- `POST /api/v1/jobs/:id/cancel`
- `GET /api/v1/jobs/:id/archive`
- `GET /api/v1/healthz`

`POST /api/v1/jobs` accepts:

- `requestedSuccesses`
- `source`
- `zipRequired`
- optional task-specific overrides

### CLIProxyAPI

Internal additions:

- replenishment scheduler state
- job tracking store
- import pipeline that can consume `cdx-rt` archives
- configuration API for replenishment settings

## UI Changes

### CLIProxyAPI Console

Add:

- replenishment settings section
- target pool count field
- auto-replenishment toggle
- `cdx-rt` service URL and token fields
- scheduler and refresh interval fields
- live replenishment status card
- manual action: `补到目标值`
- import source markers and replenishment history

### cdx-rt Console

Replace simulation-only behavior with:

- first-run secret setup
- real settings page
- manual replenishment form
- live task list
- task detail page
- zip download action
- recent output and error summary

## Configuration Model

### Shared Principles

- no operator-authored config file required before first boot
- service starts with generated bootstrap state
- first browser visit asks only for management secret
- remaining settings are edited inside the UI

### CPA-managed settings

- management secret
- proxy API keys
- bind host and port
- replenishment enable flag
- target pool count
- scheduler interval
- quota refresh interval
- invalid cleanup toggle
- `cdx-rt` service URL
- `cdx-rt` service token

### cdx-rt-managed settings

- management secret
- mail provider settings
- proxy / proxy pool settings
- concurrency
- default task timeout
- archive retention
- manual job enable flag

## Persistence

### CPA

- account auth files
- account metadata and health state
- replenishment job tracking
- proxy usage and operation logs

### cdx-rt

- bootstrap config state
- task queue state
- task history
- generated account artifacts
- downloadable zip archives
- service logs

Both services should persist under `/data` in containerized deployments.

## Failure Handling

- network and transient upstream errors should mark a task attempt as retryable
- explicit auth failures should mark accounts or runs as invalid
- exhausted quota should move accounts to `cooldown`, not deletion
- duplicate artifacts should be ignored by CPA import
- if `cdx-rt` is unavailable, CPA should continue serving existing healthy accounts

## Testing Strategy

### CLIProxyAPI

- unit tests for deficit calculation and scheduler decisions
- unit tests for dedupe during import
- unit tests for replenishment config validation
- integration tests for archive import pipeline

### cdx-rt

- unit tests for job state machine
- unit tests for archive packaging
- unit tests for success-rate-based attempt sizing
- integration tests for job creation, completion, and archive download

### End-to-end

- bring up both services locally
- configure secrets from first-boot UI
- set target count
- simulate a low-account condition
- confirm CPA triggers replenishment
- confirm `cdx-rt` completes a task and returns artifacts
- confirm CPA imports new accounts and stops once target is met

## Non-Goals

- supporting non-Codex providers
- supporting multi-node task execution in the first iteration
- building a general-purpose workflow engine
- merging the two services into one binary/runtime

## Implementation Order

1. Make `cdx-rt` a real service with task storage and API
2. Add archive packaging and manual web operations in `cdx-rt`
3. Add replenishment configuration and scheduler to CPA
4. Add CPA-side archive import and job tracking
5. Verify local two-container workflow
