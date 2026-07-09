package cli

import "testing"

// A global value flag that is actually the value of a preceding subcommand
// flag must not be hijacked (regression: `--description --account` stole
// --account and consumed the following token as the account).
func TestGlobalValueFlagNotStolenAsSubcommandValue(t *testing.T) {
	h := newHarness(t)
	h.route("POST", "/acme/boards/3/cards", stub{Status: 201, Headers: map[string]string{"Location": "/acme/cards/9"}})

	res := h.run("card", "create", "--board-id", "3", "--title", "T", "--description", "--account", "foo")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	rec := h.lastRequest()
	// Still the default account, not "foo".
	assertRequest(t, rec, "POST", "/acme/boards/3/cards")
	assertJSONBody(t, rec, map[string]any{"card": map[string]any{"title": "T", "description": "--account"}})
}

// A trailing global value flag that follows a real value (not another flag) is
// still honoured in any position.
func TestGlobalValueFlagTrailingHonoured(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/globex/boards", stub{Status: 200, Body: boardsFixture})

	res := h.run("board", "list", "--account", "globex")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "GET", "/globex/boards")
}

// "--" ends flag parsing and does not become the command.
func TestDoubleDashTerminator(t *testing.T) {
	h := newHarness(t)
	h.route("GET", "/acme/boards", stub{Status: 200, Body: boardsFixture})

	res := h.run("--", "board", "list")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr=%q", res.code, res.stderr)
	}
	assertRequest(t, h.lastRequest(), "GET", "/acme/boards")
}
