package sites

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebash/barn/backend/internal/docker"
)

func (s *Service) StreamContainerLogs(ctx context.Context, siteID uuid.UUID, tail int, w io.Writer, flusher http.Flusher) error {
	if s.docker == nil {
		return fmt.Errorf("container logs not configured")
	}

	site, err := s.queries.GetSite(ctx, siteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("get site: %w", err)
	}

	names := docker.ContainerNamesForSite(site.Slug, site.PrimaryUrl)
	var seq int64

	writeLine := func(line docker.ContainerLogLine) error {
		seq++
		data, err := docker.MarshalLogLine(seq, line)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "id: %d\nevent: log\ndata: %s\n\n", seq, data); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}

	// Send container name hint for UI.
	st, _ := s.docker.InspectContainer(ctx, names...)
	hint, _ := json.Marshal(map[string]string{
		"container": st.Container,
		"state":     st.State,
	})
	if st.Found {
		_, _ = fmt.Fprintf(w, "event: meta\ndata: %s\n\n", hint)
		if flusher != nil {
			flusher.Flush()
		}
	} else {
		_, _ = fmt.Fprintf(w, "event: notice\ndata: {\"message\":\"container not found — deploy first\"}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}

	return s.docker.StreamContainerLogs(ctx, tail, true, names, writeLine)
}

func ParseLogTail(query string, defaultTail int) int {
	if query == "" {
		return defaultTail
	}
	n, err := strconv.Atoi(query)
	if err != nil || n < 1 {
		return defaultTail
	}
	if n > 2000 {
		return 2000
	}
	return n
}
