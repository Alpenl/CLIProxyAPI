# Three-Round Slimming Design

## Goal

Continue slimming the current Codex-only rewrite until the repository, runtime,
and shipped binary all reflect the same product boundary:

- only Codex request handling
- only imported Codex auth JSON accounts as the primary credential source
- only the current Chinese management UI and its required backend routes
- no legacy multi-provider, cloud-standby, external panel-sync, or generic
  storage layers

## Approved Scope

This design follows the already approved three-round sequence:

1. Remove safe dead surfaces and legacy repository baggage.
2. Remove the `codex-api-key` configuration injection chain.
3. Remove remaining API and model-catalog runtime layers that are not required
   by the current Codex usage path.

## Round 1: Safe Repository And Config Prune

The first round removes fields and packages that no longer provide product
value, or are only leftovers from older deployment modes.

Remove from config and change-tracking:

- `quota-exceeded`
- `ws-auth`
- `remote-management.disable-control-panel`
- `remote-management.panel-github-repository`

Remove dead packages and module requirements that no longer participate in the
main runtime:

- `internal/store`
- `internal/browser`
- `internal/managementasset`

Remove CLI and SDK leftovers that are no longer used by this application mode:

- `internal/cmd.StartServiceBackground`
- `internal/cmd.WaitForCloudDeploy`
- `sdk/cliproxy.Service.RegisterUsagePlugin`

This round should leave the current runtime behavior intact.

## Round 2: Remove Config-Defined Codex API Keys

The current product centers on importing Codex account JSON files and managing
them from the web panel. The `codex-api-key` chain is still a second credential
source layered into config parsing, watcher synthesis, scheduler registration,
and executor resolution.

That entire path should be removed so runtime credentials come from imported
auth files only.

This requires deleting:

- config schema for `codex-api-key`
- config sanitization and tests for that block
- watcher synthesis that turns config API keys into runtime auth entries
- executor and service helpers that resolve auth behavior back into config API
  key entries
- docs and examples that mention `codex-api-key`

After this round, imported account files remain the only supported credential
source.

## Round 3: Runtime Surface Reduction

Two remaining runtime layers are still broader than the current product:

- the remote model catalog updater
- the wide OpenAI-compatible API surface

The model updater should be collapsed to embedded static Codex model metadata so
startup no longer launches a background fetch loop or a refresh callback chain.

The API surface should then be reduced to the minimum required by the current
Codex usage path. The preserved routes are:

- `/`
- `/management.html`
- bootstrap and authenticated management routes under `/v0/management`
- the request routes actually exercised by the current Codex client flow

Routes and handlers not required by the current path should be removed.

## Compatibility Boundary

The following must continue to work after all three rounds:

- zero-config startup
- first-run bootstrap config flow
- Chinese management UI
- imported account directory and file upload flows
- quota refresh from the management UI
- invalid account cleanup
- login with management key
- usage statistics and Codex logs
- binary build and Docker build

## Verification Strategy

Each round will be verified before moving on:

- focused tests for the changed packages
- full `go test ./... -count=1` at the end
- `go build -o ./bin/codex-proxy ./cmd/server`
- `docker build -t codex-proxy:slim .`
- runtime smoke check for bootstrap UI, login, account list, quota refresh path,
  and configuration page
