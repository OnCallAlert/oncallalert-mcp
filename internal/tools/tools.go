// Package tools implements the MCP tools. Each tool maps to one or two GraphQL calls.
package tools

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Doer runs a GraphQL operation and decodes the "data" object into out.
type Doer interface {
	Do(ctx context.Context, query string, vars map[string]any, out any) error
}

// Tools holds what every tool needs. Now is replaceable in tests.
type Tools struct {
	gql Doer
	now func() time.Time
}

// Register adds all tools to srv. With readOnly, tools that change data are left out.
func Register(srv *mcp.Server, gql Doer, readOnly bool) {
	t := &Tools{gql: gql, now: time.Now}
	r := &registry{srv: srv, readOnly: readOnly}
	t.registerSchedules(r)
	t.registerServices(r)
	t.registerAlerts(r)
	t.registerOnCall(r)
	t.registerUsers(r)
}

type registry struct {
	srv      *mcp.Server
	readOnly bool
}

// add registers one typed tool. Handler errors become tool errors (IsError), not protocol errors.
func add[In, Out any](r *registry, name, desc string, mutating bool, h func(context.Context, In) (Out, error)) {
	if mutating && r.readOnly {
		return
	}
	ann := &mcp.ToolAnnotations{ReadOnlyHint: !mutating, OpenWorldHint: ptr(false)}
	if mutating {
		ann.DestructiveHint = ptr(false)
	}
	mcp.AddTool(r.srv, &mcp.Tool{Name: name, Description: desc, Annotations: ann},
		func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
			out, err := h(ctx, in)
			return nil, out, err
		})
}

func ptr[T any](v T) *T { return &v }
