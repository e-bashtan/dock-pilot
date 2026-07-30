# Barn Fleet Agent (Агент Флота)

Lightweight host agent that heartbeats metrics (and optional systemd unit status) to a Barn Master. No Docker, nginx, or panel UI.

> **Ребрендинг:** Binary теперь `barn-agent` (user `barn-agent`, config `/etc/barn-agent/config.json`). Legacy `dockpilot-agent` builds доступны для совместимости.

## Install (manual)

### Новые установки (barn-agent)

```bash
# 1. User + dirs
sudo useradd --system --home /var/lib/barn-agent --shell /usr/sbin/nologin barn-agent
sudo mkdir -p /etc/barn-agent /var/lib/barn-agent/outbox
sudo chown -R barn-agent:barn-agent /etc/barn-agent /var/lib/barn-agent
sudo chmod 750 /etc/barn-agent /var/lib/barn-agent

# 2. Binary (example: linux amd64)
sudo install -m 755 barn-agent-linux-amd64 /usr/local/bin/barn-agent

# 3. Register once (one-time token from Master)
sudo -u barn-agent /usr/local/bin/barn-agent \
  -register \
  -master-url 'https://barn.example.com' \
  -registration-token 'reg_…' \
  -node-uid '<uuid-from-master>' \
  -config /etc/barn-agent/config.json

# 4. systemd unit
sudo cp install/barn-agent.service /etc/systemd/system/barn-agent.service
sudo systemctl daemon-reload
sudo systemctl enable --now barn-agent
```

Config path defaults to `/etc/barn-agent/config.json` (mode `0600`):

```json
{
  "master_url": "https://barn.example.com",
  "node_uid": "…",
  "node_token": "…",
  "heartbeat_interval_seconds": 30,
  "monitored_units": ["nginx.service", "wg-quick@wg0.service"]
}
```

### Legacy (dockpilot-agent)

```bash
# 1. User + dirs
sudo useradd --system --home /var/lib/dockpilot-agent --shell /usr/sbin/nologin dockpilot-agent
sudo mkdir -p /etc/dockpilot-agent /var/lib/dockpilot-agent/outbox
sudo chown -R dockpilot-agent:dockpilot-agent /etc/dockpilot-agent /var/lib/dockpilot-agent
sudo chmod 750 /etc/dockpilot-agent /var/lib/dockpilot-agent

# 2. Binary
sudo install -m 755 dockpilot-agent-linux-amd64 /usr/local/bin/dockpilot-agent

# 3. Register + systemd
# (similar steps with dockpilot-agent.service)
```

Config path (legacy): `/etc/dockpilot-agent/config.json` (mode `0600`):

```json
{
  "master_url": "https://pilot.example.com",
  "node_uid": "…",
  "node_token": "…",
  "heartbeat_interval_seconds": 30,
  "monitored_units": ["nginx.service", "wg-quick@wg0.service"]
}
```

`monitored_units` is optional. Only names matching `^[a-zA-Z0-9@._:-]+\.service$` are queried via `systemctl is-active` / `systemctl show` (read-only).

Durable undelivered events live under `/var/lib/barn-agent/outbox` (или `/var/lib/dockpilot-agent/outbox` для legacy) (size-capped).

## Build

From `backend/`:

```bash
# Barn agent
CGO_ENABLED=0 go build -ldflags "-X main.version=v0.0.0" -o /tmp/barn-agent ./cmd/agent

# Or use Makefile (builds both barn-agent and dockpilot-agent):
make barn-agent-binaries VERSION=v0.3.0
```
