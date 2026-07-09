package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
)

var (
	boardListHeaders        = []string{"ID", "NAME", "ALL_ACCESS", "CREATED"}
	cardListHeaders         = []string{"#", "TITLE", "STATUS", "BOARD", "LAST_ACTIVE"}
	commentListHeaders      = []string{"ID", "CREATOR", "BODY", "CREATED"}
	tagListHeaders          = []string{"ID", "TITLE"}
	columnListHeaders       = []string{"ID", "NAME", "COLOR"}
	userListHeaders         = []string{"ID", "NAME", "ROLE", "EMAIL"}
	notificationListHeaders = []string{"ID", "READ", "TITLE", "CARD", "CREATED"}
	accessTokenListHeaders  = []string{"ID", "DESCRIPTION", "PERMISSION", "CREATED"}
	stepListHeaders         = []string{"ID", "DONE", "CONTENT"}
	reactionListHeaders     = []string{"ID", "CONTENT", "REACTER"}
	activityListHeaders     = []string{"TIME", "ACTION", "DESCRIPTION", "CARD", "BOARD", "CREATOR"}
	boardAccessListHeaders  = []string{"USER", "ROLE", "HAS_ACCESS", "INVOLVEMENT"}
	webhookListHeaders      = []string{"ID", "NAME", "ACTIVE", "URL"}
)

type board struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AllAccess bool   `json:"all_access"`
	CreatedAt string `json:"created_at"`
	Creator   user   `json:"creator"`
	URL       string `json:"url"`
}

type card struct {
	ID           string   `json:"id"`
	Number       int      `json:"number"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags"`
	Golden       bool     `json:"golden"`
	LastActiveAt string   `json:"last_active_at"`
	CreatedAt    string   `json:"created_at"`
	Board        board    `json:"board"`
	Creator      user     `json:"creator"`
	Steps        []step   `json:"steps"`
}

type comment struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Body      struct {
		Plain string `json:"plain_text"`
	} `json:"body"`
	Creator user `json:"creator"`
}

type tag struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type column struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Color     columnColor `json:"color"`
	CreatedAt string      `json:"created_at"`
}

// columnColor tolerates the several shapes the API uses for a column color:
// null, a bare string, or an object {"name":...,"value":...}.
type columnColor struct {
	Name  string
	Value string
}

func (c *columnColor) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		c.Name = ""
		c.Value = ""
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		c.Name = s
		c.Value = ""
		return nil
	}
	var obj struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	c.Name = obj.Name
	c.Value = obj.Value
	return nil
}

type user struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Email string `json:"email_address"`
}

type notification struct {
	ID        string `json:"id"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"created_at"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Card      struct {
		Title string `json:"title"`
	} `json:"card"`
}

type accessToken struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Permission  string `json:"permission"`
	CreatedAt   string `json:"created_at"`
}

// accessTokenCreated is the create-response shape: the access token fields
// plus the one-time-shown token value.
type accessTokenCreated struct {
	accessToken
	Token string `json:"token"`
}

type reaction struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Reacter user   `json:"reacter"`
	URL     string `json:"url"`
}

type accountSettings struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	CardsCount               int    `json:"cards_count"`
	CreatedAt                string `json:"created_at"`
	AutoPostponePeriodInDays int    `json:"auto_postpone_period_in_days"`
}

type joinCode struct {
	Code       string `json:"code"`
	UsageCount int    `json:"usage_count"`
	UsageLimit int    `json:"usage_limit"`
	URL        string `json:"url"`
	Active     bool   `json:"active"`
}

// boardAccessUser is a user entry as returned by GET .../boards/:id/accesses,
// which augments the plain user fields with board-specific access info.
type boardAccessUser struct {
	user
	HasAccess   bool   `json:"has_access"`
	Involvement string `json:"involvement"`
}

// boardAccesses is the object shape returned by GET .../boards/:id/accesses
// (not a bare array).
type boardAccesses struct {
	BoardID   string            `json:"board_id"`
	AllAccess bool              `json:"all_access"`
	Users     []boardAccessUser `json:"users"`
}

type identity struct {
	Accounts []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
		User user   `json:"user"`
	} `json:"accounts"`
}

type step struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Completed bool   `json:"completed"`
}

type webhook struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Active            bool     `json:"active"`
	SigningSecret     string   `json:"signing_secret"`
	SubscribedActions []string `json:"subscribed_actions"`
	PayloadURL        string   `json:"payload_url"`
	CreatedAt         string   `json:"created_at"`
	URL               string   `json:"url"`
}

type exportJob struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	DownloadURL string `json:"download_url"`
}

type notificationSettings struct {
	BundleEmailFrequency string `json:"bundle_email_frequency"`
}

type activity struct {
	ID            string          `json:"id"`
	Action        string          `json:"action"`
	CreatedAt     string          `json:"created_at"`
	Description   string          `json:"description"`
	EventableType string          `json:"eventable_type"`
	Eventable     json.RawMessage `json:"eventable"`
	Board         board           `json:"board"`
	Creator       user            `json:"creator"`
}

// stripHTML removes HTML tags and unescapes entities, for rendering
// server-provided description strings that may contain markup in a plain
// terminal table.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(html.UnescapeString(b.String()))
}

func identityToRows(body []byte) ([][]string, error) {
	var id identity
	if err := json.Unmarshal(body, &id); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(id.Accounts))
	for _, acct := range id.Accounts {
		rows = append(rows, []string{strings.TrimPrefix(acct.Slug, "/"), acct.Name, acct.User.Name})
	}
	return rows, nil
}

func boardListRows(body []byte) ([][]string, error) {
	var boards []board
	if err := json.Unmarshal(body, &boards); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(boards))
	for _, b := range boards {
		rows = append(rows, []string{b.ID, b.Name, fmt.Sprintf("%t", b.AllAccess), b.CreatedAt})
	}
	return rows, nil
}

func cardListRows(body []byte) ([][]string, error) {
	var cards []card
	if err := json.Unmarshal(body, &cards); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(cards))
	for _, c := range cards {
		rows = append(rows, []string{fmt.Sprintf("%d", c.Number), c.Title, c.Status, c.Board.Name, c.LastActiveAt})
	}
	return rows, nil
}

func commentListRows(body []byte) ([][]string, error) {
	var comments []comment
	if err := json.Unmarshal(body, &comments); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(comments))
	for _, c := range comments {
		rows = append(rows, []string{c.ID, c.Creator.Name, c.Body.Plain, c.CreatedAt})
	}
	return rows, nil
}

func tagListRows(body []byte) ([][]string, error) {
	var tags []tag
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(tags))
	for _, t := range tags {
		rows = append(rows, []string{t.ID, t.Title})
	}
	return rows, nil
}

func columnListRows(body []byte) ([][]string, error) {
	var cols []column
	if err := json.Unmarshal(body, &cols); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(cols))
	for _, c := range cols {
		rows = append(rows, []string{c.ID, c.Name, c.Color.Name})
	}
	return rows, nil
}

func userListRows(body []byte) ([][]string, error) {
	var users []user
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(users))
	for _, u := range users {
		rows = append(rows, []string{u.ID, u.Name, u.Role, u.Email})
	}
	return rows, nil
}

func notificationListRows(body []byte) ([][]string, error) {
	var notes []notification
	if err := json.Unmarshal(body, &notes); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(notes))
	for _, n := range notes {
		read := "no"
		if n.Read {
			read = "yes"
		}
		rows = append(rows, []string{n.ID, read, n.Title, n.Card.Title, n.CreatedAt})
	}
	return rows, nil
}

func stepListRows(body []byte) ([][]string, error) {
	var steps []step
	if err := json.Unmarshal(body, &steps); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(steps))
	for _, s := range steps {
		done := " "
		if s.Completed {
			done = "✓"
		}
		rows = append(rows, []string{s.ID, done, s.Content})
	}
	return rows, nil
}

func reactionListRows(body []byte) ([][]string, error) {
	var reactions []reaction
	if err := json.Unmarshal(body, &reactions); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(reactions))
	for _, r := range reactions {
		rows = append(rows, []string{r.ID, r.Content, r.Reacter.Name})
	}
	return rows, nil
}

func activityListRows(body []byte) ([][]string, error) {
	var items []activity
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(items))
	for _, a := range items {
		cardNumber := ""
		if a.EventableType == "Card" && len(a.Eventable) > 0 {
			var ev struct {
				Number int `json:"number"`
			}
			if err := json.Unmarshal(a.Eventable, &ev); err == nil && ev.Number != 0 {
				cardNumber = fmt.Sprintf("%d", ev.Number)
			}
		}
		rows = append(rows, []string{a.CreatedAt, a.Action, stripHTML(a.Description), cardNumber, a.Board.Name, a.Creator.Name})
	}
	return rows, nil
}

func boardAccessListRows(body []byte) ([][]string, error) {
	var ba boardAccesses
	if err := json.Unmarshal(body, &ba); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(ba.Users))
	for _, u := range ba.Users {
		rows = append(rows, []string{u.Name, u.Role, fmt.Sprintf("%t", u.HasAccess), u.Involvement})
	}
	return rows, nil
}

func accessTokenListRows(body []byte) ([][]string, error) {
	var tokens []accessToken
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(tokens))
	for _, t := range tokens {
		rows = append(rows, []string{t.ID, t.Description, t.Permission, t.CreatedAt})
	}
	return rows, nil
}

func formatBoard(body []byte) (string, error) {
	var b board
	if err := json.Unmarshal(body, &b); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"ID: %s\nName: %s\nAll access: %t\nCreated: %s\nCreator: %s\nURL: %s",
		b.ID, b.Name, b.AllAccess, b.CreatedAt, b.Creator.Name, b.URL,
	), nil
}

func formatCard(body []byte) (string, error) {
	var c card
	if err := json.Unmarshal(body, &c); err != nil {
		return "", err
	}
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "ID: %s\n", c.ID)
	fmt.Fprintf(builder, "Number: %d\n", c.Number)
	fmt.Fprintf(builder, "Title: %s\n", c.Title)
	fmt.Fprintf(builder, "Status: %s\n", c.Status)
	fmt.Fprintf(builder, "Board: %s\n", c.Board.Name)
	fmt.Fprintf(builder, "Creator: %s\n", c.Creator.Name)
	if len(c.Tags) > 0 {
		fmt.Fprintf(builder, "Tags: %s\n", strings.Join(c.Tags, ", "))
	}
	if c.Golden {
		fmt.Fprintf(builder, "Golden: true\n")
	}
	if c.LastActiveAt != "" {
		fmt.Fprintf(builder, "Last active: %s\n", c.LastActiveAt)
	}
	if c.CreatedAt != "" {
		fmt.Fprintf(builder, "Created: %s\n", c.CreatedAt)
	}
	if c.Description != "" {
		fmt.Fprintf(builder, "\nDescription:\n%s\n", c.Description)
	}
	if len(c.Steps) > 0 {
		fmt.Fprintf(builder, "\nSteps:\n")
		for _, s := range c.Steps {
			status := " "
			if s.Completed {
				status = "x"
			}
			fmt.Fprintf(builder, "- [%s] %s\n", status, s.Content)
		}
	}
	return strings.TrimSpace(builder.String()), nil
}

func formatComment(body []byte) (string, error) {
	var c comment
	if err := json.Unmarshal(body, &c); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"ID: %s\nCreator: %s\nCreated: %s\nBody: %s",
		c.ID, c.Creator.Name, c.CreatedAt, c.Body.Plain,
	), nil
}

func formatColumn(body []byte) (string, error) {
	var c column
	if err := json.Unmarshal(body, &c); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"ID: %s\nName: %s\nColor: %s\nCreated: %s",
		c.ID, c.Name, c.Color.Name, c.CreatedAt,
	), nil
}

func formatAccountSettings(body []byte) (string, error) {
	var a accountSettings
	if err := json.Unmarshal(body, &a); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"ID: %s\nName: %s\nCards: %d\nCreated: %s\nAuto-postpone (days): %d",
		a.ID, a.Name, a.CardsCount, a.CreatedAt, a.AutoPostponePeriodInDays,
	), nil
}

func formatJoinCode(body []byte) (string, error) {
	var j joinCode
	if err := json.Unmarshal(body, &j); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"Code: %s\nURL: %s\nActive: %t\nUsage: %d/%d",
		j.Code, j.URL, j.Active, j.UsageCount, j.UsageLimit,
	), nil
}

func formatUser(body []byte) (string, error) {
	var u user
	if err := json.Unmarshal(body, &u); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"ID: %s\nName: %s\nRole: %s\nEmail: %s",
		u.ID, u.Name, u.Role, u.Email,
	), nil
}

func webhookListRows(body []byte) ([][]string, error) {
	var hooks []webhook
	if err := json.Unmarshal(body, &hooks); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(hooks))
	for _, h := range hooks {
		rows = append(rows, []string{h.ID, h.Name, fmt.Sprintf("%t", h.Active), h.PayloadURL})
	}
	return rows, nil
}

func formatWebhook(body []byte) (string, error) {
	var h webhook
	if err := json.Unmarshal(body, &h); err != nil {
		return "", err
	}
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "ID: %s\n", h.ID)
	fmt.Fprintf(builder, "Name: %s\n", h.Name)
	fmt.Fprintf(builder, "Active: %t\n", h.Active)
	fmt.Fprintf(builder, "URL: %s\n", h.PayloadURL)
	fmt.Fprintf(builder, "Signing secret: %s\n", h.SigningSecret)
	fmt.Fprintf(builder, "Subscribed actions: %s\n", strings.Join(h.SubscribedActions, ", "))
	if h.CreatedAt != "" {
		fmt.Fprintf(builder, "Created: %s\n", h.CreatedAt)
	}
	return strings.TrimSpace(builder.String()), nil
}

func formatExport(body []byte) (string, error) {
	var e exportJob
	if err := json.Unmarshal(body, &e); err != nil {
		return "", err
	}
	builder := &strings.Builder{}
	fmt.Fprintf(builder, "ID: %s\n", e.ID)
	fmt.Fprintf(builder, "Status: %s\n", e.Status)
	if e.CreatedAt != "" {
		fmt.Fprintf(builder, "Created: %s\n", e.CreatedAt)
	}
	if e.DownloadURL != "" {
		fmt.Fprintf(builder, "Download URL: %s\n", e.DownloadURL)
	}
	return strings.TrimSpace(builder.String()), nil
}

func formatNotificationSettings(body []byte) (string, error) {
	var s notificationSettings
	if err := json.Unmarshal(body, &s); err != nil {
		return "", err
	}
	return fmt.Sprintf("Bundle email frequency: %s", s.BundleEmailFrequency), nil
}

// webhookDeliveryRows renders webhook delivery log entries generically. The
// delivery shape is not part of the documented API and is empty until real
// deliveries occur, so columns are discovered dynamically from the JSON keys
// (with "id" first, then remaining keys sorted). Nested values are rendered as
// compact JSON. It returns the derived headers alongside the rows.
func webhookDeliveryRows(body []byte) ([]string, [][]string, error) {
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, nil, err
	}
	keySet := map[string]bool{}
	for _, item := range items {
		for k := range item {
			keySet[k] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		if k != "id" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if keySet["id"] {
		keys = append([]string{"id"}, keys...)
	}
	headers := make([]string, len(keys))
	for i, k := range keys {
		headers[i] = strings.ToUpper(k)
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		row := make([]string, len(keys))
		for i, k := range keys {
			raw, ok := item[k]
			if !ok {
				row[i] = ""
				continue
			}
			row[i] = renderRawValue(raw)
		}
		rows = append(rows, row)
	}
	return headers, rows, nil
}

// renderRawValue converts a raw JSON value into a compact table cell: strings
// are unquoted, scalars printed verbatim, and objects/arrays kept as compact
// JSON.
func renderRawValue(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	return trimmed
}
