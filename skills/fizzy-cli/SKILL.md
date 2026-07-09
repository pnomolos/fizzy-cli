---
name: fizzy-cli
description: Use the fizzy-cli tool to authenticate and manage Fizzy kanban boards, cards, comments, steps, reactions, pins, tags, columns, users, webhooks, exports, and notifications from the command line. Apply this skill when you need to list, create, update, or delete Fizzy resources or when scripting Fizzy workflows.
metadata:
  author: tobiasbischoff
  version: "1.1"
---

# Fizzy CLI Skill

Use this skill to operate the Fizzy kanban board via the `fizzy-cli` command. It covers authentication, configuration, and common CRUD workflows.

## Quick Start

1) Authenticate
- Token:
  - `fizzy-cli auth login --token $FIZZY_TOKEN`
- Magic link:
  - `fizzy-cli auth login --email user@example.com`
  - If non-interactive, pass `--code ABC123`.

2) Set defaults
- Account only: `fizzy-cli account set 897362094`
- Persist base URL + account: `fizzy-cli config set --base-url https://app.fizzy.do --account 897362094`

3) Verify access
- `fizzy-cli auth status`
- `fizzy-cli account list`

## Common Tasks

Global flags (`--json`, `--plain`, `--account`, `--token`, etc.) work in any
position, before or after the subcommand.

### Boards
- List: `fizzy-cli board list`
- Create: `fizzy-cli board create --name "Roadmap"`
- Update: `fizzy-cli board update <board-id> --name "New name"`
- Delete: `fizzy-cli board delete <board-id>`
- Publish / unpublish: `fizzy-cli board publish <board-id>` / `fizzy-cli board unpublish <board-id>`
- List who has access: `fizzy-cli board accesses <board-id>`
- Watch / unwatch: `fizzy-cli board watch <board-id>` / `fizzy-cli board unwatch <board-id>`
- **No per-board auto-postpone:** the API cannot persist a board's
  auto-postpone period (`create` 422s; `update` silently no-ops it). Use
  the account-wide setting instead: `fizzy-cli account auto-postpone <days>`
  (valid: 3, 7, 11, 30, 90, 365).

### Cards
- List cards on a board:
  - `fizzy-cli card list --board-id <board-id>`
- Create card (tags are repeatable `--tag <title>`, not `--tag-id`):
  - `fizzy-cli card create --board-id <board-id> --title "Add dark mode" --description "Switch theme" --tag frontend`
- Upload image:
  - `fizzy-cli card create --board-id <board-id> --title "Add hero" --image ./hero.png`
- Update card:
  - `fizzy-cli card update <card-number> --title "Updated" --tag frontend`
- Publish a draft card:
  - `fizzy-cli card publish <card-number>`
- Move to Not Now:
  - `fizzy-cli card not-now <card-number>`
- Close / reopen:
  - `fizzy-cli card close <card-number>`
  - `fizzy-cli card reopen <card-number>`
- Triage / untriage:
  - `fizzy-cli card triage <card-number> --column-id <column-id>`
  - `fizzy-cli card untriage <card-number>`
- Assign:
  - `fizzy-cli card assign <card-number> --me`
  - `fizzy-cli card assign <card-number> --assignee-id <user-id>`
- Watch / pin / mark golden / mark read, and their inverses:
  - `fizzy-cli card watch <card-number>` / `unwatch`
  - `fizzy-cli card pin <card-number>` / `unpin`
  - `fizzy-cli card golden <card-number>` / `ungolden`
  - `fizzy-cli card read <card-number>` / `unread`
- Move to another board:
  - `fizzy-cli card move <card-number> --board-id <board-id>`

### Comments
- List comments:
  - `fizzy-cli comment list <card-number>`
- Create comment:
  - `fizzy-cli comment create <card-number> --body "Looks good"`

### Steps (checklist items)
- List: `fizzy-cli step list <card-number>`
- Add: `fizzy-cli step add <card-number> --content "Write tests"`
- Check / uncheck: `fizzy-cli step check <card-number> <step-id>` / `fizzy-cli step uncheck <card-number> <step-id>`

### Reactions
- List: `fizzy-cli reaction list <card-number>`
- Add: `fizzy-cli reaction add <card-number> --content "🎉"`
- Remove: `fizzy-cli reaction remove <card-number> <reaction-id>`
- Pass `--comment-id <id>` to react to a comment instead of the card.

### Pins
- List your pinned cards: `fizzy-cli pin list` (pin/unpin via `card pin`/`card unpin`)

### Tags, Columns, Users, Notifications
- Tags: `fizzy-cli tag list`
- Columns: `fizzy-cli column list --board-id <board-id>`
  - Create/update/delete: `fizzy-cli column create --board-id <board-id> --name "In Progress"`
  - List cards in a column: `fizzy-cli column cards --board-id <board-id> <column-id>`
  - Reorder: `fizzy-cli column move --board-id <board-id> <column-id> --left`
- Users: `fizzy-cli user list`
  - Set your timezone: `fizzy-cli user set-timezone America/New_York`
- Notifications: `fizzy-cli notification list --unread` (filters to unread only)
  - Mark read/unread: `fizzy-cli notification read <notification-id>` / `unread`
  - Mark all read: `fizzy-cli notification read-all`
  - Email settings: `fizzy-cli notification settings set --email-frequency daily`

### Activity & Search
- Recent activity: `fizzy-cli activity list --board-id <board-id>`
- Full-text card search: `fizzy-cli search "dark mode"`

### Auth Tokens
- List: `fizzy-cli auth token list`
- Create (and save as active token): `fizzy-cli auth token create --description "CI token" --permission write --save`
- Revoke: `fizzy-cli auth token revoke <token-id>`

### Webhooks (board admin)
- List: `fizzy-cli webhook list --board-id <board-id>`
- Create: `fizzy-cli webhook create --board-id <board-id> --name "CI hook" --url https://example.com/hook --event card_published`
- View deliveries: `fizzy-cli webhook deliveries --board-id <board-id> <webhook-id>`

### Exports
- Create and wait for a full account export: `fizzy-cli export create --wait`
- Create a per-user export: `fizzy-cli export create --user <user-id>`
- Download a completed export: `fizzy-cli export download <export-id> -o export.zip`

## Output Modes
- Default: human-readable tables.
- Machine output:
  - `--json` for raw API JSON.
  - `--plain` for stable line-based output.

## Config & Auth Notes
- Config file: `~/.config/fizzy/config.json`.
- Env vars: `FIZZY_BASE_URL`, `FIZZY_TOKEN`, `FIZZY_ACCOUNT`, `FIZZY_CONFIG`.
- Precedence: flags > env > config file > defaults.

## Troubleshooting
- If requests fail with auth errors, run `fizzy-cli auth status` and re-login.
- If account is missing, set it via `fizzy-cli account set <slug>` or `fizzy-cli config set --account <slug>`.
- Use `fizzy-cli --help` or `fizzy-cli help <command>` for full usage.
