# Contributing

Issues and pull requests are welcome.

## Setup

You need Go 1.25 or newer.

```sh
make build   # builds ./oncallalert-mcp
make test    # unit tests, no network or server needed
make lint    # go vet and gofmt check
```

## Guidelines

- Tests use a fake GraphQL server (`net/http/httptest`). Do not call a real server in tests.
- Every tool change needs a test for its GraphQL variables and its output.
- Use only fields and operations that exist in the OnCallAlert GraphQL schema.
- Keep files under about 400 lines and one responsibility per file. Tools live in `internal/tools`, one file per area.
- Tools that change data must be registered as mutating so `--read-only` hides them.
- Run `make lint test` before opening a pull request.

## License

By contributing you agree that your work is licensed under the MPL-2.0 (see `LICENSE`).
