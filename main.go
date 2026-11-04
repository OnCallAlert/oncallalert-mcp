// Command oncallalert-mcp is a local MCP server (stdio) for a self-hosted OnCallAlert server.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OnCallAlert/oncallalert-mcp/internal/config"
	"github.com/OnCallAlert/oncallalert-mcp/internal/gql"
	"github.com/OnCallAlert/oncallalert-mcp/internal/server"
)

// Set by goreleaser ldflags.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "oncallalert-mcp:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Parse(os.Args[1:], os.Getenv, os.Stderr)
	if err != nil {
		return err
	}
	if cfg.ShowVersion {
		fmt.Printf("oncallalert-mcp %s (%s)\n", version, commit)
		return nil
	}
	client, err := gql.New(gql.Config{
		URL: cfg.URL, Token: cfg.Token, UserAgent: "oncallalert-mcp/" + version,
		Timeout: cfg.Timeout, InsecureSkipVerify: cfg.InsecureSkipVerify,
	})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return server.New(client, version, cfg.ReadOnly).Run(ctx, &mcp.StdioTransport{})
}
