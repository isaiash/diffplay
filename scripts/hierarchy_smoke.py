#!/usr/bin/env python3
"""Real-PTY checks for nested folder groups and solid shell/core bubbles."""
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import termios

from list_smoke import Terminal


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], stderr=subprocess.PIPE)


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as temp:
        root = Path(temp) / "repo"
        root.mkdir()
        git(root, "init", "-q")
        originals = {"A/C/plus.go": 10, "A/D/minus.go": 30, "A/E/little.go": 2, "B/Y/other.go": 4}
        for name, count in originals.items():
            file = root / name
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_text("before\n" * count)
        git(root, "add", ".")
        git(root, "-c", "user.name=Graph Test", "-c", "user.email=graph@example.invalid", "commit", "-qm", "baseline")
        for name, count in {"A/C/plus.go": 30, "A/D/minus.go": 10, "A/E/little.go": 2, "B/Y/other.go": 6}.items():
            (root / name).write_text("after\n" * count)
        (root / "B/X").mkdir()
        (root / "B/X/new.go").write_text("new\n" * 20)
        terminal = Terminal(binary, root)
        try:
            terminal.resize(160, 48)
            terminal.frame(lambda raw, rows: "CHANGES" in raw and "plus.go" in raw)
            terminal.send(b"\t")
            raw, rows = terminal.frame(lambda raw, rows: "CLUSTERS" in raw and "5 files" in raw and all(label in raw for label in ["A/C/", "A/D/", "A/E/", "B/X/", "B/Y/"]))
            assert any(re.search(r"(?:^|\s)A/(?:\s|$)", row) for row in rows)
            assert any(re.search(r"(?:^|\s)B/(?:\s|$)", row) for row in rows)
            assert all(name not in raw for name in ["plus.go", "minus.go", "little.go", "new.go", "other.go"])
            assert "●" not in raw and "•" not in raw
            assert any(char in raw for char in "█▀▄")
            assert "38;2;115;211;153" in raw and "38;2;244;92;105" in raw and "38;2;139;202;255" in raw
            print("PASS: parent and child folder titles form the changed-file hierarchy without file labels")
            print("PASS: graph uses solid block bubbles and persistent green/red/blue colors")
            evidence = Path(os.environ.get("TMPDIR", str(Path.home() / ".cache"))) / "diffplay-hierarchy-frame.ansi"
            evidence.write_text(raw)
            print(f"Captured terminal frame: {evidence}")
            terminal.send(b"n")
            terminal.frame(lambda raw, rows: "A/C/plus.go" in raw and "+30" in raw and "-10" in raw)
            print("PASS: keyboard inspection exposes the +30/-10 file behind a green shell/red core bubble")
            terminal.send(b"\x1b[<35;1;1M")
            terminal.frame(lambda raw, rows: "plus.go" not in raw)
            for width, height in [(80, 24), (40, 12), (160, 48)]:
                terminal.resize(width, height)
                terminal.frame(lambda raw, rows: len(rows) == height and all(len(row) == width for row in rows))
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: "CHANGES" in raw and "plus.go" in raw)
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0 and termios.tcgetattr(terminal.slave) == terminal.before
            print("PASS: hierarchical graph resizes and switches views without losing activity")
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
