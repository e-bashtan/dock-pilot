package healthcheck

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ebash/barn/backend/internal/db"
)

func TestSiteURL(t *testing.T) {
	if got := siteURL("example.com"); got != "https://example.com" {
		t.Fatalf("got %q", got)
	}
	if got := siteURL("telegram://bot"); got != "" {
		t.Fatalf("telegram should be empty, got %q", got)
	}
}

func TestTelegramOverall(t *testing.T) {
	o, _ := telegramOverall(ContainerInfo{Found: true, Running: true, State: "running", Health: "none"})
	if o != "healthy" {
		t.Fatalf("expected healthy, got %s", o)
	}
	o, _ = telegramOverall(ContainerInfo{Found: false})
	if o != "unhealthy" {
		t.Fatalf("expected unhealthy, got %s", o)
	}
}

func TestHealthCheckPaths(t *testing.T) {
	if got := healthCheckPaths(""); len(got) != 2 || got[0] != "/health" {
		t.Fatalf("default paths: %#v", got)
	}
	if got := healthCheckPaths("/api/health"); len(got) != 1 || got[0] != "/api/health" {
		t.Fatalf("custom path: %#v", got)
	}
	if got := healthCheckPaths("api/health"); len(got) != 1 || got[0] != "/api/health" {
		t.Fatalf("custom path without slash: %#v", got)
	}
}

func TestProbeTargetsPrefersLocal(t *testing.T) {
	site := db.Site{
		PrimaryUrl:    "https://mylink.tech",
		ContainerPort: 3000,
		HostPort:      pgtype.Int4{Int32: 18080, Valid: true},
	}
	targets := probeTargets(site, "/api/health")
	if len(targets) < 3 {
		t.Fatalf("expected local + nginx + public targets, got %#v", targets)
	}
	if targets[0].URL != "http://127.0.0.1:18080/api/health" || targets[0].Host != "mylink.tech" {
		t.Fatalf("first target: %#v", targets[0])
	}
	if targets[1].URL != "http://127.0.0.1/api/health" || targets[1].Host != "mylink.tech" {
		t.Fatalf("nginx target: %#v", targets[1])
	}
	if targets[2].URL != "https://mylink.tech/api/health" {
		t.Fatalf("public target: %#v", targets[2])
	}
}

func TestPrimaryHost(t *testing.T) {
	if got := primaryHost("https://mylink.tech/foo"); got != "mylink.tech" {
		t.Fatalf("got %q", got)
	}
}

func TestShouldRetryHTTP(t *testing.T) {
	if !shouldRetryHTTP(&HTTPInfo{Error: "timeout"}) {
		t.Fatal("expected retry on error")
	}
	if !shouldRetryHTTP(&HTTPInfo{StatusCode: 503}) {
		t.Fatal("expected retry on 503")
	}
	if shouldRetryHTTP(&HTTPInfo{StatusCode: 404}) {
		t.Fatal("expected no retry on 404")
	}
}
