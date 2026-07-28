package docker

import (
	"strings"
	"testing"
)

func TestConsumeBuildOutput_errorInStream(t *testing.T) {
	body := strings.NewReader(`{"stream":"Step 1/1\n"}
{"error":"Dockerfile not found"}
`)
	var lines []string
	log, err := consumeBuildOutput(body, func(line string) {
		lines = append(lines, line)
	})
	if len(lines) == 0 || !strings.Contains(lines[0], "Step 1") {
		t.Fatalf("expected OnOutput lines, got %#v", lines)
	}
	if err == nil {
		t.Fatal("expected build error")
	}
	if !strings.Contains(err.Error(), "Dockerfile not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(log, "Step 1") {
		t.Fatalf("expected stream in log: %q", log)
	}
}

func TestImageTagForSlug(t *testing.T) {
	if got := ImageTagForSlug("Eugen-Bash"); got != "dockpilot/eugen-bash:latest" {
		t.Fatalf("got %q", got)
	}
}
