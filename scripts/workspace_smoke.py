#!/usr/bin/env python3
"""Exercise discovery, single/multi selection, and live grouped monitoring."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import termios

from list_smoke import Terminal


def make_repo(root, count):
    root.mkdir(parents=True)
    subprocess.check_call(["git", "-C", str(root), "init", "-q"])
    (root / "same.txt").write_text("new\n" * count)
    (root / ".gitignore").write_text("*.cache\n")
    (root / "hidden.cache").write_text("ignored\n")


def bars(row):
    return row.count("█")


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as temp:
        workspace = Path(temp)
        empty = workspace / "empty"
        empty.mkdir()
        make_repo(empty / ".cache/hidden-repository", 50)
        result = subprocess.run([binary], cwd=empty, text=True, capture_output=True)
        assert result.returncode != 0 and "open the application inside a Git repository" in result.stderr
        print("PASS: a non-Git directory without repositories returns English guidance")
        make_repo(workspace / "alpha", 100)
        make_repo(workspace / "nested/beta", 10)
        terminal = Terminal(binary, workspace)
        try:
            raw, rows = terminal.frame(lambda raw, rows: "SELECT REPOSITORIES" in raw and "alpha" in raw and "nested/beta" in raw)
            assert "2 repositories found" in raw and "hidden-repository" not in raw
            terminal.send(b"\t\x1b[B\t\r")
            raw, rows = terminal.frame(lambda raw, rows: "CHANGES" in raw and "alpha/" in raw and "nested/beta/" in raw and "+112" in rows[-2])
            assert "✦" not in raw
            assert "hidden.cache" not in raw
            changed_rows = [row for row in rows[2:-2] if "same.txt" in row]
            assert len(changed_rows) == 2 and all(bars(row) == 4 for row in changed_rows), changed_rows
            print("PASS: discovery and multi-selection show repository groups and independent histogram scales")
            (workspace / "alpha/same.txt").write_text("new\n" * 101)
            raw, rows = terminal.frame(lambda raw, rows: "✦" in raw and "+113" in rows[-2])
            same_rows = [row for row in rows[2:-2] if "same.txt" in row]
            assert len(same_rows) == 2 and sum("✦" in row for row in same_rows) == 1, same_rows
            (workspace / "nested/beta/second.txt").write_text("more\n" * 3)
            terminal.frame(lambda raw, rows: "second.txt" in raw and "+116" in rows[-2])
            print("PASS: both repositories update independently; only new live events flash")
            terminal.resize(160, 48)
            terminal.frame(lambda raw, rows: len(rows) == 48)
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: "CHANGES" not in raw)
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: "CHANGES" in raw and "alpha/" in raw and "nested/beta/" in raw)
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0 and termios.tcgetattr(terminal.slave) == terminal.before
            print("PASS: selected repositories survive resizing/view changes and clean exit")
        finally:
            terminal.close()
        terminal = Terminal(binary, workspace)
        try:
            terminal.frame(lambda raw, rows: "SELECT REPOSITORIES" in raw)
            terminal.send(b"\x1b[B\r")
            raw, rows = terminal.frame(lambda raw, rows: "CHANGES" in raw and "nested/beta/" in raw)
            assert "alpha/" not in raw
            assert "✦" not in raw
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0
            print("PASS: Enter without checks selects only the focused repository")
        finally:
            terminal.close()
        terminal = Terminal(binary, workspace)
        try:
            terminal.frame(lambda raw, rows: "SELECT REPOSITORIES" in raw)
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0 and termios.tcgetattr(terminal.slave) == terminal.before
            print("PASS: picker cancellation restores terminal state")
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
