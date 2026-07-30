# Barn Fleet (Флот)

One Barn instance can run as **standalone** (default), **master**, or **managed_node**.

> **Ребрендинг:** DockPilot → Barn. Agent binaries теперь `barn-agent`, но `dockpilot-agent` остаётся как compat alias.

## Modes

| Mode | Home | Fleet UI | Notes |
|------|------|----------|-------|
| `standalone` | `/sites` | hidden | Existing installs unchanged after migration |
| `master` | `/fleet` | yes | Local apps + remote Barn panels + agents |
| `managed_node` | `/sites` | hidden | Paired to one Master; optional centralized Telegram |

## Enable Master

1. Open **Fleet settings** (`/fleet/settings`) or enable via API:
   - `PUT /api/fleet/settings` with `{"enable_master": true, "node_name": "…", "public_url": "https://…"}`
2. Public URL is used for pairing and agent registration.
3. `/` redirects to `/fleet`. Local server appears with badge **MASTER**.

## Pair remote Barn instance

1. On the remote panel: generate pairing code (`POST /api/fleet/pairing-code`, 10 min, one-time, hash stored).
2. On Master: **Add server → Connect Barn** (или legacy "Connect DockPilot") with name, URL, code.
3. Master receives a scoped read token (encrypted at rest). Node receives a heartbeat/events token (encrypted on node). Global `API_TOKEN` is never shared.

## Install monitoring agent

From Master: **Add server → Install agent**. SSH password stays in memory only (TTL ~10 min), never in PostgreSQL or logs. Host key fingerprint must be confirmed before install. API image embeds `barn-agent-linux-amd64` (и `dockpilot-agent` для compat) and `arm64` under `/app/agents`.

See also [fleet-agent.md](./fleet-agent.md).

## Notifications

Fleet `notification_mode`:

- `local` — existing Telegram worker
- `master` — managed node enqueues events to Master outbox; Master dedups incidents and sends Telegram
- `disabled` — no user alerts

## Env

| Variable | Default | Purpose |
|----------|---------|---------|
| `FLEET_AGENT_DIR` | `/app/agents` | Directory with agent binaries + checksums |
| `APP_VERSION` | `dev` | Reported panel/agent version |

## Build agent binaries

```bash
make barn-agent-binaries VERSION=v0.3.0
# Legacy alias: make agent-binaries
# or via API image build (includes agents)
make docker-build-api
```
