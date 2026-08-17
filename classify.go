package main

import (
	"path"
	"strings"
)

// shellNames are the interactive shells a pane idles in. A pane whose
// foreground job consists only of these is considered idle and closes without
// a prompt. Nested shells (bash inside zsh) therefore never prompt either.
var shellNames = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true,
	"ksh": true, "mksh": true, "tcsh": true, "csh": true, "nu": true,
	"nushell": true, "elvish": true, "xonsh": true, "pwsh": true,
	"powershell": true, "ash": true, "login": true,
}

// verdict is the outcome of inspecting a pane's foreground job.
type verdict struct {
	// Busy is true when something other than an idle shell owns the pane.
	Busy bool
	// Process is the short name shown to the user (e.g. "claude", "webpack").
	// Empty when the pane is idle; "unknown process" when herdr could not
	// describe the job.
	Process string
	// Cmdline is the full command line of that process when available.
	Cmdline string
}

// processDisplayName picks the name a user would recognize for a process:
// the argv0 basename when present, else the kernel-reported name. Leading
// dashes from login shells ("-zsh") are stripped.
func processDisplayName(p processEntry) string {
	name := p.Argv0
	if name == "" && len(p.Argv) > 0 {
		name = p.Argv[0]
	}
	if name == "" {
		name = p.Name
	}
	// argv0 may carry a whole command line ("npm exec foo"); keep the first
	// token so the basename is meaningful.
	if i := strings.IndexByte(name, ' '); i > 0 {
		name = name[:i]
	}
	name = path.Base(name)
	name = strings.TrimLeft(name, "-")
	if name == "" || name == "." || name == "/" {
		name = strings.TrimLeft(p.Name, "-")
	}
	return name
}

// isShell reports whether a display name is a known interactive shell or is
// listed in the user's ignore set (compared case-insensitively).
func isShell(name string, ignore map[string]bool) bool {
	lower := strings.ToLower(name)
	return shellNames[lower] || ignore[lower]
}

// classify decides whether closing the pane deserves a confirmation.
//
// Rules:
//   - a foreground job that contains any process that is not a shell (and not
//     in the ignore list) makes the pane busy;
//   - the process reported to the user is the foreground group leader when it
//     is busy, otherwise the first busy process in the job;
//   - an empty job whose group id differs from the shell pid is treated as busy
//     with an unknown process, since herdr could not describe it;
//   - anything else is idle.
func classify(info processInfo, ignore map[string]bool) verdict {
	var leader *processEntry
	var first *processEntry
	for i := range info.ForegroundProcesses {
		p := &info.ForegroundProcesses[i]
		if isShell(processDisplayName(*p), ignore) {
			continue
		}
		if first == nil {
			first = p
		}
		if info.ForegroundProcessGroup != nil && p.PID == *info.ForegroundProcessGroup {
			leader = p
			break
		}
	}
	pick := leader
	if pick == nil {
		pick = first
	}
	if pick != nil {
		return verdict{
			Busy:    true,
			Process: processDisplayName(*pick),
			Cmdline: strings.TrimSpace(pick.Cmdline),
		}
	}
	if len(info.ForegroundProcesses) == 0 &&
		info.ForegroundProcessGroup != nil && info.ShellPID != nil &&
		*info.ForegroundProcessGroup != *info.ShellPID {
		return verdict{Busy: true, Process: unknownProcess}
	}
	return verdict{}
}

const unknownProcess = "unknown process"
