# CLIProxyAPI

Codex-only proxy and management console.

This branch intentionally removes every non-Codex provider path from the product surface:

- only Codex OAuth accounts and Codex API keys
- only OpenAI-compatible `/v1/*` endpoints that route into Codex
- only the built-in Chinese management console at `/management.html`
- only Codex account import, quota refresh, invalid-account cleanup, and usage statistics

## What It Is

CLIProxyAPI in this branch is a small self-hosted gateway for a Codex account pool.

Typical use cases:

- expose a single OpenAI-compatible endpoint for Codex clients
- import many `codex-*.json` OAuth account files
- auto-refresh and prune invalid accounts
- inspect request count, account health, and quota status from the web console

## Quick Start

1. Copy [config.example.yaml](config.example.yaml) to your own config file.
2. Set `remote-management.secret-key`.
3. Set at least one downstream `api-keys` entry.
4. Start the server.
5. Open `http://127.0.0.1:8317/management.html`.

## Minimal Config

The shipped [config.example.yaml](config.example.yaml) is already trimmed to the supported surface:

- `auth-dir`
- `api-keys`
- `codex-api-key` (optional)
- retry / routing / management settings
- optional Codex header defaults

Legacy provider blocks are ignored on load and stripped on save.

## Management Console

The embedded management console is part of the binary and does not download external panel assets.

Main pages:

- dashboard
- account quota
- account import
- operation log

Main operations:

- import from a server directory
- upload exported Codex OAuth JSON files
- refresh quota manually
- clean invalid accounts
- delete local account files

## Development

Useful verification commands:

```bash
go test ./... -count=1
go build -o ./bin/codex-proxy ./cmd/server
docker build -t codex-proxy:codex-only .
```

## License

MIT. See [LICENSE](LICENSE).
