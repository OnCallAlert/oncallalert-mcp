package gql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newClient(t *testing.T, h http.HandlerFunc, urlSuffix string) *Client {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	c, err := New(Config{URL: ts.URL + urlSuffix, Token: "secret", UserAgent: "test-agent", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDo_SendsAuthAndDecodes(t *testing.T) {
	var gotAuth, gotUA, gotPath string
	var body struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotUA, gotPath = r.Header.Get("Authorization"), r.Header.Get("User-Agent"), r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"data":{"x":{"n":3}}}`))
	}, "/") // trailing slash is stripped
	var out struct{ X struct{ N int } }
	if err := c.Do(context.Background(), "query { x }", map[string]any{"a": 1}, &out); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret" || gotUA != "test-agent" || gotPath != "/api/graphql" {
		t.Fatalf("auth=%q ua=%q path=%q", gotAuth, gotUA, gotPath)
	}
	if body.Query != "query { x }" || body.Variables["a"] != float64(1) || out.X.N != 3 {
		t.Fatalf("body=%+v out=%+v", body, out)
	}
}

func TestDo_ErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"graphql error", 200, `{"errors":[{"message":"boom"},{"message":"bang"}],"data":null}`, "boom; bang"},
		{"license locked passes message", 200, `{"errors":[{"message":"license required: server is locked"}]}`, "license required: server is locked"},
		{"401", 401, ``, "token invalid or lacks permission"},
		{"403 with message", 403, `{"errors":[{"message":"insufficient scope"}]}`, "insufficient scope"},
		{"non json", 502, `<html>bad gateway</html>`, "502"},
		{"graphql error on 4xx", 400, `{"errors":[{"message":"bad query"}]}`, "bad query"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}, "")
			err := c.Do(context.Background(), "query { x }", nil, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestNew_Validation(t *testing.T) {
	for name, cfg := range map[string]Config{
		"no url":   {Token: "t"},
		"bad url":  {URL: "example.com", Token: "t"},
		"no token": {URL: "https://x"},
	} {
		if _, err := New(cfg); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
