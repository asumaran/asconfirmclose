package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// configFileName lives in HERDR_PLUGIN_CONFIG_DIR (created by herdr on
// install/link; `herdr plugin config-dir asumaran.confirm-close` prints it).
const configFileName = "config.json"

// Default popup geometry, in outer terminal cells (herdr draws the border).
const (
	defaultPopupWidth  = "64"
	defaultPopupHeight = "9"
)

// config is the user-editable plugin configuration.
type config struct {
	// Ignore lists process names that never trigger a prompt, in addition to
	// the built-in shells. Compared case-insensitively against the process
	// display name (argv0 basename), e.g. ["less", "man", "htop"].
	Ignore []string `json:"ignore"`
	// Popup overrides the confirmation popup size. Values are herdr popup
	// sizes: cell counts ("64") or percentages ("50%").
	Popup struct {
		Width  string `json:"width"`
		Height string `json:"height"`
	} `json:"popup"`
}

func defaultConfig() config {
	var c config
	c.Popup.Width = defaultPopupWidth
	c.Popup.Height = defaultPopupHeight
	return c
}

// loadConfig reads config.json from dir. A missing file yields defaults; a
// malformed file is an error so typos do not silently disable settings.
func loadConfig(dir string) (config, error) {
	c := defaultConfig()
	if dir == "" {
		return c, nil
	}
	raw, err := os.ReadFile(filepath.Join(dir, configFileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return c, nil
		}
		return c, err
	}
	var file config
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return c, fmt.Errorf("%s: %w", configFileName, err)
	}
	c.Ignore = file.Ignore
	if file.Popup.Width != "" {
		c.Popup.Width = file.Popup.Width
	}
	if file.Popup.Height != "" {
		c.Popup.Height = file.Popup.Height
	}
	return c, nil
}

// ignoreSet lowercases the ignore list into a lookup set.
func (c config) ignoreSet() map[string]bool {
	set := make(map[string]bool, len(c.Ignore))
	for _, name := range c.Ignore {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			set[name] = true
		}
	}
	return set
}
