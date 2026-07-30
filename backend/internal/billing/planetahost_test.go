package billing

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBillmgrFieldUnmarshal(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`{"$":"1.2.3.4"}`, "1.2.3.4"},
		{`{"$":8030090}`, "8030090"},
		{`"plain"`, "plain"},
		{`42`, "42"},
		{`null`, ""},
	}
	for _, tc := range cases {
		var f billmgrField
		if err := json.Unmarshal([]byte(tc.raw), &f); err != nil {
			t.Fatalf("%s: %v", tc.raw, err)
		}
		if f.Value != tc.want {
			t.Fatalf("%s: got %q want %q", tc.raw, f.Value, tc.want)
		}
	}
}

func TestFindByIPInName(t *testing.T) {
	list := []VDSInfo{
		{
			ID:   "1",
			Name: "Cloud CPU #221728 (153.56.133.56, uniformly.cuddly.dragon)",
			IPs:  []string{"153.56.133.56"},
			IP:   "153.56.133.56",
			Cost: "288.75",
		},
	}
	v, ok := findByIP(list, "153.56.133.56")
	if !ok || v.ID != "1" {
		t.Fatal("expected match by IP list")
	}
	list2 := []VDSInfo{{ID: "2", Name: "svc 10.0.0.1 other", IP: ""}}
	if _, ok := findByIP(list2, "10.0.0.1"); !ok {
		t.Fatal("expected match by name substring")
	}
}

func TestCollectIPs(t *testing.T) {
	ips := collectIPs("153.56.133.56, 10.0.0.1", "host (153.56.133.56)")
	if len(ips) != 2 {
		t.Fatalf("got %#v", ips)
	}
}

func TestBillmgrErrorMessage(t *testing.T) {
	raw := []byte(`{"code":"auth","obj":"forbidden_auth_method","msg":"forbidden_auth_method77.238.238.224"}`)
	msg := billmgrErrorMessage(raw)
	if !strings.Contains(msg, "forbidden_auth_method") {
		t.Fatalf("got %q", msg)
	}
}

func TestExtractSessionID(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`"abc123"`, "abc123"},
		{`{"$":"sess-1","id":"sess-1"}`, "sess-1"},
		{`{"id":"x9"}`, "x9"},
	}
	for _, tc := range cases {
		if got := extractSessionID(json.RawMessage(tc.raw)); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.raw, got, tc.want)
		}
	}
}
