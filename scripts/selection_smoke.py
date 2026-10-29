#!/usr/bin/env python3
"""Real PTY checks for diff text, resize handles and native terminal selection."""
import os
from pathlib import Path
import select
import subprocess
import sys
import tempfile
import termios
import time

from list_smoke import Terminal, divider, mode


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as temp:
        root = Path(temp)
        target = root / "file.txt"
        target.write_bytes(b"\told\r\n")
        subprocess.check_call(["git", "-C", str(root), "init", "-q"])
        subprocess.check_call(["git", "-C", str(root), "add", "."])
        subprocess.check_call(["git", "-C", str(root), "-c", "user.name=Selection Test", "-c", "user.email=selection@example.invalid", "commit", "-qm", "baseline"])
        target.write_bytes("\t日本語 e\u0301\r\n".encode())
        terminal = Terminal(binary, root, "combined")
        terminal.resize(160, 36)
        try:
            raw, rows = terminal.frame(lambda raw, rows: mode(rows, "combined") and "file.txt" in rows[1])
            terminal.send(b"\x1b[B")
            raw, rows = terminal.frame(lambda raw, rows: "+       日本語 e\u0301" in raw)
            assert "�" not in raw, "valid tabs, CRLF and combining accents must not become replacement characters"
            x = rows[1].rfind("│")
            terminal.send(f"\x1b[<35;{x + 1};5M".encode())
            terminal.frame(lambda raw, rows: rows[4][x] == "↔")
            terminal.send(f"\x1b[<0;{x + 1};5M\x1b[<32;61;30M\x1b[<0;61;30m".encode())
            raw, rows = terminal.frame(lambda raw, rows: rows[1][60] == "│")
            top = divider(rows)
            assert top is not None, "horizontal divider missing"
            terminal.send(f"\x1b[<35;8;{top + 1}M".encode())
            terminal.frame(lambda raw, rows: rows[top][7] == "↕")
            terminal.send(f"\x1b[<0;8;{top + 1}M\x1b[<32;8;13M\x1b[<0;8;13m".encode())
            terminal.frame(lambda raw, rows: divider(rows) == 12)
            terminal.resize(200, 48)
            terminal.frame(lambda raw, rows: len(rows) == 48 and divider(rows) == 16 and rows[1][75] == "│")
            print("PASS: both resize handles hover, drag across panes, release and retain proportions", flush=True)

            terminal.send(b"m")
            terminal.frame(lambda raw, rows: "Select with mouse" in rows[-1])
            assert "\x1b[?1003l" in terminal.tail and "\x1b[?1006l" in terminal.tail
            target.write_bytes("\t日本語 e\u0301\r\n\twhile copied\r\n".encode())
            held = terminal.buffer
            terminal.buffer = ""
            deadline = time.monotonic() + .5
            while time.monotonic() < deadline:
                if select.select([terminal.master], [], [], .05)[0]:
                    held += terminal.decoder.decode(os.read(terminal.master, 262144))
            assert "\x1b[H" not in held, "selection mode must not redraw over the native selection"
            terminal.send(b"m")
            terminal.frame(lambda raw, rows: "+       while copied" in raw and mode(rows, "combined"))
            assert terminal.tail.rfind("\x1b[?1003h") > terminal.tail.rfind("\x1b[?1003l")
            assert terminal.tail.rfind("\x1b[?1006h") > terminal.tail.rfind("\x1b[?1006l")
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0
            assert termios.tcgetattr(terminal.slave) == terminal.before
            print("PASS: Unicode/tab/CRLF diff, native selection releases mouse, drawing freezes, monitoring resumes and terminal restores", flush=True)
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
