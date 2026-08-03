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
	if got := ImageTagForSlug("Eugen-Bash"); got != "barn/eugen-bash:latest" {
		t.Fatalf("got %q", got)
	}
}

func TestImageTagsForSlugIncludesLegacy(t *testing.T) {
	got := ImageTagsForSlug("coachman")
	want := []string{
		"barn/coachman:latest",
		"dockpilot/coachman:latest",
		"dock-pilot/coachman:latest",
	}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}
