# DockPilot Fleet Agent

Lightweight host agent that heartbeats metrics (and optional systemd unit status) to a DockPilot Master. No Docker, nginx, or panel UI.

## Install (manual)

```bash
# 1. User + dirs
sudo useradd --system --home /var/lib/dockpilot-agent --shell /usr/sbin/nologin dockpilot-agent
sudo mkdir -p /etc/dockpilot-agent /var/lib/dockpilot-agent/outbox
sudo chown -R dockpilot-agent:dockpilot-agent /etc/dockpilot-agent /var/lib/dockpilot-agent
sudo chmod 750 /etc/dockpilot-agent /var/lib/dockpilot-agent

# 2. Binary (example: linux amd64)
sudo install -m 755 dockpilot-agent /usr/local/bin/dockpilot-agent

# 3. Register once (one-time token from Master)
sudo -u dockpilot-agent /usr/local/bin/dockpilot-agent \
  -register \
  -master-url 'https://pilot.example.com' \
  -registration-token 'reg_…' \
  -node-uid '<uuid-from-master>' \
  -config /etc/dockpilot-agent/config.json

# 4. systemd unit
sudo cp install/dockpilot-agent.service /etc/systemd/system/dockpilot-agent.service
sudo systemctl daemon-reload
sudo systemctl enable --now dockpilot-agent
```

Config path defaults to `/etc/dockpilot-agent/config.json` (mode `0600`):

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

Durable undelivered events live under `/var/lib/dockpilot-agent/outbox` (size-capped).

## Build

From `backend/`:

```bash
CGO_ENABLED=0 go build -ldflags "-X main.version=v0.0.0" -o /tmp/dockpilot-agent ./cmd/agent
```
