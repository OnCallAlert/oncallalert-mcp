// Package server builds the MCP server and wires the tools to a GraphQL client.
package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OnCallAlert/oncallalert-mcp/internal/tools"
)

// New returns an MCP server exposing the OnCallAlert tools. With readOnly, tools that change data are not registered.
func New(gql tools.Doer, version string, readOnly bool) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "oncallalert", Version: version}, &mcp.ServerOptions{
		Instructions: "Tools for a self-hosted OnCallAlert server: alerts, on-call schedules, services and users. " +
			"Times are RFC 3339. List tools return page_info; pass its end_cursor as `after` for the next page.",
	})
	tools.Register(srv, gql, readOnly)
	return srv
}
