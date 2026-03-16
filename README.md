# CLIProxyAPI

Codex-only proxy and management console.

This branch intentionally removes every non-Codex product path from the product surface:

- only imported Codex OAuth accounts
- only `/v1/models`, `/v1/responses`, `/v1/responses/compact`, and websocket `GET /v1/responses`
- only the built-in Chinese management console at `/management.html`
- only Codex account import, quota refresh, invalid-account cleanup, and usage statistics

## What It Is

CLIProxyAPI in this branch is a small self-hosted gateway for a Codex account pool.

Typical use cases:

- expose the minimal Codex-compatible HTTP surface for clients
- import many `codex-*.json` OAuth account files
- auto-refresh and prune invalid accounts
- inspect request count, account health, and quota status from the web console

## Quick Start

1. Start the server directly, even if you do not have a `config.yaml` yet.
2. The server will auto-create a bootstrap config file on first boot.
3. Open `http://127.0.0.1:8317/` and let it redirect to `/management.html`.
4. Complete the first-time setup wizard in the browser:
   - set the management secret
   - set at least one downstream `api-keys` entry
   - confirm `auth-dir` and optional proxy/logging settings
5. After the first save, the console switches back to the normal management-key login flow.

If you prefer writing config by hand, you can still start from [config.example.yaml](config.example.yaml).

## Minimal Config

The shipped [config.example.yaml](config.example.yaml) is already trimmed to the supported surface:

- `auth-dir`
- `api-keys`
- retry / routing / management settings
- optional Codex header defaults

Unsupported config blocks are ignored on load.

## Management Console

The embedded management console is part of the binary and does not download external panel assets.

Main pages:

- dashboard
- account quota
- account import
- config management
- operation log

Main operations:

- import from a server directory
- upload exported Codex OAuth JSON files
- refresh quota manually
- clean invalid accounts
- delete local account files

Current API surface:

- `GET /v1/models`
- `POST /v1/responses`
- `POST /v1/responses/compact`
- `GET /v1/responses` for websocket upgrade requests

## Development

Useful verification commands:

```bash
go test ./... -count=1
go build -o ./bin/codex-proxy ./cmd/server
docker build -t codex-proxy:bootstrap-ui .
```

## License

MIT. See [LICENSE](LICENSE).
