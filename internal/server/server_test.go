package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OnCallAlert/oncallalert-mcp/internal/gql"
)

var mutating = []string{"ack_alert", "create_alert", "create_user_override", "escalate_alert", "resolve_alert"}

func connect(t *testing.T, readOnly bool, handler http.HandlerFunc) *mcp.ClientSession {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	cl, err := gql.New(gql.Config{URL: ts.URL, Token: "tok", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	srvT, cliT := mcp.NewInMemoryTransports()
	srv := New(cl, "test", readOnly)
	if _, err := srv.Connect(ctx, srvT, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, cliT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func toolsByName(t *testing.T, s *mcp.ClientSession) map[string]*mcp.Tool {
	t.Helper()
	res, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]*mcp.Tool{}
	for _, tl := range res.Tools {
		m[tl.Name] = tl
	}
	return m
}

func names(m map[string]*mcp.Tool) []string {
	var n []string
	for k := range m {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}

func TestListTools_ReadOnlyHidesMutating(t *testing.T) {
	h := func(http.ResponseWriter, *http.Request) {}
	full := toolsByName(t, connect(t, false, h))
	if len(full) != 14 {
		t.Fatalf("got %d tools: %v", len(full), names(full))
	}
	for _, n := range mutating {
		tl := full[n]
		if tl == nil || tl.Annotations == nil || tl.Annotations.ReadOnlyHint || tl.Annotations.DestructiveHint == nil || *tl.Annotations.DestructiveHint {
			t.Errorf("%s: want present, not read-only, destructive=false", n)
		}
	}
	ro := toolsByName(t, connect(t, true, h))
	if len(ro) != 9 {
		t.Fatalf("read-only: got %d tools: %v", len(ro), names(ro))
	}
	for _, n := range mutating {
		if ro[n] != nil {
			t.Errorf("%s must be hidden in read-only mode", n)
		}
	}
	for n, tl := range ro {
		if tl.Annotations == nil || !tl.Annotations.ReadOnlyHint {
			t.Errorf("%s: want ReadOnlyHint", n)
		}
	}
}

func TestCallTool_EndToEnd(t *testing.T) {
	var auth, query string
	s := connect(t, false, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		var req struct{ Query string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		query = req.Query
		_, _ = w.Write([]byte(`{"data":{"user":{"id":"u1","name":"Ann","email":"a@x.io","role":"admin"}}}`))
	})
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: "whoami"})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	if auth != "Bearer tok" || !strings.Contains(query, "user") {
		t.Fatalf("auth=%q query=%q", auth, query)
	}
	sc, _ := json.Marshal(res.StructuredContent)
	if string(sc) != `{"email":"a@x.io","id":"u1","name":"Ann","role":"admin"}` {
		t.Fatalf("structured = %s", sc)
	}
	txt := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(txt, `"name":"Ann"`) {
		t.Fatalf("text = %s", txt)
	}
}

func TestCallTool_GraphQLErrorIsToolError(t *testing.T) {
	s := connect(t, false, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"license expired"}]}`))
	})
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_alert", Arguments: map[string]any{"id": 3}})
	if err != nil {
		t.Fatalf("want a tool error, got protocol error: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "license expired") {
		t.Fatalf("res = %+v", res)
	}
}

func TestCallTool_Unauthorized(t *testing.T) {
	s := connect(t, false, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: "whoami"})
	if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "token invalid or lacks permission") {
		t.Fatalf("err=%v res=%+v", err, res)
	}
}
