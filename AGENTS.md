# Repository Guidelines

## Project Structure & Module Organization
- `cmd/fizzy-cli/` contains the CLI entrypoint (`main.go`).
- `internal/cli/` holds command parsing, help text, output formatting, and command implementations.
- `internal/api/` contains the HTTP client used to talk to the Fizzy API.
- `internal/config/` handles config loading/saving (`config.json`).
- API reference and CLI conventions live in the Fizzy backend repository; this repo tracks them via help text in `internal/cli/help.go`.

## Build, Test, and Development Commands
- `go build ./...` builds the CLI binary and all packages.
- `GOCACHE=/path/to/.gocache go build ./...` is useful if the default Go cache path is restricted.
- `go test -race ./...` runs all Go tests.
- `gofmt -w cmd/fizzy-cli/*.go internal/**/*.go` formats Go sources.

## Coding Style & Naming Conventions
- Go formatting: use `gofmt` (tabs, standard Go style).
- Packages use short, lowercase names (`cli`, `api`, `config`).
- Commands and flags follow common CLI conventions (e.g., `--json`, `--plain`, `--help`).
- Keep help text and usage in `internal/cli/help.go` and command logic in `internal/cli/commands.go`.

## Testing Guidelines
- Tests use only the Go standard library (no third-party test frameworks).
- Follow Go conventions: `*_test.go` files and `TestXxx` functions.
- CLI command tests run through the real `cli.Run` entrypoint against an `httptest.Server`; see `internal/cli/harness_test.go`.
- Recommended command: `go test -race ./...`.

## Commit & Pull Request Guidelines
- This repository is not a git repo, so there are no established commit conventions.
- If you initialize git, prefer concise, imperative commit messages (e.g., "Add card image upload").
- For PRs, include a short summary, test commands run, and any user-visible CLI changes.

## Security & Configuration Tips
- Config file defaults to `~/.config/fizzy/config.json`.
- Credentials can be set via env vars (`FIZZY_TOKEN`, `FIZZY_ACCOUNT`, `FIZZY_BASE_URL`) or `fizzy-cli auth login`.
- Avoid logging tokens or session cookies; `fizzy-cli config show` reports only whether secrets are set.
