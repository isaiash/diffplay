#!/usr/bin/env python3
"""Real-PTY coverage of a smoothly fading one-second list hit."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

from list_smoke import ANSI, Terminal, assert_terminal_restored, mode, sgr_colors, sgr_inverse


def background_at(raw, column, row):
    fg = bg = None
    inverse = False
    for y, line in enumerate(raw.split("\r\n")):
        x = offset = 0
        for match in ANSI.finditer(line):
            text = line[offset:match.start()]
            if y == row and x <= column < x + len(text):
                return fg if inverse else bg
            x += len(text)
            fg, bg = sgr_colors(match.group(), fg, bg)
            inverse = sgr_inverse(match.group(), inverse)
            offset = match.end()
        if y == row and x <= column < x + len(line[offset:]):
            return fg if inverse else bg
    raise AssertionError(f"missing cell ({column}, {row})")


assert background_at("\x1b[48;2;12;34;56mA\r\nB", 0, 1) == (12, 34, 56)
assert background_at("\x1b[100mA\r\n\x1b[49mB", 0, 1) is None
assert background_at("\x1b[31;7mA", 0, 0) == 1


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as temp:
        root = Path(temp)
        target = root / "target.txt"
        target.write_text("baseline\n")
        subprocess.check_call(["git", "-C", str(root), "init", "-q"])
        subprocess.check_call(["git", "-C", str(root), "add", "."])
        subprocess.check_call(["git", "-C", str(root), "-c", "user.name=Burst Test", "-c", "user.email=burst@example.invalid", "commit", "-qm", "baseline"])
        target.write_text("startup\n")
        for view in ("list", "combined"):
            terminal = Terminal(binary, root, view)
            terminal.resize(160, 36)
            try:
                terminal.frame(lambda raw, rows: mode(rows, view) and "target.txt" in raw)
                # Simulate terminal theme replies; these are input, never source edits.
                terminal.send(b"\x1b]11;rgb:2020/2020/2020\x1b\\\x1b]4;8;rgb:4040/4040/4040\x1b\\")
                raw, rows = terminal.frame(lambda raw, rows: mode(rows, view) and "target.txt" in raw and "✦" not in raw)
                y = next(y for y, row in enumerate(rows) if "target.txt" in row[:79])
                resting = background_at(raw, 0, y)
                target.write_text(f"{view} live hit\n")
                raw, rows = terminal.frame(lambda raw, rows: "✦" in raw)
                started = time.monotonic()
                colors = [background_at(raw, 0, y)]
                while time.monotonic() - started < .8:
                    raw, rows = terminal.frame()
                    colors.append(background_at(raw, 0, y))
                assert len(set(colors)) >= 5, f"{view}: background held instead of fading: {colors!r}"
                raw, rows = terminal.frame(lambda raw, rows: "✦" not in raw and background_at(raw, 0, y) == resting, timeout=.6)
                assert time.monotonic() - started < 1.5, "burst outlasted one second plus scheduling tolerance"
                assert "target.txt" in raw, "fade must not remove activity"
                # A truncated theme reply must never swallow either exit key.
                exit_key = b"q" if view == "list" else b"\x03"
                terminal.send(b"\x1b]11;rgb:0000/0000/0000" + exit_key)
                terminal.proc.wait(timeout=5)
                assert terminal.proc.returncode == 0, terminal.tail[-2000:]
                assert_terminal_restored(terminal.slave, terminal.before)
                print(f"PASS [{view}]: hit background uses {len(set(colors))} RGB steps and returns to selection within one second")
                print(f"PASS [{view}]: truncated OSC reply preserves exit and terminal restoration")
            finally:
                terminal.close()


if __name__ == "__main__":
    main()
