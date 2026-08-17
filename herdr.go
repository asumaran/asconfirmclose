package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runner executes a herdr CLI command and returns its stdout.
// It is an interface so tests can drive the plugin without spawning herdr.
type runner interface {
	run(args ...string) ([]byte, error)
}

// herdrBin resolves the herdr executable. Plugin commands receive
// HERDR_BIN_PATH from the running server; fall back to PATH lookup so the
// binary is still usable by hand.
func herdrBin() string {
	if bin := os.Getenv("HERDR_BIN_PATH"); bin != "" {
		return bin
	}
	return "herdr"
}

// execRunner runs the real herdr binary.
type execRunner struct {
	bin string
}

// cliError is a structured error returned by the herdr CLI as
// {"error":{"code":..,"message":..}}.
type cliError struct {
	Code    string
	Message string
}

func (e *cliError) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (r execRunner) run(args ...string) ([]byte, error) {
	cmd := exec.Command(r.bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.Bytes()
	if err != nil {
		// herdr prints the JSON error envelope on stdout and exits 1; surface
		// its code/message when present so callers can decide what to do.
		if ce := parseCLIError(out); ce != nil {
			return out, ce
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(string(out))
		}
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("herdr %s: %s", strings.Join(args, " "), msg)
	}
	if ce := parseCLIError(out); ce != nil {
		return out, ce
	}
	return out, nil
}

// envelope is the common shape of herdr CLI JSON responses.
type envelope struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseCLIError(out []byte) *cliError {
	var env envelope
	if err := json.Unmarshal(out, &env); err != nil || env.Error == nil {
		return nil
	}
	return &cliError{Code: env.Error.Code, Message: env.Error.Message}
}

// decodeResult unmarshals the "result" object of a herdr response into dst.
func decodeResult(out []byte, dst any) error {
	var env envelope
	if err := json.Unmarshal(out, &env); err != nil {
		return fmt.Errorf("parse herdr response: %w", err)
	}
	if env.Error != nil {
		return &cliError{Code: env.Error.Code, Message: env.Error.Message}
	}
	if len(env.Result) == 0 {
		return errors.New("herdr response has no result")
	}
	if err := json.Unmarshal(env.Result, dst); err != nil {
		return fmt.Errorf("parse herdr result: %w", err)
	}
	return nil
}

// processInfo mirrors the result of `herdr pane process-info`.
type processInfo struct {
	PaneID                 string         `json:"pane_id"`
	ShellPID               *uint32        `json:"shell_pid"`
	ForegroundProcessGroup *uint32        `json:"foreground_process_group_id"`
	ForegroundProcesses    []processEntry `json:"foreground_processes"`
}

type processEntry struct {
	PID     uint32   `json:"pid"`
	Name    string   `json:"name"`
	Argv0   string   `json:"argv0"`
	Argv    []string `json:"argv"`
	Cmdline string   `json:"cmdline"`
	Cwd     string   `json:"cwd"`
}

func fetchProcessInfo(r runner, paneID string) (processInfo, error) {
	out, err := r.run("pane", "process-info", "--pane", paneID)
	if err != nil {
		return processInfo{}, err
	}
	var res struct {
		ProcessInfo processInfo `json:"process_info"`
	}
	if err := decodeResult(out, &res); err != nil {
		return processInfo{}, err
	}
	return res.ProcessInfo, nil
}

// currentPaneID asks herdr for the focused pane, used when the action was
// invoked without HERDR_PANE_ID in the environment.
func currentPaneID(r runner) (string, error) {
	out, err := r.run("pane", "current")
	if err != nil {
		return "", err
	}
	var res struct {
		Pane struct {
			PaneID string `json:"pane_id"`
		} `json:"pane"`
	}
	if err := decodeResult(out, &res); err != nil {
		return "", err
	}
	if res.Pane.PaneID == "" {
		return "", errors.New("herdr pane current returned no pane_id")
	}
	return res.Pane.PaneID, nil
}

func closePane(r runner, paneID string) error {
	_, err := r.run("pane", "close", paneID)
	return err
}
