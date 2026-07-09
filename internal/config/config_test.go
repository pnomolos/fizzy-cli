package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load missing: unexpected error %v", err)
	}
	if cfg != (Config{}) {
		t.Errorf("Load missing = %+v, want zero Config", cfg)
	}
}

func TestLoadValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := `{"base_url":"https://x","token":"t","session_token":"s","account":"acme"}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{BaseURL: "https://x", Token: "t", SessionToken: "s", Account: "acme"}
	if cfg != want {
		t.Errorf("Load = %+v, want %+v", cfg, want)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	want := Config{BaseURL: "https://x", Token: "t", Account: "acme"}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestSavePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on windows")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := Save(path, Config{Token: "secret"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 600", perm)
	}
}

func TestSaveAtomicNoTempLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := Save(path, Config{Token: "t"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file should be renamed away, stat err = %v", err)
	}
	// File content is valid JSON with a trailing newline.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Errorf("expected trailing newline, got %q", data)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Errorf("saved file not valid JSON: %v", err)
	}
}
