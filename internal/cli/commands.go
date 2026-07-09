package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"fizzy-cli/internal/api"
	"fizzy-cli/internal/config"
)

func runAuth(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForAuth())
		return 2
	}
	switch args[0] {
	case "login":
		fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		token := fs.String("token", "", "Personal access token")
		email := fs.String("email", "", "Email address for magic-link login")
		code := fs.String("code", "", "Magic-link code")
		if err := fs.Parse(args[1:]); err != nil {
			return ctx.usageError(helpForAuth(), err)
		}
		if strings.TrimSpace(*token) != "" && strings.TrimSpace(*email) != "" {
			return ctx.handleErr(helpForAuth(), UsageError{Msg: "--token and --email cannot be used together"})
		}
		if strings.TrimSpace(*email) != "" {
			return authMagicLink(ctx, strings.TrimSpace(*email), strings.TrimSpace(*code))
		}
		val := strings.TrimSpace(*token)
		if val == "" {
			readToken, err := readSecret(ctx, "Token")
			if err != nil {
				return ctx.handleErr(helpForAuth(), err)
			}
			val = readToken
		}
		if val == "" {
			return ctx.handleErr(helpForAuth(), UsageError{Msg: "token is required"})
		}
		cfg := ctx.Config
		cfg.Token = val
		cfg.SessionToken = ""
		if err := configSave(ctx.ConfigPath, cfg); err != nil {
			return ctx.handleErr(helpForAuth(), err)
		}
		fmt.Fprintf(ctx.Stdout, "Token saved to %s\n", ctx.ConfigPath)
		return 0
	case "logout":
		cfg := ctx.Config
		cfg.Token = ""
		cfg.SessionToken = ""
		if err := configSave(ctx.ConfigPath, cfg); err != nil {
			return ctx.handleErr(helpForAuth(), err)
		}
		fmt.Fprintln(ctx.Stdout, "Credentials cleared.")
		return 0
	case "status":
		if ctx.Token == "" && ctx.SessionToken == "" {
			fmt.Fprintln(ctx.Stdout, "Not logged in (no credentials configured).")
			return 0
		}
		if err := ensureToken(ctx); err != nil {
			return ctx.handleErr(helpForAuth(), err)
		}
		resp, err := ctx.Client.Do(requestContext(), "GET", "/my/identity", nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForAuth(), err)
		}
		if ctx.Output.JSON {
			return ctx.printJSONResponse(resp)
		}
		authType := "session token"
		if ctx.Token != "" {
			authType = "personal access token"
		}
		fmt.Fprintf(ctx.Stdout, "Authenticated using %s. Accessible accounts:\n", authType)
		rows, err := identityToRows(resp.Body)
		if err != nil {
			return ctx.handleErr(helpForAuth(), err)
		}
		printTable(ctx.Stdout, []string{"SLUG", "NAME", "USER"}, rows, ctx.Output.Plain)
		return 0
	case "token":
		return runAuthToken(ctx, args[1:])
	default:
		fmt.Fprint(ctx.Stderr, helpForAuth())
		return 2
	}
}

// runAuthToken manages personal access tokens via the unscoped
// /my/access_tokens endpoints (no account prefix).
func runAuthToken(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForAuth())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForAuth(), err)
	}
	switch args[0] {
	case "list":
		resp, err := ctx.Client.Do(requestContext(), "GET", "/my/access_tokens", nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForAuth(), err)
		}
		return outputListOrJSON(ctx, resp, accessTokenListHeaders, accessTokenListRows)
	case "create":
		fs := flag.NewFlagSet("auth token create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		description := fs.String("description", "", "Token description")
		permission := fs.String("permission", "", "Token permission: read|write")
		save := fs.Bool("save", false, "Save the created token to config")
		if err := fs.Parse(args[1:]); err != nil {
			return ctx.usageError(helpForAuth(), err)
		}
		description2 := strings.TrimSpace(*description)
		if description2 == "" {
			return ctx.handleErr(helpForAuth(), UsageError{Msg: "--description is required"})
		}
		perm := strings.TrimSpace(*permission)
		if perm != "read" && perm != "write" {
			return ctx.handleErr(helpForAuth(), UsageError{Msg: "--permission must be read or write"})
		}
		payload := map[string]any{"access_token": map[string]any{
			"description": description2,
			"permission":  perm,
		}}
		resp, err := ctx.Client.Do(requestContext(), "POST", "/my/access_tokens", nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForAuth(), err)
		}
		var created accessTokenCreated
		if err := json.Unmarshal(resp.Body, &created); err != nil {
			return ctx.handleErr(helpForAuth(), err)
		}
		if *save {
			cfg := ctx.Config
			cfg.Token = created.Token
			cfg.SessionToken = ""
			if err := configSave(ctx.ConfigPath, cfg); err != nil {
				return ctx.handleErr(helpForAuth(), err)
			}
		}
		if ctx.Output.JSON {
			return ctx.printJSONResponse(resp)
		}
		// The token value is only ever shown once; print it plainly so it can be
		// captured/piped.
		fmt.Fprintf(ctx.Stdout, "Token: %s\n", created.Token)
		fmt.Fprintln(ctx.Stdout, "Store this token now; it will not be shown again.")
		if *save {
			fmt.Fprintf(ctx.Stdout, "Saved to %s\n", ctx.ConfigPath)
		}
		return 0
	case "revoke":
		if len(args) < 2 {
			return ctx.handleErr(helpForAuth(), UsageError{Msg: "token id is required"})
		}
		resp, err := ctx.Client.Do(requestContext(), "DELETE", "/my/access_tokens/"+args[1], nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForAuth(), err)
		}
		return outputNoContent(ctx, resp, "Token revoked")
	default:
		fmt.Fprint(ctx.Stderr, helpForAuth())
		return 2
	}
}

type pendingAuthResponse struct {
	PendingAuthenticationToken string `json:"pending_authentication_token"`
}

type sessionAuthResponse struct {
	SessionToken string `json:"session_token"`
}

func authMagicLink(ctx Context, email, code string) int {
	if strings.TrimSpace(email) == "" {
		return ctx.handleErr(helpForAuth(), UsageError{Msg: "--email is required"})
	}
	client := api.NewClient(ctx.BaseURL, "", "", fmt.Sprintf("fizzy-cli/%s", ctx.Version))
	request := map[string]any{"email_address": email}
	resp, err := client.Do(requestContext(), "POST", "/session", nil, bytes.NewBuffer(mustJSON(request)), "application/json", nil)
	if err != nil {
		return ctx.handleErr(helpForAuth(), err)
	}
	var pending pendingAuthResponse
	if err := json.Unmarshal(resp.Body, &pending); err != nil {
		return ctx.handleErr(helpForAuth(), err)
	}
	if strings.TrimSpace(pending.PendingAuthenticationToken) == "" {
		return ctx.handleErr(helpForAuth(), errors.New("missing pending_authentication_token in response"))
	}

	if strings.TrimSpace(code) == "" {
		if !isTTY(os.Stdin) {
			return ctx.handleErr(helpForAuth(), UsageError{Msg: "--code is required when not running in a TTY"})
		}
		readCode, err := readSecret(ctx, "Magic link code")
		if err != nil {
			return ctx.handleErr(helpForAuth(), err)
		}
		code = readCode
	}
	if strings.TrimSpace(code) == "" {
		return ctx.handleErr(helpForAuth(), UsageError{Msg: "magic link code is required"})
	}

	verifyRequest := map[string]any{"code": strings.TrimSpace(code)}
	headers := map[string]string{"Cookie": "pending_authentication_token=" + pending.PendingAuthenticationToken}
	verifyResp, err := client.Do(requestContext(), "POST", "/session/magic_link", nil, bytes.NewBuffer(mustJSON(verifyRequest)), "application/json", headers)
	if err != nil {
		return ctx.handleErr(helpForAuth(), err)
	}
	var session sessionAuthResponse
	if err := json.Unmarshal(verifyResp.Body, &session); err != nil {
		return ctx.handleErr(helpForAuth(), err)
	}
	if strings.TrimSpace(session.SessionToken) == "" {
		return ctx.handleErr(helpForAuth(), errors.New("missing session_token in response"))
	}
	cfg := ctx.Config
	cfg.SessionToken = session.SessionToken
	cfg.Token = ""
	if err := configSave(ctx.ConfigPath, cfg); err != nil {
		return ctx.handleErr(helpForAuth(), err)
	}
	fmt.Fprintf(ctx.Stdout, "Session saved to %s\n", ctx.ConfigPath)
	return 0
}

func runAccount(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForAccount())
		return 2
	}
	switch args[0] {
	case "list":
		if err := ensureToken(ctx); err != nil {
			return ctx.handleErr(helpForAccount(), err)
		}
		resp, err := ctx.Client.Do(requestContext(), "GET", "/my/identity", nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForAccount(), err)
		}
		if ctx.Output.JSON {
			return ctx.printJSONResponse(resp)
		}
		rows, err := identityToRows(resp.Body)
		if err != nil {
			return ctx.handleErr(helpForAccount(), err)
		}
		printTable(ctx.Stdout, []string{"SLUG", "NAME", "USER"}, rows, ctx.Output.Plain)
		return 0
	case "set":
		if len(args) < 2 {
			return ctx.handleErr(helpForAccount(), UsageError{Msg: "account slug is required"})
		}
		slug := normalizeAccount(args[1])
		if slug == "" {
			return ctx.handleErr(helpForAccount(), UsageError{Msg: "account slug is required"})
		}
		cfg := ctx.Config
		cfg.Account = slug
		if err := configSave(ctx.ConfigPath, cfg); err != nil {
			return ctx.handleErr(helpForAccount(), err)
		}
		fmt.Fprintf(ctx.Stdout, "Default account set to %s\n", slug)
		return 0
	default:
		fmt.Fprint(ctx.Stderr, helpForAccount())
		return 2
	}
}

func runConfig(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForConfig())
		return 2
	}
	switch args[0] {
	case "show":
		if ctx.Output.JSON {
			payload := map[string]any{
				"base_url":          firstNonEmpty(ctx.Config.BaseURL, ctx.BaseURL),
				"account":           ctx.Config.Account,
				"token_set":         ctx.Config.Token != "",
				"session_token_set": ctx.Config.SessionToken != "",
				"config_path":       ctx.ConfigPath,
			}
			if err := printJSON(ctx.Stdout, payload); err != nil {
				return ctx.handleErr(helpForConfig(), err)
			}
			return 0
		}
		fmt.Fprintf(ctx.Stdout, "Config path: %s\n", ctx.ConfigPath)
		fmt.Fprintf(ctx.Stdout, "Base URL: %s\n", firstNonEmpty(ctx.Config.BaseURL, ctx.BaseURL))
		if ctx.Config.Account != "" {
			fmt.Fprintf(ctx.Stdout, "Account: %s\n", ctx.Config.Account)
		}
		fmt.Fprintf(ctx.Stdout, "Token set: %t\n", ctx.Config.Token != "")
		fmt.Fprintf(ctx.Stdout, "Session token set: %t\n", ctx.Config.SessionToken != "")
		return 0
	case "set":
		fs := flag.NewFlagSet("config set", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		baseURL := fs.String("base-url", "", "API base URL")
		account := fs.String("account", "", "Account slug")
		if err := fs.Parse(args[1:]); err != nil {
			return ctx.usageError(helpForConfig(), err)
		}
		if strings.TrimSpace(*baseURL) == "" && strings.TrimSpace(*account) == "" {
			return ctx.handleErr(helpForConfig(), UsageError{Msg: "at least one of --base-url or --account is required"})
		}
		cfg := ctx.Config
		if strings.TrimSpace(*baseURL) != "" {
			cfg.BaseURL = strings.TrimSpace(*baseURL)
		}
		if strings.TrimSpace(*account) != "" {
			cfg.Account = normalizeAccount(*account)
		}
		if err := configSave(ctx.ConfigPath, cfg); err != nil {
			return ctx.handleErr(helpForConfig(), err)
		}
		fmt.Fprintln(ctx.Stdout, "Config updated.")
		return 0
	default:
		fmt.Fprint(ctx.Stderr, helpForConfig())
		return 2
	}
}

func runBoard(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForBoard())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForBoard(), err)
	}
	if err := ensureAccount(ctx); err != nil {
		return ctx.handleErr(helpForBoard(), err)
	}
	switch args[0] {
	case "list":
		resp, err := ctx.Client.Do(requestContext(), "GET", withAccount(ctx, "/boards"), nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForBoard(), err)
		}
		return outputListOrJSON(ctx, resp, boardListHeaders, boardListRows)
	case "get":
		if len(args) < 2 {
			return ctx.handleErr(helpForBoard(), UsageError{Msg: "board id is required"})
		}
		resp, err := ctx.Client.Do(requestContext(), "GET", withAccount(ctx, "/boards/"+args[1]), nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForBoard(), err)
		}
		return outputJSONOrPretty(ctx, resp.Body, formatBoard)
	case "create":
		fs := flag.NewFlagSet("board create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		name := fs.String("name", "", "Board name")
		allAccess := fs.Bool("all-access", true, "Allow all access")
		publicDesc := fs.String("public-description", "", "Public description")
		if err := fs.Parse(args[1:]); err != nil {
			return ctx.usageError(helpForBoard(), err)
		}
		if strings.TrimSpace(*name) == "" {
			return ctx.handleErr(helpForBoard(), UsageError{Msg: "--name is required"})
		}
		payload := map[string]any{
			"board": map[string]any{
				"name":       strings.TrimSpace(*name),
				"all_access": *allAccess,
			},
		}
		if strings.TrimSpace(*publicDesc) != "" {
			payload["board"].(map[string]any)["public_description"] = *publicDesc
		}
		resp, err := ctx.Client.Do(requestContext(), "POST", withAccount(ctx, "/boards"), nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForBoard(), err)
		}
		return outputLocation(ctx, resp, "Board created")
	case "update":
		if len(args) < 2 {
			return ctx.handleErr(helpForBoard(), UsageError{Msg: "board id is required"})
		}
		fs := flag.NewFlagSet("board update", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		name := fs.String("name", "", "Board name")
		allAccess := fs.Bool("all-access", false, "Allow all access")
		noAllAccess := fs.Bool("no-all-access", false, "Disable all access")
		publicDesc := fs.String("public-description", "", "Public description")
		userIDs := multiString{}
		fs.Var(&userIDs, "user-id", "User ID (repeatable)")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForBoard(), err)
		}
		if *allAccess && *noAllAccess {
			return ctx.handleErr(helpForBoard(), UsageError{Msg: "--all-access and --no-all-access cannot be used together"})
		}
		board := map[string]any{}
		if strings.TrimSpace(*name) != "" {
			board["name"] = strings.TrimSpace(*name)
		}
		if *allAccess {
			board["all_access"] = true
		}
		if *noAllAccess {
			board["all_access"] = false
		}
		if strings.TrimSpace(*publicDesc) != "" {
			board["public_description"] = *publicDesc
		}
		if len(board) == 0 && len(userIDs.values) == 0 {
			return ctx.handleErr(helpForBoard(), UsageError{Msg: "no fields to update"})
		}
		// The server reads user_ids as a top-level param, a sibling of board
		// (not nested inside it), so send it alongside the board object. It also
		// rejects an empty board object, and a board restricted to a specific
		// member list is by definition not all-access, so default all_access to
		// false when only members are being changed.
		if len(userIDs.values) > 0 && len(board) == 0 {
			board["all_access"] = false
		}
		payload := map[string]any{"board": board}
		if len(userIDs.values) > 0 {
			payload["user_ids"] = userIDs.values
		}
		resp, err := ctx.Client.Do(requestContext(), "PUT", withAccount(ctx, "/boards/"+args[1]), nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForBoard(), err)
		}
		return outputNoContent(ctx, resp, "Board updated")
	case "delete":
		if len(args) < 2 {
			return ctx.handleErr(helpForBoard(), UsageError{Msg: "board id is required"})
		}
		resp, err := ctx.Client.Do(requestContext(), "DELETE", withAccount(ctx, "/boards/"+args[1]), nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForBoard(), err)
		}
		return outputNoContent(ctx, resp, "Board deleted")
	default:
		fmt.Fprint(ctx.Stderr, helpForBoard())
		return 2
	}
}

func runCard(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForCard())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForCard(), err)
	}
	if err := ensureAccount(ctx); err != nil {
		return ctx.handleErr(helpForCard(), err)
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("card list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		boardIDs := multiString{}
		tagIDs := multiString{}
		columnIDs := multiString{}
		assigneeIDs := multiString{}
		creatorIDs := multiString{}
		closerIDs := multiString{}
		cardIDs := multiString{}
		terms := multiString{}
		indexedBy := fs.String("indexed-by", "", "Index filter")
		sortedBy := fs.String("sorted-by", "", "Sort order")
		assignmentStatus := fs.String("assignment-status", "", "Assignment status")
		creation := fs.String("creation", "", "Creation date filter")
		closure := fs.String("closure", "", "Closure date filter")
		all := fs.Bool("all", false, "Fetch all pages")
		fs.Var(&boardIDs, "board-id", "Board ID filter")
		fs.Var(&tagIDs, "tag-id", "Tag ID filter")
		fs.Var(&columnIDs, "column-id", "Column ID filter")
		fs.Var(&assigneeIDs, "assignee-id", "Assignee ID filter")
		fs.Var(&creatorIDs, "creator-id", "Creator ID filter")
		fs.Var(&closerIDs, "closer-id", "Closer ID filter")
		fs.Var(&cardIDs, "card-id", "Card ID filter")
		fs.Var(&terms, "term", "Search term")
		if err := fs.Parse(args[1:]); err != nil {
			return ctx.usageError(helpForCard(), err)
		}
		query := url.Values{}
		addListParam(query, "board_ids[]", boardIDs.values)
		addListParam(query, "tag_ids[]", tagIDs.values)
		addListParam(query, "column_ids[]", columnIDs.values)
		addListParam(query, "assignee_ids[]", assigneeIDs.values)
		addListParam(query, "creator_ids[]", creatorIDs.values)
		addListParam(query, "closer_ids[]", closerIDs.values)
		addListParam(query, "card_ids[]", cardIDs.values)
		addListParam(query, "terms[]", terms.values)
		setStringParam(query, "indexed_by", *indexedBy)
		setStringParam(query, "sorted_by", *sortedBy)
		setStringParam(query, "assignment_status", *assignmentStatus)
		setStringParam(query, "creation", *creation)
		setStringParam(query, "closure", *closure)

		return listWithPagination(ctx, helpForCard(), withAccount(ctx, "/cards"), query, *all, cardListHeaders, cardListRows)
	case "get":
		if len(args) < 2 {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "card number is required"})
		}
		resp, err := ctx.Client.Do(requestContext(), "GET", withAccount(ctx, "/cards/"+args[1]), nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForCard(), err)
		}
		return outputJSONOrPretty(ctx, resp.Body, formatCard)
	case "create":
		fs := flag.NewFlagSet("card create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		boardID := fs.String("board-id", "", "Board ID")
		title := fs.String("title", "", "Card title")
		description := fs.String("description", "", "Card description")
		imagePath := fs.String("image", "", "Image file path")
		tags := multiString{}
		fs.Var(&tags, "tag", "Tag title (repeatable)")
		if err := fs.Parse(args[1:]); err != nil {
			return ctx.usageError(helpForCard(), err)
		}
		if strings.TrimSpace(*boardID) == "" || strings.TrimSpace(*title) == "" {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "--board-id and --title are required"})
		}
		path := withAccount(ctx, "/boards/"+strings.TrimSpace(*boardID)+"/cards")
		var resp *api.Response
		if strings.TrimSpace(*imagePath) != "" {
			fields := map[string][]string{
				"title": {strings.TrimSpace(*title)},
			}
			if strings.TrimSpace(*description) != "" {
				fields["description"] = []string{*description}
			}
			body, contentType, err := multipartBody("card", fields, "image", *imagePath)
			if err != nil {
				return ctx.handleErr(helpForCard(), err)
			}
			resp, err = ctx.Client.Do(requestContext(), "POST", path, nil, body, contentType, nil)
			if err != nil {
				return ctx.handleErr(helpForCard(), err)
			}
		} else {
			card := map[string]any{
				"title": strings.TrimSpace(*title),
			}
			if strings.TrimSpace(*description) != "" {
				card["description"] = *description
			}
			payload := map[string]any{"card": card}
			var err error
			resp, err = ctx.Client.Do(requestContext(), "POST", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
			if err != nil {
				return ctx.handleErr(helpForCard(), err)
			}
		}
		if len(tags.values) > 0 {
			number := cardNumberFromLocation(resp.Headers.Get("Location"))
			if number == "" {
				return ctx.handleErr(helpForCard(), errors.New("could not determine created card number for tagging"))
			}
			if err := attachTags(ctx, number, tags.values); err != nil {
				return ctx.handleErr(helpForCard(), err)
			}
		}
		return outputLocation(ctx, resp, "Card created")
	case "update":
		if len(args) < 2 {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "card number is required"})
		}
		fs := flag.NewFlagSet("card update", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		title := fs.String("title", "", "Card title")
		description := fs.String("description", "", "Card description")
		imagePath := fs.String("image", "", "Image file path")
		tags := multiString{}
		fs.Var(&tags, "tag", "Tag title (repeatable)")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForCard(), err)
		}
		card := map[string]any{}
		if strings.TrimSpace(*title) != "" {
			card["title"] = strings.TrimSpace(*title)
		}
		if strings.TrimSpace(*description) != "" {
			card["description"] = *description
		}
		hasTags := len(tags.values) > 0
		if len(card) == 0 && !hasTags && strings.TrimSpace(*imagePath) == "" {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "no fields to update"})
		}
		var resp *api.Response
		if strings.TrimSpace(*imagePath) != "" {
			fields := map[string][]string{}
			if titleVal, ok := card["title"].(string); ok && titleVal != "" {
				fields["title"] = []string{titleVal}
			}
			if descVal, ok := card["description"].(string); ok && descVal != "" {
				fields["description"] = []string{descVal}
			}
			body, contentType, err := multipartBody("card", fields, "image", *imagePath)
			if err != nil {
				return ctx.handleErr(helpForCard(), err)
			}
			resp, err = ctx.Client.Do(requestContext(), "PUT", withAccount(ctx, "/cards/"+args[1]), nil, body, contentType, nil)
			if err != nil {
				return ctx.handleErr(helpForCard(), err)
			}
		} else if len(card) > 0 {
			payload := map[string]any{"card": card}
			var err error
			resp, err = ctx.Client.Do(requestContext(), "PUT", withAccount(ctx, "/cards/"+args[1]), nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
			if err != nil {
				return ctx.handleErr(helpForCard(), err)
			}
		}
		if hasTags {
			if err := attachTags(ctx, args[1], tags.values); err != nil {
				return ctx.handleErr(helpForCard(), err)
			}
		}
		if ctx.Output.JSON {
			if resp != nil {
				return ctx.printJSONResponse(resp)
			}
			if err := printJSON(ctx.Stdout, map[string]any{"status": 200}); err != nil {
				return ctx.handleErr(helpForCard(), err)
			}
			return 0
		}
		fmt.Fprintln(ctx.Stdout, "Card updated.")
		return 0
	case "delete":
		if len(args) < 2 {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "card number is required"})
		}
		resp, err := ctx.Client.Do(requestContext(), "DELETE", withAccount(ctx, "/cards/"+args[1]), nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForCard(), err)
		}
		return outputNoContent(ctx, resp, "Card deleted")
	case "publish":
		return simpleCardAction(ctx, helpForCard(), args, "publish", "POST", "/publish", "Card published")
	case "close":
		return simpleCardAction(ctx, helpForCard(), args, "close", "POST", "/closure", "Card closed")
	case "reopen":
		return simpleCardAction(ctx, helpForCard(), args, "reopen", "DELETE", "/closure", "Card reopened")
	case "not-now":
		return simpleCardAction(ctx, helpForCard(), args, "not-now", "POST", "/not_now", "Card moved to Not Now")
	case "triage":
		if len(args) < 2 {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "card number is required"})
		}
		fs := flag.NewFlagSet("card triage", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		columnID := fs.String("column-id", "", "Column ID")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForCard(), err)
		}
		if strings.TrimSpace(*columnID) == "" {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "--column-id is required"})
		}
		payload := map[string]any{"column_id": strings.TrimSpace(*columnID)}
		resp, err := ctx.Client.Do(requestContext(), "POST", withAccount(ctx, "/cards/"+args[1]+"/triage"), nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForCard(), err)
		}
		return outputNoContent(ctx, resp, "Card moved into column")
	case "untriage":
		return simpleCardAction(ctx, helpForCard(), args, "untriage", "DELETE", "/triage", "Card moved back to triage")
	case "tag":
		if len(args) < 2 {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "card number is required"})
		}
		fs := flag.NewFlagSet("card tag", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		title := fs.String("title", "", "Tag title")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForCard(), err)
		}
		if strings.TrimSpace(*title) == "" {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "--title is required"})
		}
		payload := map[string]any{"tag_title": strings.TrimPrefix(strings.TrimSpace(*title), "#")}
		resp, err := ctx.Client.Do(requestContext(), "POST", withAccount(ctx, "/cards/"+args[1]+"/taggings"), nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForCard(), err)
		}
		return outputNoContent(ctx, resp, "Tag toggled")
	case "assign":
		if len(args) < 2 {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "card number is required"})
		}
		fs := flag.NewFlagSet("card assign", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		assignee := fs.String("assignee-id", "", "Assignee ID")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForCard(), err)
		}
		if strings.TrimSpace(*assignee) == "" {
			return ctx.handleErr(helpForCard(), UsageError{Msg: "--assignee-id is required"})
		}
		payload := map[string]any{"assignee_id": strings.TrimSpace(*assignee)}
		resp, err := ctx.Client.Do(requestContext(), "POST", withAccount(ctx, "/cards/"+args[1]+"/assignments"), nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForCard(), err)
		}
		return outputNoContent(ctx, resp, "Assignment toggled")
	case "watch":
		return simpleCardAction(ctx, helpForCard(), args, "watch", "POST", "/watch", "Subscribed to card")
	case "unwatch":
		return simpleCardAction(ctx, helpForCard(), args, "unwatch", "DELETE", "/watch", "Unsubscribed from card")
	case "pin":
		return simpleCardAction(ctx, helpForCard(), args, "pin", "POST", "/pin", "Card pinned")
	case "unpin":
		return simpleCardAction(ctx, helpForCard(), args, "unpin", "DELETE", "/pin", "Card unpinned")
	default:
		fmt.Fprint(ctx.Stderr, helpForCard())
		return 2
	}
}

func runComment(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForComment())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForComment(), err)
	}
	if err := ensureAccount(ctx); err != nil {
		return ctx.handleErr(helpForComment(), err)
	}
	switch args[0] {
	case "list":
		if len(args) < 2 {
			return ctx.handleErr(helpForComment(), UsageError{Msg: "card number is required"})
		}
		path := withAccount(ctx, "/cards/"+args[1]+"/comments")
		resp, err := ctx.Client.Do(requestContext(), "GET", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForComment(), err)
		}
		return outputListOrJSON(ctx, resp, commentListHeaders, commentListRows)
	case "get":
		if len(args) < 3 {
			return ctx.handleErr(helpForComment(), UsageError{Msg: "card number and comment id are required"})
		}
		path := withAccount(ctx, "/cards/"+args[1]+"/comments/"+args[2])
		resp, err := ctx.Client.Do(requestContext(), "GET", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForComment(), err)
		}
		return outputJSONOrPretty(ctx, resp.Body, formatComment)
	case "create":
		if len(args) < 2 {
			return ctx.handleErr(helpForComment(), UsageError{Msg: "card number is required"})
		}
		fs := flag.NewFlagSet("comment create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		body := fs.String("body", "", "Comment body")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForComment(), err)
		}
		if strings.TrimSpace(*body) == "" {
			return ctx.handleErr(helpForComment(), UsageError{Msg: "--body is required"})
		}
		payload := map[string]any{"comment": map[string]any{"body": *body}}
		path := withAccount(ctx, "/cards/"+args[1]+"/comments")
		resp, err := ctx.Client.Do(requestContext(), "POST", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForComment(), err)
		}
		return outputLocation(ctx, resp, "Comment created")
	case "update":
		if len(args) < 3 {
			return ctx.handleErr(helpForComment(), UsageError{Msg: "card number and comment id are required"})
		}
		fs := flag.NewFlagSet("comment update", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		body := fs.String("body", "", "Comment body")
		if err := fs.Parse(args[3:]); err != nil {
			return ctx.usageError(helpForComment(), err)
		}
		if strings.TrimSpace(*body) == "" {
			return ctx.handleErr(helpForComment(), UsageError{Msg: "--body is required"})
		}
		payload := map[string]any{"comment": map[string]any{"body": *body}}
		path := withAccount(ctx, "/cards/"+args[1]+"/comments/"+args[2])
		resp, err := ctx.Client.Do(requestContext(), "PUT", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForComment(), err)
		}
		if ctx.Output.JSON {
			return ctx.printJSONResponse(resp)
		}
		fmt.Fprintln(ctx.Stdout, "Comment updated.")
		return 0
	case "delete":
		if len(args) < 3 {
			return ctx.handleErr(helpForComment(), UsageError{Msg: "card number and comment id are required"})
		}
		path := withAccount(ctx, "/cards/"+args[1]+"/comments/"+args[2])
		resp, err := ctx.Client.Do(requestContext(), "DELETE", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForComment(), err)
		}
		return outputNoContent(ctx, resp, "Comment deleted")
	default:
		fmt.Fprint(ctx.Stderr, helpForComment())
		return 2
	}
}

func runTag(ctx Context, args []string) int {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprint(ctx.Stderr, helpForTag())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForTag(), err)
	}
	if err := ensureAccount(ctx); err != nil {
		return ctx.handleErr(helpForTag(), err)
	}
	resp, err := ctx.Client.Do(requestContext(), "GET", withAccount(ctx, "/tags"), nil, nil, "", nil)
	if err != nil {
		return ctx.handleErr(helpForTag(), err)
	}
	return outputListOrJSON(ctx, resp, tagListHeaders, tagListRows)
}

func runColumn(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForColumn())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForColumn(), err)
	}
	if err := ensureAccount(ctx); err != nil {
		return ctx.handleErr(helpForColumn(), err)
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("column list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		boardID := fs.String("board-id", "", "Board ID")
		if err := fs.Parse(args[1:]); err != nil {
			return ctx.usageError(helpForColumn(), err)
		}
		if strings.TrimSpace(*boardID) == "" {
			return ctx.handleErr(helpForColumn(), UsageError{Msg: "--board-id is required"})
		}
		path := withAccount(ctx, "/boards/"+strings.TrimSpace(*boardID)+"/columns")
		resp, err := ctx.Client.Do(requestContext(), "GET", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForColumn(), err)
		}
		return outputListOrJSON(ctx, resp, columnListHeaders, columnListRows)
	case "get":
		if len(args) < 2 {
			return ctx.handleErr(helpForColumn(), UsageError{Msg: "column id is required"})
		}
		fs := flag.NewFlagSet("column get", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		boardID := fs.String("board-id", "", "Board ID")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForColumn(), err)
		}
		if strings.TrimSpace(*boardID) == "" {
			return ctx.handleErr(helpForColumn(), UsageError{Msg: "--board-id is required"})
		}
		path := withAccount(ctx, "/boards/"+strings.TrimSpace(*boardID)+"/columns/"+args[1])
		resp, err := ctx.Client.Do(requestContext(), "GET", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForColumn(), err)
		}
		return outputJSONOrPretty(ctx, resp.Body, formatColumn)
	case "create":
		fs := flag.NewFlagSet("column create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		boardID := fs.String("board-id", "", "Board ID")
		name := fs.String("name", "", "Column name")
		color := fs.String("color", "", "Column color")
		if err := fs.Parse(args[1:]); err != nil {
			return ctx.usageError(helpForColumn(), err)
		}
		if strings.TrimSpace(*boardID) == "" || strings.TrimSpace(*name) == "" {
			return ctx.handleErr(helpForColumn(), UsageError{Msg: "--board-id and --name are required"})
		}
		column := map[string]any{"name": strings.TrimSpace(*name)}
		if strings.TrimSpace(*color) != "" {
			column["color"] = strings.TrimSpace(*color)
		}
		payload := map[string]any{"column": column}
		path := withAccount(ctx, "/boards/"+strings.TrimSpace(*boardID)+"/columns")
		resp, err := ctx.Client.Do(requestContext(), "POST", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForColumn(), err)
		}
		return outputLocation(ctx, resp, "Column created")
	case "update":
		if len(args) < 2 {
			return ctx.handleErr(helpForColumn(), UsageError{Msg: "column id is required"})
		}
		fs := flag.NewFlagSet("column update", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		boardID := fs.String("board-id", "", "Board ID")
		name := fs.String("name", "", "Column name")
		color := fs.String("color", "", "Column color")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForColumn(), err)
		}
		if strings.TrimSpace(*boardID) == "" {
			return ctx.handleErr(helpForColumn(), UsageError{Msg: "--board-id is required"})
		}
		column := map[string]any{}
		if strings.TrimSpace(*name) != "" {
			column["name"] = strings.TrimSpace(*name)
		}
		if strings.TrimSpace(*color) != "" {
			column["color"] = strings.TrimSpace(*color)
		}
		if len(column) == 0 {
			return ctx.handleErr(helpForColumn(), UsageError{Msg: "no fields to update"})
		}
		payload := map[string]any{"column": column}
		path := withAccount(ctx, "/boards/"+strings.TrimSpace(*boardID)+"/columns/"+args[1])
		resp, err := ctx.Client.Do(requestContext(), "PUT", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForColumn(), err)
		}
		return outputNoContent(ctx, resp, "Column updated")
	case "delete":
		if len(args) < 2 {
			return ctx.handleErr(helpForColumn(), UsageError{Msg: "column id is required"})
		}
		fs := flag.NewFlagSet("column delete", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		boardID := fs.String("board-id", "", "Board ID")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForColumn(), err)
		}
		if strings.TrimSpace(*boardID) == "" {
			return ctx.handleErr(helpForColumn(), UsageError{Msg: "--board-id is required"})
		}
		path := withAccount(ctx, "/boards/"+strings.TrimSpace(*boardID)+"/columns/"+args[1])
		resp, err := ctx.Client.Do(requestContext(), "DELETE", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForColumn(), err)
		}
		return outputNoContent(ctx, resp, "Column deleted")
	default:
		fmt.Fprint(ctx.Stderr, helpForColumn())
		return 2
	}
}

func runUser(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForUser())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForUser(), err)
	}
	if err := ensureAccount(ctx); err != nil {
		return ctx.handleErr(helpForUser(), err)
	}
	switch args[0] {
	case "list":
		resp, err := ctx.Client.Do(requestContext(), "GET", withAccount(ctx, "/users"), nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForUser(), err)
		}
		return outputListOrJSON(ctx, resp, userListHeaders, userListRows)
	case "get":
		if len(args) < 2 {
			return ctx.handleErr(helpForUser(), UsageError{Msg: "user id is required"})
		}
		resp, err := ctx.Client.Do(requestContext(), "GET", withAccount(ctx, "/users/"+args[1]), nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForUser(), err)
		}
		return outputJSONOrPretty(ctx, resp.Body, formatUser)
	case "update":
		if len(args) < 2 {
			return ctx.handleErr(helpForUser(), UsageError{Msg: "user id is required"})
		}
		fs := flag.NewFlagSet("user update", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		name := fs.String("name", "", "User name")
		avatar := fs.String("avatar", "", "Avatar file path")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForUser(), err)
		}
		if strings.TrimSpace(*name) == "" && strings.TrimSpace(*avatar) == "" {
			return ctx.handleErr(helpForUser(), UsageError{Msg: "--name or --avatar is required"})
		}
		path := withAccount(ctx, "/users/"+args[1])
		if strings.TrimSpace(*avatar) != "" {
			body, contentType, err := multipartBody("user", map[string][]string{"name": {strings.TrimSpace(*name)}}, "avatar", *avatar)
			if err != nil {
				return ctx.handleErr(helpForUser(), err)
			}
			resp, err := ctx.Client.Do(requestContext(), "PUT", path, nil, body, contentType, nil)
			if err != nil {
				return ctx.handleErr(helpForUser(), err)
			}
			return outputNoContent(ctx, resp, "User updated")
		}
		payload := map[string]any{"user": map[string]any{"name": strings.TrimSpace(*name)}}
		resp, err := ctx.Client.Do(requestContext(), "PUT", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForUser(), err)
		}
		return outputNoContent(ctx, resp, "User updated")
	case "deactivate":
		if len(args) < 2 {
			return ctx.handleErr(helpForUser(), UsageError{Msg: "user id is required"})
		}
		resp, err := ctx.Client.Do(requestContext(), "DELETE", withAccount(ctx, "/users/"+args[1]), nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForUser(), err)
		}
		return outputNoContent(ctx, resp, "User deactivated")
	default:
		fmt.Fprint(ctx.Stderr, helpForUser())
		return 2
	}
}

func runNotification(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForNotification())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForNotification(), err)
	}
	if err := ensureAccount(ctx); err != nil {
		return ctx.handleErr(helpForNotification(), err)
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("notification list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		unread := fs.Bool("unread", false, "Show only unread")
		if err := fs.Parse(args[1:]); err != nil {
			return ctx.usageError(helpForNotification(), err)
		}
		resp, err := ctx.Client.Do(requestContext(), "GET", withAccount(ctx, "/notifications"), nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForNotification(), err)
		}
		if *unread {
			// The server has no unread filter, so drop read items client-side.
			filtered, err := filterUnreadNotifications(resp.Body)
			if err != nil {
				return ctx.handleErr(helpForNotification(), err)
			}
			resp.Body = filtered
		}
		return outputListOrJSON(ctx, resp, notificationListHeaders, notificationListRows)
	case "read":
		if len(args) < 2 {
			return ctx.handleErr(helpForNotification(), UsageError{Msg: "notification id is required"})
		}
		path := withAccount(ctx, "/notifications/"+args[1]+"/reading")
		resp, err := ctx.Client.Do(requestContext(), "POST", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForNotification(), err)
		}
		return outputNoContent(ctx, resp, "Notification marked read")
	case "unread":
		if len(args) < 2 {
			return ctx.handleErr(helpForNotification(), UsageError{Msg: "notification id is required"})
		}
		path := withAccount(ctx, "/notifications/"+args[1]+"/reading")
		resp, err := ctx.Client.Do(requestContext(), "DELETE", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForNotification(), err)
		}
		return outputNoContent(ctx, resp, "Notification marked unread")
	case "read-all":
		resp, err := ctx.Client.Do(requestContext(), "POST", withAccount(ctx, "/notifications/bulk_reading"), nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForNotification(), err)
		}
		return outputNoContent(ctx, resp, "Notifications marked read")
	default:
		fmt.Fprint(ctx.Stderr, helpForNotification())
		return 2
	}
}

func runStep(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForStep())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForStep(), err)
	}
	if err := ensureAccount(ctx); err != nil {
		return ctx.handleErr(helpForStep(), err)
	}
	switch args[0] {
	case "list":
		if len(args) < 2 {
			return ctx.handleErr(helpForStep(), UsageError{Msg: "card number is required"})
		}
		path := withAccount(ctx, "/cards/"+args[1]+"/steps")
		resp, err := ctx.Client.Do(requestContext(), "GET", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForStep(), err)
		}
		return outputListOrJSON(ctx, resp, stepListHeaders, stepListRows)
	case "add":
		if len(args) < 2 {
			return ctx.handleErr(helpForStep(), UsageError{Msg: "card number is required"})
		}
		fs := flag.NewFlagSet("step add", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		content := fs.String("content", "", "Step content")
		completed := fs.Bool("completed", false, "Mark the step completed")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForStep(), err)
		}
		if strings.TrimSpace(*content) == "" {
			return ctx.handleErr(helpForStep(), UsageError{Msg: "--content is required"})
		}
		stepPayload := map[string]any{"content": strings.TrimSpace(*content)}
		if *completed {
			stepPayload["completed"] = true
		}
		payload := map[string]any{"step": stepPayload}
		path := withAccount(ctx, "/cards/"+args[1]+"/steps")
		resp, err := ctx.Client.Do(requestContext(), "POST", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForStep(), err)
		}
		return outputLocation(ctx, resp, "Step added")
	case "update":
		if len(args) < 3 {
			return ctx.handleErr(helpForStep(), UsageError{Msg: "card number and step id are required"})
		}
		fs := flag.NewFlagSet("step update", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		content := fs.String("content", "", "Step content")
		completed := fs.Bool("completed", false, "Mark the step completed")
		notCompleted := fs.Bool("not-completed", false, "Mark the step not completed")
		if err := fs.Parse(args[3:]); err != nil {
			return ctx.usageError(helpForStep(), err)
		}
		if *completed && *notCompleted {
			return ctx.handleErr(helpForStep(), UsageError{Msg: "--completed and --not-completed cannot be used together"})
		}
		stepPayload := map[string]any{}
		if strings.TrimSpace(*content) != "" {
			stepPayload["content"] = strings.TrimSpace(*content)
		}
		if *completed {
			stepPayload["completed"] = true
		}
		if *notCompleted {
			stepPayload["completed"] = false
		}
		if len(stepPayload) == 0 {
			return ctx.handleErr(helpForStep(), UsageError{Msg: "no fields to update"})
		}
		payload := map[string]any{"step": stepPayload}
		path := withAccount(ctx, "/cards/"+args[1]+"/steps/"+args[2])
		resp, err := ctx.Client.Do(requestContext(), "PUT", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForStep(), err)
		}
		return outputNoContent(ctx, resp, "Step updated")
	case "check":
		return setStepCompleted(ctx, args, true)
	case "uncheck":
		return setStepCompleted(ctx, args, false)
	case "delete":
		if len(args) < 3 {
			return ctx.handleErr(helpForStep(), UsageError{Msg: "card number and step id are required"})
		}
		path := withAccount(ctx, "/cards/"+args[1]+"/steps/"+args[2])
		resp, err := ctx.Client.Do(requestContext(), "DELETE", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForStep(), err)
		}
		return outputNoContent(ctx, resp, "Step deleted")
	default:
		fmt.Fprint(ctx.Stderr, helpForStep())
		return 2
	}
}

func setStepCompleted(ctx Context, args []string, completed bool) int {
	if len(args) < 3 {
		return ctx.handleErr(helpForStep(), UsageError{Msg: "card number and step id are required"})
	}
	payload := map[string]any{"step": map[string]any{"completed": completed}}
	path := withAccount(ctx, "/cards/"+args[1]+"/steps/"+args[2])
	resp, err := ctx.Client.Do(requestContext(), "PUT", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
	if err != nil {
		return ctx.handleErr(helpForStep(), err)
	}
	message := "Step checked"
	if !completed {
		message = "Step unchecked"
	}
	return outputNoContent(ctx, resp, message)
}

func runReaction(ctx Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(ctx.Stderr, helpForReaction())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForReaction(), err)
	}
	if err := ensureAccount(ctx); err != nil {
		return ctx.handleErr(helpForReaction(), err)
	}
	switch args[0] {
	case "list":
		if len(args) < 2 {
			return ctx.handleErr(helpForReaction(), UsageError{Msg: "card number is required"})
		}
		fs := flag.NewFlagSet("reaction list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		commentID := fs.String("comment-id", "", "Comment ID (react to a comment instead of the card)")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForReaction(), err)
		}
		path := withAccount(ctx, reactionsPath(args[1], *commentID))
		resp, err := ctx.Client.Do(requestContext(), "GET", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForReaction(), err)
		}
		return outputListOrJSON(ctx, resp, reactionListHeaders, reactionListRows)
	case "add":
		if len(args) < 2 {
			return ctx.handleErr(helpForReaction(), UsageError{Msg: "card number is required"})
		}
		fs := flag.NewFlagSet("reaction add", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		content := fs.String("content", "", "Reaction content (e.g. an emoji)")
		commentID := fs.String("comment-id", "", "Comment ID (react to a comment instead of the card)")
		if err := fs.Parse(args[2:]); err != nil {
			return ctx.usageError(helpForReaction(), err)
		}
		contentVal := strings.TrimSpace(*content)
		if contentVal == "" {
			return ctx.handleErr(helpForReaction(), UsageError{Msg: "--content is required"})
		}
		if len(contentVal) > 16 {
			return ctx.handleErr(helpForReaction(), UsageError{Msg: "--content must be 16 characters or fewer"})
		}
		payload := map[string]any{"reaction": map[string]any{"content": contentVal}}
		path := withAccount(ctx, reactionsPath(args[1], *commentID))
		resp, err := ctx.Client.Do(requestContext(), "POST", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil)
		if err != nil {
			return ctx.handleErr(helpForReaction(), err)
		}
		return outputCreatedID(ctx, resp, "Reaction added")
	case "remove":
		if len(args) < 3 {
			return ctx.handleErr(helpForReaction(), UsageError{Msg: "card number and reaction id are required"})
		}
		fs := flag.NewFlagSet("reaction remove", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		commentID := fs.String("comment-id", "", "Comment ID (remove a reaction from a comment instead of the card)")
		if err := fs.Parse(args[3:]); err != nil {
			return ctx.usageError(helpForReaction(), err)
		}
		path := withAccount(ctx, reactionsPath(args[1], *commentID)+"/"+args[2])
		resp, err := ctx.Client.Do(requestContext(), "DELETE", path, nil, nil, "", nil)
		if err != nil {
			return ctx.handleErr(helpForReaction(), err)
		}
		return outputNoContent(ctx, resp, "Reaction removed")
	default:
		fmt.Fprint(ctx.Stderr, helpForReaction())
		return 2
	}
}

// reactionsPath builds the reactions collection path for a card, or for one of
// its comments when commentID is non-empty.
func reactionsPath(cardNumber, commentID string) string {
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return "/cards/" + cardNumber + "/reactions"
	}
	return "/cards/" + cardNumber + "/comments/" + commentID + "/reactions"
}

// outputCreatedID prints the id of a just-created resource whose response has
// no Location header (the id lives in the JSON body instead).
func outputCreatedID(ctx Context, resp *api.Response, successMessage string) int {
	if ctx.Output.JSON {
		return ctx.printJSONResponse(resp)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil {
		return ctx.handleErr("", err)
	}
	if created.ID != "" {
		fmt.Fprintf(ctx.Stdout, "%s: %s\n", successMessage, created.ID)
		return 0
	}
	fmt.Fprintln(ctx.Stdout, successMessage+".")
	return 0
}

func runPin(ctx Context, args []string) int {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprint(ctx.Stderr, helpForPin())
		return 2
	}
	if err := ensureToken(ctx); err != nil {
		return ctx.handleErr(helpForPin(), err)
	}
	if err := ensureAccount(ctx); err != nil {
		return ctx.handleErr(helpForPin(), err)
	}
	resp, err := ctx.Client.Do(requestContext(), "GET", withAccount(ctx, "/my/pins"), nil, nil, "", nil)
	if err != nil {
		return ctx.handleErr(helpForPin(), err)
	}
	// Pins are full card objects, so reuse the card list rendering.
	return outputListOrJSON(ctx, resp, cardListHeaders, cardListRows)
}

func listWithPagination(ctx Context, help string, path string, query url.Values, all bool, headers []string, rowFn func([]byte) ([][]string, error)) int {
	if !all {
		resp, err := ctx.Client.Do(requestContext(), "GET", path, query, nil, "", nil)
		if err != nil {
			return ctx.handleErr(help, err)
		}
		return outputListOrJSON(ctx, resp, headers, rowFn)
	}

	if ctx.Output.JSON {
		combined := []json.RawMessage{}
		nextPath := path
		nextQuery := query
		for {
			resp, err := ctx.Client.Do(requestContext(), "GET", nextPath, nextQuery, nil, "", nil)
			if err != nil {
				return ctx.handleErr(help, err)
			}
			var page []json.RawMessage
			if err := json.Unmarshal(resp.Body, &page); err != nil {
				return ctx.handleErr(help, err)
			}
			combined = append(combined, page...)
			next := nextLink(resp.Headers)
			if next == "" {
				break
			}
			nextPath = next
			nextQuery = nil
		}
		if err := printJSON(ctx.Stdout, combined); err != nil {
			return ctx.handleErr(help, err)
		}
		return 0
	}

	nextPath := path
	nextQuery := query
	printedHeader := false
	for {
		resp, err := ctx.Client.Do(requestContext(), "GET", nextPath, nextQuery, nil, "", nil)
		if err != nil {
			return ctx.handleErr(help, err)
		}
		rows, err := rowFn(resp.Body)
		if err != nil {
			return ctx.handleErr(help, err)
		}
		if !printedHeader {
			printTable(ctx.Stdout, headers, rows, ctx.Output.Plain)
			printedHeader = true
		} else {
			printTable(ctx.Stdout, nil, rows, true)
		}
		next := nextLink(resp.Headers)
		if next == "" {
			break
		}
		nextPath = next
		nextQuery = nil
	}
	return 0
}

// filterUnreadNotifications returns a JSON array containing only the items whose
// "read" field is false, preserving each item's original fields.
func filterUnreadNotifications(body []byte) ([]byte, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	filtered := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		var meta struct {
			Read bool `json:"read"`
		}
		if err := json.Unmarshal(item, &meta); err != nil {
			return nil, err
		}
		if !meta.Read {
			filtered = append(filtered, item)
		}
	}
	return json.Marshal(filtered)
}

// cardNumberFromLocation extracts the card number from a create response
// Location header of the form "/{account}/cards/{number}.json".
func cardNumberFromLocation(location string) string {
	location = strings.TrimSpace(location)
	if location == "" {
		return ""
	}
	location = strings.TrimSuffix(location, ".json")
	idx := strings.LastIndex(location, "/cards/")
	if idx < 0 {
		return ""
	}
	number := location[idx+len("/cards/"):]
	if slash := strings.IndexByte(number, '/'); slash >= 0 {
		number = number[:slash]
	}
	return number
}

// attachTags toggles each tag onto a card via the taggings endpoint. Titles are
// trimmed and any leading '#' is stripped; empty titles are skipped. The server
// creates the tag if it does not yet exist.
func attachTags(ctx Context, cardNumber string, titles []string) error {
	for _, raw := range titles {
		title := strings.TrimPrefix(strings.TrimSpace(raw), "#")
		if title == "" {
			continue
		}
		payload := map[string]any{"tag_title": title}
		path := withAccount(ctx, "/cards/"+cardNumber+"/taggings")
		if _, err := ctx.Client.Do(requestContext(), "POST", path, nil, bytes.NewBuffer(mustJSON(payload)), "application/json", nil); err != nil {
			return err
		}
	}
	return nil
}

func simpleCardAction(ctx Context, help string, args []string, name string, method string, suffix string, message string) int {
	if len(args) < 2 {
		return ctx.handleErr(help, UsageError{Msg: "card number is required"})
	}
	path := withAccount(ctx, "/cards/"+args[1]+suffix)
	resp, err := ctx.Client.Do(requestContext(), method, path, nil, nil, "", nil)
	if err != nil {
		return ctx.handleErr(help, err)
	}
	return outputNoContent(ctx, resp, message)
}

func configSave(path string, cfg config.Config) error {
	return config.Save(path, cfg)
}

func (ctx Context) usageError(help string, err error) int {
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(ctx.Stdout, help)
		return 0
	}
	return ctx.handleErr(help, UsageError{Msg: err.Error()})
}

func (ctx Context) handleErr(help string, err error) int {
	ctx.printErr(err)
	var usage UsageError
	if errors.As(err, &usage) {
		fmt.Fprint(ctx.Stderr, "\n")
		fmt.Fprint(ctx.Stderr, help)
	}
	return exitCode(err)
}

func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

func outputLocation(ctx Context, resp *api.Response, successMessage string) int {
	location := resp.Headers.Get("Location")
	if ctx.Output.JSON {
		payload := map[string]any{"status": resp.Status, "location": location}
		if err := printJSON(ctx.Stdout, payload); err != nil {
			return ctx.handleErr("", err)
		}
		return 0
	}
	if location != "" {
		fmt.Fprintf(ctx.Stdout, "%s: %s\n", successMessage, location)
		return 0
	}
	fmt.Fprintln(ctx.Stdout, successMessage+".")
	return 0
}

func outputNoContent(ctx Context, resp *api.Response, successMessage string) int {
	if ctx.Output.JSON {
		payload := map[string]any{"status": resp.Status}
		if err := printJSON(ctx.Stdout, payload); err != nil {
			return ctx.handleErr("", err)
		}
		return 0
	}
	fmt.Fprintln(ctx.Stdout, successMessage+".")
	return 0
}

func outputListOrJSON(ctx Context, resp *api.Response, headers []string, rowFn func([]byte) ([][]string, error)) int {
	if ctx.Output.JSON {
		return ctx.printJSONResponse(resp)
	}
	return outputListRows(ctx, resp.Body, headers, rowFn)
}

func outputListRows(ctx Context, body []byte, headers []string, rowFn func([]byte) ([][]string, error)) int {
	rows, err := rowFn(body)
	if err != nil {
		return ctx.handleErr("", err)
	}
	printTable(ctx.Stdout, headers, rows, ctx.Output.Plain)
	return 0
}

func outputJSONOrPretty(ctx Context, body []byte, formatFn func([]byte) (string, error)) int {
	if ctx.Output.JSON {
		if err := printJSONBytes(ctx.Stdout, body); err != nil {
			return ctx.handleErr("", err)
		}
		return 0
	}
	text, err := formatFn(body)
	if err != nil {
		return ctx.handleErr("", err)
	}
	fmt.Fprintln(ctx.Stdout, text)
	return 0
}

func (ctx Context) printJSONResponse(resp *api.Response) int {
	if len(resp.Body) == 0 {
		payload := map[string]any{"status": resp.Status}
		if err := printJSON(ctx.Stdout, payload); err != nil {
			return ctx.handleErr("", err)
		}
		return 0
	}
	if err := printJSONBytes(ctx.Stdout, resp.Body); err != nil {
		return ctx.handleErr("", err)
	}
	return 0
}

func addListParam(values url.Values, key string, list []string) {
	for _, v := range list {
		if strings.TrimSpace(v) != "" {
			values.Add(key, strings.TrimSpace(v))
		}
	}
}

func setStringParam(values url.Values, key, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		values.Set(key, value)
	}
}

func nextLink(headers map[string][]string) string {
	linkHeader := ""
	for k, v := range headers {
		if strings.EqualFold(k, "Link") && len(v) > 0 {
			linkHeader = v[0]
			break
		}
	}
	if linkHeader == "" {
		return ""
	}
	parts := strings.Split(linkHeader, ",")
	for _, part := range parts {
		sections := strings.Split(strings.TrimSpace(part), ";")
		if len(sections) < 2 {
			continue
		}
		urlPart := strings.TrimSpace(sections[0])
		relPart := strings.TrimSpace(sections[1])
		if strings.Contains(relPart, "rel=\"next\"") {
			return strings.Trim(urlPart, "<>")
		}
	}
	return ""
}

func readSecret(ctx Context, label string) (string, error) {
	if isTTY(os.Stdin) {
		fmt.Fprintf(ctx.Stderr, "%s: ", label)
		reader := bufio.NewReader(os.Stdin)
		text, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimSpace(text), nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func isTTY(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func multipartBody(rootKey string, fields map[string][]string, fileField, filePath string) (io.Reader, string, error) {
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	for key, values := range fields {
		fieldName := buildFieldName(rootKey, key)
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				continue
			}
			if err := writer.WriteField(fieldName, value); err != nil {
				return nil, "", err
			}
		}
	}
	if fileField != "" && strings.TrimSpace(filePath) != "" {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, "", err
		}
		defer file.Close()

		part, err := writer.CreateFormFile(fmt.Sprintf("%s[%s]", rootKey, fileField), filepath.Base(filePath))
		if err != nil {
			return nil, "", err
		}
		if _, err := io.Copy(part, file); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buf, writer.FormDataContentType(), nil
}

func buildFieldName(rootKey, key string) string {
	suffix := ""
	if strings.HasSuffix(key, "[]") {
		key = strings.TrimSuffix(key, "[]")
		suffix = "[]"
	}
	if rootKey == "" {
		return key + suffix
	}
	return fmt.Sprintf("%s[%s]%s", rootKey, key, suffix)
}
