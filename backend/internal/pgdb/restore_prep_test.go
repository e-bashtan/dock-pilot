package pgdb

import "testing"

func TestRestorePrep_requiresDropWhenExists(t *testing.T) {
	_, _, err := restorePrep(true, true, false)
	if err == nil {
		t.Fatal("expected error when DB exists and drop is false")
	}
}

func TestRestorePrep_dropForcesRecreate(t *testing.T) {
	drop, create, err := restorePrep(true, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !drop || !create {
		t.Fatalf("drop=%v create=%v", drop, create)
	}
}

func TestRestorePrep_createNew(t *testing.T) {
	drop, create, err := restorePrep(false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if drop || !create {
		t.Fatalf("drop=%v create=%v", drop, create)
	}
}

func TestClusterHasDatabase(t *testing.T) {
	if !clusterHasDatabase([]string{"a", "test"}, "test") {
		t.Fatal("expected true")
	}
	if clusterHasDatabase([]string{"a"}, "test") {
		t.Fatal("expected false")
	}
}
