package agent

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var unitNameRE = regexp.MustCompile(`^[a-zA-Z0-9@._:-]+\.service$`)

// ServiceStatus is the agent→master systemd unit snapshot.
type ServiceStatus struct {
	UnitName string `json:"unit_name"`
	State    string `json:"state"`
}

// ValidUnitName reports whether name is an allowlisted systemd unit name.
func ValidUnitName(name string) bool {
	return unitNameRE.MatchString(name)
}

// CheckUnit returns the normalized state for a single allowlisted unit.
// States: active, inactive, failed, not-found, unknown.
func CheckUnit(ctx context.Context, unit string) (ServiceStatus, error) {
	if !ValidUnitName(unit) {
		return ServiceStatus{}, fmt.Errorf("unit name rejected: %q", unit)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	activeOut, err := runSystemctl(ctx, "is-active", unit)
	state := normalizeActive(strings.TrimSpace(activeOut), err)

	if state == "unknown" || state == "inactive" {
		showOut, showErr := runSystemctl(ctx, "show", unit, "--property=LoadState,ActiveState,SubState", "--no-page")
		if showErr == nil {
			state = stateFromShow(showOut, state)
		}
	}
	return ServiceStatus{UnitName: unit, State: state}, nil
}

// CheckUnits checks each unit; invalid names are skipped with state "invalid".
func CheckUnits(ctx context.Context, units []string) []ServiceStatus {
	out := make([]ServiceStatus, 0, len(units))
	for _, u := range units {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !ValidUnitName(u) {
			out = append(out, ServiceStatus{UnitName: u, State: "invalid"})
			continue
		}
		st, err := CheckUnit(ctx, u)
		if err != nil {
			out = append(out, ServiceStatus{UnitName: u, State: "unknown"})
			continue
		}
		out = append(out, st)
	}
	return out
}

func runSystemctl(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	b, err := cmd.CombinedOutput()
	return string(b), err
}

func normalizeActive(s string, err error) string {
	s = strings.TrimSpace(strings.ToLower(s))
	// systemctl is-active exits non-zero for inactive/failed/unknown; still parse stdout.
	switch s {
	case "active":
		return "active"
	case "inactive":
		return "inactive"
	case "failed":
		return "failed"
	case "activating", "deactivating", "reloading":
		return s
	case "unknown", "not-found":
		return s
	}
	if err != nil && (s == "" || strings.Contains(s, "not-found") || strings.Contains(s, "could not be found")) {
		return "not-found"
	}
	if s == "" {
		return "unknown"
	}
	return s
}

func stateFromShow(out, fallback string) string {
	loadState := ""
	activeState := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if k, v, ok := strings.Cut(line, "="); ok {
			switch k {
			case "LoadState":
				loadState = strings.ToLower(v)
			case "ActiveState":
				activeState = strings.ToLower(v)
			}
		}
	}
	if loadState == "not-found" {
		return "not-found"
	}
	switch activeState {
	case "active", "inactive", "failed":
		return activeState
	}
	return fallback
}
