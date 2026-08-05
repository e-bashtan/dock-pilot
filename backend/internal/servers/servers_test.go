package servers

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestComputeStatus(t *testing.T) {
	now := time.Now().UTC()
	online := now.Add(-30 * time.Second)
	warn := now.Add(-120 * time.Second)
	off := now.Add(-5 * time.Minute)

	if got := ComputeStatus(&online, now); got != StatusOnline {
		t.Fatalf("online: got %s", got)
	}
	if got := ComputeStatus(&warn, now); got != StatusWarning {
		t.Fatalf("warning: got %s", got)
	}
	if got := ComputeStatus(&off, now); got != StatusOffline {
		t.Fatalf("offline: got %s", got)
	}
	if got := ComputeStatus(nil, now); got != StatusOffline {
		t.Fatalf("nil: got %s", got)
	}
}

func TestValidUnitName(t *testing.T) {
	ok := []string{"nginx.service", "wg-quick@wg0.service", "fail2ban.service"}
	bad := []string{"", "nginx", "../evil.service", "rm -rf.service", "a;b.service"}
	for _, u := range ok {
		if !ValidUnitName(u) {
			t.Fatalf("expected valid: %s", u)
		}
	}
	for _, u := range bad {
		if ValidUnitName(u) {
			t.Fatalf("expected invalid: %s", u)
		}
	}
}

func TestTokenHashEqual(t *testing.T) {
	raw := "agt_deadbeef"
	h1 := HashToken(raw)
	h2 := HashToken(raw)
	if !tokenHashEqual(h1, h2) {
		t.Fatal("same hash should match")
	}
	if tokenHashEqual(h1, HashToken("other")) {
		t.Fatal("different should not match")
	}
}

func TestHasScope(t *testing.T) {
	scopes := []string{ScopeStatusRead, ScopeAppsRead}
	if !HasScope(scopes, ScopeStatusRead) {
		t.Fatal("expected scope")
	}
	if HasScope(scopes, ScopeEventsWrite) {
		t.Fatal("unexpected scope")
	}
	if !HasScope([]string{"*"}, ScopeEventsWrite) {
		t.Fatal("wildcard")
	}
}

func TestDedupKey(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	k := DedupKey(id, "node.offline", "node", id.String())
	if k == "" {
		t.Fatal("empty key")
	}
}

func TestSanitizeInstallLog(t *testing.T) {
	if got := sanitizeInstallLog("password=secret"); got != "[redacted]" {
		t.Fatalf("got %q", got)
	}
	if got := sanitizeInstallLog("Подключение к серверу"); got != "Подключение к серверу" {
		t.Fatalf("got %q", got)
	}
}

func TestOutboxBackoff(t *testing.T) {
	if outboxBackoff(1) != 10*time.Second {
		t.Fatal("1")
	}
	if outboxBackoff(5) != 15*time.Minute {
		t.Fatal("max")
	}
}

func TestValidateRemoteURL(t *testing.T) {
	if err := validateRemoteURL("https://pilot.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := validateRemoteURL("http://pilot.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := validateRemoteURL("file:///etc/passwd"); err == nil {
		t.Fatal("file scheme")
	}
	if err := validateRemoteURL("ftp://x"); err == nil {
		t.Fatal("ftp")
	}
	if err := validateRemoteURL(""); err == nil {
		t.Fatal("empty")
	}
}
