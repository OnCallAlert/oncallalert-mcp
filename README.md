# oncallalert-mcp

A small local [MCP](https://modelcontextprotocol.io) server that lets AI clients such as Claude and Cursor work with your self-hosted OnCallAlert server: list and acknowledge alerts, see who is on call, read schedules, create overrides.

You run the binary on your own machine. It talks to your OnCallAlert server over HTTPS with your own API token. Nothing is hosted by us.

## Requirements

- An OnCallAlert server newer than v1.0.0 (the first release with user API tokens).
- A user API token. Create one in the web UI: open your profile, go to **API Tokens**, choose **Create token**, and pick the scope:
  - **Read** for read-only use.
  - **Write** if the AI should also acknowledge, resolve or escalate alerts, create alerts, or create overrides.
    Admins create Write tokens directly. Other users request one, an admin approves it on **Admin → API Keys**, and then the user reveals the token once from their profile.

## Install

Download the archive for your platform from the [GitHub Releases](https://github.com/OnCallAlert/oncallalert-mcp/releases) page, check it against the checksums file, and put the `oncallalert-mcp` binary on your `PATH`.

```sh
sha256sum --check --ignore-missing oncallalert-mcp_*_checksums.txt
tar -xzf oncallalert-mcp_<version>_<os>_<arch>.tar.gz
sudo mv oncallalert-mcp /usr/local/bin/
oncallalert-mcp --version
```

Or build from source with Go 1.25 or newer: `go install github.com/OnCallAlert/oncallalert-mcp@latest`.

## Configuration

| Setting | Env var | Flag | Default |
|---|---|---|---|
| Server URL, for example `https://oncall.example.com` | `ONCALLALERT_URL` | `--url` | required |
| API token | `ONCALLALERT_TOKEN` | `--token` | required |
| Hide all tools that change data | | `--read-only` | off |
| Skip TLS certificate checks (self-signed test servers only) | | `--insecure-skip-verify` | off |
| Timeout per request | | `--timeout` | `30s` |
| Print the version and exit | | `--version` | |

Use the environment variables where you can. Flags override them, but a token passed with `--token` is visible in the process list.

### Claude Code

```sh
claude mcp add oncallalert \
  --env ONCALLALERT_URL=https://oncall.example.com \
  --env ONCALLALERT_TOKEN=your-token \
  -- oncallalert-mcp
```

Add `--read-only` after `oncallalert-mcp` to hide the tools that change data.

### Claude Desktop

Add this to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "oncallalert": {
      "command": "oncallalert-mcp",
      "env": {
        "ONCALLALERT_URL": "https://oncall.example.com",
        "ONCALLALERT_TOKEN": "your-token"
      }
    }
  }
}
```

### Cursor and VS Code

Cursor: `~/.cursor/mcp.json` (or `.cursor/mcp.json` in a project). VS Code: `.vscode/mcp.json`, where the top-level key is `servers` instead of `mcpServers`.

```json
{
  "mcpServers": {
    "oncallalert": {
      "command": "oncallalert-mcp",
      "args": ["--read-only"],
      "env": {
        "ONCALLALERT_URL": "https://oncall.example.com",
        "ONCALLALERT_TOKEN": "your-token"
      }
    }
  }
}
```

## Tools

List tools take `first` (default 15, max 100) and `after`, and return `page_info` with `end_cursor` and `has_next_page`.

| Tool | Changes data | What it does |
|---|---|---|
| `list_schedules` | no | List on-call schedules, with search. |
| `get_schedule` | no | A schedule with its shifts, rules and overrides in a time window (default next 7 days). |
| `create_user_override` | yes | Add, remove or replace a user on a schedule for a time range. |
| `list_services` | no | List services with escalation policy and maintenance state. |
| `list_alerts` | no | List alerts, newest first. Filter by service, status (`open`, `acknowledged`, `closed`), time or text. |
| `get_alert` | no | One alert with its service name and recent events. |
| `ack_alert` | yes | Acknowledge up to 50 alerts. |
| `resolve_alert` | yes | Close up to 50 alerts. |
| `escalate_alert` | yes | Escalate an alert to the next step now. |
| `create_alert` | yes | Create an alert on a service. This pages whoever is on call. |
| `get_oncall` | no | Who is on call for a service, an escalation policy or a schedule (a schedule can be asked for a past or future time). |
| `search_users` | no | Search users by name or email. |
| `get_user` | no | A user with contact methods and current on-call assignments. |
| `whoami` | no | The user the token belongs to, with their role. |

With `--read-only`, the five tools marked "yes" are not offered at all. Use a Read-scope token as well, so the server enforces it too.

Errors from the server (for example a locked license or missing permission) are returned to the AI as the tool's error message. An HTTP 401 or 403 gives "token invalid or lacks permission".

## Security

- The token stays on your machine. It is read from the environment (or flags) and sent only to the server URL you configure, in an `Authorization` header.
- The binary makes no other network calls and has no telemetry.
- The AI acts with the permissions of the token's user. Prefer a Read token unless you want the AI to change things.
- Tool results go from your machine to your AI client and its model provider: alert text, names and emails leave your machine that way.

## License

[MPL-2.0](LICENSE)
