package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ebash/barn/backend/internal/metrics"
)

const defaultHTTPTimeout = 15 * time.Second

// Client talks to the Barn Master fleet ingest/register APIs.
type Client struct {
	MasterURL  string
	NodeToken  string
	HTTPClient *http.Client
	UserAgent  string
}

func NewClient(masterURL, nodeToken string) *Client {
	return &Client{
		MasterURL: strings.TrimRight(strings.TrimSpace(masterURL), "/"),
		NodeToken: nodeToken,
		HTTPClient: &http.Client{
			Timeout: defaultHTTPTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				switch req.URL.Scheme {
				case "http", "https":
					return nil
				default:
					return fmt.Errorf("refusing redirect to %s", req.URL.Scheme)
				}
			},
		},
		UserAgent: "barn-agent",
	}
}

type RegisterRequest struct {
	RegistrationToken string           `json:"registration_token"`
	NodeUID           string           `json:"node_uid"`
	Hostname          string           `json:"hostname"`
	AgentVersion      string           `json:"agent_version"`
	Metrics           metrics.Snapshot `json:"metrics"`
}

type RegisterResponse struct {
	NodeToken        string `json:"node_token"`
	MasterURL        string `json:"master_url"`
	HeartbeatSeconds int    `json:"heartbeat_interval_seconds"`
}

type HeartbeatRequest struct {
	NodeUID      string            `json:"node_uid"`
	Version      string            `json:"version"`
	AgentVersion string            `json:"agent_version"`
	Metrics      metrics.Snapshot  `json:"metrics"`
	Services     []ServiceStatus   `json:"services,omitempty"`
}

type Event struct {
	EventID      uuid.UUID       `json:"event_id"`
	EventType    string          `json:"event_type"`
	Severity     string          `json:"severity"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Title        string          `json:"title"`
	Message      string          `json:"message"`
	Payload      json.RawMessage `json:"payload"`
	OccurredAt   time.Time       `json:"occurred_at"`
	NodeUID      string          `json:"node_uid"`
}

type EventsBatchRequest struct {
	Events []Event `json:"events"`
}

func (c *Client) Register(ctx context.Context, req RegisterRequest) (RegisterResponse, error) {
	var out RegisterResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/fleet/agent/register", "", req, &out); err != nil {
		return RegisterResponse{}, err
	}
	return out, nil
}

func (c *Client) Heartbeat(ctx context.Context, req HeartbeatRequest) error {
	return c.doJSON(ctx, http.MethodPost, "/api/fleet/ingest/heartbeat", c.NodeToken, req, nil)
}

func (c *Client) PostEventsBatch(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	return c.doJSON(ctx, http.MethodPost, "/api/fleet/ingest/events/batch", c.NodeToken, EventsBatchRequest{Events: events}, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path, bearer string, body any, out any) error {
	u, err := url.JoinPath(c.MasterURL, path)
	if err != nil {
		return fmt.Errorf("join url: %w", err)
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if len(msg) > 256 {
			msg = msg[:256]
		}
		return fmt.Errorf("http %s %s: status %d: %s", method, path, resp.StatusCode, msg)
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
