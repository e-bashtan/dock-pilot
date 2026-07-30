# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

ENV GOTOOLCHAIN=local

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP_VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=${APP_VERSION}" -o /out/barn-agent-linux-amd64 ./cmd/agent \
 && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w -X main.version=${APP_VERSION}" -o /out/barn-agent-linux-arm64 ./cmd/agent \
 && cp /out/barn-agent-linux-amd64 /out/dockpilot-agent-linux-amd64 \
 && cp /out/barn-agent-linux-arm64 /out/dockpilot-agent-linux-arm64 \
 && sha256sum /out/barn-agent-linux-amd64 /out/barn-agent-linux-arm64 /out/dockpilot-agent-linux-amd64 /out/dockpilot-agent-linux-arm64 > /out/agent.sha256

FROM alpine:3.21

# git: clone repos; access host nginx/certbot via chroot when HOST_ROOT=/host
RUN apk add --no-cache ca-certificates git openssh-client util-linux

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /out/barn-agent-linux-amd64 /app/agents/barn-agent-linux-amd64
COPY --from=builder /out/barn-agent-linux-arm64 /app/agents/barn-agent-linux-arm64
COPY --from=builder /out/dockpilot-agent-linux-amd64 /app/agents/dockpilot-agent-linux-amd64
COPY --from=builder /out/dockpilot-agent-linux-arm64 /app/agents/dockpilot-agent-linux-arm64
COPY --from=builder /out/agent.sha256 /app/agents/SHA256SUMS
ENV FLEET_AGENT_DIR=/app/agents

# Root required for Docker socket and writing nginx configs on mounted host paths.
USER root

EXPOSE 8080

ENTRYPOINT ["/app/server"]
