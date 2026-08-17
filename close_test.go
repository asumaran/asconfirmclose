package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// fakeRunner scripts herdr responses per command prefix and records every
// argv it receives.
type fakeRunner struct {
	t         *testing.T
	responses map[string]fakeResponse // key: first two args joined by space
	calls     [][]string
}

type fakeResponse struct {
	out string
	err error
}

func (f *fakeRunner) run(args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	key := strings.Join(args[:min(2, len(args))], " ")
	if len(args) >= 3 && args[0] == "plugin" {
		key = strings.Join(args[:3], " ")
	}
	resp, ok := f.responses[key]
	if !ok {
		f.t.Fatalf("unexpected herdr call: %v", args)
	}
	return []byte(resp.out), resp.err
}

func (f *fakeRunner) call(i int) []string {
	if i >= len(f.calls) {
		f.t.Fatalf("expected at least %d herdr calls, got %d: %v", i+1, len(f.calls), f.calls)
	}
	return f.calls[i]
}

const idleInfo = `{"id":"x","result":{"process_info":{"foreground_process_group_id":10,"foreground_processes":[{"argv0":"-zsh","cmdline":"-zsh","name":"zsh","pid":10}],"pane_id":"w1:p1","shell_pid":10},"type":"pane_process_info"}}`

const busyInfo = `{"id":"x","result":{"process_info":{"foreground_process_group_id":20,"foreground_processes":[{"argv":["claude","--enable-auto-mode"],"argv0":"claude","cmdline":"claude --enable-auto-mode","name":"2.1.233","pid":20}],"pane_id":"w1:p1","shell_pid":10},"type":"pane_process_info"}}`

func TestRunCloseIdlePaneClosesImmediately(t *testing.T) {
	f := &fakeRunner{t: t, responses: map[string]fakeResponse{
		"pane process-info": {out: idleInfo},
		"pane close":        {out: `{"id":"x","result":{"type":"ok"}}`},
	}}
	var log bytes.Buffer
	outcome, err := runClose(f, defaultConfig(), "w1:p1", &log)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != outcomeClosed {
		t.Fatalf("outcome = %v, want closed", outcome)
	}
	if got := f.call(0); strings.Join(got, " ") != "pane process-info --pane w1:p1" {
		t.Fatalf("first call = %v", got)
	}
	if got := f.call(1); strings.Join(got, " ") != "pane close w1:p1" {
		t.Fatalf("second call = %v", got)
	}
	if len(f.calls) != 2 {
		t.Fatalf("expected exactly 2 calls, got %v", f.calls)
	}
	if !strings.Contains(log.String(), "closed idle pane w1:p1") {
		t.Fatalf("log = %q", log.String())
	}
}

func TestRunCloseBusyPaneOpensPopup(t *testing.T) {
	f := &fakeRunner{t: t, responses: map[string]fakeResponse{
		"pane process-info": {out: busyInfo},
		"plugin pane open":  {out: `{"id":"x","result":{"type":"ok"}}`},
	}}
	cfg := defaultConfig()
	outcome, err := runClose(f, cfg, "w1:p1", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if outcome != outcomePrompted {
		t.Fatalf("outcome = %v, want prompted", outcome)
	}
	for _, c := range f.calls {
		if c[0] == "pane" && c[1] == "close" {
			t.Fatalf("busy pane must not be closed without confirmation: %v", f.calls)
		}
	}
	open := strings.Join(f.call(1), " ")
	for _, want := range []string{
		"plugin pane open",
		"--plugin " + pluginID,
		"--entrypoint " + promptEntrypoint,
		"--placement popup",
		"--width " + defaultPopupWidth,
		"--height " + defaultPopupHeight,
		"--env " + envPaneID + "=w1:p1",
		"--env " + envProcess + "=claude",
		"--env " + envCmdline + "=claude --enable-auto-mode",
	} {
		if !strings.Contains(open, want) {
			t.Errorf("popup open command missing %q: %s", want, open)
		}
	}
}

func TestRunClosePopupUsesConfiguredSize(t *testing.T) {
	f := &fakeRunner{t: t, responses: map[string]fakeResponse{
		"pane process-info": {out: busyInfo},
		"plugin pane open":  {out: `{"id":"x","result":{"type":"ok"}}`},
	}}
	cfg := defaultConfig()
	cfg.Popup.Width = "50%"
	cfg.Popup.Height = "12"
	if _, err := runClose(f, cfg, "w1:p1", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	open := strings.Join(f.call(1), " ")
	if !strings.Contains(open, "--width 50% --height 12") {
		t.Fatalf("popup size not applied: %s", open)
	}
}

func TestRunCloseIgnoredProcessCloses(t *testing.T) {
	f := &fakeRunner{t: t, responses: map[string]fakeResponse{
		"pane process-info": {out: busyInfo},
		"pane close":        {out: `{"id":"x","result":{"type":"ok"}}`},
	}}
	cfg := defaultConfig()
	cfg.Ignore = []string{"Claude"}
	outcome, err := runClose(f, cfg, "w1:p1", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if outcome != outcomeClosed {
		t.Fatalf("outcome = %v, want closed", outcome)
	}
}

func TestRunCloseResolvesFocusedPaneWhenEnvMissing(t *testing.T) {
	f := &fakeRunner{t: t, responses: map[string]fakeResponse{
		"pane current":      {out: `{"id":"x","result":{"pane":{"pane_id":"w9:pZ"},"type":"pane_info"}}`},
		"pane process-info": {out: idleInfo},
		"pane close":        {out: `{"id":"x","result":{"type":"ok"}}`},
	}}
	if _, err := runClose(f, defaultConfig(), "", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(f.call(0), " "); got != "pane current" {
		t.Fatalf("first call = %q", got)
	}
	if got := strings.Join(f.call(1), " "); got != "pane process-info --pane w9:pZ" {
		t.Fatalf("second call = %q", got)
	}
	if got := strings.Join(f.call(2), " "); got != "pane close w9:pZ" {
		t.Fatalf("third call = %q", got)
	}
}

func TestRunCloseProcessInfoFailureFallsBackToPrompt(t *testing.T) {
	f := &fakeRunner{t: t, responses: map[string]fakeResponse{
		"pane process-info": {err: errors.New("boom")},
		"plugin pane open":  {out: `{"id":"x","result":{"type":"ok"}}`},
	}}
	var log bytes.Buffer
	outcome, err := runClose(f, defaultConfig(), "w1:p1", &log)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != outcomePrompted {
		t.Fatalf("outcome = %v, want prompted", outcome)
	}
	open := strings.Join(f.call(1), " ")
	if !strings.Contains(open, "--env "+envProcess+"="+unknownProcess) {
		t.Fatalf("expected unknown process in popup env: %s", open)
	}
	if strings.Contains(open, envCmdline+"=") {
		t.Fatalf("no cmdline should be passed for unknown process: %s", open)
	}
	if !strings.Contains(log.String(), "asking before closing") {
		t.Fatalf("log = %q", log.String())
	}
}

func TestRunCloseUnknownPaneIsAnError(t *testing.T) {
	f := &fakeRunner{t: t, responses: map[string]fakeResponse{
		"pane process-info": {
			out: `{"error":{"code":"pane_not_found","message":"pane not found"},"id":"x"}`,
			err: &cliError{Code: "pane_not_found", Message: "pane not found"},
		},
	}}
	_, err := runClose(f, defaultConfig(), "w1:p1", &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for unknown pane")
	}
	if !isPaneNotFound(err) {
		t.Fatalf("error = %v, want pane_not_found", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("no further herdr calls expected, got %v", f.calls)
	}
}

func TestRunClosePopupFailureIsReported(t *testing.T) {
	f := &fakeRunner{t: t, responses: map[string]fakeResponse{
		"pane process-info": {out: busyInfo},
		"plugin pane open":  {err: &cliError{Code: "ui_busy", Message: "another modal is active"}},
	}}
	_, err := runClose(f, defaultConfig(), "w1:p1", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "ui_busy") {
		t.Fatalf("err = %v, want ui_busy", err)
	}
}
