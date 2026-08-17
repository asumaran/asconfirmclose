package main

import (
	"errors"
	"fmt"
	"io"
)

const (
	pluginID         = "asumaran.confirm-close"
	promptEntrypoint = "confirm"

	// Environment handed to the popup process so it knows what to close.
	envPaneID  = "HCC_PANE_ID"
	envProcess = "HCC_PROCESS"
	envCmdline = "HCC_CMDLINE"
)

// closeOutcome describes what runClose did, for logging and tests.
type closeOutcome int

const (
	outcomeClosed closeOutcome = iota
	outcomePrompted
)

// runClose implements the "close" action: inspect the focused pane and either
// close it right away (idle shell) or open the confirmation popup (busy).
//
// paneID may be empty, in which case the focused pane is looked up. When herdr
// cannot describe the pane's processes the pane is treated as busy so an
// accidental keypress never destroys work silently.
func runClose(r runner, cfg config, paneID string, log io.Writer) (closeOutcome, error) {
	if paneID == "" {
		id, err := currentPaneID(r)
		if err != nil {
			return 0, fmt.Errorf("resolve focused pane: %w", err)
		}
		paneID = id
	}

	var v verdict
	info, err := fetchProcessInfo(r, paneID)
	switch {
	case err == nil:
		v = classify(info, cfg.ignoreSet())
	case isPaneNotFound(err):
		return 0, err
	default:
		fmt.Fprintf(log, "process-info failed (%v); asking before closing\n", err)
		v = verdict{Busy: true, Process: unknownProcess}
	}

	if !v.Busy {
		if err := closePane(r, paneID); err != nil {
			return 0, fmt.Errorf("close pane %s: %w", paneID, err)
		}
		fmt.Fprintf(log, "closed idle pane %s\n", paneID)
		return outcomeClosed, nil
	}

	args := []string{
		"plugin", "pane", "open",
		"--plugin", pluginID,
		"--entrypoint", promptEntrypoint,
		"--placement", "popup",
		"--width", cfg.Popup.Width,
		"--height", cfg.Popup.Height,
		"--env", envPaneID + "=" + paneID,
		"--env", envProcess + "=" + v.Process,
	}
	if v.Cmdline != "" {
		args = append(args, "--env", envCmdline+"="+v.Cmdline)
	}
	if _, err := r.run(args...); err != nil {
		return 0, fmt.Errorf("open confirmation popup: %w", err)
	}
	fmt.Fprintf(log, "pane %s is running %s; opened confirmation\n", paneID, v.Process)
	return outcomePrompted, nil
}

func isPaneNotFound(err error) bool {
	var ce *cliError
	return errors.As(err, &ce) && ce.Code == "pane_not_found"
}
