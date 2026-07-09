package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDoBearerAuthHeader(t *testing.T) {
	var gotAuth, gotCookie, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		gotAccept = r.Header.Get("Accept")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok123", "sess456", "agent/1")
	if _, err := c.Do(context.Background(), "GET", "/x", nil, nil, "", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "Bearer tok123" {
		t.Errorf("Authorization = %q, want Bearer tok123", gotAuth)
	}
	if gotCookie != "" {
		t.Errorf("Cookie = %q, want empty when token set", gotCookie)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
}

func TestDoSessionCookieHeader(t *testing.T) {
	var gotAuth, gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", "sess456", "agent/1")
	if _, err := c.Do(context.Background(), "GET", "/x", nil, nil, "", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty when only session token", gotAuth)
	}
	if gotCookie != "session_token=sess456" {
		t.Errorf("Cookie = %q, want session_token=sess456", gotCookie)
	}
}

func TestDoCustomHeadersOverrideCookie(t *testing.T) {
	var gotCookie, gotCustom string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotCustom = r.Header.Get("X-Custom")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", "sess456", "agent/1")
	headers := map[string]string{
		"Cookie":   "pending_authentication_token=abc",
		"X-Custom": "yes",
	}
	if _, err := c.Do(context.Background(), "GET", "/x", nil, nil, "", headers); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotCookie != "pending_authentication_token=abc" {
		t.Errorf("Cookie = %q, want the caller-supplied cookie to win", gotCookie)
	}
	if gotCustom != "yes" {
		t.Errorf("X-Custom = %q, want yes", gotCustom)
	}
}

func TestDoErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "", "agent/1")
	_, err := c.Do(context.Background(), "POST", "/x", nil, nil, "", nil)
	if err == nil {
		t.Fatal("expected error for status 422")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != 422 {
		t.Errorf("Status = %d, want 422", apiErr.Status)
	}
	if !strings.Contains(apiErr.Error(), "422") || !strings.Contains(apiErr.Error(), "bad") {
		t.Errorf("Error() = %q, want to contain status and body", apiErr.Error())
	}
}

func TestDoSuccessResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/acme/boards/9")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "", "agent/1")
	resp, err := c.Do(context.Background(), "POST", "/x", nil, bytes.NewBufferString("{}"), "application/json", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.Status != 201 {
		t.Errorf("Status = %d, want 201", resp.Status)
	}
	if got := resp.Headers.Get("Location"); got != "/acme/boards/9" {
		t.Errorf("Location = %q", got)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Errorf("Body = %q", resp.Body)
	}
}

func TestAPIErrorError(t *testing.T) {
	withBody := &APIError{Status: 500, Body: []byte("  boom  ")}
	if got := withBody.Error(); got != "api error: status 500: boom" {
		t.Errorf("Error() = %q", got)
	}
	empty := &APIError{Status: 404, Body: []byte("   ")}
	if got := empty.Error(); got != "api error: status 404" {
		t.Errorf("Error() = %q", got)
	}
}

func TestBuildURLRelative(t *testing.T) {
	got, err := buildURL("https://api.example.com", "/acme/boards", nil)
	if err != nil {
		t.Fatalf("buildURL: %v", err)
	}
	if got != "https://api.example.com/acme/boards" {
		t.Errorf("got %q", got)
	}
}

func TestBuildURLQueryEncoding(t *testing.T) {
	q := url.Values{}
	q.Add("board_ids[]", "1")
	q.Add("board_ids[]", "2")
	q.Set("terms", "a b")
	got, err := buildURL("https://api.example.com", "/acme/cards", q)
	if err != nil {
		t.Fatalf("buildURL: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Path != "/acme/cards" {
		t.Errorf("path = %q", u.Path)
	}
	vals := u.Query()
	if ids := vals["board_ids[]"]; len(ids) != 2 || ids[0] != "1" || ids[1] != "2" {
		t.Errorf("board_ids[] = %v", ids)
	}
	if vals.Get("terms") != "a b" {
		t.Errorf("terms = %q", vals.Get("terms"))
	}
}

func TestBuildURLAbsolute(t *testing.T) {
	got, err := buildURL("https://api.example.com", "https://other.example.com/next?page=2", nil)
	if err != nil {
		t.Fatalf("buildURL: %v", err)
	}
	if got != "https://other.example.com/next?page=2" {
		t.Errorf("got %q", got)
	}
}

func TestBuildURLAbsoluteWithQueryOverride(t *testing.T) {
	q := url.Values{}
	q.Set("page", "3")
	got, err := buildURL("https://api.example.com", "https://other.example.com/next?page=2", q)
	if err != nil {
		t.Fatalf("buildURL: %v", err)
	}
	if got != "https://other.example.com/next?page=3" {
		t.Errorf("got %q, want query overridden", got)
	}
}
