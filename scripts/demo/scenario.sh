# shellcheck shell=bash
# scenario.sh — demo session for the README GIF, run by `herdr-demo record`
# (asumaran/herdr-demokit). Sourced by the kit; the helpers used below
# (demo_*) come from it.
#
# Layout: one workspace on a personal repo with a bottom split, focused on
# the split. keys.json then closes the idle split instantly, opens a file in
# vim in the remaining pane, and shows the confirmation popup declining once
# and confirming once. A second workspace exists so closing the last pane
# lands somewhere rather than ending the session.

DEMO_SESSION="confirmclosedemo"
DEMO_OUT="docs/demo.gif"
DEMO_START_CWD="$HOME/Developer/aspage"

SECOND_REPO="$HOME/Developer/asdev"

# Build ./herdr-confirm-close stamped with the manifest version; the plugin
# runs the binary from this checkout. demo_teardown restores the dev build.
demo_build() {
  local version
  version="$(sed -n 's/^version = "\(.*\)"/\1/p' herdr-plugin.toml)"
  go build -ldflags "-X main.version=v${version}" -o herdr-confirm-close .
}

demo_teardown() {
  go build -o herdr-confirm-close . 2>/dev/null || true
}

demo_setup() {
  demo_adopt_repo "$(demo_first_workspace)" "$DEMO_START_CWD"
  demo_open_repo "$SECOND_REPO" >/dev/null
  demo_split_below "$DEMO_START_CWD" 0.6 focus >/dev/null
}
