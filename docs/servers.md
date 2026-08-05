# Barn servers (master / managed nodes)

One Barn instance can run as **standalone** (default), **master**, or **managed_node**.

> **Ребрендинг:** DockPilot → Barn. Agent binaries теперь `barn-agent`, но `dockpilot-agent` остаётся как compat alias.
>
> UI/API paths: `/fleet` → `/servers`, `/api/fleet` → `/api/servers`.
> DB tables: `fleet_*` → `servers_*` (migration `00023_rename_fleet_to_servers`).

## Modes

| Mode | Home | Servers UI | Notes |
|------|------|------------|-------|
| `standalone` | `/sites` | settings only | Existing installs unchanged after migration |
| `master` | `/sites` | yes (`/servers`) | Local apps + remote Barn panels + agents |
| `managed_node` | `/sites` | settings only | Paired to one Master; optional centralized Telegram |

## Enable Master

1. Open **Barn settings** (`/servers/settings`) or enable via API:
   - `PUT /api/servers/settings` with `{"enable_master": true, "node_name": "…", "public_url": "https://…"}`
2. Public URL is used for pairing and agent registration.
3. `/` redirects to `/sites`. On Master, nav shows **Servers** as the second tab. Local server appears with badge **MASTER**.

## Pair remote Barn instance

1. On the remote panel: generate pairing code (`POST /api/servers/pairing-code`, 10 min, one-time, hash stored).
2. On Master: **Add server → Connect Barn** (или legacy "Connect DockPilot") with name, URL, code.
3. Master receives a scoped read token (encrypted at rest). Node receives a heartbeat/events token (encrypted on node). Global `API_TOKEN` is never shared.

## Install monitoring agent

From Master: **Add server → Install agent**. SSH password stays in memory only (TTL ~10 min), never in PostgreSQL or logs. Host key fingerprint must be confirmed before install. API image embeds `barn-agent-linux-amd64` (и `dockpilot-agent` для compat) and `arm64` under `/app/agents`.

See also [barn-agent.md](./barn-agent.md).

## Notifications

`notification_mode`:

- `local` — existing Telegram worker
- `master` — managed node enqueues events to Master outbox; Master dedups incidents and sends Telegram
- `disabled` — no user alerts

## Env

| Variable | Default | Purpose |
|----------|---------|---------|
| `BARN_AGENT_DIR` | `/app/agents` | Directory with agent binaries + checksums |
| `APP_VERSION` | `dev` | Reported panel/agent version |

## Build agent binaries

```bash
make barn-agent-binaries VERSION=v0.3.0
# Legacy alias: make agent-binaries
# or via API image build (includes agents)
make docker-build-api
```
