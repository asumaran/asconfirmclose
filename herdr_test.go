package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDecodeResultSuccess(t *testing.T) {
	var res struct {
		ProcessInfo processInfo `json:"process_info"`
	}
	if err := decodeResult([]byte(busyInfo), &res); err != nil {
		t.Fatal(err)
	}
	if res.ProcessInfo.PaneID != "w1:p1" || res.ProcessInfo.ShellPID == nil || *res.ProcessInfo.ShellPID != 10 {
		t.Fatalf("unexpected decode: %+v", res.ProcessInfo)
	}
	if len(res.ProcessInfo.ForegroundProcesses) != 1 || res.ProcessInfo.ForegroundProcesses[0].Argv0 != "claude" {
		t.Fatalf("unexpected processes: %+v", res.ProcessInfo.ForegroundProcesses)
	}
}

func TestDecodeResultErrorEnvelope(t *testing.T) {
	var dst struct{}
	err := decodeResult([]byte(`{"error":{"code":"pane_not_found","message":"pane not found"},"id":"x"}`), &dst)
	var ce *cliError
	if !errors.As(err, &ce) || ce.Code != "pane_not_found" {
		t.Fatalf("err = %v, want cliError pane_not_found", err)
	}
	if ce.Error() != "pane_not_found: pane not found" {
		t.Fatalf("Error() = %q", ce.Error())
	}
}

func TestDecodeResultMalformed(t *testing.T) {
	var dst struct{}
	if err := decodeResult([]byte(`not json`), &dst); err == nil {
		t.Fatal("expected parse error")
	}
	if err := decodeResult([]byte(`{"id":"x"}`), &dst); err == nil {
		t.Fatal("expected missing-result error")
	}
}

func TestParseCLIError(t *testing.T) {
	if parseCLIError([]byte(`{"id":"x","result":{}}`)) != nil {
		t.Fatal("success envelope must not parse as error")
	}
	if parseCLIError([]byte(``)) != nil {
		t.Fatal("empty output must not parse as error")
	}
	if ce := parseCLIError([]byte(`{"error":{"code":"ui_busy"},"id":"x"}`)); ce == nil || ce.Code != "ui_busy" || ce.Error() != "ui_busy" {
		t.Fatalf("got %+v", ce)
	}
}

// writeFakeHerdr creates a shell script that logs its argv and replies with
// canned JSON depending on the subcommand, mimicking the herdr CLI contract
// (JSON on stdout, exit 1 on error envelopes).
func writeFakeHerdr(t *testing.T, dir string) (bin, log string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake herdr script requires a POSIX shell")
	}
	log = filepath.Join(dir, "argv.log")
	bin = filepath.Join(dir, "herdr")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + log + `"
case "$1 $2" in
  "pane process-info")
    if [ "$4" = "w1:busy" ]; then
      printf '%s' '` + busyInfo + `'
    elif [ "$4" = "w1:idle" ]; then
      printf '%s' '` + idleInfo + `'
    else
      printf '%s' '{"error":{"code":"pane_not_found","message":"pane not found"},"id":"x"}'
      exit 1
    fi ;;
  "pane close")
    printf '%s' '{"id":"x","result":{"type":"ok"}}' ;;
  "plugin pane")
    printf '%s' '{"id":"x","result":{"type":"ok"}}' ;;
  *)
    echo "unexpected: $*" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, log
}

func TestExecRunnerParsesJSONAndErrors(t *testing.T) {
	bin, _ := writeFakeHerdr(t, t.TempDir())
	r := execRunner{bin: bin}

	info, err := fetchProcessInfo(r, "w1:busy")
	if err != nil {
		t.Fatal(err)
	}
	if v := classify(info, nil); !v.Busy || v.Process != "claude" {
		t.Fatalf("got %+v", v)
	}

	_, err = fetchProcessInfo(r, "w1:missing")
	if !isPaneNotFound(err) {
		t.Fatalf("err = %v, want pane_not_found", err)
	}

	if err := closePane(r, "w1:idle"); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunnerSurfacesSpawnFailure(t *testing.T) {
	r := execRunner{bin: filepath.Join(t.TempDir(), "does-not-exist")}
	if _, err := r.run("pane", "current"); err == nil {
		t.Fatal("expected spawn error")
	}
}
