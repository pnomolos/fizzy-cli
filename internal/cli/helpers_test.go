package cli

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFirstNonEmpty(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"", "  ", "a", "b"}, "a"},
		{[]string{"  trimmed  "}, "trimmed"},
		{[]string{"", ""}, ""},
		{nil, ""},
	}
	for _, c := range cases {
		if got := firstNonEmpty(c.in...); got != c.want {
			t.Errorf("firstNonEmpty(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeAccount(t *testing.T) {
	cases := map[string]string{
		"/acme/":   "acme",
		"acme":     "acme",
		"/acme":    "acme",
		"acme/":    "acme",
		"//acme//": "acme",
	}
	for in, want := range cases {
		if got := normalizeAccount(in); got != want {
			t.Errorf("normalizeAccount(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMultiStringSetAndValues(t *testing.T) {
	m := multiString{}
	if err := m.Set(""); err != nil {
		t.Fatalf("Set empty: %v", err)
	}
	if len(m.values) != 0 {
		t.Errorf("empty Set should be ignored, got %v", m.values)
	}
	_ = m.Set("a")
	_ = m.Set("b")
	if got := m.Values(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("Values = %v", got)
	}
	if got := m.String(); got != "a,b" {
		t.Errorf("String = %q", got)
	}
	// Values returns a copy, not the backing slice.
	vals := m.Values()
	vals[0] = "mutated"
	if m.values[0] != "a" {
		t.Errorf("Values should return a copy; backing slice mutated to %v", m.values)
	}
}

func TestBuildFieldName(t *testing.T) {
	cases := []struct {
		root, key, want string
	}{
		{"card", "title", "card[title]"},
		{"card", "tag_ids[]", "card[tag_ids][]"},
		{"", "title", "title"},
		{"", "tag_ids[]", "tag_ids[]"},
	}
	for _, c := range cases {
		if got := buildFieldName(c.root, c.key); got != c.want {
			t.Errorf("buildFieldName(%q,%q) = %q, want %q", c.root, c.key, got, c.want)
		}
	}
}

func TestAddListParam(t *testing.T) {
	v := url.Values{}
	addListParam(v, "board_ids[]", []string{"1", "  ", "2", ""})
	got := v["board_ids[]"]
	if !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Errorf("board_ids[] = %v, want [1 2]", got)
	}
}

func TestSetStringParam(t *testing.T) {
	v := url.Values{}
	setStringParam(v, "sorted_by", "  recent  ")
	setStringParam(v, "skip", "   ")
	if got := v.Get("sorted_by"); got != "recent" {
		t.Errorf("sorted_by = %q, want recent", got)
	}
	if _, ok := v["skip"]; ok {
		t.Error("blank value should not be set")
	}
}

func TestNextLink(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string][]string
		want    string
	}{
		{
			name:    "present",
			headers: map[string][]string{"Link": {`<https://x/next?page=2>; rel="next", <https://x/last?page=9>; rel="last"`}},
			want:    "https://x/next?page=2",
		},
		{
			name:    "case-insensitive header key",
			headers: map[string][]string{"link": {`<https://x/next>; rel="next"`}},
			want:    "https://x/next",
		},
		{
			name:    "absent rel next",
			headers: map[string][]string{"Link": {`<https://x/prev>; rel="prev"`}},
			want:    "",
		},
		{
			name:    "no link header",
			headers: map[string][]string{"X-Other": {"y"}},
			want:    "",
		},
		{
			name:    "malformed no sections",
			headers: map[string][]string{"Link": {`https://x/next`}},
			want:    "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := nextLink(c.headers); got != c.want {
				t.Errorf("nextLink = %q, want %q", got, c.want)
			}
		})
	}
}

// writeConfig writes a config file for parseGlobal precedence tests.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseGlobalBaseURLPrecedence(t *testing.T) {
	cfgPath := writeConfig(t, `{"base_url":"https://config.example","token":"ct","account":"cacct"}`)

	t.Run("default when nothing set", func(t *testing.T) {
		t.Setenv("FIZZY_BASE_URL", "")
		empty := writeConfig(t, `{}`)
		ctx := mustParse(t, "--config", empty, "board")
		if ctx.BaseURL != defaultBaseURL {
			t.Errorf("BaseURL = %q, want default %q", ctx.BaseURL, defaultBaseURL)
		}
	})

	t.Run("config beats default", func(t *testing.T) {
		t.Setenv("FIZZY_BASE_URL", "")
		ctx := mustParse(t, "--config", cfgPath, "board")
		if ctx.BaseURL != "https://config.example" {
			t.Errorf("BaseURL = %q, want config value", ctx.BaseURL)
		}
	})

	t.Run("env beats config", func(t *testing.T) {
		t.Setenv("FIZZY_BASE_URL", "https://env.example")
		ctx := mustParse(t, "--config", cfgPath, "board")
		if ctx.BaseURL != "https://env.example" {
			t.Errorf("BaseURL = %q, want env value", ctx.BaseURL)
		}
	})

	t.Run("flag beats env", func(t *testing.T) {
		t.Setenv("FIZZY_BASE_URL", "https://env.example")
		ctx := mustParse(t, "--config", cfgPath, "--base-url", "https://flag.example", "board")
		if ctx.BaseURL != "https://flag.example" {
			t.Errorf("BaseURL = %q, want flag value", ctx.BaseURL)
		}
	})
}

func TestParseGlobalTokenAndAccountPrecedence(t *testing.T) {
	cfgPath := writeConfig(t, `{"token":"ctoken","account":"/caccount/"}`)

	t.Run("config values used", func(t *testing.T) {
		t.Setenv("FIZZY_TOKEN", "")
		t.Setenv("FIZZY_ACCOUNT", "")
		ctx := mustParse(t, "--config", cfgPath, "board")
		if ctx.Token != "ctoken" {
			t.Errorf("Token = %q, want ctoken", ctx.Token)
		}
		if ctx.Account != "caccount" {
			t.Errorf("Account = %q, want normalized caccount", ctx.Account)
		}
	})

	t.Run("env beats config, flag beats env", func(t *testing.T) {
		t.Setenv("FIZZY_TOKEN", "etoken")
		t.Setenv("FIZZY_ACCOUNT", "eaccount")
		ctx := mustParse(t, "--config", cfgPath, "board")
		if ctx.Token != "etoken" || ctx.Account != "eaccount" {
			t.Errorf("env precedence failed: token=%q account=%q", ctx.Token, ctx.Account)
		}
		ctx = mustParse(t, "--config", cfgPath, "--token", "ftoken", "--account", "faccount", "board")
		if ctx.Token != "ftoken" || ctx.Account != "faccount" {
			t.Errorf("flag precedence failed: token=%q account=%q", ctx.Token, ctx.Account)
		}
	})
}

func TestParseGlobalJSONPlainConflict(t *testing.T) {
	empty := writeConfig(t, `{}`)
	t.Setenv("FIZZY_BASE_URL", "")
	t.Setenv("FIZZY_TOKEN", "")
	t.Setenv("FIZZY_ACCOUNT", "")
	var buf bytes.Buffer
	_, _, _, _, err := parseGlobal(&buf, []string{"fizzy-cli", "--config", empty, "--json", "--plain", "board"})
	if err == nil {
		t.Fatal("expected error for --json and --plain together")
	}
}

// mustParse runs parseGlobal and fails on error, returning the Context.
func mustParse(t *testing.T, args ...string) Context {
	t.Helper()
	t.Setenv("FIZZY_CONFIG", "")
	var buf bytes.Buffer
	full := append([]string{"fizzy-cli"}, args...)
	ctx, _, _, _, err := parseGlobal(&buf, full)
	if err != nil {
		t.Fatalf("parseGlobal(%v): %v", args, err)
	}
	return ctx
}
