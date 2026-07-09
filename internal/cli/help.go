package cli

import "fmt"

const rootHelp = `fizzy-cli - CLI for the Fizzy kanban board

USAGE:
  fizzy-cli [global flags] <command> [args]

EXAMPLES:
  fizzy-cli auth login --token $FIZZY_TOKEN
  fizzy-cli account list
  fizzy-cli account set 897362094
  fizzy-cli board list
  fizzy-cli card list --board-id 03f5v9zkft4hj9qq0lsn9ohcm
  fizzy-cli card create --board-id 03f5v9zkft4hj9qq0lsn9ohcm --title "Add dark mode" --description "Switch theme"
  fizzy-cli comment list 4
  fizzy-cli notification list --unread

COMMANDS:
  auth              Manage authentication
  account           Account selection and identity info
  config            Manage persisted defaults
  board             Manage boards
  card              Manage cards
  comment           Manage card comments
  step              Manage card steps (checklist items)
  reaction          Manage reactions on cards and comments
  pin               List your pinned cards
  tag               List tags
  column            Manage columns
  user              Manage users
  notification      Manage notifications
  activity          List account activity
  search            Search cards
  help              Show help for a command

GLOBAL FLAGS:
  --base-url string   API base URL (env: FIZZY_BASE_URL, default: https://app.fizzy.do)
  --token string      Personal access token (env: FIZZY_TOKEN)
  --account string    Account slug (env: FIZZY_ACCOUNT)
  --config string     Config file path (env: FIZZY_CONFIG)
  --json              JSON output
  --plain             Plain, line-oriented output
  --no-color          Disable color (respects NO_COLOR by default)
  -h, --help          Show help
  --version           Print version
`

func helpForAuth() string {
	return `USAGE:
  fizzy-cli auth login [--token TOKEN]
  fizzy-cli auth login --email user@example.com [--code ABC123]
  fizzy-cli auth logout
  fizzy-cli auth status
  fizzy-cli auth token list
  fizzy-cli auth token create --description TEXT --permission read|write [--save]
  fizzy-cli auth token revoke <token-id>

FLAGS:
  --token string   Personal access token (reads from stdin or prompt if omitted)
  --email string   Email address for magic-link login
  --code string    Magic-link code (required if not running in a TTY)

NOTES:
  'auth token' manages personal access tokens and is unscoped (not
  account-specific). 'create' prints the token value once; use --save to
  also write it to the config file as the active token.
`
}

func helpForAccount() string {
	return `USAGE:
  fizzy-cli account list
  fizzy-cli account set <account-slug>
  fizzy-cli account get
  fizzy-cli account update --name TEXT
  fizzy-cli account auto-postpone <days>
  fizzy-cli account join-code [get]
  fizzy-cli account join-code set-limit <n>
  fizzy-cli account join-code reset

NOTES:
  Account slugs can be provided with or without a leading slash.
  'auto-postpone' sets the account-wide default (persists, unlike a
  board's auto-postpone setting); valid values are 3, 7, 11, 30, 90, 365.
  'join-code' subcommands require admin privileges to modify.
`
}

func helpForConfig() string {
	return `USAGE:
  fizzy-cli config show
  fizzy-cli config set [--base-url URL] [--account SLUG]
`
}

func helpForBoard() string {
	return `USAGE:
  fizzy-cli board list
  fizzy-cli board get <board-id>
  fizzy-cli board create --name <name> [--all-access] [--public-description TEXT]
  fizzy-cli board update <board-id> [--name <name>] [--all-access] [--no-all-access] [--public-description TEXT] [--user-id ID ...]
  fizzy-cli board delete <board-id>
`
}

func helpForCard() string {
	return `USAGE:
  fizzy-cli card list [filters]
  fizzy-cli card get <card-number>
  fizzy-cli card create --board-id <board-id> --title <title> [--description TEXT] [--tag TITLE ...] [--image PATH]
  fizzy-cli card update <card-number> [--title TEXT] [--description TEXT] [--tag TITLE ...] [--image PATH]
  fizzy-cli card delete <card-number>
  fizzy-cli card publish <card-number>
  fizzy-cli card close <card-number>
  fizzy-cli card reopen <card-number>
  fizzy-cli card not-now <card-number>
  fizzy-cli card triage <card-number> --column-id <column-id>
  fizzy-cli card untriage <card-number>
  fizzy-cli card tag <card-number> --title <tag-title>
  fizzy-cli card assign <card-number> --assignee-id <user-id>
  fizzy-cli card assign <card-number> --me
  fizzy-cli card watch <card-number>
  fizzy-cli card unwatch <card-number>
  fizzy-cli card pin <card-number>
  fizzy-cli card unpin <card-number>
  fizzy-cli card move <card-number> --board-id <board-id>
  fizzy-cli card golden <card-number>
  fizzy-cli card ungolden <card-number>
  fizzy-cli card remove-image <card-number>
  fizzy-cli card read <card-number>
  fizzy-cli card unread <card-number>

NOTES:
  --me and --assignee-id are mutually exclusive on 'card assign'.

FILTERS:
  --board-id ID           repeatable
  --tag-id ID             repeatable
  --column-id ID          repeatable
  --assignee-id ID        repeatable
  --creator-id ID         repeatable
  --closer-id ID          repeatable
  --card-id ID            repeatable
  --indexed-by VALUE      all|closed|not_now|stalled|postponing_soon|golden
  --sorted-by VALUE       latest|newest|oldest
  --assignment-status     unassigned
  --creation VALUE        today|yesterday|thisweek|lastweek|thismonth|lastmonth|thisyear|lastyear
  --closure VALUE         today|yesterday|thisweek|lastweek|thismonth|lastmonth|thisyear|lastyear
  --term VALUE            repeatable search terms
  --all                   follow pagination
`
}

func helpForComment() string {
	return `USAGE:
  fizzy-cli comment list <card-number>
  fizzy-cli comment get <card-number> <comment-id>
  fizzy-cli comment create <card-number> --body <text>
  fizzy-cli comment update <card-number> <comment-id> --body <text>
  fizzy-cli comment delete <card-number> <comment-id>
`
}

func helpForStep() string {
	return `USAGE:
  fizzy-cli step list <card-number>
  fizzy-cli step add <card-number> --content TEXT [--completed]
  fizzy-cli step update <card-number> <step-id> [--content TEXT] [--completed|--not-completed]
  fizzy-cli step check <card-number> <step-id>
  fizzy-cli step uncheck <card-number> <step-id>
  fizzy-cli step delete <card-number> <step-id>
`
}

func helpForReaction() string {
	return `USAGE:
  fizzy-cli reaction list <card-number> [--comment-id ID]
  fizzy-cli reaction add <card-number> --content EMOJI [--comment-id ID]
  fizzy-cli reaction remove <card-number> <reaction-id> [--comment-id ID]

NOTES:
  --content is limited to 16 characters. Pass --comment-id to react to a
  comment instead of the card itself.
`
}

func helpForPin() string {
	return `USAGE:
  fizzy-cli pin list

NOTES:
  Lists your pinned cards. Pin or unpin a card with 'card pin <n>' /
  'card unpin <n>'.
`
}

func helpForTag() string {
	return `USAGE:
  fizzy-cli tag list
`
}

func helpForColumn() string {
	return `USAGE:
  fizzy-cli column list --board-id <board-id>
  fizzy-cli column get --board-id <board-id> <column-id>
  fizzy-cli column create --board-id <board-id> --name <name> [--color <color>]
  fizzy-cli column update --board-id <board-id> <column-id> [--name <name>] [--color <color>]
  fizzy-cli column delete --board-id <board-id> <column-id>
`
}

func helpForUser() string {
	return `USAGE:
  fizzy-cli user list
  fizzy-cli user get <user-id>
  fizzy-cli user update <user-id> [--name <name>] [--avatar PATH]
  fizzy-cli user deactivate <user-id>
`
}

func helpForNotification() string {
	return `USAGE:
  fizzy-cli notification list [--unread]
  fizzy-cli notification read <notification-id>
  fizzy-cli notification unread <notification-id>
  fizzy-cli notification read-all
`
}

func helpForActivity() string {
	return `USAGE:
  fizzy-cli activity list [--board-id ID ...] [--creator-id ID ...] [--all]

NOTES:
  Description text has HTML tags/entities stripped for table display; use
  --json for the raw description and particulars.
`
}

func helpForSearch() string {
	return `USAGE:
  fizzy-cli search <query> [--all]

NOTES:
  Uses an undocumented-but-stable Fizzy endpoint (experimental: shape may
  change without notice). Results are full card objects, rendered like
  'card list'.
`
}

func helpForCommand(cmd string) string {
	switch cmd {
	case "auth":
		return helpForAuth()
	case "account":
		return helpForAccount()
	case "config":
		return helpForConfig()
	case "board":
		return helpForBoard()
	case "card":
		return helpForCard()
	case "comment":
		return helpForComment()
	case "step":
		return helpForStep()
	case "reaction":
		return helpForReaction()
	case "pin":
		return helpForPin()
	case "tag":
		return helpForTag()
	case "column":
		return helpForColumn()
	case "user":
		return helpForUser()
	case "notification":
		return helpForNotification()
	case "activity":
		return helpForActivity()
	case "search":
		return helpForSearch()
	default:
		return fmt.Sprintf("Unknown command %q.\n\n%s", cmd, rootHelp)
	}
}
