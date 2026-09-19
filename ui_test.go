package main

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func key(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

func newTestModel(closeErr error) (promptModel, *int) {
	calls := 0
	m := newPromptModel("w1:p1", "claude", "claude --enable-auto-mode", func() error {
		calls++
		return closeErr
	})
	return m, &calls
}

func TestPromptViewShowsProcess(t *testing.T) {
	m, _ := newTestModel(nil)
	m.width = 64
	view := m.render()
	for _, want := range []string{"Close pane?", "w1:p1", "claude", "claude --enable-auto-mode", "close", "keep"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestPromptViewHidesCmdlineWhenSameAsProcess(t *testing.T) {
	m := newPromptModel("w1:p1", "vim", "vim", nil)
	if n := strings.Count(m.render(), "vim"); n != 1 {
		t.Fatalf("expected process shown once, got %d:\n%s", n, m.render())
	}
}

func TestPromptViewUnknownProcess(t *testing.T) {
	m := newPromptModel("w1:p1", unknownProcess, "", nil)
	view := m.render()
	if !strings.Contains(view, "unknown process") || !strings.Contains(view, "could not inspect") {
		t.Fatalf("view:\n%s", view)
	}
}

func TestPromptYesClosesPane(t *testing.T) {
	m, calls := newTestModel(nil)
	next, cmd := m.Update(key("y"))
	pm := next.(promptModel)
	if pm.decision != decisionClose || !pm.closing {
		t.Fatalf("after y: %+v", pm)
	}
	if cmd == nil {
		t.Fatal("expected close command")
	}
	msg := cmd()
	if *calls != 1 {
		t.Fatalf("closeFn calls = %d", *calls)
	}
	cm, ok := msg.(closedMsg)
	if !ok || cm.err != nil {
		t.Fatalf("msg = %#v", msg)
	}
	next, cmd = pm.Update(cm)
	pm = next.(promptModel)
	if !pm.done || cmd == nil {
		t.Fatalf("expected quit after successful close: %+v", pm)
	}
	if !strings.Contains(pm.render(), "Close pane?") {
		t.Fatalf("view after close: %q", pm.render())
	}
}

func TestPromptUppercaseYAlsoCloses(t *testing.T) {
	m, _ := newTestModel(nil)
	next, _ := m.Update(key("Y"))
	if next.(promptModel).decision != decisionClose {
		t.Fatal("Y should close")
	}
}

func TestPromptOtherKeysKeepPane(t *testing.T) {
	for _, k := range []string{"n", "N", "esc", "enter", "q", "ctrl+c", " ", "x"} {
		m, calls := newTestModel(nil)
		next, cmd := m.Update(key(k))
		pm := next.(promptModel)
		if pm.decision != decisionKeep || !pm.done {
			t.Errorf("key %q: %+v", k, pm)
		}
		if cmd == nil {
			t.Errorf("key %q: expected quit command", k)
		}
		if *calls != 0 {
			t.Errorf("key %q: closeFn must not run, calls=%d", k, *calls)
		}
	}
}

func TestPromptIgnoresKeysWhileClosing(t *testing.T) {
	m, calls := newTestModel(nil)
	next, _ := m.Update(key("y"))
	pm := next.(promptModel)
	next, cmd := pm.Update(key("n"))
	pm = next.(promptModel)
	if pm.done || cmd != nil || pm.decision != decisionClose {
		t.Fatalf("keys must be ignored while closing: %+v", pm)
	}
	if *calls != 0 {
		// closeFn only runs when the returned tea.Cmd executes.
		t.Fatalf("closeFn calls = %d", *calls)
	}
}

func TestPromptCloseFailureShowsErrorThenKeeps(t *testing.T) {
	m, _ := newTestModel(errors.New("pane_not_found: pane not found"))
	next, cmd := m.Update(key("y"))
	pm := next.(promptModel)
	next, cmd = pm.Update(cmd())
	pm = next.(promptModel)
	if pm.err == nil || pm.closing || pm.done || cmd != nil {
		t.Fatalf("expected error state: %+v", pm)
	}
	view := pm.render()
	if !strings.Contains(view, "Could not close") || !strings.Contains(view, "pane_not_found") {
		t.Fatalf("view:\n%s", view)
	}
	next, cmd = pm.Update(key("x"))
	pm = next.(promptModel)
	if !pm.done || cmd == nil || pm.decision != decisionKeep {
		t.Fatalf("any key should dismiss error and quit: %+v", pm)
	}
}

func TestPromptWindowSize(t *testing.T) {
	m, _ := newTestModel(nil)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 5})
	pm := next.(promptModel)
	if pm.width != 30 || pm.height != 5 {
		t.Fatalf("size not stored: %+v", pm)
	}
	pm.cmdline = strings.Repeat("a", 200)
	if !strings.Contains(pm.render(), "…") {
		t.Fatal("long cmdline should be truncated")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := truncate("hello world", 6); got != "hello…" {
		t.Fatalf("got %q", got)
	}
	if got := truncate("héllo wörld", 4); got != "hél…" {
		t.Fatalf("got %q", got)
	}
	if got := truncate("abc", 1); got != "abc" {
		t.Fatalf("n<=1 should be a no-op, got %q", got)
	}
}
