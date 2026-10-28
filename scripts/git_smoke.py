#!/usr/bin/env python3
"""Verify Git-seeded activity, ignore filtering, and the revised tree in a PTY."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from list_smoke import Terminal


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], stderr=subprocess.PIPE)


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as temp:
        root = Path(temp) / "repo"
        root.mkdir()
        git(root, "init", "-q")
        (root / ".gitignore").write_text("ignored/\n*.cache\n")
        (root / "existing.txt").write_text("keep\nold\n")
        (root / "staged.txt").write_text("old stage\n")
        (root / "deleted.txt").write_text("gone1\ngone2\n")
        (root / "clean.txt").write_text("untouched\n")
        (root / "nested").mkdir()
        (root / "nested/real.txt").write_text("base\n")
        git(root, "add", ".")
        git(root, "-c", "user.name=Graph Test", "-c", "user.email=graph@example.invalid", "commit", "-qm", "baseline")
        (root / "existing.txt").write_text("keep\nnew1\nnew2\n")
        (root / "staged.txt").write_text("stage one\nstage two\n")
        git(root, "add", "staged.txt")
        (root / "staged.txt").write_text("stage one\nstage two\nstage three\n")
        (root / "deleted.txt").unlink()
        (root / "new.txt").write_text("new\n" * 4)
        (root / "nested/real.txt").write_text("one\ntwo\n")
        (root / "ignored").mkdir()
        (root / "ignored/secret.txt").write_text("must stay hidden\n")
        (root / "nested/scratch.cache").write_text("ignored\n")
        listing = json.loads(subprocess.check_output([binary, "--snapshot", str(root)]))
        names = {item["path"] for item in listing}
        assert not any(name.startswith(".git/") or name == ".git" or name.startswith("ignored") or name.endswith(".cache") for name in names), names
        print("PASS: snapshots exclude Git metadata and ignored files/directories")
        terminal = Terminal(binary, root)
        try:
            expected_added = sum([2, 3, 4, 2])
            expected_removed = sum([1, 1, 2, 1])
            raw, rows = terminal.frame(lambda raw, rows: all(name in raw for name in ["existing.txt", "staged.txt", "deleted.txt", "new.txt", "real.txt"]) and f"+{expected_added}" in rows[-2] and f"-{expected_removed}" in rows[-2])
            assert "✦" not in raw, "startup Git data must not flash"
            assert not any("repo/" in row for row in rows[2:-2]), rows
            assert not any("secret.txt" in row or "scratch.cache" in row for row in rows)
            assert not any("+0" in row or "-0" in row for row in rows[2:-2]), rows
            print("PASS: startup seeds staged + unstaged + untracked + deleted changes without double counting")
            print("PASS: zero cells are blank and the tree starts at depth one")
            # An ignored write must not create an activity row or change totals.
            (root / "ignored/secret.txt").write_text("ignored new content\n" * 30)
            terminal.frame(lambda raw, rows: f"+{expected_added}" in rows[-2] and "secret.txt" not in raw)
            # A previously visible untracked file becomes ignored while running.
            (root / "new.txt").write_text("live addition\n" * 5)
            terminal.frame(lambda raw, rows: "✦" in raw and "new.txt" in raw)
            ignore = root / ".gitignore"
            ignore.write_text(ignore.read_text() + "new.txt\n")
            terminal.frame(lambda raw, rows: "new.txt" not in raw and ".gitignore" in raw)
            print("PASS: live ignore-rule edits remove activity without false deletion rows")
            terminal.send(b"\tl")
            raw, rows = terminal.frame(lambda raw, rows: "CHANGES" not in raw)
            assert "clean.txt" not in raw
            print("PASS: graph view excludes unchanged Git-tracked files")
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: "CHANGES" in raw and "staged.txt" in raw)
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0
            print("PASS: Git-seeded state survives view switching and clean exit")
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
