// Package gql is a small GraphQL client for the OnCallAlert server API.
package gql

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxResponseBytes caps how much of a response is read.
const maxResponseBytes = 10 << 20

// Config holds the settings needed to build a Client.
type Config struct {
	URL                string
	Token              string
	UserAgent          string
	Timeout            time.Duration
	InsecureSkipVerify bool
}

// Client sends GraphQL requests to one OnCallAlert server.
type Client struct {
	endpoint  string
	token     string
	userAgent string
	http      *http.Client
}

// New returns a Client for the server at cfg.URL.
func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		return nil, errors.New("server URL is required")
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return nil, fmt.Errorf("server URL %q must start with http:// or https://", base)
	}
	if cfg.Token == "" {
		return nil, errors.New("API token is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InsecureSkipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit opt-in flag
	}
	return &Client{
		endpoint:  base + "/api/graphql",
		token:     cfg.Token,
		userAgent: cfg.UserAgent,
		http:      &http.Client{Timeout: timeout, Transport: tr},
	}, nil
}

type request struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type response struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Error is one or more GraphQL errors returned by the server.
type Error struct {
	Messages []string
}

func (e *Error) Error() string { return strings.Join(e.Messages, "; ") }

// Do runs query with vars and decodes the "data" object into out (if out is not nil).
func (c *Client) Do(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(request{Query: query, Variables: vars})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", c.endpoint, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var gr response
	parsed := json.Unmarshal(raw, &gr) == nil
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return authError(resp.Status, gr, parsed)
	}
	if !parsed {
		return fmt.Errorf("server returned %s with an unexpected body: %.200s", resp.Status, raw)
	}
	if len(gr.Errors) > 0 {
		e := &Error{}
		for _, ge := range gr.Errors {
			e.Messages = append(e.Messages, ge.Message)
		}
		return e
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	if out == nil || len(gr.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(gr.Data, out); err != nil {
		return fmt.Errorf("decode response data: %w", err)
	}
	return nil
}

func authError(status string, gr response, parsed bool) error {
	msg := "token invalid or lacks permission (" + status + "). " +
		"Check ONCALLALERT_TOKEN and that the token scope allows this action (Read or Write)."
	if parsed && len(gr.Errors) > 0 {
		msg += " Server said: " + gr.Errors[0].Message
	}
	return errors.New(msg)
}
