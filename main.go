// asconfirmclose is a herdr plugin that closes the focused pane
// immediately when it only holds an idle shell, and asks for confirmation
// when a process (an agent, a dev server, an editor…) is running in it.
//
// Two subcommands, both launched by herdr from the plugin manifest:
//
//	close   the keybound action: inspect the pane, close it or open the popup
//	prompt  the popup UI: y closes the pane, anything else keeps it
package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "dev"

func usage() {
	fmt.Fprintf(os.Stderr, `usage: asconfirmclose <close|prompt> [flags]

  close   inspect the focused pane (HERDR_PANE_ID) and close it, or open the
          confirmation popup when a process is running in it
  prompt  the confirmation popup (run by herdr; reads %s, %s, %s)

flags:
  -version   print the version and exit
`, envPaneID, envProcess, envCmdline)
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("asconfirmclose", flag.ContinueOnError)
	fs.Usage = usage
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Println("asconfirmclose " + version)
		return 0
	}
	if fs.NArg() != 1 {
		usage()
		return 2
	}

	cfg, err := loadConfig(os.Getenv("HERDR_PLUGIN_CONFIG_DIR"))
	if err != nil {
		// Keep working with defaults; a broken config must not disable the
		// safety net, but the user deserves to know via the plugin log.
		fmt.Fprintf(os.Stderr, "config: %v (using defaults)\n", err)
	}
	r := execRunner{bin: herdrBin()}

	switch fs.Arg(0) {
	case "close":
		if _, err := runClose(r, cfg, os.Getenv("HERDR_PANE_ID"), os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		return 0
	case "prompt":
		paneID := os.Getenv(envPaneID)
		if paneID == "" {
			fmt.Fprintf(os.Stderr, "error: %s is not set; run through the close action\n", envPaneID)
			return 2
		}
		process := os.Getenv(envProcess)
		if process == "" {
			process = unknownProcess
		}
		_, err := runPrompt(paneID, process, os.Getenv(envCmdline), func() error {
			return closePane(r, paneID)
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		return 0
	default:
		usage()
		return 2
	}
}
