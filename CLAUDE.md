# CLAUDE.md

Guidance for working in this repository.

## What this is

`herdr-confirm-close` is a herdr plugin that replaces the close-pane key. The
keybound action inspects the focused pane's foreground job through
`herdr pane process-info`; if only shells are running it closes the pane at
once, otherwise it opens a popup naming the process and closes only on `y`.
It exists because herdr's `ui.confirm_close` covers workspaces, not panes.

Distributed as a herdr plugin (`herdr plugin install asumaran/herdr-confirm-close`;
the manifest's `[[build]]` runs `scripts/fetch-binary.sh`). Each GitHub
Release attaches static binaries for darwin/linux × arm64/amd64. There is no
published library.

## Stack & layout

Go single module, single `package main`, static binary, no CGO. TUI: Bubble
Tea + lipgloss (fixed ANSI colors, no adaptive colors). Files by concern:

- `main.go` — subcommands `close` and `prompt`, `-version`, env wiring.
- `herdr.go` — `runner` interface, `execRunner` over `HERDR_BIN_PATH`, JSON
  envelope decoding (`{"result":..}` / `{"error":{code,message}}`),
  `fetchProcessInfo`, `currentPaneID`, `closePane`.
- `classify.go` — shell list, `processDisplayName`, `classify` (busy/idle
  verdict and which process to show).
- `close.go` — `runClose`: the action's decision flow and the exact
  `plugin pane open` argv (popup placement, size, `HCC_*` env).
- `config.go` — optional `config.json` in `HERDR_PLUGIN_CONFIG_DIR`
  (`ignore` list, popup size), strict decoding, defaults on error.
- `ui.go` — `promptModel` (Bubble Tea), `runPrompt`.
- `scripts/fetch-binary.sh` — `[[build]]`: download release asset or
  `go build`. `scripts/release.sh` — tag + GitHub release.
- `scripts/demo/` — the demo scenario (`scenario.sh` + `keys.json`) that
  `herdr-demo record` (asumaran/herdr-demokit, the recording tool shared by
  the herdr plugins) uses to re-record `docs/demo.gif`; see
  `scripts/demo/README.md`. Uses a disposable herdr session
  (`confirmclosedemo`), never the user's default session.
- `.github/workflows/ci.yml` (gofmt/vet/test on push and PR) and
  `release.yml` (cross-compile and upload assets on release publish).

## Build & run

```bash
go build -o herdr-confirm-close .   # manifest runs ./herdr-confirm-close from the repo root
go vet ./... && go test ./...
herdr plugin link ~/Developer/herdr-confirm-close   # link does NOT run [[build]]
HERDR_PANE_ID=<pane> ./herdr-confirm-close close    # drive the action by hand
```

Keybinding (user config): `prefix+x` / `ctrl+alt+x` → `plugin_action`
`asumaran.confirm-close.close`, with `close_pane = []`.

## Behaviour / decisions

- **Busy rule** lives in `classify`: any foreground process whose display
  name (argv0 basename, login-shell dash stripped, first token of a
  multi-word argv0) is not a known shell and not in the ignore list makes the
  pane busy. Reported process: the group leader when busy, else the first busy
  process. Empty job with group id ≠ shell pid → busy "unknown process".
- **Fail safe**: `process-info` errors other than `pane_not_found` prompt
  instead of closing. A broken `config.json` logs a warning and uses defaults.
- **Popup contract**: the action opens
  `plugin pane open --plugin asumaran.confirm-close --entrypoint confirm --placement popup --width W --height H --env HCC_PANE_ID=.. --env HCC_PROCESS=.. [--env HCC_CMDLINE=..]`.
  Popups are session-modal, have no pane id and receive Escape; the popup
  process reads only the `HCC_*` env. Keep the argv stable; tests assert it.
- **Keys**: only `y`/`Y` close. Everything else (n, Esc, Enter, q, ctrl+c…)
  keeps the pane. Input is ignored while the close request is in flight; a
  failed close shows the error and any key dismisses it.
- **Terminal queries**: Bubble Tea's `init` triggers lipgloss's OSC 11
  background query before the program starts; do not add adaptive colors or
  other terminal queries at runtime (they race the input reader).
- `HERDR_PANE_ID` is set by herdr for `contexts = ["pane"]` actions to the
  focused pane; `pane current` is only a fallback and, when run from inside a
  pane, returns the calling pane rather than the UI focus.

## Testing

`go test ./...` needs no running herdr. Coverage: `classify_test.go` (rules,
real Claude process shape), `config_test.go`, `close_test.go` (decision flow
over a scripted `fakeRunner`, exact popup argv), `herdr_test.go` (envelope
parsing, `execRunner` against a POSIX fake `herdr` script), `ui_test.go`
(model transitions, views), `main_test.go` (builds the binary and runs the
`close` action end to end against the fake `herdr`).

For a manual check against real herdr, split a throwaway pane, `herdr pane run
<pane> "sleep 300"`, then `HERDR_PANE_ID=<pane> ./herdr-confirm-close close`;
the popup process can be dismissed with `pkill -f 'herdr-confirm-close prompt'`.
Never point a manual run at a pane you care about: the idle path really closes
it.

## Commits & branches

- Conventional Commits: `type(scope): description`.
- Never mention AI tooling in commits, PRs, or any repo-visible text.
- Default branch is `main`. Don't commit, tag, or push unless explicitly
  asked (releasing is an explicit, separate request).

## Releasing

`scripts/release.sh <X.Y.Z>` — clean-tree + vet/build/test gate, CHANGELOG
generation from commit subjects, manifest version sync, commit + tag + GitHub
release; CI (`.github/workflows/release.yml`) attaches the platform binaries.
Releasing never touches the linked plugin's `./herdr-confirm-close`; rebuild
locally to keep testing dev code.
