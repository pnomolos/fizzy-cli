package cli

import (
	"context"
	"errors"
	"flag"
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
	if showHelp || len(rest) == 0 {
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
	default:
		ctx.printErr(UsageError{Msg: fmt.Sprintf("unknown command %q", rest[0])})
		fmt.Fprint(stderr, "\n")
		fmt.Fprint(stderr, rootHelp)
		return 2
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

	fs := flag.NewFlagSet("fizzy-cli", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		flagBaseURL string
		flagToken   string
		flagAccount string
		flagConfig  string
		flagJSON    bool
		flagPlain   bool
		flagNoColor bool
		flagHelp    bool
		flagVersion bool
	)

	fs.StringVar(&flagBaseURL, "base-url", "", "API base URL")
	fs.StringVar(&flagToken, "token", "", "Personal access token")
	fs.StringVar(&flagAccount, "account", "", "Account slug")
	fs.StringVar(&flagConfig, "config", defaultConfigPath, "Config file path")
	fs.BoolVar(&flagJSON, "json", false, "JSON output")
	fs.BoolVar(&flagPlain, "plain", false, "Plain output")
	fs.BoolVar(&flagNoColor, "no-color", false, "Disable color")
	fs.BoolVar(&flagHelp, "help", false, "Show help")
	fs.BoolVar(&flagHelp, "h", false, "Show help")
	fs.BoolVar(&flagVersion, "version", false, "Print version")

	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ctx, nil, true, false, nil
		}
		return ctx, nil, false, false, UsageError{Msg: err.Error()}
	}

	cfg, err := config.Load(flagConfig)
	if err != nil {
		return ctx, nil, false, false, err
	}

	ctx.ConfigPath = flagConfig
	ctx.Config = cfg
	ctx.Output = OutputMode{JSON: flagJSON, Plain: flagPlain}
	_ = flagNoColor

	ctx.BaseURL = firstNonEmpty(flagBaseURL, os.Getenv("FIZZY_BASE_URL"), cfg.BaseURL, defaultBaseURL)
	ctx.Token = firstNonEmpty(flagToken, os.Getenv("FIZZY_TOKEN"), cfg.Token)
	ctx.SessionToken = cfg.SessionToken
	ctx.Account = normalizeAccount(firstNonEmpty(flagAccount, os.Getenv("FIZZY_ACCOUNT"), cfg.Account))

	if ctx.Output.JSON && ctx.Output.Plain {
		return ctx, nil, false, false, UsageError{Msg: "--json and --plain cannot be used together"}
	}

	return ctx, fs.Args(), flagHelp, flagVersion, nil
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
