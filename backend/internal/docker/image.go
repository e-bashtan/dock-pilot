package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ImageTagForSlug returns the Docker image reference used for site deployments.
func ImageTagForSlug(slug string) string {
	name := sanitizeImageName(slug)
	if name == "" {
		name = "site"
	}
	return fmt.Sprintf("dockpilot/%s:latest", name)
}

func sanitizeImageName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), ".-_")
	if len(out) > 128 {
		out = out[:128]
		out = strings.TrimRight(out, ".-_")
	}
	return out
}

func consumeBuildOutput(r io.Reader, onOutput func(string)) (log string, err error) {
	var buf strings.Builder
	var errMsgs []string

	emit := func(s string) {
		s = strings.TrimRight(s, "\r")
		s = strings.TrimSpace(s)
		if s == "" || onOutput == nil {
			return
		}
		onOutput(s)
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg struct {
			Stream      string `json:"stream"`
			Status      string `json:"status"`
			Progress    string `json:"progress"`
			ID          string `json:"id"`
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if json.Unmarshal([]byte(line), &msg) != nil {
			continue
		}
		if msg.Stream != "" {
			buf.WriteString(msg.Stream)
			for _, part := range strings.Split(msg.Stream, "\n") {
				emit(part)
			}
		}
		// Pull/layer milestones only — skip noisy progress bars (Downloading [====>]).
		if msg.Status != "" && strings.TrimSpace(msg.Progress) == "" {
			progressLine := msg.Status
			if msg.ID != "" {
				progressLine = msg.ID + ": " + progressLine
			}
			buf.WriteString(progressLine)
			buf.WriteByte('\n')
			emit(progressLine)
		}
		if strings.TrimSpace(msg.Error) != "" {
			errMsgs = append(errMsgs, strings.TrimSpace(msg.Error))
			emit("ERROR: " + strings.TrimSpace(msg.Error))
		}
		if strings.TrimSpace(msg.ErrorDetail.Message) != "" {
			detail := strings.TrimSpace(msg.ErrorDetail.Message)
			errMsgs = append(errMsgs, detail)
			emit("ERROR: " + detail)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return buf.String(), scanErr
	}
	if len(errMsgs) > 0 {
		return buf.String(), fmt.Errorf("%s", strings.Join(errMsgs, "; "))
	}
	return buf.String(), nil
}

func normalizeImageRef(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "dockpilot/site:latest"
	}
	name, vers, ok := strings.Cut(tag, ":")
	if !ok {
		vers = "latest"
	}
	if i := strings.LastIndex(name, "/"); i >= 0 {
		repo := sanitizeImageName(name[i+1:])
		if repo == "" {
			repo = "site"
		}
		name = name[:i+1] + repo
	} else {
		name = sanitizeImageName(name)
		if name == "" {
			name = "site"
		}
	}
	return name + ":" + vers
}

func buildError(imageTag string, buildErr error, buildLog string) error {
	log := strings.TrimSpace(buildLog)
	if log == "" {
		return fmt.Errorf("docker build %s failed: %w", imageTag, buildErr)
	}
	const maxLog = 4000
	if len(log) > maxLog {
		log = log[len(log)-maxLog:]
	}
	return fmt.Errorf("docker build %s failed: %w\n--- build log (tail) ---\n%s", imageTag, buildErr, log)
}
