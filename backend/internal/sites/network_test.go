package sites

import (
	"testing"

	"github.com/ebash/barn/backend/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestUpstreamPort_hostNetwork(t *testing.T) {
	site := db.Site{
		ContainerPort:     8080,
		HostPort:          pgtype.Int4{Int32: 18080, Valid: true},
		DockerNetworkHost: true,
	}
	if got := UpstreamPort(site); got != 8080 {
		t.Fatalf("got %d want 8080", got)
	}
}

func TestUpstreamPort_bridge(t *testing.T) {
	site := db.Site{
		ContainerPort:     3000,
		HostPort:          pgtype.Int4{Int32: 18080, Valid: true},
		DockerNetworkHost: false,
	}
	if got := UpstreamPort(site); got != 18080 {
		t.Fatalf("got %d want 18080", got)
	}
}
