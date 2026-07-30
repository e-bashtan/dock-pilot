package docker

import (
	"fmt"
	"strings"
)

// Mount describes a container bind or named volume mount (Compose-style).
type Mount struct {
	Source   string // host path or Docker volume name (after prefixing)
	Target   string // path inside container
	ReadOnly bool
	Type     string // "volume" or "bind"
}

// ParseVolumeConfig resolves Compose-style volume lines for a site.
// Named volumes (not absolute paths) are prefixed with barn-{slug}- unless already prefixed
// (including legacy dockpilot-{slug}- names).
func ParseVolumeConfig(slug string, mountLines, namedVolumeLines []string) ([]Mount, []string, error) {
	prefix := volumePrefix(slug)
	seenVolumes := map[string]struct{}{}
	var mounts []Mount

	for _, line := range mountLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m, volName, err := parseMountLine(line)
		if err != nil {
			return nil, nil, fmt.Errorf("volume mount %q: %w", line, err)
		}
		if m.Type == "volume" {
			m.Source = qualifyVolumeName(prefix, volName)
			seenVolumes[m.Source] = struct{}{}
		}
		mounts = append(mounts, m)
	}

	var ensure []string
	for _, line := range namedVolumeLines {
		name := strings.TrimSpace(line)
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		full := qualifyVolumeName(prefix, name)
		if _, ok := seenVolumes[full]; ok {
			continue
		}
		seenVolumes[full] = struct{}{}
		ensure = append(ensure, full)
	}

	for v := range seenVolumes {
		ensure = append(ensure, v)
	}
	return mounts, uniqueStrings(ensure), nil
}

func volumePrefix(slug string) string {
	s := SanitizeContainerName(slug)
	if s == "" {
		s = "site"
	}
	return "barn-" + s + "-"
}

func hasManagedVolumePrefix(name string) bool {
	return strings.HasPrefix(name, "barn-") || strings.HasPrefix(name, "dockpilot-")
}

func qualifyVolumeName(prefix, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return prefix + "data"
	}
	if hasManagedVolumePrefix(name) {
		return name
	}
	return prefix + sanitizeVolumeComponent(name)
}

func sanitizeVolumeComponent(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "data"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), ".-_")
	if out == "" {
		return "data"
	}
	if len(out) > 64 {
		out = out[:64]
		out = strings.TrimRight(out, ".-_")
	}
	return out
}

func isBindSource(source string) bool {
	if strings.HasPrefix(source, "/") {
		return true
	}
	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		return true
	}
	return false
}

func parseMountLine(line string) (Mount, string, error) {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return Mount{}, "", fmt.Errorf("expected SOURCE:TARGET[:ro]")
	}

	source := strings.TrimSpace(parts[0])
	target := strings.TrimSpace(parts[1])
	readOnly := false

	if len(parts) >= 3 {
		for _, opt := range parts[2:] {
			switch strings.ToLower(strings.TrimSpace(opt)) {
			case "ro", "readonly":
				readOnly = true
			case "rw", "readwrite", "":
			default:
				return Mount{}, "", fmt.Errorf("unknown mount option %q", opt)
			}
		}
	}

	if source == "" || target == "" {
		return Mount{}, "", fmt.Errorf("source and target are required")
	}
	if !strings.HasPrefix(target, "/") {
		return Mount{}, "", fmt.Errorf("container path must be absolute, got %q", target)
	}

	if isBindSource(source) {
		return Mount{
			Source:   source,
			Target:   target,
			ReadOnly: readOnly,
			Type:     "bind",
		}, "", nil
	}

	return Mount{
		Source:   source, // filled by caller after prefix
		Target:   target,
		ReadOnly: readOnly,
		Type:     "volume",
	}, source, nil
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
