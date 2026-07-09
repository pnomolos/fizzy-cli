package cli

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"fizzy-cli/internal/config"
)

func TestBoardListTable(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/boards", stub{Status: 200, Body: boardsFixture})

	res := h.run("board", "list")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "ID  NAME     ALL_ACCESS  CREATED\nb1  Roadmap  true        2024-01-02\nb2  Bugs     false       2024-03-04\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/boards")
}

func TestBoardListJSONBeforeSubcommand(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/boards", stub{Status: 200, Body: boardsFixture})

	res := h.run("--json", "board", "list")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	var got, want any
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("stdout not JSON: %v (%q)", err, res.stdout)
	}
	if err := json.Unmarshal([]byte(boardsFixture), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("json passthrough mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestBoardGet(t *testing.T) {
	h := newHarness(t)
	body := `{"id":"b1","name":"Roadmap","all_access":true,"created_at":"2024-01-02","creator":{"name":"Ada"},"url":"https://app/x"}`
	h.route("GET", "/acme/boards/5", stub{Status: 200, Body: body})

	res := h.run("board", "get", "5")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "ID: b1\nName: Roadmap\nAll access: true\nCreated: 2024-01-02\nCreator: Ada\nURL: https://app/x\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/boards/5")
}

func TestBoardCreate(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/boards", stub{Status: 201, Headers: map[string]string{"Location": "/acme/boards/9"}})

	res := h.run("board", "create", "--name", "Roadmap")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Board created: /acme/boards/9\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "POST", "/acme/boards")
	assertJSONBody(t, rec, map[string]any{"board": map[string]any{"name": "Roadmap", "all_access": true}})
	if ct := rec.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestCardListFilters(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/cards", stub{Status: 200, Body: cardsFixture})

	res := h.run("card", "list", "--board-id", "1", "--board-id", "2", "--term", "foo", "--sorted-by", "recent")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "GET", "/acme/cards")
	q, err := url.ParseQuery(rec.RawQuery)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if ids := q["board_ids[]"]; !reflect.DeepEqual(ids, []string{"1", "2"}) {
		t.Errorf("board_ids[] = %v, want [1 2]", ids)
	}
	if terms := q["terms[]"]; !reflect.DeepEqual(terms, []string{"foo"}) {
		t.Errorf("terms[] = %v", terms)
	}
	if q.Get("sorted_by") != "recent" {
		t.Errorf("sorted_by = %q", q.Get("sorted_by"))
	}
}

func TestCardCreate(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/boards/3/cards", stub{Status: 201, Headers: map[string]string{"Location": "/acme/cards/42"}})

	res := h.run("card", "create", "--board-id", "3", "--title", "New card", "--description", "Some details")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card created: /acme/cards/42\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "POST", "/acme/boards/3/cards")
	assertJSONBody(t, rec, map[string]any{"card": map[string]any{"title": "New card", "description": "Some details"}})
}

func TestCardCreateWithTags(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/boards/3/cards", stub{Status: 201, Headers: map[string]string{"Location": "/acme/cards/42.json"}})
	h.route("POST", "/acme/cards/42/taggings", stub{Status: 204})

	res := h.run("card", "create", "--board-id", "3", "--title", "New card", "--tag", "Design", "--tag", "#urgent")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card created: /acme/cards/42.json\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	if h.requestCount() != 3 {
		t.Fatalf("request count = %d, want 3", h.requestCount())
	}
	create := h.requests[0]
	assertRequest(t, create, "POST", "/acme/boards/3/cards")
	assertJSONBody(t, create, map[string]any{"card": map[string]any{"title": "New card"}})
	tag1 := h.requests[1]
	assertRequest(t, tag1, "POST", "/acme/cards/42/taggings")
	assertJSONBody(t, tag1, map[string]any{"tag_title": "Design"})
	tag2 := h.requests[2]
	assertRequest(t, tag2, "POST", "/acme/cards/42/taggings")
	assertJSONBody(t, tag2, map[string]any{"tag_title": "urgent"})
}

func TestCardUpdateWithTagsOnly(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/taggings", stub{Status: 204})

	res := h.run("card", "update", "7", "--tag", "Design")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card updated.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	if h.requestCount() != 1 {
		t.Fatalf("request count = %d, want 1 (no card PUT when only tags)", h.requestCount())
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "POST", "/acme/cards/7/taggings")
	assertJSONBody(t, rec, map[string]any{"tag_title": "Design"})
}

func TestCardPublish(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/publish", stub{Status: 201})

	res := h.run("card", "publish", "7")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card published.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/cards/7/publish")
}

func TestCardNumberFromLocation(t *testing.T) {
	cases := map[string]string{
		"/1/cards/18.json":    "18",
		"/acme/cards/42.json": "42",
		"/acme/cards/42":      "42",
		"":                    "",
		"/acme/boards/9":      "",
	}
	for in, want := range cases {
		if got := cardNumberFromLocation(in); got != want {
			t.Errorf("cardNumberFromLocation(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCardClose(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/5/closure", stub{Status: 201})

	res := h.run("card", "close", "5")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card closed.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/cards/5/closure")
}

func TestCardReopen(t *testing.T) {
	h := newHarness(t)
	h.route("DELETE", "/acme/cards/5/closure", stub{Status: 204})

	res := h.run("card", "reopen", "5")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card reopened.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/cards/5/closure")
}

func TestCommentCreate(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/comments", stub{Status: 201, Headers: map[string]string{"Location": "/acme/cards/7/comments/1"}})

	res := h.run("comment", "create", "7", "--body", "Hello there")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Comment created: /acme/cards/7/comments/1\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "POST", "/acme/cards/7/comments")
	assertJSONBody(t, rec, map[string]any{"comment": map[string]any{"body": "Hello there"}})
}

const notificationsFixture = `[{"id":"n1","read":false,"title":"Assigned","card":{"title":"Fix login"},"created_at":"2024-07-01"},{"id":"n2","read":true,"title":"Mentioned","card":{"title":"Add search"},"created_at":"2024-07-02"}]`

func TestCardListColumnIDFilter(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/cards", stub{Status: 200, Body: cardsFixture})

	res := h.run("card", "list", "--column-id", "col1", "--column-id", "col2")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "GET", "/acme/cards")
	q, err := url.ParseQuery(rec.RawQuery)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if ids := q["column_ids[]"]; !reflect.DeepEqual(ids, []string{"col1", "col2"}) {
		t.Errorf("column_ids[] = %v, want [col1 col2]", ids)
	}
}

func TestNotificationListUnreadFilters(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/notifications", stub{Status: 200, Body: notificationsFixture})

	res := h.run("notification", "list", "--unread")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "Assigned") {
		t.Errorf("unread item missing from output: %q", res.stdout)
	}
	if strings.Contains(res.stdout, "Mentioned") {
		t.Errorf("read item should be filtered out: %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "GET", "/acme/notifications")
	if rec.RawQuery != "" {
		t.Errorf("expected no query params (no server unread filter), got %q", rec.RawQuery)
	}
}

func TestNotificationListUnreadJSON(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/notifications", stub{Status: 200, Body: notificationsFixture})

	res := h.run("--json", "notification", "list", "--unread")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("stdout not JSON array: %v (%q)", err, res.stdout)
	}
	if len(got) != 1 {
		t.Fatalf("filtered length = %d, want 1", len(got))
	}
	if got[0]["id"] != "n1" {
		t.Errorf("kept id = %v, want n1", got[0]["id"])
	}
}

func TestNotificationRead(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/notifications/3/reading", stub{Status: 201})

	res := h.run("notification", "read", "3")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Notification marked read.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/notifications/3/reading")
}

func TestNotificationReadAll(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/notifications/bulk_reading", stub{Status: 201})

	res := h.run("notification", "read-all")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Notifications marked read.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/notifications/bulk_reading")
}

const identityFixture = `{"accounts":[{"slug":"/acme","name":"Acme Inc","user":{"name":"Ada"}},{"slug":"globex","name":"Globex","user":{"name":"Bob"}}]}`

func TestAccountList(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/my/identity", stub{Status: 200, Body: identityFixture})

	res := h.run("account", "list")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "SLUG    NAME      USER\nacme    Acme Inc  Ada\nglobex  Globex    Bob\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/my/identity")
}

func TestAuthStatus(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/my/identity", stub{Status: 200, Body: identityFixture})

	res := h.run("auth", "status")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "Authenticated using personal access token. Accessible accounts:\n" +
		"SLUG    NAME      USER\nacme    Acme Inc  Ada\nglobex  Globex    Bob\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/my/identity")
}

func TestCardListJSONTrailing(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/cards", stub{Status: 200, Body: cardsFixture})

	res := h.run("card", "list", "--board-id", "3", "--json")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	var got, want any
	if err := json.Unmarshal([]byte(res.stdout), &got); err != nil {
		t.Fatalf("stdout not JSON: %v (%q)", err, res.stdout)
	}
	if err := json.Unmarshal([]byte(cardsFixture), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("json passthrough mismatch\n got: %#v\nwant: %#v", got, want)
	}
	rec := h.lastRequest()
	q, err := url.ParseQuery(rec.RawQuery)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if ids := q["board_ids[]"]; !reflect.DeepEqual(ids, []string{"3"}) {
		t.Errorf("board_ids[] = %v, want [3]", ids)
	}
}

func TestJSONLeadingStillWorks(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/cards", stub{Status: 200, Body: cardsFixture})

	res := h.run("--json", "card", "list")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if !strings.HasPrefix(strings.TrimSpace(res.stdout), "[") {
		t.Errorf("expected JSON output, got %q", res.stdout)
	}
}

func TestBoardListPlainTrailing(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/boards", stub{Status: 200, Body: boardsFixture})

	res := h.run("board", "list", "--plain")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if strings.Contains(res.stdout, "ALL_ACCESS") {
		t.Errorf("--plain should omit the header, got %q", res.stdout)
	}
	if !strings.Contains(res.stdout, "Roadmap") {
		t.Errorf("expected rows, got %q", res.stdout)
	}
}

func TestAccountValueFlagTrailing(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/2/boards", stub{Status: 200, Body: boardsFixture})

	res := h.run("board", "list", "--account", "2")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "GET", "/2/boards")
}

func TestJSONPlainMutualExclusionTrailing(t *testing.T) {
	h := newHarness(t)

	res := h.run("board", "list", "--json", "--plain")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "cannot be used together") {
		t.Errorf("stderr = %q, want mutual-exclusion error", res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP requests, got %d", h.requestCount())
	}
}

func TestConfigSetAccountNotHijacked(t *testing.T) {
	h := newHarness(t)
	// `config set --account` owns its own --account flag; the global scanner
	// must not steal it.
	res := h.run("config", "set", "--account", "globex")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "Config updated") {
		t.Errorf("stdout = %q, want config updated", res.stdout)
	}
}

func TestAuthMissingCredentials(t *testing.T) {
	h := newHarness(t)
	h.token = "" // no token via flag, env cleared, no config file

	res := h.run("board", "list")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stdout=%q stderr=%q", res.code, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stderr, "missing credentials") {
		t.Errorf("stderr = %q, want it to mention missing credentials", res.stderr)
	}
	if res.stdout != "" {
		t.Errorf("stdout = %q, want empty", res.stdout)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP requests, got %d", h.requestCount())
	}
}

func TestBoardUpdateUserIDsTopLevel(t *testing.T) {
	h := newHarness(t)
	h.route("PUT", "/acme/boards/b1", stub{Status: 204})

	res := h.run("board", "update", "b1", "--name", "Renamed", "--user-id", "u1", "--user-id", "u2")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "PUT", "/acme/boards/b1")
	// user_ids must be a top-level sibling of board, not nested inside it.
	assertJSONBody(t, rec, map[string]any{
		"board":    map[string]any{"name": "Renamed"},
		"user_ids": []any{"u1", "u2"},
	})
}

func TestBoardUpdateUserIDsOnly(t *testing.T) {
	h := newHarness(t)
	h.route("PUT", "/acme/boards/b1", stub{Status: 204})

	res := h.run("board", "update", "b1", "--user-id", "u1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	// A membership-only update must not send an empty board object (the server
	// rejects it with 400); restricting to a member list implies all_access:false.
	assertJSONBody(t, h.lastRequest(), map[string]any{
		"board":    map[string]any{"all_access": false},
		"user_ids": []any{"u1"},
	})
}

// The --auto-postpone-days flag was removed: the current Fizzy backend does not
// persist board auto-postpone changes through any API path, so the flag is gone
// rather than silently lying. Passing it should be an unknown-flag usage error.
func TestBoardCreateRejectsAutoPostponeFlag(t *testing.T) {
	h := newHarness(t)
	res := h.run("board", "create", "--name", "X", "--auto-postpone-days", "7")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

const accessTokensFixture = `[{"id":"t1","description":"laptop","permission":"read","created_at":"2024-01-02"},{"id":"t2","description":"CI","permission":"write","created_at":"2024-03-04"}]`

func TestAuthTokenList(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/my/access_tokens", stub{Status: 200, Body: accessTokensFixture})

	res := h.run("auth", "token", "list")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "ID  DESCRIPTION  PERMISSION  CREATED\nt1  laptop       read        2024-01-02\nt2  CI           write       2024-03-04\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	// Unscoped: no account prefix on the path.
	assertRequest(t, h.lastRequest(), "GET", "/my/access_tokens")
}

func TestAuthTokenCreate(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/my/access_tokens", stub{Status: 201, Body: `{"id":"t9","token":"secret-value","description":"laptop","permission":"read","created_at":"2024-01-02"}`})

	res := h.run("auth", "token", "create", "--description", "laptop", "--permission", "read")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "Token: secret-value") {
		t.Errorf("stdout = %q, want it to contain the token", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "POST", "/my/access_tokens")
	assertJSONBody(t, rec, map[string]any{"access_token": map[string]any{"description": "laptop", "permission": "read"}})

	// Without --save, the token must not be persisted to config.
	cfg, err := config.Load(h.configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Token != "" {
		t.Errorf("token should not be saved without --save, got %q", cfg.Token)
	}
}

func TestAuthTokenCreateSave(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/my/access_tokens", stub{Status: 201, Body: `{"id":"t9","token":"secret-value","description":"laptop","permission":"write","created_at":"2024-01-02"}`})

	res := h.run("auth", "token", "create", "--description", "laptop", "--permission", "write", "--save")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Token != "secret-value" {
		t.Errorf("cfg.Token = %q, want secret-value", cfg.Token)
	}
	if cfg.SessionToken != "" {
		t.Errorf("cfg.SessionToken = %q, want cleared", cfg.SessionToken)
	}
}

func TestAuthTokenCreateInvalidPermission(t *testing.T) {
	h := newHarness(t)
	res := h.run("auth", "token", "create", "--description", "laptop", "--permission", "admin")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

const stepsFixture = `[{"id":"s1","content":"Write tests","completed":true},{"id":"s2","content":"Ship it","completed":false}]`

func TestStepList(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/cards/7/steps", stub{Status: 200, Body: stepsFixture})

	res := h.run("step", "list", "7")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "ID  DONE  CONTENT\ns1  ✓     Write tests\ns2        Ship it\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/cards/7/steps")
}

func TestStepAdd(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/steps", stub{Status: 201, Headers: map[string]string{"Location": "/acme/cards/7/steps/s9"}})

	res := h.run("step", "add", "7", "--content", "Do the thing", "--completed")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Step added: /acme/cards/7/steps/s9\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "POST", "/acme/cards/7/steps")
	assertJSONBody(t, rec, map[string]any{"step": map[string]any{"content": "Do the thing", "completed": true}})
}

func TestStepUpdate(t *testing.T) {
	h := newHarness(t)
	h.route("PUT", "/acme/cards/7/steps/s9", stub{Status: 200, Body: `{"id":"s9","content":"Renamed","completed":true}`})

	res := h.run("step", "update", "7", "s9", "--content", "Renamed", "--completed")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Step updated.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "PUT", "/acme/cards/7/steps/s9")
	assertJSONBody(t, rec, map[string]any{"step": map[string]any{"content": "Renamed", "completed": true}})
}

func TestStepCheckUncheck(t *testing.T) {
	h := newHarness(t)
	h.route("PUT", "/acme/cards/7/steps/s9", stub{Status: 200})

	res := h.run("step", "check", "7", "s9")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Step checked.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertJSONBody(t, h.lastRequest(), map[string]any{"step": map[string]any{"completed": true}})

	res = h.run("step", "uncheck", "7", "s9")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Step unchecked.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertJSONBody(t, h.lastRequest(), map[string]any{"step": map[string]any{"completed": false}})
}

func TestStepDelete(t *testing.T) {
	h := newHarness(t)
	h.route("DELETE", "/acme/cards/7/steps/s9", stub{Status: 204})

	res := h.run("step", "delete", "7", "s9")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Step deleted.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/cards/7/steps/s9")
}

const reactionsFixture = `[{"id":"r1","content":"👍","reacter":{"name":"Ada"}},{"id":"r2","content":"🎉","reacter":{"name":"Bob"}}]`

func TestReactionList(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/cards/7/reactions", stub{Status: 200, Body: reactionsFixture})

	res := h.run("reaction", "list", "7")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "ID  CONTENT  REACTER\nr1  👍        Ada\nr2  🎉        Bob\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/cards/7/reactions")
}

func TestReactionListOnComment(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/cards/7/comments/c1/reactions", stub{Status: 200, Body: reactionsFixture})

	res := h.run("reaction", "list", "7", "--comment-id", "c1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/cards/7/comments/c1/reactions")
}

func TestReactionAdd(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/reactions", stub{Status: 201, Body: `{"id":"r9","content":"👍","reacter":{"name":"Ada"}}`})

	res := h.run("reaction", "add", "7", "--content", "👍")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Reaction added: r9\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "POST", "/acme/cards/7/reactions")
	assertJSONBody(t, rec, map[string]any{"reaction": map[string]any{"content": "👍"}})
}

func TestReactionAddOnComment(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/comments/c1/reactions", stub{Status: 201, Body: `{"id":"r9","content":"🎉","reacter":{"name":"Ada"}}`})

	res := h.run("reaction", "add", "7", "--comment-id", "c1", "--content", "🎉")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/cards/7/comments/c1/reactions")
}

func TestReactionAddContentTooLong(t *testing.T) {
	h := newHarness(t)
	res := h.run("reaction", "add", "7", "--content", "this-is-way-too-long")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestReactionRemove(t *testing.T) {
	h := newHarness(t)
	h.route("DELETE", "/acme/cards/7/reactions/r9", stub{Status: 204})

	res := h.run("reaction", "remove", "7", "r9")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Reaction removed.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/cards/7/reactions/r9")
}

func TestReactionRemoveOnComment(t *testing.T) {
	h := newHarness(t)
	h.route("DELETE", "/acme/cards/7/comments/c1/reactions/r9", stub{Status: 204})

	res := h.run("reaction", "remove", "7", "r9", "--comment-id", "c1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/cards/7/comments/c1/reactions/r9")
}

func TestPinList(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/my/pins", stub{Status: 200, Body: cardsFixture})

	res := h.run("pin", "list")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "#   TITLE       STATUS  BOARD    LAST_ACTIVE\n7   Fix login   open    Bugs     2024-05-06\n12  Add search  closed  Roadmap  2024-05-07\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/my/pins")
}

func TestCardPinUnpin(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/pin", stub{Status: 204})
	h.route("DELETE", "/acme/cards/7/pin", stub{Status: 204})

	res := h.run("card", "pin", "7")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card pinned.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/cards/7/pin")

	res = h.run("card", "unpin", "7")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card unpinned.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/cards/7/pin")
}

func TestAuthTokenRevoke(t *testing.T) {
	h := newHarness(t)
	h.route("DELETE", "/my/access_tokens/t9", stub{Status: 204})

	res := h.run("auth", "token", "revoke", "t9")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Token revoked.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/my/access_tokens/t9")
}
