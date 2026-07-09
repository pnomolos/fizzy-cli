package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

// recordedRequest captures everything the harness observed about an inbound
// HTTP request so tests can assert on method, path, query, headers and body.
type recordedRequest struct {
	Method   string
	Path     string
	RawQuery string
	Header   http.Header
	Body     []byte
}

// stub is a programmable response returned for a matched route.
type stub struct {
	Status  int
	Headers map[string]string
	Body    string
}

// harness wires a hermetic httptest.Server to the real CLI entrypoint. Every
// request is recorded; responses are looked up by "METHOD /path".
type harness struct {
	t          *testing.T
	server     *httptest.Server
	configPath string

	// Default credentials/account injected as global flags. Empty values are
	// omitted so tests can exercise the auth-missing paths.
	token   string
	account string

	mu       sync.Mutex
	routes   map[string]stub
	requests []recordedRequest
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	// Neutralise any ambient configuration so tests are hermetic.
	t.Setenv("FIZZY_TOKEN", "")
	t.Setenv("FIZZY_ACCOUNT", "")
	t.Setenv("FIZZY_BASE_URL", "")
	t.Setenv("FIZZY_CONFIG", "")

	h := &harness{
		t:          t,
		configPath: filepath.Join(t.TempDir(), "config.json"),
		token:      "test-token",
		account:    "acme",
		routes:     map[string]stub{},
	}
	h.server = httptest.NewServer(http.HandlerFunc(h.handle))
	t.Cleanup(h.server.Close)
	return h
}

func (h *harness) handle(w http.ResponseWriter, r *http.Request) {
	body := readAll(h.t, r)
	rec := recordedRequest{
		Method:   r.Method,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
		Header:   r.Header.Clone(),
		Body:     body,
	}
	h.mu.Lock()
	h.requests = append(h.requests, rec)
	s, ok := h.routes[r.Method+" "+r.URL.Path]
	h.mu.Unlock()

	if !ok {
		s = stub{Status: 200, Body: ""}
	}
	for k, v := range s.Headers {
		w.Header().Set(k, v)
	}
	status := s.Status
	if status == 0 {
		status = 200
	}
	w.WriteHeader(status)
	if s.Body != "" {
		_, _ = w.Write([]byte(s.Body))
	}
}

// route registers a programmable response for a method+path pair.
func (h *harness) route(method, path string, s stub) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.routes[method+" "+path] = s
}

// result is the outcome of a single CLI invocation.
type result struct {
	stdout string
	stderr string
	code   int
}

// run executes the CLI through the real Run entrypoint. Global flags for the
// test server, config path and (optionally) credentials are prepended, so args
// should be the command as a user would type it (e.g. "board", "list").
func (h *harness) run(args ...string) result {
	h.t.Helper()
	full := []string{"fizzy-cli", "--base-url", h.server.URL, "--config", h.configPath}
	if h.token != "" {
		full = append(full, "--token", h.token)
	}
	if h.account != "" {
		full = append(full, "--account", h.account)
	}
	full = append(full, args...)
	return h.runRaw(full...)
}

// runRaw executes the CLI with the exact args given (program name first), doing
// no injection. Useful for exercising flag placement and error paths.
func (h *harness) runRaw(args ...string) result {
	h.t.Helper()
	var out, errBuf bytes.Buffer
	code := Run(&out, &errBuf, "test", "testcommit", "testdate", args)
	return result{stdout: out.String(), stderr: errBuf.String(), code: code}
}

func (h *harness) lastRequest() recordedRequest {
	h.t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) == 0 {
		h.t.Fatal("no requests were recorded")
	}
	return h.requests[len(h.requests)-1]
}

func (h *harness) requestCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.requests)
}

// assertRequest checks the method and path of a recorded request.
func assertRequest(t *testing.T, rec recordedRequest, method, path string) {
	t.Helper()
	if rec.Method != method {
		t.Errorf("method = %q, want %q", rec.Method, method)
	}
	if rec.Path != path {
		t.Errorf("path = %q, want %q", rec.Path, path)
	}
}

// assertJSONBody asserts that the recorded body equals the expected JSON value.
func assertJSONBody(t *testing.T, rec recordedRequest, want any) {
	t.Helper()
	var got any
	if err := json.Unmarshal(rec.Body, &got); err != nil {
		t.Fatalf("body is not JSON: %v (body=%q)", err, rec.Body)
	}
	wantBytes, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	var wantNorm any
	if err := json.Unmarshal(wantBytes, &wantNorm); err != nil {
		t.Fatalf("normalise want: %v", err)
	}
	if !reflect.DeepEqual(got, wantNorm) {
		t.Errorf("body JSON mismatch\n got: %#v\nwant: %#v", got, wantNorm)
	}
}

func readAll(t *testing.T, r *http.Request) []byte {
	t.Helper()
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	buf := &bytes.Buffer{}
	if _, err := buf.ReadFrom(r.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return buf.Bytes()
}
