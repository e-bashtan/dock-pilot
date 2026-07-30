package docker

import (
	"testing"
)

func TestParseVolumeConfig_namedVolume(t *testing.T) {
	mounts, ensure, err := ParseVolumeConfig("eugen-bash", []string{"dict-data:/data"}, []string{"dict-data"})
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 1 || mounts[0].Target != "/data" || mounts[0].Type != "volume" {
		t.Fatalf("mounts: %+v", mounts)
	}
	want := "barn-eugen-bash-dict-data"
	if mounts[0].Source != want {
		t.Fatalf("source %q want %q", mounts[0].Source, want)
	}
	if len(ensure) != 1 || ensure[0] != want {
		t.Fatalf("ensure: %v", ensure)
	}
}

func TestParseVolumeConfig_bindMount(t *testing.T) {
	mounts, ensure, err := ParseVolumeConfig("x", []string{"/host/data:/data:ro"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if mounts[0].Type != "bind" || mounts[0].Source != "/host/data" || !mounts[0].ReadOnly {
		t.Fatalf("%+v", mounts)
	}
	if len(ensure) != 0 {
		t.Fatalf("unexpected volumes: %v", ensure)
	}
}

func TestParseVolumeConfig_legacyPrefixKept(t *testing.T) {
	mounts, _, err := ParseVolumeConfig("site", []string{"dockpilot-site-data:/data"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if mounts[0].Source != "dockpilot-site-data" {
		t.Fatalf("got %q", mounts[0].Source)
	}
}
