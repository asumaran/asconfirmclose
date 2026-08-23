# Demo recording

Scenario for re-recording the README demo GIF (`docs/demo.gif`) with
[herdr-demokit](https://github.com/asumaran/herdr-demokit):

```bash
herdr-demo record            # from the repo root; writes docs/demo.gif
herdr-demo doctor            # check the toolchain first
```

- `scenario.sh` — the isolated herdr session (`confirmclosedemo`): a
  workspace on a personal repo with a bottom split, focused on the split,
  plus a second workspace so that closing the last pane of the first lands
  somewhere instead of ending the session. `demo_build` stamps the binary
  with the manifest version; `demo_teardown` restores the dev build.
- `keys.json` — `prefix+x` on the idle split (closes at once) -> `vim
  README.md` in the remaining pane -> `prefix+x` (the "Close pane?" popup
  naming vim) -> `n` (kept) -> `prefix+x` -> `y` (closed; herdr moves to the
  second workspace).

Besides the kit's toolchain, this needs the `asumaran.confirm-close` plugin
registered and the close key handed to it in the user's herdr config
(`close_pane = []` plus the `prefix+x` `plugin_action` binding, as in the
README), and the repos listed in `scenario.sh` to exist.
