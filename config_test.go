package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	for _, dir := range []string{"", t.TempDir()} {
		cfg, err := loadConfig(dir)
		if err != nil {
			t.Fatalf("dir=%q: %v", dir, err)
		}
		if cfg.Popup.Width != defaultPopupWidth || cfg.Popup.Height != defaultPopupHeight {
			t.Fatalf("dir=%q: popup = %+v", dir, cfg.Popup)
		}
		if len(cfg.Ignore) != 0 {
			t.Fatalf("dir=%q: ignore = %v", dir, cfg.Ignore)
		}
	}
}

func TestLoadConfigFile(t *testing.T) {
	dir := t.TempDir()
	body := `{"ignore": ["less", " MAN ", ""], "popup": {"width": "50%"}}`
	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	set := cfg.ignoreSet()
	if !set["less"] || !set["man"] || len(set) != 2 {
		t.Fatalf("ignoreSet = %v", set)
	}
	if cfg.Popup.Width != "50%" {
		t.Fatalf("width = %q", cfg.Popup.Width)
	}
	if cfg.Popup.Height != defaultPopupHeight {
		t.Fatalf("height should keep default, got %q", cfg.Popup.Height)
	}
}

func TestLoadConfigRejectsUnknownAndMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)

	if err := os.WriteFile(path, []byte(`{"ignroe": ["less"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(dir)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	// Defaults are still returned so the caller can keep going.
	if cfg.Popup.Width != defaultPopupWidth {
		t.Fatalf("defaults not returned on error: %+v", cfg)
	}

	if err := os.WriteFile(path, []byte(`{"ignore": [`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(dir); err == nil {
		t.Fatal("expected error for malformed json")
	}
}
