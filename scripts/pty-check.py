#!/usr/bin/env python3
"""End-to-end check of the confirmation popup without a real terminal.

Spawns `asconfirmclose prompt` on a pty the size of the popup, answers
the terminal queries bubbletea sends, replays a key and asserts on the frame
rendered with pyte and on what was asked of herdr. herdr is a logging stub
(HERDR_BIN_PATH), so no pane is ever closed and no server is contacted.

Usage: scripts/pty-check.py ./asconfirmclose   (needs python3 + pyte)
"""
NAME, ROWS, COLS = "asconfirmclose", 9, 64
import atexit, fcntl, json, os, pty, select, shutil, signal, struct, subprocess, sys, tempfile, termios, time
import pyte

BIN = os.path.abspath(sys.argv[1])
SANDBOX = os.path.realpath(tempfile.mkdtemp(prefix="%s-pty-" % NAME))
home = os.path.join(SANDBOX, "home")
os.makedirs(home)

def write(path, text, mode=None):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        f.write(text)
    if mode: os.chmod(path, mode)
    return path

QUERIES = [(b"\x1b]11;?", b"\x1b]11;rgb:0000/0000/0000\x1b\\"), (b"\x1b]10;?", b"\x1b]10;rgb:ffff/ffff/ffff\x1b\\"),
           (b"\x1b[6n", b"\x1b[1;1R"), (b"\x1b[c", b"\x1b[?62c")]

failures = []
def check(cond, msg):
    print(("  ok   " if cond else "  FAIL ") + msg)
    if not cond: failures.append(msg)

class Session:
    """One run of the binary on a pty."""
    def __init__(self, env, args=(), cwd=None):
        self.master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, 0, 0))
        self.proc = subprocess.Popen([BIN, *args], stdin=slave, stdout=slave, stderr=slave, env=env,
                                     close_fds=True, cwd=cwd or SANDBOX)
        os.close(slave)
        # A failed assertion must not leave the binary running on a dead pty.
        atexit.register(lambda p=self.proc: p.poll() is None and p.kill())
        self.screen = pyte.Screen(COLS, ROWS)
        self.stream = pyte.ByteStream(self.screen)
        self.raw = bytearray()
        self.answered = 0

    def pump(self, seconds):
        end = time.time() + seconds
        while True:
            left = end - time.time()
            if left <= 0: break
            r, _, _ = select.select([self.master], [], [], left)
            if not r: continue
            try:
                data = os.read(self.master, 65536)
            except OSError:
                break
            if not data: break
            self.raw.extend(data); self.stream.feed(data)
            tail = bytes(self.raw[self.answered:])
            for q, reply in QUERIES:
                for _ in range(tail.count(q)):
                    os.write(self.master, reply)
            self.answered = len(self.raw)

    def repaint(self):
        # The v2 renderer updates the screen with scroll regions and SU, which
        # pyte ignores; a resize forces a full redraw it can follow.
        for cols in (COLS - 1, COLS):
            fcntl.ioctl(self.master, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, cols, 0, 0))
            self.screen.resize(ROWS, cols)
            if self.proc.poll() is None: os.kill(self.proc.pid, signal.SIGWINCH)
            self.pump(0.3)

    def frame(self):
        return [line.rstrip() for line in self.screen.display]

    def send(self, b, wait=0.4):
        os.write(self.master, b); self.pump(wait); self.repaint()
        return self.frame()

    def start(self, marker):
        for _ in range(50):
            self.pump(0.1)
            if marker in "\n".join(self.frame()): break
        self.pump(0.5); self.repaint()
        return self.frame()

    def finish(self):
        try:
            self.proc.wait(timeout=3)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            return None
        self.pump(0.2)
        return self.proc.returncode

def dump(title, f):
    print("--- %s ---" % title)
    for i, l in enumerate(f): print("%2d|%s" % (i, l))

def done():
    shutil.rmtree(SANDBOX, ignore_errors=True)
    print("\n%d failure(s)" % len(failures))
    sys.exit(1 if failures else 0)

CTRL_A, CTRL_S, CTRL_T, ESC, ENTER, TAB, DOWN, UP = b"\x01", b"\x13", b"\x14", b"\x1b", b"\r", b"\t", b"\x1b[B", b"\x1b[A"

# ---------- sandbox: herdr stub ----------
calls_log = os.path.join(SANDBOX, "calls.log")
herdr = write(os.path.join(SANDBOX, "herdr"), """#!/bin/sh
printf '%%s\\n' "$*" >> "%s"
[ -n "${STUB_FAIL:-}" ] && { echo "pane not found" >&2; exit 1; }
printf '{}'
""" % calls_log, 0o755)

def session(process="vim", cmdline="vim notes.md", fail=False):
    env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", HOME=home, HERDR_BIN_PATH=herdr,
               ASCONFIRMCLOSE_PANE_ID="w1:p1", ASCONFIRMCLOSE_PROCESS=process, ASCONFIRMCLOSE_CMDLINE=cmdline)
    env.pop("HERDR_PLUGIN_CONFIG_DIR", None)
    env.pop("STUB_FAIL", None)
    if fail: env["STUB_FAIL"] = "1"
    if os.path.exists(calls_log): os.remove(calls_log)
    return Session(env, args=("prompt",))

def calls():
    return open(calls_log).read().splitlines() if os.path.exists(calls_log) else []

print("== asconfirmclose pty driver (%dx%d) ==" % (COLS, ROWS))

# ---------- run 1: n keeps the pane ----------
s = session()
f = s.start("Close pane?"); dump("prompt", f)
body = "\n".join(f)
check("Pane w1:p1 is running vim" in body and "vim notes.md" in body, "names the pane, the process and its command line")
check("y close" in body and "esc keep" in body, "shows the two decisions")
check(b"\x1b[?1049h" not in s.raw, "drawn inline: no alt screen")
os.write(s.master, b"n"); s.pump(0.4)
check(s.finish() == 0 and calls() == [], "n exits cleanly and closes nothing: %r" % calls())

# ---------- run 2: esc keeps the pane too ----------
s = session()
s.start("Close pane?")
os.write(s.master, ESC); s.pump(0.4)
check(s.finish() == 0 and calls() == [], "esc keeps the pane")

# ---------- run 3: y closes it through herdr ----------
s = session()
s.start("Close pane?")
os.write(s.master, b"y"); s.pump(0.6)
check(s.finish() == 0, "clean exit after y")
check(calls() == ["pane close w1:p1"], "y asks herdr to close the pane: %r" % calls())

# ---------- run 4: a failed close is shown and any key dismisses it ----------
s = session(fail=True)
s.start("Close pane?")
f = s.send(b"y", 0.6); dump("failed close", f)
check(s.proc.poll() is None and "Could not close the pane" in "\n".join(f), "the error stays on screen")
os.write(s.master, b"x"); s.pump(0.4)
check(s.finish() == 1, "exits non-zero after a failed close: %r" % s.proc.returncode)

# ---------- run 5: unknown process ----------
s = session(process="", cmdline="")
f = s.start("Close pane?")
check("an unknown process" in "\n".join(f), "an uninspectable pane is reported as busy")
os.write(s.master, b"n"); s.pump(0.3); s.finish()

done()
