package metrics

import "testing"

func TestCollectorCollects(t *testing.T) {
	c := New("")
	snap, err := c.Collect()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Hostname == "" {
		t.Fatal("empty hostname")
	}
	if snap.Architecture == "" {
		t.Fatal("empty arch")
	}
	if snap.CollectedAt.IsZero() {
		t.Fatal("collected_at")
	}
}
