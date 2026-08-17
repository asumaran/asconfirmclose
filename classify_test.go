package main

import "testing"

func u32(v uint32) *uint32 { return &v }

func TestProcessDisplayName(t *testing.T) {
	cases := []struct {
		name string
		p    processEntry
		want string
	}{
		{"argv0 basename", processEntry{Name: "2.1.233", Argv0: "claude", Argv: []string{"claude", "--flag"}}, "claude"},
		{"login shell dash", processEntry{Name: "zsh", Argv0: "-zsh"}, "zsh"},
		{"argv0 with path", processEntry{Name: "node", Argv0: "/usr/local/bin/node"}, "node"},
		{"argv0 command line", processEntry{Name: "node", Argv0: "npm exec chrome-devtools-mcp@latest"}, "npm"},
		{"argv fallback", processEntry{Name: "x", Argv: []string{"/opt/webpack/bin/webpack", "serve"}}, "webpack"},
		{"name fallback", processEntry{Name: "sleep"}, "sleep"},
		{"empty everything", processEntry{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := processDisplayName(tc.p); got != tc.want {
				t.Fatalf("processDisplayName(%+v) = %q, want %q", tc.p, got, tc.want)
			}
		})
	}
}

func TestClassifyIdleShell(t *testing.T) {
	info := processInfo{
		PaneID:                 "w1:p1",
		ShellPID:               u32(100),
		ForegroundProcessGroup: u32(100),
		ForegroundProcesses:    []processEntry{{PID: 100, Name: "zsh", Argv0: "-zsh", Cmdline: "-zsh"}},
	}
	if v := classify(info, nil); v.Busy {
		t.Fatalf("idle zsh classified busy: %+v", v)
	}
}

func TestClassifyNestedShellsAreIdle(t *testing.T) {
	info := processInfo{
		ShellPID:               u32(100),
		ForegroundProcessGroup: u32(200),
		ForegroundProcesses: []processEntry{
			{PID: 200, Name: "bash", Argv0: "bash"},
			{PID: 201, Name: "fish", Argv0: "/opt/homebrew/bin/fish"},
		},
	}
	if v := classify(info, nil); v.Busy {
		t.Fatalf("nested shells classified busy: %+v", v)
	}
}

func TestClassifyAgentPicksGroupLeader(t *testing.T) {
	// Real shape captured from `herdr pane process-info` on a Claude pane: the
	// leader (claude) is listed last, after helper children.
	info := processInfo{
		ShellPID:               u32(55707),
		ForegroundProcessGroup: u32(93720),
		ForegroundProcesses: []processEntry{
			{PID: 9314, Name: "caffeinate", Argv0: "caffeinate", Cmdline: "caffeinate -i -t 300"},
			{PID: 94236, Name: "node", Argv0: "chrome-devtools-mcp"},
			{PID: 94216, Name: "node", Argv0: "npm exec chrome-devtools-mcp@latest"},
			{PID: 93720, Name: "2.1.233", Argv0: "claude", Cmdline: "claude --enable-auto-mode"},
		},
	}
	v := classify(info, nil)
	if !v.Busy {
		t.Fatal("claude pane classified idle")
	}
	if v.Process != "claude" {
		t.Fatalf("process = %q, want claude", v.Process)
	}
	if v.Cmdline != "claude --enable-auto-mode" {
		t.Fatalf("cmdline = %q", v.Cmdline)
	}
}

func TestClassifyFallsBackToFirstBusyProcess(t *testing.T) {
	// Leader is a shell wrapper (bash -c "webpack serve"); the child is what
	// the user cares about.
	info := processInfo{
		ShellPID:               u32(1),
		ForegroundProcessGroup: u32(50),
		ForegroundProcesses: []processEntry{
			{PID: 50, Name: "bash", Argv0: "bash", Cmdline: "bash -c webpack serve"},
			{PID: 51, Name: "node", Argv0: "webpack", Cmdline: "webpack serve --hot"},
		},
	}
	v := classify(info, nil)
	if !v.Busy || v.Process != "webpack" {
		t.Fatalf("got %+v, want busy webpack", v)
	}
}

func TestClassifyDirectProcessWithoutShell(t *testing.T) {
	// Pane launched straight into a program: shell_pid == leader pid, but the
	// process is not a shell so it must still prompt.
	info := processInfo{
		ShellPID:               u32(300),
		ForegroundProcessGroup: u32(300),
		ForegroundProcesses:    []processEntry{{PID: 300, Name: "vim", Argv0: "vim", Cmdline: "vim notes.md"}},
	}
	v := classify(info, nil)
	if !v.Busy || v.Process != "vim" {
		t.Fatalf("got %+v, want busy vim", v)
	}
}

func TestClassifyIgnoreList(t *testing.T) {
	info := processInfo{
		ShellPID:               u32(1),
		ForegroundProcessGroup: u32(2),
		ForegroundProcesses:    []processEntry{{PID: 2, Name: "less", Argv0: "less"}},
	}
	if v := classify(info, map[string]bool{"less": true}); v.Busy {
		t.Fatalf("ignored process classified busy: %+v", v)
	}
	if v := classify(info, nil); !v.Busy {
		t.Fatal("less without ignore should be busy")
	}
	// Ignore matching is case-insensitive on the display name.
	info.ForegroundProcesses[0].Argv0 = "/usr/bin/LESS"
	if v := classify(info, map[string]bool{"less": true}); v.Busy {
		t.Fatalf("case-insensitive ignore failed: %+v", v)
	}
}

func TestClassifyUnknownJobIsBusy(t *testing.T) {
	// herdr knows a foreground group exists but could not list its processes.
	info := processInfo{ShellPID: u32(1), ForegroundProcessGroup: u32(2)}
	v := classify(info, nil)
	if !v.Busy || v.Process != unknownProcess {
		t.Fatalf("got %+v, want busy unknown", v)
	}
}

func TestClassifyNoInfoIsIdle(t *testing.T) {
	// No processes and no group mismatch: nothing to protect.
	if v := classify(processInfo{ShellPID: u32(1), ForegroundProcessGroup: u32(1)}, nil); v.Busy {
		t.Fatalf("empty job with matching group classified busy: %+v", v)
	}
	if v := classify(processInfo{}, nil); v.Busy {
		t.Fatalf("empty info classified busy: %+v", v)
	}
}
