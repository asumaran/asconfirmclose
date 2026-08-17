package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildBinary compiles the plugin once per test binary so end-to-end tests
// exercise the real argv/env contract herdr uses.
func buildBinary(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("end-to-end tests use a POSIX fake herdr")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "herdr-confirm-close")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func runBinary(t *testing.T, bin string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run %v: %v\n%s", args, err, out)
		}
		code = ee.ExitCode()
	}
	return string(out), code
}

func TestEndToEndCloseAction(t *testing.T) {
	bin := buildBinary(t)
	fakeDir := t.TempDir()
	herdr, log := writeFakeHerdr(t, fakeDir)
	baseEnv := []string{"HERDR_BIN_PATH=" + herdr, "HERDR_PLUGIN_CONFIG_DIR=" + t.TempDir()}

	t.Run("idle pane closes", func(t *testing.T) {
		os.Remove(log)
		out, code := runBinary(t, bin, append(baseEnv, "HERDR_PANE_ID=w1:idle"), "close")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, out)
		}
		argv, _ := os.ReadFile(log)
		if !strings.Contains(string(argv), "pane close w1:idle") {
			t.Fatalf("argv log:\n%s", argv)
		}
	})

	t.Run("busy pane opens popup with env", func(t *testing.T) {
		os.Remove(log)
		out, code := runBinary(t, bin, append(baseEnv, "HERDR_PANE_ID=w1:busy"), "close")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, out)
		}
		argv, _ := os.ReadFile(log)
		s := string(argv)
		if strings.Contains(s, "pane close") {
			t.Fatalf("busy pane closed without confirmation:\n%s", s)
		}
		for _, want := range []string{
			"plugin pane open --plugin " + pluginID + " --entrypoint " + promptEntrypoint + " --placement popup",
			"--env " + envPaneID + "=w1:busy",
			"--env " + envProcess + "=claude",
			"--env " + envCmdline + "=claude --enable-auto-mode",
		} {
			if !strings.Contains(s, want) {
				t.Errorf("argv log missing %q:\n%s", want, s)
			}
		}
	})

	t.Run("ignore list from config dir", func(t *testing.T) {
		os.Remove(log)
		cfgDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cfgDir, configFileName), []byte(`{"ignore":["claude"]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		env := []string{"HERDR_BIN_PATH=" + herdr, "HERDR_PLUGIN_CONFIG_DIR=" + cfgDir, "HERDR_PANE_ID=w1:busy"}
		out, code := runBinary(t, bin, env, "close")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, out)
		}
		argv, _ := os.ReadFile(log)
		if !strings.Contains(string(argv), "pane close w1:busy") {
			t.Fatalf("ignored process should close directly:\n%s", argv)
		}
	})

	t.Run("broken config keeps working with defaults", func(t *testing.T) {
		os.Remove(log)
		cfgDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(cfgDir, configFileName), []byte(`{oops`), 0o644); err != nil {
			t.Fatal(err)
		}
		env := []string{"HERDR_BIN_PATH=" + herdr, "HERDR_PLUGIN_CONFIG_DIR=" + cfgDir, "HERDR_PANE_ID=w1:busy"}
		out, code := runBinary(t, bin, env, "close")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, out)
		}
		if !strings.Contains(out, "config:") || !strings.Contains(out, "using defaults") {
			t.Fatalf("expected config warning, got: %s", out)
		}
		argv, _ := os.ReadFile(log)
		if !strings.Contains(string(argv), "plugin pane open") {
			t.Fatalf("busy pane should still prompt:\n%s", argv)
		}
	})

	t.Run("unknown pane fails", func(t *testing.T) {
		out, code := runBinary(t, bin, append(baseEnv, "HERDR_PANE_ID=w1:nope"), "close")
		if code != 1 || !strings.Contains(out, "pane_not_found") {
			t.Fatalf("exit %d: %s", code, out)
		}
	})

	t.Run("usage and version", func(t *testing.T) {
		out, code := runBinary(t, bin, baseEnv)
		if code != 2 || !strings.Contains(out, "usage:") {
			t.Fatalf("exit %d: %s", code, out)
		}
		out, code = runBinary(t, bin, baseEnv, "-version")
		if code != 0 || !strings.HasPrefix(out, "herdr-confirm-close ") {
			t.Fatalf("exit %d: %s", code, out)
		}
		out, code = runBinary(t, bin, baseEnv, "prompt")
		if code != 2 || !strings.Contains(out, envPaneID) {
			t.Fatalf("prompt without env should fail: exit %d: %s", code, out)
		}
	})
}
