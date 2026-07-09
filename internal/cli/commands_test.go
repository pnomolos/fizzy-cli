package cli

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func TestCardMove(t *testing.T) {
	h := newHarness(t)
	body := `{"id":"cardid","number":7,"title":"Fix login","status":"open","board":{"name":"New Board"},"creator":{"name":"Ada"}}`
	h.route("PUT", "/acme/cards/7/board", stub{Status: 200, Body: body})

	res := h.run("card", "move", "7", "--board-id", "b9")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "Board: New Board") {
		t.Errorf("stdout = %q, want it to show the new board", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "PUT", "/acme/cards/7/board")
	assertJSONBody(t, rec, map[string]any{"board_id": "b9"})
}

func TestCardMoveRequiresBoardID(t *testing.T) {
	h := newHarness(t)
	res := h.run("card", "move", "7")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestCardGoldenUngolden(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/goldness", stub{Status: 204})
	h.route("DELETE", "/acme/cards/7/goldness", stub{Status: 204})

	res := h.run("card", "golden", "7")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card marked golden.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/cards/7/goldness")

	res = h.run("card", "ungolden", "7")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card unmarked golden.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/cards/7/goldness")
}

func TestCardRemoveImage(t *testing.T) {
	h := newHarness(t)
	h.route("DELETE", "/acme/cards/7/image", stub{Status: 204})

	res := h.run("card", "remove-image", "7")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card image removed.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/cards/7/image")
}

func TestCardReadUnread(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/reading", stub{Status: 201})
	h.route("DELETE", "/acme/cards/7/reading", stub{Status: 204})

	res := h.run("card", "read", "7")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card marked read.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/cards/7/reading")

	res = h.run("card", "unread", "7")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Card marked unread.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/cards/7/reading")
}

func TestCardAssignMe(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/self_assignment", stub{Status: 204})

	res := h.run("card", "assign", "7", "--me")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Self-assignment toggled.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/cards/7/self_assignment")
}

func TestCardAssignMeAndAssigneeIDMutuallyExclusive(t *testing.T) {
	h := newHarness(t)
	res := h.run("card", "assign", "7", "--me", "--assignee-id", "u1")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "cannot be used together") {
		t.Errorf("stderr = %q, want mutual-exclusion error", res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestCardAssignAssigneeIDStillWorks(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/cards/7/assignments", stub{Status: 204})

	res := h.run("card", "assign", "7", "--assignee-id", "u1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "POST", "/acme/cards/7/assignments")
	assertJSONBody(t, rec, map[string]any{"assignee_id": "u1"})
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

const activitiesFixture = `[{"id":"a1","action":"card_published","created_at":"2024-08-01","description":"Ada added &quot;Fix login&quot;","eventable_type":"Card","eventable":{"number":7},"board":{"name":"Bugs"},"creator":{"name":"Ada"}}]`

func TestActivityList(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/activities", stub{Status: 200, Body: activitiesFixture})

	res := h.run("activity", "list", "--board-id", "b1", "--creator-id", "u1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "TIME        ACTION          DESCRIPTION            CARD  BOARD  CREATOR\n" +
		"2024-08-01  card_published  Ada added \"Fix login\"  7     Bugs   Ada\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "GET", "/acme/activities")
	if rec.RawQuery != "board_ids%5B%5D=b1&creator_ids%5B%5D=u1" {
		t.Errorf("query = %q", rec.RawQuery)
	}
}

func TestActivityListAll(t *testing.T) {
	h := newHarness(t)
	page1 := `[{"id":"a1","action":"card_published","created_at":"2024-08-01","description":"First","board":{"name":"Bugs"},"creator":{"name":"Ada"}}]`
	page2 := `[{"id":"a2","action":"card_closed","created_at":"2024-08-02","description":"Second","board":{"name":"Bugs"},"creator":{"name":"Bob"}}]`
	h.route("GET", "/acme/activities", stub{Status: 200, Body: page1, Headers: map[string]string{
		"Link": `<` + h.server.URL + `/acme/activities/page2>; rel="next"`,
	}})
	h.route("GET", "/acme/activities/page2", stub{Status: 200, Body: page2})

	res := h.run("activity", "list", "--all")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 2 {
		t.Fatalf("expected 2 requests, got %d", h.requestCount())
	}
	if strings.Count(res.stdout, "TIME") != 1 {
		t.Errorf("expected header to appear exactly once, stdout=%q", res.stdout)
	}
	if !strings.Contains(res.stdout, "card_published") || !strings.Contains(res.stdout, "card_closed") {
		t.Errorf("expected rows from both pages, stdout=%q", res.stdout)
	}
}

func TestSearch(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/search", stub{Status: 200, Body: cardsFixture})

	res := h.run("search", "login")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "#   TITLE       STATUS  BOARD    LAST_ACTIVE\n7   Fix login   open    Bugs     2024-05-06\n12  Add search  closed  Roadmap  2024-05-07\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "GET", "/acme/search")
	if rec.RawQuery != "q=login" {
		t.Errorf("query = %q", rec.RawQuery)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	h := newHarness(t)
	res := h.run("search")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestSearchAll(t *testing.T) {
	h := newHarness(t)
	page1 := `[{"number":7,"title":"Fix login","status":"open","board":{"name":"Bugs"},"last_active_at":"2024-05-06"}]`
	page2 := `[{"number":12,"title":"Add search","status":"closed","board":{"name":"Roadmap"},"last_active_at":"2024-05-07"}]`
	h.route("GET", "/acme/search", stub{Status: 200, Body: page1, Headers: map[string]string{
		"Link": `<` + h.server.URL + `/acme/search/page2>; rel="next"`,
	}})
	h.route("GET", "/acme/search/page2", stub{Status: 200, Body: page2})

	res := h.run("search", "x", "--all")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 2 {
		t.Fatalf("expected 2 requests, got %d", h.requestCount())
	}
	if !strings.Contains(res.stdout, "Fix login") || !strings.Contains(res.stdout, "Add search") {
		t.Errorf("expected rows from both pages, stdout=%q", res.stdout)
	}
}

// The query may sit before --all, and a lone --all is not the query.
func TestSearchFlagBeforeQuery(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/search", stub{Status: 200, Body: cardsFixture})

	res := h.run("search", "--all", "login")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if rec := h.lastRequest(); rec.RawQuery != "q=login" {
		t.Errorf("query = %q, want q=login", rec.RawQuery)
	}
}

func TestSearchAllWithoutQuery(t *testing.T) {
	h := newHarness(t)
	res := h.run("search", "--all")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestAccountGet(t *testing.T) {
	h := newHarness(t)
	body := `{"id":"a1","name":"Acme Inc","cards_count":42,"created_at":"2024-01-01","auto_postpone_period_in_days":30}`
	h.route("GET", "/acme/account/settings", stub{Status: 200, Body: body})

	res := h.run("account", "get")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "ID: a1\nName: Acme Inc\nCards: 42\nCreated: 2024-01-01\nAuto-postpone (days): 30\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/account/settings")
}

func TestAccountUpdate(t *testing.T) {
	h := newHarness(t)
	h.route("PUT", "/acme/account/settings", stub{Status: 204})

	res := h.run("account", "update", "--name", "New Name")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Account updated.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "PUT", "/acme/account/settings")
	assertJSONBody(t, rec, map[string]any{"account": map[string]any{"name": "New Name"}})
}

func TestAccountUpdateRequiresName(t *testing.T) {
	h := newHarness(t)
	res := h.run("account", "update")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestAccountAutoPostpone(t *testing.T) {
	h := newHarness(t)
	body := `{"id":"a1","name":"Acme Inc","cards_count":42,"created_at":"2024-01-01","auto_postpone_period_in_days":90}`
	h.route("PUT", "/acme/account/entropy", stub{Status: 200, Body: body})

	res := h.run("account", "auto-postpone", "90")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "Auto-postpone (days): 90") {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "PUT", "/acme/account/entropy")
	assertJSONBody(t, rec, map[string]any{"entropy": map[string]any{"auto_postpone_period_in_days": float64(90)}})
}

func TestAccountAutoPostponeInvalidValue(t *testing.T) {
	h := newHarness(t)
	res := h.run("account", "auto-postpone", "42")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "3, 7, 11, 30, 90, 365") {
		t.Errorf("stderr = %q, want valid-values message", res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestAccountJoinCodeGet(t *testing.T) {
	h := newHarness(t)
	body := `{"code":"ABC-123","usage_count":2,"usage_limit":10,"url":"https://app/join/ABC-123","active":true}`
	h.route("GET", "/acme/account/join_code", stub{Status: 200, Body: body})

	res := h.run("account", "join-code")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "Code: ABC-123\nURL: https://app/join/ABC-123\nActive: true\nUsage: 2/10\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/account/join_code")

	res = h.run("account", "join-code", "get")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/account/join_code")
}

func TestAccountJoinCodeSetLimit(t *testing.T) {
	h := newHarness(t)
	h.route("PUT", "/acme/account/join_code", stub{Status: 204})

	res := h.run("account", "join-code", "set-limit", "25")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Join code usage limit updated.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "PUT", "/acme/account/join_code")
	assertJSONBody(t, rec, map[string]any{"account_join_code": map[string]any{"usage_limit": float64(25)}})
}

func TestAccountJoinCodeReset(t *testing.T) {
	h := newHarness(t)
	h.route("DELETE", "/acme/account/join_code", stub{Status: 204})

	res := h.run("account", "join-code", "reset")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Join code reset.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/account/join_code")
}

func TestBoardPublish(t *testing.T) {
	h := newHarness(t)
	body := `{"id":"b1","name":"Roadmap","all_access":true,"created_at":"2024-01-02","creator":{"name":"Ada"},"url":"https://app/x"}`
	h.route("POST", "/acme/boards/b1/publication", stub{Status: 201, Body: body})

	res := h.run("board", "publish", "b1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "Name: Roadmap") {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/boards/b1/publication")
}

func TestBoardUnpublish(t *testing.T) {
	h := newHarness(t)
	h.route("DELETE", "/acme/boards/b1/publication", stub{Status: 204})

	res := h.run("board", "unpublish", "b1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Board unpublished.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/boards/b1/publication")
}

func TestBoardAccesses(t *testing.T) {
	h := newHarness(t)
	body := `{"board_id":"b1","all_access":true,"users":[{"id":"u1","name":"Ada","role":"admin","has_access":true,"involvement":"watching"},{"id":"u2","name":"Bob","role":"member","has_access":false,"involvement":"access_only"}]}`
	h.route("GET", "/acme/boards/b1/accesses", stub{Status: 200, Body: body})

	res := h.run("board", "accesses", "b1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "Ada") || !strings.Contains(res.stdout, "watching") || !strings.Contains(res.stdout, "Bob") {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/boards/b1/accesses")
}

func TestBoardWatchUnwatch(t *testing.T) {
	h := newHarness(t)
	h.route("PUT", "/acme/boards/b1/involvement", stub{Status: 204})

	res := h.run("board", "watch", "b1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Now watching board.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "PUT", "/acme/boards/b1/involvement")
	assertJSONBody(t, rec, map[string]any{"involvement": "watching"})

	res = h.run("board", "unwatch", "b1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Stopped watching board.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	rec = h.lastRequest()
	assertRequest(t, rec, "PUT", "/acme/boards/b1/involvement")
	assertJSONBody(t, rec, map[string]any{"involvement": "access_only"})
}

func TestColumnCards(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/boards/b1/columns/c1/cards", stub{Status: 200, Body: cardsFixture})

	res := h.run("column", "cards", "c1", "--board-id", "b1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "#   TITLE       STATUS  BOARD    LAST_ACTIVE\n7   Fix login   open    Bugs     2024-05-06\n12  Add search  closed  Roadmap  2024-05-07\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/boards/b1/columns/c1/cards")
}

// The documented order puts the flags before the column id; both orders work.
func TestColumnCardsFlagsBeforeID(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/boards/b1/columns/c1/cards", stub{Status: 200, Body: cardsFixture})

	res := h.run("column", "cards", "--board-id", "b1", "c1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/boards/b1/columns/c1/cards")
}

func TestColumnMoveFlagsBeforeID(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/columns/c1/right_position", stub{Status: 201})

	res := h.run("column", "move", "--board-id", "b1", "c1", "--right")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/columns/c1/right_position")
}

func TestColumnCardsRequiresBoardID(t *testing.T) {
	h := newHarness(t)
	res := h.run("column", "cards", "c1")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestColumnMoveLeftRight(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/columns/c1/left_position", stub{Status: 201})
	h.route("POST", "/acme/columns/c1/right_position", stub{Status: 201})

	res := h.run("column", "move", "c1", "--board-id", "b1", "--left")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Column moved left.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/columns/c1/left_position")

	res = h.run("column", "move", "c1", "--board-id", "b1", "--right")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Column moved right.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/columns/c1/right_position")
}

func TestColumnMoveRequiresDirection(t *testing.T) {
	h := newHarness(t)
	res := h.run("column", "move", "c1", "--board-id", "b1")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestColumnMoveDirectionMutuallyExclusive(t *testing.T) {
	h := newHarness(t)
	res := h.run("column", "move", "c1", "--board-id", "b1", "--left", "--right")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "cannot be used together") {
		t.Errorf("stderr = %q, want mutual-exclusion error", res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

const webhooksFixture = `[{"id":"w1","name":"CI hook","active":true,"payload_url":"https://ci.example/hook","subscribed_actions":["card_published"]},{"id":"w2","name":"Slack","active":false,"payload_url":"https://slack.example/hook","subscribed_actions":["comment_created"]}]`

const webhookFixture = `{"id":"w1","name":"CI hook","active":true,"signing_secret":"sek","subscribed_actions":["card_published","comment_created"],"payload_url":"https://ci.example/hook","created_at":"2024-09-01"}`

func TestWebhookList(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/boards/b1/webhooks", stub{Status: 200, Body: webhooksFixture})

	res := h.run("webhook", "list", "--board-id", "b1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "ID  NAME     ACTIVE  URL\n" +
		"w1  CI hook  true    https://ci.example/hook\n" +
		"w2  Slack    false   https://slack.example/hook\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/boards/b1/webhooks")
}

func TestWebhookListRequiresBoard(t *testing.T) {
	h := newHarness(t)
	res := h.run("webhook", "list")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestWebhookGet(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/boards/b1/webhooks/w1", stub{Status: 200, Body: webhookFixture})

	res := h.run("webhook", "get", "--board-id", "b1", "w1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "ID: w1\nName: CI hook\nActive: true\nURL: https://ci.example/hook\n" +
		"Signing secret: sek\nSubscribed actions: card_published, comment_created\nCreated: 2024-09-01\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/boards/b1/webhooks/w1")
}

func TestWebhookCreate(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/boards/b1/webhooks", stub{Status: 201, Body: webhookFixture})

	res := h.run("webhook", "create", "--board-id", "b1", "--name", "CI hook", "--url", "https://ci.example/hook", "--event", "card_published", "--event", "comment_created")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "POST", "/acme/boards/b1/webhooks")
	assertJSONBody(t, rec, map[string]any{"webhook": map[string]any{
		"name":               "CI hook",
		"url":                "https://ci.example/hook",
		"subscribed_actions": []any{"card_published", "comment_created"},
	}})
}

func TestWebhookCreateRequiresEvent(t *testing.T) {
	h := newHarness(t)
	res := h.run("webhook", "create", "--board-id", "b1", "--name", "CI", "--url", "https://x/y")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "at least one --event") {
		t.Errorf("stderr = %q", res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestWebhookCreateRejectsInvalidEvent(t *testing.T) {
	h := newHarness(t)
	res := h.run("webhook", "create", "--board-id", "b1", "--name", "CI", "--url", "https://x/y", "--event", "card_exploded")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "unknown webhook event") {
		t.Errorf("stderr = %q, want unknown-event error", res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestWebhookUpdate(t *testing.T) {
	h := newHarness(t)
	h.route("PATCH", "/acme/boards/b1/webhooks/w1", stub{Status: 200, Body: webhookFixture})

	res := h.run("webhook", "update", "--board-id", "b1", "w1", "--name", "Renamed", "--event", "card_closed")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	rec := h.lastRequest()
	assertRequest(t, rec, "PATCH", "/acme/boards/b1/webhooks/w1")
	assertJSONBody(t, rec, map[string]any{"webhook": map[string]any{
		"name":               "Renamed",
		"subscribed_actions": []any{"card_closed"},
	}})
}

func TestWebhookUpdateRejectsURL(t *testing.T) {
	h := newHarness(t)
	res := h.run("webhook", "update", "--board-id", "b1", "w1", "--url", "https://new/url")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "immutable") {
		t.Errorf("stderr = %q, want immutable error", res.stderr)
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls, got %d", h.requestCount())
	}
}

func TestWebhookDelete(t *testing.T) {
	h := newHarness(t)
	h.route("DELETE", "/acme/boards/b1/webhooks/w1", stub{Status: 204})

	res := h.run("webhook", "delete", "--board-id", "b1", "w1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "Webhook deleted.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "DELETE", "/acme/boards/b1/webhooks/w1")
}

func TestWebhookActivate(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/boards/b1/webhooks/w1/activation", stub{Status: 201, Body: webhookFixture})

	res := h.run("webhook", "activate", "--board-id", "b1", "w1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/boards/b1/webhooks/w1/activation")
	if !strings.Contains(res.stdout, "ID: w1") {
		t.Errorf("stdout = %q", res.stdout)
	}
}

func TestWebhookDeliveries(t *testing.T) {
	h := newHarness(t)
	body := `[{"id":"d1","status":"delivered","response_code":200,"created_at":"2024-09-02"}]`
	h.route("GET", "/acme/boards/b1/webhooks/w1/deliveries", stub{Status: 200, Body: body})

	res := h.run("webhook", "deliveries", "--board-id", "b1", "w1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "ID  CREATED_AT  RESPONSE_CODE  STATUS\n" +
		"d1  2024-09-02  200            delivered\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/boards/b1/webhooks/w1/deliveries")
}

func TestWebhookDeliveriesEmpty(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/boards/b1/webhooks/w1/deliveries", stub{Status: 200, Body: `[]`})

	res := h.run("webhook", "deliveries", "--board-id", "b1", "w1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if res.stdout != "" {
		t.Errorf("stdout = %q, want empty", res.stdout)
	}
}

func TestExportCreate(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/account/exports", stub{Status: 201, Body: `{"id":"e1","status":"pending","created_at":"2024-10-01"}`})

	res := h.run("export", "create")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	want := "ID: e1\nStatus: pending\nCreated: 2024-10-01\n"
	if res.stdout != want {
		t.Errorf("stdout\n got: %q\nwant: %q", res.stdout, want)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/account/exports")
}

func TestExportCreateUser(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/users/u9/data_exports", stub{Status: 201, Body: `{"id":"e2","status":"pending","created_at":"2024-10-02"}`})

	res := h.run("export", "create", "--user", "u9")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "POST", "/acme/users/u9/data_exports")
}

func TestExportCreateWaitPolls(t *testing.T) {
	h := newHarness(t)
	old := exportPollInterval
	exportPollInterval = time.Millisecond
	defer func() { exportPollInterval = old }()

	h.route("POST", "/acme/account/exports", stub{Status: 201, Body: `{"id":"e1","status":"pending","created_at":"2024-10-01"}`})
	var polls int32
	h.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/acme/account/exports/e1" {
			n := atomic.AddInt32(&polls, 1)
			h.mu.Lock()
			h.requests = append(h.requests, recordedRequest{Method: r.Method, Path: r.URL.Path})
			h.mu.Unlock()
			w.WriteHeader(200)
			if n < 2 {
				_, _ = w.Write([]byte(`{"id":"e1","status":"processing","created_at":"2024-10-01"}`))
			} else {
				_, _ = w.Write([]byte(`{"id":"e1","status":"completed","created_at":"2024-10-01","download_url":"http://x/d"}`))
			}
			return
		}
		h.handle(w, r)
	})

	res := h.run("export", "create", "--wait", "--timeout", "5s")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if atomic.LoadInt32(&polls) < 2 {
		t.Errorf("polls = %d, want >= 2", polls)
	}
	if !strings.Contains(res.stdout, "Status: completed") || !strings.Contains(res.stdout, "Download URL: http://x/d") {
		t.Errorf("stdout = %q", res.stdout)
	}
}

// A failed export must exit non-zero so scripts don't proceed to download.
func TestExportCreateWaitFailsNonZero(t *testing.T) {
	h := newHarness(t)
	old := exportPollInterval
	exportPollInterval = time.Millisecond
	defer func() { exportPollInterval = old }()

	h.route("POST", "/acme/account/exports", stub{Status: 201, Body: `{"id":"e2","status":"pending","created_at":"2024-10-01"}`})
	h.route("GET", "/acme/account/exports/e2", stub{Status: 200, Body: `{"id":"e2","status":"failed","created_at":"2024-10-01"}`})

	res := h.run("export", "create", "--wait", "--timeout", "5s")
	if res.code == 0 {
		t.Fatalf("exit = 0, want non-zero for a failed export; stdout=%q", res.stdout)
	}
}

// An invalid --timeout must be rejected before the export is created (no orphan).
func TestExportCreateInvalidTimeoutNoRequest(t *testing.T) {
	h := newHarness(t)
	res := h.run("export", "create", "--wait", "--timeout", "notaduration")
	if res.code == 0 {
		t.Fatalf("exit = 0, want non-zero for a bad --timeout")
	}
	if h.requestCount() != 0 {
		t.Errorf("expected no HTTP calls (no orphan export), got %d", h.requestCount())
	}
}

func TestExportGet(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/account/exports/e1", stub{Status: 200, Body: `{"id":"e1","status":"completed","created_at":"2024-10-01","download_url":"http://x/d"}`})

	res := h.run("export", "get", "e1")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "Status: completed") {
		t.Errorf("stdout = %q", res.stdout)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/account/exports/e1")
}

func TestExportDownload(t *testing.T) {
	h := newHarness(t)
	zip := "PK\x03\x04zipbytes"
	h.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/acme/account/exports/e1":
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"id":"e1","status":"completed","created_at":"2024-10-01","download_url":"` + h.server.URL + `/blob/e1"}`))
		case r.Method == "GET" && r.URL.Path == "/blob/e1":
			w.Header().Set("Content-Disposition", `attachment; filename="fizzy-export-e1.zip"`)
			w.Header().Set("Content-Type", "application/zip")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(zip))
		default:
			w.WriteHeader(404)
		}
	})

	dir := t.TempDir()
	out := filepath.Join(dir, "dl.zip")
	res := h.run("export", "download", "e1", "-o", out)
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != zip {
		t.Errorf("downloaded bytes = %q, want %q", data, zip)
	}
	if !strings.Contains(res.stdout, out) {
		t.Errorf("stdout = %q, want path %q", res.stdout, out)
	}
}

func TestExportDownloadNotCompleted(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/account/exports/e1", stub{Status: 200, Body: `{"id":"e1","status":"processing","created_at":"2024-10-01"}`})

	res := h.run("export", "download", "e1")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "not ready to download") {
		t.Errorf("stderr = %q", res.stderr)
	}
}

func TestExportDownloadDefaultFilename(t *testing.T) {
	got := exportDownloadFilename(`attachment; filename="account-export.zip"`, "e1")
	if got != "account-export.zip" {
		t.Errorf("filename = %q", got)
	}
	if got := exportDownloadFilename("", "e1"); got != "export-e1.zip" {
		t.Errorf("fallback filename = %q", got)
	}
}
