package cli

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"
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
