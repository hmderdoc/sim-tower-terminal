#!/usr/bin/env python3
"""Headless smoke run of the SimTower door on a pseudo-terminal.

Builds a lobby, a floor, an office and an elevator shaft with scripted mouse
and keyboard input, lets the real sim run, opens the finance window and
quits. Fails on a Go panic, on bytes the terminal would act on outside
escape sequences (NUL, BEL, BS, TAB, LF, CR, DEL, 0xFF), or when no save
was written.

    go build -o /tmp/simtower ./cmd/simtower
    python3 tools/pty_smoke.py /tmp/simtower [workdir]

The door is started as: <binary> -size 80x24 -user adapter-test -data <workdir>
At 80x24 the start view puts 1F on screen rows 17-19 (1-based) and 2F on
rows 14-16; the viewport's unit 0 is screen column 5.
"""
import os
import pty
import re
import select
import sys
import tempfile
import time

UNSAFE = {0x00, 0x07, 0x08, 0x09, 0x0A, 0x0D, 0x7F, 0xFF}


def sgr(button, x, y, release=False):
    return "\x1b[<%d;%d;%d%s" % (button, x, y, "m" if release else "M")


def drag(x0, y0, x1, y1):
    seq = sgr(0, x0, y0)
    steps = max(abs(x1 - x0), abs(y1 - y0))
    for i in range(1, steps + 1):
        x = x0 + (x1 - x0) * i // steps
        y = y0 + (y1 - y0) * i // steps
        seq += sgr(32, x, y)
    return seq + sgr(0, x1, y1, release=True)


def click(x, y):
    return sgr(0, x, y) + sgr(0, x, y, release=True)


SCRIPT = [
    (2.0, "n"),                         # start menu: new tower (no save yet)
    (0.6, "bl"),                        # build palette -> lobby
    (0.4, drag(8, 18, 70, 18)),         # lay a 63-unit lobby on 1F
    (0.6, "bw"),                        # floor tool
    (0.4, drag(8, 15, 70, 15)),         # 2F floor
    (0.6, "bo"),                        # office
    (0.4, click(10, 15)),
    (0.4, click(19, 15)),
    (0.6, "be"),                        # standard elevator
    (0.4, drag(40, 18, 40, 15)),        # shaft 1F..2F
    (0.6, "\x1b[<2;40;10M\x1b[<2;40;10m"),  # right click: disarm the tool
    (12.0, ""),                         # let the sim run (~240 ticks)
    (0.5, "f"),                         # finance window
    (1.0, "f"),                         # close it
    (0.5, "q"),                         # quit (autosaves)
]


def main():
    binary = sys.argv[1]
    work = sys.argv[2] if len(sys.argv) > 2 else tempfile.mkdtemp(prefix="simtower-smoke-")
    os.makedirs(work, exist_ok=True)
    log = os.path.join(work, "simtower.log")
    argv = [binary, "-size", "80x24", "-user", "adapter-test", "-data", work, "-log", log]
    pid, fd = pty.fork()
    if pid == 0:
        os.environ["TERM"] = "xterm"
        os.execv(binary, argv)
    out = bytearray()

    def pump(secs):
        end = time.time() + secs
        while time.time() < end:
            r, _, _ = select.select([fd], [], [], 0.05)
            if fd in r:
                try:
                    data = os.read(fd, 65536)
                except OSError:
                    return False
                if not data:
                    return False
                out.extend(data)
        return True

    for delay, keys in SCRIPT:
        if not pump(delay):
            break
        if keys:
            os.write(fd, keys.encode("latin-1"))
    pump(2.0)
    try:
        _, status = os.waitpid(pid, 0)
    except ChildProcessError:
        status = 0
    text = out.decode("latin-1")
    problems = []
    if "panic:" in text or "goroutine " in text:
        problems.append("panic in the output")
    if os.path.exists(log) and "panic" in open(log, errors="replace").read():
        problems.append("panic in the log")
    # strip escape sequences, then look for control bytes
    stripped = re.sub(r"\x1b\[[0-9;?<>=]*[ -/]*[@-~]|\x1b[78=>]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)", "", text)
    bad = [(i, ord(c)) for i, c in enumerate(stripped) if ord(c) in UNSAFE]
    if bad:
        problems.append("unsafe bytes outside escapes: %s" % bad[:10])
    save = os.path.join(work, "adapter-test.json")
    if not os.path.exists(save) or os.path.getsize(save) < 1000:
        problems.append("no save written at %s" % save)
    print("exit status %d, %d bytes of output, save %s (%d bytes)" % (
        status, len(out), save, os.path.getsize(save) if os.path.exists(save) else 0))
    if problems:
        print("FAIL: " + "; ".join(problems))
        sys.exit(1)
    print("OK")


if __name__ == "__main__":
    main()
