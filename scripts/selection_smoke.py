#!/usr/bin/env python3
"""Real PTY checks for diff text, resize handles and native terminal selection."""
import os
from pathlib import Path
import select
import subprocess
import sys
import tempfile
import time

from list_smoke import Terminal, assert_terminal_restored, divider, mode


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
            raw, rows = terminal.frame(lambda raw, rows: mode(rows, "combined") and "file.txt" in rows[2])
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

            assert "Shift+drag" in rows[-1], "native selection must not need an application mode"
            terminal.send(b"m")  # The old toggle must no longer freeze the display.
            target.write_bytes("\t日本語 e\u0301\r\n\twhile copied\r\n".encode())
            terminal.frame(lambda raw, rows: "+       while copied" in raw and mode(rows, "combined"))
            terminal.frame(lambda raw, rows: "+       while copied" in raw and "✦" not in raw)
            # Drain frames already queued by the final animation tick.
            deadline = time.monotonic() + .2
            while time.monotonic() < deadline:
                if select.select([terminal.master], [], [], .05)[0]:
                    terminal.decoder.decode(os.read(terminal.master, 262144))
            terminal.buffer = ""
            held = ""
            deadline = time.monotonic() + .5
            while time.monotonic() < deadline:
                if select.select([terminal.master], [], [], .05)[0]:
                    held += terminal.decoder.decode(os.read(terminal.master, 262144))
            assert "\x1b[H" not in held, "idle redraws must not erase native terminal selection"
            assert "\x1b[?1003l" not in terminal.tail and "\x1b[?1006l" not in terminal.tail
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0
            assert_terminal_restored(terminal.slave, terminal.before)
            print("PASS: Unicode/tab/CRLF diff, Shift+drag guidance, no m mode, no idle redraws, live monitoring and terminal restoration", flush=True)
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
