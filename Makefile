.PHONY: up down reset migrate logs backend frontend dev dev-run setup \
	docker-build docker-export docker-build-api docker-build-frontend \
	release install pushandrelease barn-agent-binaries agent-binaries

# --- Docker (PostgreSQL) ---

up:
	@chmod +x scripts/*.sh 2>/dev/null || true
	@./scripts/local-up.sh

down:
	@./scripts/local-down.sh

reset: down
	docker volume rm barn_postgres_data dock-pilot_postgres_data 2>/dev/null || docker volume rm $$(docker volume ls -q | grep postgres_data) 2>/dev/null || true
	@$(MAKE) up

migrate:
	@./scripts/migrate.sh

logs:
	docker compose logs -f postgres

# --- App (runs on host, DB in Docker) ---

setup:
	@test -f .env || cp .env.example .env
	@cd frontend && npm install
	@cd backend && go mod download
	@echo "Run 'make up' to start PostgreSQL, then 'make dev-run'"

backend:
	@set -a && . ./.env && set +a && cd backend && go run ./cmd/server

frontend:
	@cd frontend && npm run dev

# DB + migrations only (legacy alias)
dev: up

# Backend + frontend together (DB must be running)
dev-run:
	@chmod +x scripts/*.sh 2>/dev/null || true
	@./scripts/dev-run.sh

# --- Production images (copy to VPS) ---

docker-build:
	@chmod +x scripts/*.sh 2>/dev/null || true
	@./scripts/docker-build.sh

docker-build-api:
	@set -a && . ./.env 2>/dev/null || true; set +a; \
		docker compose -f docker-compose.build.yml build api

docker-build-frontend:
	@set -a && . ./.env 2>/dev/null || true; set +a; \
		docker compose -f docker-compose.build.yml build frontend

docker-export:
	@chmod +x scripts/*.sh 2>/dev/null || true
	@./scripts/docker-export.sh

barn-migrate:
	@./scripts/barn-migrate.sh

# Legacy alias
dock-pilot-migrate: barn-migrate

release:
	@chmod +x scripts/*.sh 2>/dev/null || true
	@./scripts/make-release.sh $(VERSION)

# Commit all, tag, push branch + tag (CI builds release on tag push).
#   make pushandrelease MSG="Release message"
#   make pushandrelease MSG="Release message" TAG=v0.1.13
# TAG is optional — bumps patch on latest v* tag (v0.1.12 -> v0.1.13).
pushandrelease:
	@chmod +x scripts/*.sh 2>/dev/null || true
	@MSG="$(MSG)" TAG="$(TAG)" ./scripts/push-and-release.sh

install:
	@chmod +x scripts/*.sh 2>/dev/null || true
	@./scripts/install.sh $(ARGS)

# Cross-compile barn-agent (and dockpilot-agent for backward compat) for embedding / manual install
barn-agent-binaries:
	@mkdir -p dist/agents
	cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=$${VERSION:-dev}" -o ../dist/agents/barn-agent-linux-amd64 ./cmd/agent
	cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w -X main.version=$${VERSION:-dev}" -o ../dist/agents/barn-agent-linux-arm64 ./cmd/agent
	@cp dist/agents/barn-agent-linux-amd64 dist/agents/dockpilot-agent-linux-amd64
	@cp dist/agents/barn-agent-linux-arm64 dist/agents/dockpilot-agent-linux-arm64
	cd dist/agents && sha256sum barn-agent-linux-* dockpilot-agent-linux-* > SHA256SUMS
	@echo "Built barn-agent and dockpilot-agent (compat) in dist/agents/"

# Legacy alias
agent-binaries: barn-agent-binaries
