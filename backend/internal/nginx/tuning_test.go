package nginx

import (
	"strings"
	"testing"
)

func TestServerNamesHashSettings_longDomain(t *testing.T) {
	domains := []string{"short.example.com", "this-hostname-is-longer-than-thirty-two-characters.example.org"}
	bucket, maxSize := serverNamesHashSettings(domains)
	if bucket < 64 {
		t.Fatalf("bucket %d too small for long domain", bucket)
	}
	if maxSize < 512 {
		t.Fatalf("maxSize %d too small", maxSize)
	}
}

func TestServerNamesHashSettings_defaultBump(t *testing.T) {
	bucket, _ := serverNamesHashSettings([]string{"x.example.com"})
	if bucket < 128 {
		t.Fatalf("expected at least 128, got %d", bucket)
	}
}

func TestApplyNginxHashTuningScriptContainsMarkers(t *testing.T) {
	script := applyNginxHashTuningScript(128, 2048)
	for _, want := range []string{
		hashBeginMarker,
		hashEndMarker,
		"BUCKET=128",
		"client_max_body_size",
		"512m",
		"grep -qF",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q", want)
		}
	}
}

func TestPruneForeignHashScriptPreservesMarkers(t *testing.T) {
	script := pruneForeignHashScript()
	if !strings.Contains(script, hashBeginMarker) {
		t.Fatal("prune script must reference BEGIN marker")
	}
	if !strings.Contains(script, `if ! grep -qF "$BEGIN"`) {
		t.Fatal("prune must no-op when managed block is missing")
	}
	// Must not blindly comment all hash lines in nginx.conf (old bug).
	if strings.Contains(script, `sed -i -E '/^[[:space:]]*#/! s/^[[:space:]]*(server_names_hash_(bucket_size|max_size)[^;]*;)/# \\1/' "$NGINX"`) {
		t.Fatal("prune must not comment managed block via blanket sed on nginx.conf")
	}
}
