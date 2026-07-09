package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"fizzy-cli/internal/api"
	"fizzy-cli/internal/config"
)

const (
	defaultBaseURL = "https://app.fizzy.do"
)

type Context struct {
	Config       config.Config
	ConfigPath   string
	Account      string
	BaseURL      string
	Token        string
	SessionToken string
	Output       OutputMode
	NoColor      bool
	Client       *api.Client
	Version      string
	Commit       string
	BuildDate    string
	Stdout       io.Writer
	Stderr       io.Writer
}

type UsageError struct {
	Msg string
}

func (e UsageError) Error() string { return e.Msg }

func Run(stdout, stderr io.Writer, version, commit, buildDate string, args []string) int {
	ctx, rest, showHelp, showVersion, err := parseGlobal(stderr, args)
	ctx.Stdout = stdout
	ctx.Stderr = stderr
	if err != nil {
		ctx.printErr(err)
		return exitCode(err)
	}
	if showVersion {
		fmt.Fprintf(stdout, "fizzy-cli %s (%s) %s\n", version, commit, buildDate)
		return 0
	}
	if showHelp {
		if len(rest) > 0 {
			fmt.Fprint(stdout, helpForCommand(rest[0]))
			return 0
		}
		fmt.Fprint(stdout, rootHelp)
		return 0
	}
	if len(rest) == 0 {
		fmt.Fprint(stdout, rootHelp)
		return 0
	}

	ctx.Version = version
	ctx.Commit = commit
	ctx.BuildDate = buildDate
	ctx.Client = api.NewClient(ctx.BaseURL, ctx.Token, ctx.SessionToken, fmt.Sprintf("fizzy-cli/%s", version))

	switch rest[0] {
	case "help":
		if len(rest) > 1 {
			fmt.Fprint(stdout, helpForCommand(rest[1]))
			return 0
		}
		fmt.Fprint(stdout, rootHelp)
		return 0
	case "auth":
		return runAuth(ctx, rest[1:])
	case "account":
		return runAccount(ctx, rest[1:])
	case "config":
		return runConfig(ctx, rest[1:])
	case "board":
		return runBoard(ctx, rest[1:])
	case "card":
		return runCard(ctx, rest[1:])
	case "comment":
		return runComment(ctx, rest[1:])
	case "tag":
		return runTag(ctx, rest[1:])
	case "column":
		return runColumn(ctx, rest[1:])
	case "user":
		return runUser(ctx, rest[1:])
	case "notification":
		return runNotification(ctx, rest[1:])
	case "step":
		return runStep(ctx, rest[1:])
	case "reaction":
		return runReaction(ctx, rest[1:])
	case "pin":
		return runPin(ctx, rest[1:])
	case "activity":
		return runActivity(ctx, rest[1:])
	default:
		ctx.printErr(UsageError{Msg: fmt.Sprintf("unknown command %q", rest[0])})
		fmt.Fprint(stderr, "\n")
		fmt.Fprint(stderr, rootHelp)
		return 2
	}
}

// globalBoolFlags are the boolean global flags recognised in any position.
var globalBoolFlags = map[string]bool{
	"json":     true,
	"plain":    true,
	"no-color": true,
	"help":     true,
	"h":        true,
	"version":  true,
}

// globalValueFlags are the value-taking global flags recognised in any position.
var globalValueFlags = map[string]bool{
	"base-url": true,
	"token":    true,
	"account":  true,
	"config":   true,
}

// commandOwnedFlags lists flags that a given command owns and that must NOT be
// hijacked as global flags when they appear at or after that command's token
// (e.g. `config set --account X`, `auth login --token X`).
var commandOwnedFlags = map[string]map[string]bool{
	"config": {"base-url": true, "account": true},
	"auth":   {"token": true},
}

// globalFlagValues holds the extracted global flag settings.
type globalFlagValues struct {
	baseURL string
	token   string
	account string
	config  string
	json    bool
	plain   bool
	noColor bool
	help    bool
	version bool
}

// splitFlagToken breaks a "--name" / "--name=value" / "-h" token into its flag
// name and (optional) inline value. It returns ok=false for non-flag tokens.
func splitFlagToken(tok string) (name, value string, hasValue, ok bool) {
	if len(tok) < 2 || tok[0] != '-' || tok == "--" {
		return "", "", false, false
	}
	trimmed := strings.TrimLeft(tok, "-")
	if trimmed == "" {
		return "", "", false, false
	}
	if idx := strings.IndexByte(trimmed, '='); idx >= 0 {
		return trimmed[:idx], trimmed[idx+1:], true, true
	}
	return trimmed, "", false, true
}

// extractGlobalFlags scans args (excluding the program name) left to right,
// pulling out recognised global flags wherever they appear and returning the
// remaining tokens for subcommand dispatch. Flags owned by the target command
// (see commandOwnedFlags) are left untouched so subcommands keep parsing them.
func extractGlobalFlags(args []string) (globalFlagValues, []string, error) {
	var g globalFlagValues
	rest := make([]string, 0, len(args))
	command := ""
	// prevWasSubcmdFlag is true when the previous token we passed through to
	// rest was a subcommand flag (a dash token that isn't a global flag). The
	// token after such a flag may be its value, so we must not mistake a value
	// that happens to look like a global value flag (e.g. `--description
	// --account`) for a real global flag and greedily consume it.
	prevWasSubcmdFlag := false

	i := 0
	for i < len(args) {
		tok := args[i]

		// A bare "--" ends flag parsing. At the top level it is just a
		// terminator to drop; once we're inside a subcommand, pass it (and the
		// rest) through so the subcommand's own parser treats it as such.
		if tok == "--" {
			if command == "" {
				rest = append(rest, args[i+1:]...)
			} else {
				rest = append(rest, args[i:]...)
			}
			break
		}

		name, value, hasValue, ok := splitFlagToken(tok)
		if !ok {
			if command == "" {
				command = tok
			}
			rest = append(rest, tok)
			prevWasSubcmdFlag = false
			i++
			continue
		}

		// Leave flags owned by the already-identified command to the subcommand.
		if command != "" {
			if owned, found := commandOwnedFlags[command]; found && owned[name] {
				rest = append(rest, tok)
				prevWasSubcmdFlag = true
				i++
				continue
			}
		}

		switch {
		case globalBoolFlags[name]:
			if hasValue {
				b, err := parseBoolFlag(value)
				if err != nil {
					return g, nil, UsageError{Msg: fmt.Sprintf("invalid boolean value %q for --%s", value, name)}
				}
				setGlobalBool(&g, name, b)
			} else {
				setGlobalBool(&g, name, true)
			}
			prevWasSubcmdFlag = false
			i++
		case globalValueFlags[name] && !prevWasSubcmdFlag:
			if hasValue {
				setGlobalValue(&g, name, value)
				i++
			} else {
				if i+1 >= len(args) {
					return g, nil, UsageError{Msg: fmt.Sprintf("flag --%s needs an argument", name)}
				}
				setGlobalValue(&g, name, args[i+1])
				i += 2
			}
			prevWasSubcmdFlag = false
		default:
			// Unknown flag (or a global value flag that is really the value of a
			// preceding subcommand flag): belongs to the subcommand.
			rest = append(rest, tok)
			prevWasSubcmdFlag = true
			i++
		}
	}

	return g, rest, nil
}

func parseBoolFlag(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "1", "t", "true", "yes":
		return true, nil
	case "0", "f", "false", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}

func setGlobalBool(g *globalFlagValues, name string, v bool) {
	switch name {
	case "json":
		g.json = v
	case "plain":
		g.plain = v
	case "no-color":
		g.noColor = v
	case "help", "h":
		g.help = v
	case "version":
		g.version = v
	}
}

func setGlobalValue(g *globalFlagValues, name, value string) {
	switch name {
	case "base-url":
		g.baseURL = value
	case "token":
		g.token = value
	case "account":
		g.account = value
	case "config":
		g.config = value
	}
}

func parseGlobal(stderr io.Writer, args []string) (Context, []string, bool, bool, error) {
	var ctx Context
	if len(args) == 0 {
		return ctx, nil, true, false, nil
	}

	defaultConfigPath := os.Getenv("FIZZY_CONFIG")
	if defaultConfigPath == "" {
		var err error
		defaultConfigPath, err = config.DefaultPath()
		if err != nil {
			return ctx, nil, false, false, err
		}
	}

	g, rest, err := extractGlobalFlags(args[1:])
	if err != nil {
		return ctx, nil, false, false, err
	}

	configPath := firstNonEmpty(g.config, defaultConfigPath)
	cfg, err := config.Load(configPath)
	if err != nil {
		return ctx, nil, false, false, err
	}

	ctx.ConfigPath = configPath
	ctx.Config = cfg
	ctx.Output = OutputMode{JSON: g.json, Plain: g.plain}
	ctx.NoColor = g.noColor || os.Getenv("NO_COLOR") != ""

	ctx.BaseURL = firstNonEmpty(g.baseURL, os.Getenv("FIZZY_BASE_URL"), cfg.BaseURL, defaultBaseURL)
	ctx.Token = firstNonEmpty(g.token, os.Getenv("FIZZY_TOKEN"), cfg.Token)
	ctx.SessionToken = cfg.SessionToken
	ctx.Account = normalizeAccount(firstNonEmpty(g.account, os.Getenv("FIZZY_ACCOUNT"), cfg.Account))

	if ctx.Output.JSON && ctx.Output.Plain {
		return ctx, nil, false, false, UsageError{Msg: "--json and --plain cannot be used together"}
	}

	return ctx, rest, g.help, g.version, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func normalizeAccount(value string) string {
	return strings.Trim(value, "/")
}

func (ctx Context) printErr(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(ctx.Stderr, "error: %s\n", err.Error())
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var usage UsageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}

func ensureAccount(ctx Context) error {
	if ctx.Account == "" {
		return UsageError{Msg: "missing account slug; set --account or FIZZY_ACCOUNT, or run 'fizzy-cli account set'"}
	}
	return nil
}

func ensureToken(ctx Context) error {
	if ctx.Token == "" && ctx.SessionToken == "" {
		return UsageError{Msg: "missing credentials; set --token or FIZZY_TOKEN, or run 'fizzy-cli auth login'"}
	}
	return nil
}

func withAccount(ctx Context, path string) string {
	return "/" + ctx.Account + path
}

func requestContext() context.Context {
	return context.Background()
}
