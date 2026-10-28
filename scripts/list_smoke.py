#!/usr/bin/env python3
"""End-to-end checks for the change-list view using the actual executable."""
import fcntl
import os
from pathlib import Path
import pty
import re
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time

ANSI = re.compile(r"\x1b\[[0-?]*[ -/]*[@-~]")


class Terminal:
    def __init__(self, binary, root):
        self.master, self.slave = pty.openpty()
        self.before = termios.tcgetattr(self.slave)
        self.resize(100, 30, notify=False)
        self.proc = subprocess.Popen([binary, "--interval", "100ms"],
                                     cwd=root, stdin=self.slave, stdout=self.slave,
                                     stderr=self.slave, start_new_session=True)
        self.buffer = ""
        self.tail = ""

    def resize(self, width, height, notify=True):
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
        if notify:
            self.proc.send_signal(signal.SIGWINCH)

    def send(self, data):
        os.write(self.master, data)

    def frame(self, predicate=lambda raw, rows: True, timeout=8):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if select.select([self.master], [], [], .1)[0]:
                chunk = os.read(self.master, 262144).decode("utf-8", "replace")
                self.buffer += chunk
                self.tail = (self.tail + chunk)[-200000:]
                while "\x1b[H" in self.buffer:
                    start = self.buffer.index("\x1b[H")
                    end = self.buffer.find("\x1b[0m", start)
                    if end < 0:
                        self.buffer = self.buffer[start:]
                        break
                    raw = self.buffer[start:end + 4]
                    self.buffer = self.buffer[end + 4:]
                    rows = ANSI.sub("", raw).split("\r\n")
                    if predicate(raw, rows):
                        return raw, rows
            if self.proc.poll() is not None:
                raise AssertionError(f"unexpected exit {self.proc.returncode}: {self.tail[-2000:]}")
        raise AssertionError("expected frame not found: " + ANSI.sub("", self.tail[-7000:]))

    def close(self):
        if self.proc.poll() is None:
            self.proc.kill()
            self.proc.wait()
        os.close(self.master)
        os.close(self.slave)


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as temp:
        root = Path(temp) / "repo"
        (root / "docs").mkdir(parents=True)
        (root / "docs/existing.txt").write_text("keep\nold\n")
        (root / "removed.txt").write_text("gone\nagain\n")
        (root / "untouched.txt").write_text("not changed\n")
        subprocess.check_call(["git","-C",str(root),"init","-q"])
        subprocess.check_call(["git","-C",str(root),"add","."])
        subprocess.check_call(["git","-C",str(root),"-c","user.name=Graph Test","-c","user.email=graph@example.invalid","commit","-qm","baseline"])
        terminal = Terminal(binary, root)
        try:
            raw, rows = terminal.frame(lambda raw, rows: "CHANGES" in raw and "existing.txt" not in raw and "removed.txt" not in raw)
            assert not any(label in raw for label in ["ADDED", "REMOVED", "CHANGED PATHS"])
            print("PASS: default list view has no section-header row or unchanged paths")
            (root / "new.txt").write_text("new line\n" * 10)
            raw, rows = terminal.frame(lambda raw, rows: "new.txt" in raw and "+10" in raw and "✦" in raw)
            assert "new.txt" in rows[2]
            (root / "docs/existing.txt").write_text("keep\nnew1\nnew2\n")
            terminal.frame(lambda raw, rows: "existing.txt" in raw and "+12" in raw and "-1" in raw)
            (root / "deep/path").mkdir(parents=True)
            (root / "deep/path/change.txt").write_text("added\n" * 7)
            terminal.frame(lambda raw, rows: "change.txt" in raw and "+19" in raw)
            (root / "removed.txt").unlink()
            raw, rows = terminal.frame(lambda raw, rows: "removed.txt" in raw and "+19" in rows[-2] and "-3" in rows[-2])
            assert "untouched.txt" not in raw
            file_rows = [row for row in rows[2:-2] if any(name in row for name in ["new.txt", "deep/", "docs/", "removed.txt"])]
            assert [next(name for name in ["new.txt", "deep/", "docs/", "removed.txt"] if name in row) for row in file_rows] == ["new.txt", "deep/", "docs/", "removed.txt"]
            raw_lines = raw.split("\r\n")
            removed_line = next(line for line in raw_lines[2:-2] if "removed.txt" in line)
            assert "\x1b[9m" in removed_line
            for color in ["38;2;115;211;153", "38;2;139;202;255", "38;2;244;92;105"]:
                assert color in raw
            print("PASS: exact cumulative +19/-3 counts, added-line ordering, colors, and deleted-name strikethrough")
            assert all(any(name in row for row in rows) for name in ["deep/", "path/", "change.txt"])
            assert not any("repo/" in row for row in rows[2:-2])
            print("PASS: automatically revealed full ancestor paths to changed files")
            terminal.send(b"\x1b[H\x1b[B\x1b[D")  # First file -> deep; collapse.
            terminal.frame(lambda raw, rows: "deep/" in raw and "change.txt" not in raw)
            terminal.send(b"\x1b[C")
            terminal.frame(lambda raw, rows: "change.txt" in raw)
            terminal.send(b"\x1b[C\x1b[C")
            terminal.frame(lambda raw, rows: "deep/path/change.txt" in rows[-2])
            print("PASS: arrow navigation expands, collapses, and descends through folders")
            _, rows = terminal.frame()
            docs_y = next(i for i, row in enumerate(rows[2:-2], 2) if "docs/" in row)
            terminal.send(f"\x1b[<35;20;{docs_y + 1}M".encode())
            terminal.frame(lambda raw, rows: rows[-2].rstrip().endswith("docs"))
            terminal.send(f"\x1b[<0;20;{docs_y + 1}M".encode())
            terminal.frame(lambda raw, rows: "existing.txt" not in raw)
            terminal.send(b"\x1b[C")
            terminal.frame(lambda raw, rows: "existing.txt" in raw)
            print("PASS: mouse hover selects and clicking toggles folders")
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: "CHANGES" not in raw and "Tab graph/list" in raw)
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: "CHANGES" in raw and "+19" in rows[-2] and "-3" in rows[-2])
            print("PASS: Tab switches views without losing session history")
            terminal.frame(lambda raw, rows: "✦" not in raw and "removed.txt" in raw, timeout=7)
            print("PASS: change flashes fade while history and deleted rows remain")
            for width, height in [(80, 24), (160, 48), (20, 8), (100, 30)]:
                terminal.resize(width, height)
                terminal.frame(lambda raw, rows: len(rows) == height and all(len(row) == width for row in rows))
            print("PASS: list and split diff preview resize live")
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0
            assert termios.tcgetattr(terminal.slave) == terminal.before
            print("PASS: clean exit restores terminal attributes")
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
