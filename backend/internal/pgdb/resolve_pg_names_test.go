package pgdb

import (
	"context"
	"testing"

	"github.com/ebash/barn/backend/internal/docker"
)

// multiPGDocker reports which of several containers exist (in Inspect order).
type multiPGDocker struct {
	*docker.StubClient
	running []string
}

func (m *multiPGDocker) InspectContainer(ctx context.Context, names ...string) (docker.ContainerStatus, error) {
	present := map[string]bool{}
	for _, r := range m.running {
		present[r] = true
	}
	for _, n := range names {
		if present[n] {
			return docker.ContainerStatus{
				Found:     true,
				Running:   true,
				State:     "running",
				Health:    "healthy",
				Container: n,
			}, nil
		}
	}
	return docker.ContainerStatus{Health: "none"}, nil
}

func TestResolvePGNames_prefersBarnOverPanel(t *testing.T) {
	// Regression: previously dock-pilot-postgres was first, so managed dumps
	// hit the panel DB and failed with "database coachman does not exist".
	d := &multiPGDocker{
		StubClient: docker.NewStubClient(nil),
		running:    []string{"dock-pilot-postgres", "barn-postgres"},
	}
	svc := testService(d, testCipher(t))

	container, volume := svc.resolvePGNames(context.Background())
	if container != "barn-postgres" {
		t.Fatalf("container=%q want barn-postgres", container)
	}
	if volume != "barn-postgres-data" {
		t.Fatalf("volume=%q", volume)
	}
}

func TestResolvePGNames_fallsBackToPanelWhenOnlyPanelExists(t *testing.T) {
	d := &multiPGDocker{
		StubClient: docker.NewStubClient(nil),
		running:    []string{"dock-pilot-postgres"},
	}
	svc := testService(d, testCipher(t))

	container, volume := svc.resolvePGNames(context.Background())
	if container != "dock-pilot-postgres" {
		t.Fatalf("container=%q want dock-pilot-postgres", container)
	}
	if volume != "dock-pilot_dock_pilot_pg" {
		t.Fatalf("volume=%q", volume)
	}
}

func TestResolvePGNames_defaultWhenNothingRunning(t *testing.T) {
	d := &multiPGDocker{
		StubClient: docker.NewStubClient(nil),
		running:    nil,
	}
	svc := testService(d, testCipher(t))

	container, volume := svc.resolvePGNames(context.Background())
	if container != "barn-postgres" || volume != "barn-postgres-data" {
		t.Fatalf("got %s / %s", container, volume)
	}
}

func TestVolumeForManagedContainer(t *testing.T) {
	cases := map[string]string{
		"barn-postgres":       "barn-postgres-data",
		"dockpilot-postgres":  "dockpilot-postgres-data",
		"dock-pilot-postgres": "dock-pilot_dock_pilot_pg",
		"other":               "barn-postgres-data",
	}
	for c, want := range cases {
		if got := volumeForManagedContainer(c); got != want {
			t.Fatalf("%s: got %q want %q", c, got, want)
		}
	}
}

func TestManagedPostgresCandidatesOrder(t *testing.T) {
	if managedPostgresCandidates[0] != "barn-postgres" {
		t.Fatalf("barn-postgres must be first, got %v", managedPostgresCandidates)
	}
	last := managedPostgresCandidates[len(managedPostgresCandidates)-1]
	if last != "dock-pilot-postgres" {
		t.Fatalf("panel container must be last, got %v", managedPostgresCandidates)
	}
}
