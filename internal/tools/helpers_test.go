package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OnCallAlert/oncallalert-mcp/internal/gql"
)

// call is one GraphQL request seen by the fake server.
type call struct {
	Query string
	Vars  map[string]any
}

// fakeTools returns Tools wired to a fake GraphQL server. respond gets each request and
// returns the JSON body to send. calls records every request.
func fakeTools(t *testing.T, respond func(c call) string) (*Tools, *[]call) {
	t.Helper()
	var calls []call
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		c := call{Query: req.Query, Vars: req.Variables}
		calls = append(calls, c)
		_, _ = w.Write([]byte(respond(c)))
	}))
	t.Cleanup(ts.Close)
	cl, err := gql.New(gql.Config{URL: ts.URL, Token: "tok", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	tl := &Tools{gql: cl, now: func() time.Time { return time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC) }}
	return tl, &calls
}

// static makes a respond func that always returns body.
func static(body string) func(call) string { return func(call) string { return body } }

// mustJSON is the JSON encoding of v, for comparing outputs.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var ctx = context.Background()

// input returns the "i" variable of a call as a map.
func input(t *testing.T, c call) map[string]any {
	t.Helper()
	m, ok := c.Vars["i"].(map[string]any)
	if !ok {
		t.Fatalf("no input variable in %v", c.Vars)
	}
	return m
}
