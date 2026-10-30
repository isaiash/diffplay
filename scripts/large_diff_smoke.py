#!/usr/bin/env python3
"""Large Git previews must page beyond the old alignment/capture/output limits."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from list_smoke import Terminal, assert_terminal_restored, divider, mode


def git(root, *args):
    subprocess.check_call(["git", "-C", str(root), *args])


def select_file(terminal, rows, name):
    bottom = divider(rows) or len(rows)-1
    width = rows[1].rfind("│")
    y = next(y for y, row in enumerate(rows[2:bottom], 2) if name in row[:width])
    terminal.send(f"\x1b[<35;30;{y + 1}M".encode())


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as temp:
        root = Path(temp)
        count = 12000
        padding = "x" * 40
        old = [f"old-{i:05d} {padding}\n" for i in range(count)]
        sparse = [f"keep-{i:05d} {padding}\n" for i in range(count)]
        (root / "paged.txt").write_text("".join(old))
        (root / "sparse.java").write_text("".join(sparse))
        git(root, "init", "-q")
        git(root, "add", ".")
        git(root, "-c", "user.name=Large Diff Test", "-c", "user.email=large@example.invalid", "commit", "-qm", "baseline")
        (root / "paged.txt").write_text("".join(f"new-{i:05d} {padding}\n" for i in range(count)))
        sparse[-1] = "changed-after-capture-limit\n"
        (root / "sparse.java").write_text("".join(sparse))
        terminal = Terminal(binary, root, "combined")
        terminal.resize(200, 48)
        try:
            raw, rows = terminal.frame(lambda raw, rows: mode(rows, "combined") and "paged.txt" in raw and "sparse.java" in raw, timeout=20)
            select_file(terminal, rows, "paged.txt")
            raw, rows = terminal.frame(lambda raw, rows: "-old-00000" in raw, timeout=20)
            assert "Diff omitted" not in raw
            print("PASS: large modified file opens without bounded-diff omission")
            terminal.send(b"J" * (count + 8))
            raw, rows = terminal.frame(lambda raw, rows: "+new-000" in raw, timeout=20)
            print("PASS: scrolling loads added lines beyond the old 64 KiB diff limit")
            terminal.send(b"J" * (count + 20))
            raw, rows = terminal.frame(lambda raw, rows: "+new-11999" in raw, timeout=20)
            terminal.send(b"K" * (2 * count + 40))
            raw, rows = terminal.frame(lambda raw, rows: "-old-00000" in raw, timeout=20)
            select_file(terminal, rows, "sparse.java")
            raw, rows = terminal.frame(lambda raw, rows: "+changed-after-capture-limit" in raw, timeout=20)
            assert "Diff omitted" not in raw
            print("PASS: tail, reverse scroll and a sparse edit beyond captured source prefix remain accessible")
            with (root / "sparse.java").open("a") as output:
                output.write("live-large-preview-invalidation\n")
            terminal.frame(lambda raw, rows: "+live-large-preview-invalidation" in raw, timeout=20)
            terminal.resize(160, 36)
            terminal.frame(lambda raw, rows: len(rows) == 36 and all(len(row) == 160 for row in rows) and "+live-large-preview-invalidation" in raw)
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0
            assert_terminal_restored(terminal.slave, terminal.before)
            print("PASS: live invalidation, resize and clean terminal restoration with paged previews")
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
