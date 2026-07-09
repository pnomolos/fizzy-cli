package cli

import (
	"encoding/json"
	"fmt"
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
