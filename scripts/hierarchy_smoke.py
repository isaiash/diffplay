#!/usr/bin/env python3
"""Real-PTY checks for nested folder groups and dotted body/nucleus cells."""
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

from list_smoke import BLUE, GREEN, RED, WHITE, Terminal, assert_terminal_restored, braille_cells, colored_cells, mode
from contrast_smoke import cells


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], stderr=subprocess.PIPE)


def compressed_chains(binary, parent):
    root = parent / "compressed"
    (root / "a/b/c/d").mkdir(parents=True)
    (root / "a/b/c/d/file.txt").write_text("deep\n")
    (root / "a/b/file2.txt").write_text("direct\n")
    git(root, "init", "-q")
    terminal = Terminal(binary, root, "graph")
    def corners(raw):
        return [color for _, _, char, color in colored_cells(raw) if char == "┌"]
    try:
        terminal.resize(240, 80)
        raw, rows = terminal.frame(lambda raw, rows: mode(rows, "graph") and "a/b/" in raw and "c/d/" in raw)
        assert corners(raw) == [(128, 128, 128)] * 2, "expected two gray compressed frames"
        terminal.send(b"n")
        terminal.frame(lambda raw, rows: "a/b/c/d/file.txt" in raw and corners(raw).count(WHITE) == 2)
        terminal.send(b"n")
        terminal.frame(lambda raw, rows: "a/b/file2.txt" in raw and corners(raw).count(WHITE) == 1 and corners(raw).count((128, 128, 128)) == 1)
        (root / "a/b/c/e").mkdir()
        (root / "a/b/c/e/extra.txt").write_text("branch\n")
        terminal.frame(lambda raw, rows: "3 files" in rows[-1] and len(corners(raw)) == 4 and "e/" in raw)
        (root / "a/b/c/direct.txt").write_text("direct in c\n")
        terminal.frame(lambda raw, rows: "4 files" in rows[-1])
        (root / "a/b/c/e/extra.txt").unlink()
        terminal.frame(lambda raw, rows: "3 files" in rows[-1] and len(corners(raw)) == 3)
        (root / "a/b/c/direct.txt").unlink()
        terminal.frame(lambda raw, rows: "2 files" in rows[-1] and len(corners(raw)) == 2 and "c/d/" in raw)
        terminal.send(b"q")
        terminal.proc.wait(timeout=5)
        assert terminal.proc.returncode == 0
        assert_terminal_restored(terminal.slave, terminal.before)
        print("PASS: a/b and c/d compressed frames, selected ancestors, live branch/direct-file expansion and recompression")
    finally:
        terminal.close()


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as temp:
        compressed_chains(binary, Path(temp))
        root = Path(temp) / "repo"
        root.mkdir()
        git(root, "init", "-q")
        originals = {"A/C/plus.go": 10, "A/D/minus.go": 30, "A/E/little.go": 2, "B/Y/other.go": 4}
        for name, count in originals.items():
            file = root / name
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_text("before\n" * count)
        (root / "A/C/unchanged.go").write_text("quiet context\n" * 10)
        (root / "A/C/unchanged-large.go").write_text("quiet context\n" * 40)
        (root / "inactive/deep").mkdir(parents=True)
        (root / "inactive/deep/untouched.go").write_text("hidden context\n")
        git(root, "add", ".")
        git(root, "-c", "user.name=Graph Test", "-c", "user.email=graph@example.invalid", "commit", "-qm", "baseline")
        for name, count in {"A/C/plus.go": 30, "A/D/minus.go": 10, "A/E/little.go": 2, "B/Y/other.go": 6}.items():
            (root / name).write_text("after\n" * count)
        (root / "B/X").mkdir()
        (root / "B/X/new.go").write_text("new\n" * 20)
        terminal = Terminal(binary, root)
        try:
            terminal.resize(160, 48)
            terminal.frame(lambda raw, rows: mode(rows, "list") and "plus.go" in raw)
            terminal.send(b"\t")
            raw, rows = terminal.frame(lambda raw, rows: mode(rows, "graph") and "7 files" in rows[-1] and all(label in raw for label in ["C/", "D/", "E/", "X/", "Y/"]))
            assert any(re.search(r"(?:^|[\s─])A/(?:[\s─]|$)", row) for row in rows)
            assert any(re.search(r"(?:^|[\s─])B/(?:[\s─]|$)", row) for row in rows)
            assert sum(row.count("┌") for row in rows) == 7, "expected exactly seven active folder frames, without an implicit root"
            assert "./" not in raw and all(label not in raw for label in ["A/C/", "A/D/", "A/E/", "B/X/", "B/Y/"])
            assert "inactive/" not in raw and "deep/" not in raw, "untouched directory branches must be recursively absent"
            assert all(char in raw for char in "┌┐└┘─│"), "folder frames must be rectangular"
            assert all(name not in raw for name in ["plus.go", "minus.go", "little.go", "new.go", "other.go", "unchanged.go"])
            assert "●" not in raw and "•" not in raw
            assert not any(char in raw for char in "█▀▄")
            colors = {cell[3] for cell in braille_cells(raw)}
            assert {GREEN, RED, BLUE} <= colors, colors
            assert any("\u2801" <= ch <= "\u28ff" and fg is None and dim and not reverse
                       for ch, fg, _, dim, reverse, _ in cells(raw).values()), colors
            print("PASS: nested folder hierarchy includes unchanged context without exposing file labels")
            print("PASS: graph uses Braille dots in green/red/blue and gray, never solid blocks")
            evidence = Path(os.environ.get("TMPDIR", str(Path.home() / ".cache"))) / "diffplay-hierarchy-frame.ansi"
            evidence.write_text(raw)
            print(f"Captured terminal frame: {evidence}")
            terminal.send(b"n")
            terminal.frame(lambda raw, rows: "A/C/plus.go" in raw and "+30" in raw and "-10" in raw)
            print("PASS: keyboard inspection exposes the +30/-10 dotted body/nucleus file")
            terminal.send(b"n")
            terminal.frame(lambda raw, rows: "A/C/unchanged-large.go" in raw and "40 file lines" in raw)
            terminal.send(b"n")
            terminal.frame(lambda raw, rows: "A/C/unchanged.go" in raw and "10 file lines" in raw)
            print("PASS: differently sized gray context files expose exact current line counts")
            terminal.send(b"\x1b[<35;1;1M")
            terminal.frame(lambda raw, rows: "plus.go" not in raw)
            for width, height in [(80, 24), (40, 12), (160, 48)]:
                terminal.resize(width, height)
                terminal.frame(lambda raw, rows: len(rows) == height and all(len(row) == width for row in rows))
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: mode(rows, "combined"))
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: mode(rows, "list") and "plus.go" in raw)
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0, f"hierarchy view exited with status {terminal.proc.returncode}"
            assert_terminal_restored(terminal.slave, terminal.before)
            print("PASS: hierarchical graph resizes and switches views without losing activity")
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
