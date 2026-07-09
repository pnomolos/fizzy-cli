# fizzy-cli

A fast, human-friendly CLI for the Fizzy kanban board. Manage boards, cards, comments, steps, tags, users, columns, webhooks, exports, and notifications from your terminal using Fizzy's HTTP API.

## Features
- Token or magic-link authentication, plus personal access token management
- List, create, update, and delete boards/cards/comments/columns/users
- Card checklists (steps), reactions, pins, tags, activity, and full-text search
- Board webhooks and account/user data exports
- Bulk-friendly output with `--json` and `--plain`
- Config + env precedence for repeatable workflows
- Works on macOS and Linux (single Go binary)

## Install
Install with Homebrew:

```bash
brew install tobiasbischoff/tap/fizzy-cli
```

Build from source:

```bash
go build ./...
```

If your default Go cache path is restricted:

```bash
GOCACHE=/path/to/.gocache go build ./...
```

The binary is `./cmd/fizzy-cli/fizzy-cli` when built from that folder, or use `go build -o fizzy-cli ./cmd/fizzy-cli`.

## Quick Start
1) Authenticate

```bash
fizzy-cli auth login --token $FIZZY_TOKEN
# or
fizzy-cli auth login --email user@example.com
```

2) Set your default account

```bash
fizzy-cli account set 897362094
# or persist base URL + account
fizzy-cli config set --base-url https://app.fizzy.do --account 897362094
```

3) List boards

```bash
fizzy-cli board list
```

## Usage Examples

Global flags (`--json`, `--plain`, `--account`, `--token`, etc.) work in any
position — before or after the subcommand.

List cards on a board:

```bash
fizzy-cli card list --board-id 03f5v9zkft4hj9qq0lsn9ohcm
```

Create a card:

```bash
fizzy-cli card create --board-id 03f5v9zkft4hj9qq0lsn9ohcm \
  --title "Add dark mode" \
  --description "Switch theme"
```

Create a card with tags:

```bash
fizzy-cli card create --board-id 03f5v9zkft4hj9qq0lsn9ohcm \
  --title "Add dark mode" \
  --tag "frontend" --tag "polish"
```

Upload a card image:

```bash
fizzy-cli card create --board-id 03f5v9zkft4hj9qq0lsn9ohcm \
  --title "Add hero image" \
  --image ./screenshot.png
```

Update a card:

```bash
fizzy-cli card update 4 --title "Add dark mode (updated)" --tag "frontend"
```

Publish a draft card:

```bash
fizzy-cli card publish 4
```

Comment on a card:

```bash
fizzy-cli comment create 4 --body "Looks good to me"
```

Add a checklist step and check it off:

```bash
fizzy-cli step add 4 --content "Write tests"
fizzy-cli step check 4 3
```

React to a card:

```bash
fizzy-cli reaction add 4 --content "🎉"
```

List your pinned cards:

```bash
fizzy-cli pin list
```

Search across cards:

```bash
fizzy-cli search "dark mode"
```

List recent account activity:

```bash
fizzy-cli activity list --board-id 03f5v9zkft4hj9qq0lsn9ohcm
```

List notifications (unread only):

```bash
fizzy-cli notification list --unread
```

Create a personal access token:

```bash
fizzy-cli auth token create --description "CI token" --permission write --save
```

Manage board webhooks:

```bash
fizzy-cli webhook create --board-id 03f5v9zkft4hj9qq0lsn9ohcm \
  --name "CI hook" --url https://example.com/hook --event card_published
```

Export account data:

```bash
fizzy-cli export create --wait
```

Machine output:

```bash
fizzy-cli card list --board-id 03f5v9zkft4hj9qq0lsn9ohcm --json
fizzy-cli board list --plain
```

## Configuration
Config file location (default):
- `~/.config/fizzy/config.json`

Precedence (highest to lowest):
1. Flags
2. Environment variables
3. Config file
4. Built-in defaults

Supported env vars:
- `FIZZY_BASE_URL`
- `FIZZY_TOKEN`
- `FIZZY_ACCOUNT`
- `FIZZY_CONFIG`

Inspect config:

```bash
fizzy-cli config show
```

## Board Auto-Postpone Limitation
`board create` and `board update` no longer accept `--auto-postpone-days`.
The Fizzy backend cannot currently persist a board's auto-postpone period
via the API (`create` rejects it with a 422; `update` and the entropy
endpoint accept the value but silently never apply it). Use the
account-wide setting instead, which does work:

```bash
fizzy-cli account auto-postpone 30
```

Valid values are 3, 7, 11, 30, 90, 365.

## Output Modes
- Default: human-friendly tables
- `--plain`: line-oriented output (stable for scripts)
- `--json`: JSON output of raw API responses

## Security Notes
- Tokens and session cookies grant access to your account; keep them secret.
- `fizzy-cli config show` never prints secrets, only whether they are set.

## Command Reference
Run `fizzy-cli --help` or `fizzy-cli help <command>`.

Common commands:
- `auth login|logout|status|token list|token create|token revoke`
- `account list|set|get|update|auto-postpone|join-code`
- `config show|set`
- `board list|get|create|update|delete|publish|unpublish|accesses|watch|unwatch`
- `card list|get|create|update|delete|publish|close|reopen|not-now|triage|untriage|tag|assign|watch|unwatch|pin|unpin|move|golden|ungolden|remove-image|read|unread`
- `comment list|get|create|update|delete`
- `step list|add|update|check|uncheck|delete`
- `reaction list|add|remove`
- `pin list`
- `tag list`
- `column list|get|create|update|delete|cards|move`
- `user list|get|update|deactivate|set-timezone`
- `notification list|read|unread|read-all|settings`
- `activity list`
- `search <query>`
- `webhook list|get|create|update|delete|activate|deliveries`
- `export create|get|download`
