#!/usr/bin/env python3
"""End-to-end checks for the change-list view using the actual executable."""
import codecs
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
HEADER = re.compile(r"^(?:(?:\+|-)(?:\d+\??|\?) )*(?:-?\d+\.\d{2}|\?)% new lines")
BLACK, GRAY, WHITE = 0, 8, 15
BLUE, GREEN, RED = (139, 202, 255), (115, 211, 153), (244, 92, 105)


def sgr_colors(code, fg, bg):
    """Keep theme palette indexes symbolic; RGB sequences retain their tuples."""
    if not code.endswith("m"):
        return fg, bg
    values = [int(value or 0) for value in code[2:-1].split(";")]
    i = 0
    while i < len(values):
        value = values[i]
        if value == 0:
            fg = bg = None
        elif value == 39:
            fg = None
        elif value == 49:
            bg = None
        elif 30 <= value <= 37 or 90 <= value <= 97:
            fg = value - 30 if value < 90 else value - 90 + 8
        elif 40 <= value <= 47 or 100 <= value <= 107:
            bg = value - 40 if value < 100 else value - 100 + 8
        elif value in (38, 48) and values[i + 1:i + 2] == [2]:
            color = tuple(values[i + 2:i + 5])
            assert len(color) == 3, code
            if value == 38:
                fg = color
            else:
                bg = color
            i += 4
        elif value in (38, 48) and values[i + 1:i + 2] == [5]:
            assert len(values) > i + 2, code
            if value == 38:
                fg = values[i + 2]
            else:
                bg = values[i + 2]
            i += 2
        i += 1
    return fg, bg


def sgr_inverse(code, inverse):
    if code.endswith("m"):
        values = [int(value or 0) for value in code[2:-1].split(";")]
        i = 0
        while i < len(values):
            value = values[i]
            if value in (38, 48) and values[i + 1:i + 2] in ([2], [5]):
                i += 5 if values[i + 1] == 2 else 3
                continue
            if value in (0, 7, 27):
                inverse = value == 7
            i += 1
    return inverse


def colored_cells(raw):
    """Track effective foreground across rows without altering glyph masks."""
    fg = bg = None
    inverse = False
    cells = []
    for y, line in enumerate(raw.split("\r\n")):
        x, offset = 0, 0
        for match in ANSI.finditer(line):
            for char in line[offset:match.start()]:
                cells.append((x, y, char, bg if inverse else fg))
                x += 1
            fg, bg = sgr_colors(match.group(), fg, bg)
            inverse = sgr_inverse(match.group(), inverse)
            offset = match.end()
        for char in line[offset:]:
            cells.append((x, y, char, bg if inverse else fg))
            x += 1
    return cells


def parser_check():
    for index in range(16):
        fg = 30 + index if index < 8 else 90 + index - 8
        bg = fg + 10
        assert sgr_colors(f"\x1b[22;{fg};{bg}m", None, None) == (index, index)
    assert sgr_colors("\x1b[39;49m", WHITE, BLUE) == (None, None)
    assert sgr_colors("\x1b[0m", WHITE, BLUE) == (None, None)
    assert sgr_colors("\x1b[38;2;1;2;3;48;2;4;5;6m", None, None) == ((1, 2, 3), (4, 5, 6))
    assert sgr_colors("\x1b[22;38;2;139;202;255;48;2;244;92;105m", None, None) == (BLUE, RED)
    assert sgr_colors("\x1b[38;5;4;48;5;1m", None, None) == (4, 1)
    assert colored_cells("\x1b[97m⣿\r\n⡇\x1b[39m⠁") == [(0, 0, "⣿", WHITE), (0, 1, "⡇", WHITE), (1, 1, "⠁", None)]
    assert colored_cells("\x1b[22;38;2;139;202;255;48;2;244;92;105;7mA\r\nB\x1b[27mC") == [(0, 0, "A", RED), (0, 1, "B", RED), (1, 1, "C", BLUE)]


parser_check()


def header_prefix(row):
    match = HEADER.match(row)
    assert match, f"invalid repository header: {row!r}"
    return match.group()


assert header_prefix("+19 -3 76.19% new lines    21 total lines") == "+19 -3 76.19% new lines"
assert header_prefix("0.00% new lines    0 total lines") == "0.00% new lines"
assert header_prefix("+? -? ?% new lines    ? total lines") == "+? -? ?% new lines"
assert HEADER.match("repo +19 -3 76.19% new lines") is None


def header_backgrounds(raw):
    """Decode effective theme/RGB background for each first-row cell."""
    line = raw.split("\r\n", 1)[0]
    colors, fg, bg, inverse, offset = [], None, None, False, 0
    for match in ANSI.finditer(line):
        colors.extend([fg if inverse else bg] * len(line[offset:match.start()]))
        fg, bg = sgr_colors(match.group(), fg, bg)
        inverse = sgr_inverse(match.group(), inverse)
        offset = match.end()
    colors.extend([fg if inverse else bg] * len(line[offset:]))
    return colors


def divider(rows):
    matches = [i for i, row in enumerate(rows) if row and "─" in row and set(row) <= {"─", "↕"}]
    return matches[0] if len(matches) == 1 else None


def mode(rows, expected):
    if not rows:
        return False
    footer = rows[-1]
    totals = "new lines" in rows[0]
    if expected == "combined":
        return totals and divider(rows) is not None and "drag split/pan" in footer
    if expected == "list":
        return totals and divider(rows) is None and "J/K diff" in footer
    return not totals and divider(rows) is None and "drag pan" in footer and ("LIVE" in footer or "PAUSED" in footer)


def braille_cells(raw):
    """Return body glyph positions/colors, including reused ANSI foregrounds."""
    return [cell for cell in colored_cells(raw) if "\u2801" <= cell[2] <= "\u28ff"]


def assert_terminal_restored(slave, before, label="terminal"):
    if sys.platform == "darwin":
        # XNU reprocesses the transient PENDIN flag on a read readiness check.
        select.select([slave], [], [], 0)
    after = termios.tcgetattr(slave)
    assert after == before, f"{label} state: before={before!r}, after={after!r}"


class Terminal:
    def __init__(self, binary, root, view: str | None = "list"):
        self.master, self.slave = pty.openpty()
        self.before = termios.tcgetattr(self.slave)
        self.resize(100, 30, notify=False)
        args = [binary, "--interval", "100ms"]
        if view is not None:
            args += ["--view", view]
        self.proc = subprocess.Popen(args,
                                     cwd=root, stdin=self.slave, stdout=self.slave,
                                     stderr=self.slave, start_new_session=True)
        self.buffer = ""
        self.tail = ""
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")

    def resize(self, width, height, notify=True):
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
        if notify:
            self.proc.send_signal(signal.SIGWINCH)

    def send(self, data):
        os.write(self.master, data)

    def frame(self, predicate=lambda raw, rows: True, timeout: float = 8):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if select.select([self.master], [], [], .1)[0]:
                chunk = self.decoder.decode(os.read(self.master, 262144))
                self.buffer += chunk
                self.tail = (self.tail + chunk)[-200000:]
                while "\x1b[H" in self.buffer:
                    start = self.buffer.index("\x1b[H")
                    # Skip the frame's leading reset, not its trailing reset.
                    reset_start = start + len("\x1b[H")
                    if self.buffer.startswith("\x1b[0m", reset_start):
                        reset_start += len("\x1b[0m")
                    end = self.buffer.find("\x1b[0m", reset_start)
                    if end < 0:
                        self.buffer = self.buffer[start:]
                        break
                    raw = self.buffer[start:end + 4]
                    self.buffer = self.buffer[end + 4:]
                    rows = ANSI.sub("", raw).split("\r\n")
                    self.last_frame = raw, rows
                    if predicate(raw, rows):
                        return raw, rows
            if self.proc.poll() is not None:
                raise AssertionError(f"unexpected exit {self.proc.returncode}: {self.tail[-2000:]}")
            # An unchanged TUI keeps its last frame instead of repainting selections.
            cached = getattr(self, "last_frame", None)
            if cached is not None and predicate(*cached):
                return cached
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
            raw, rows = terminal.frame(lambda raw, rows: mode(rows, "list") and "existing.txt" not in raw and "removed.txt" not in raw)
            titles = rows[1]
            assert all(label in titles for label in ["HITS", "ADDED", "REMOVED", "%", root.name]), titles
            assert titles.index("HITS") < titles.index("ADDED") < titles.index("REMOVED") < titles.index("%") < titles.index(root.name), titles
            assert root.name not in rows[0] and "CHANGED PATHS" not in raw
            print("PASS: list column titles include repository and percentage; global header omits repository")
            (root / "new.txt").write_text("new line\n" * 10)
            raw, rows = terminal.frame(lambda raw, rows: "new.txt" in raw and "+10" in raw and "✦" in raw)
            assert "new.txt" in rows[2] and " 100 " in rows[2]
            (root / "docs/existing.txt").write_text("keep\nnew1\nnew2\n")
            _, rows = terminal.frame(lambda raw, rows: "existing.txt" in raw and "+12" in raw and "-1" in raw)
            existing = next(row for row in rows if "existing.txt" in row)
            assert " 100 " in existing, "file percentage must count both added and removed lines"
            (root / "deep/path").mkdir(parents=True)
            (root / "deep/path/change.txt").write_text("added\n" * 7)
            terminal.frame(lambda raw, rows: "change.txt" in raw and "+19" in raw)
            (root / "removed.txt").unlink()
            raw, rows = terminal.frame(lambda raw, rows: "removed.txt" in raw and "+19" in header_prefix(rows[0]) and "-3" in header_prefix(rows[0]))
            assert header_prefix(rows[0]) == "+19 -3 76.19% new lines"
            assert rows[0].endswith("21 total lines") and str(root) not in rows[0]
            assert "untouched.txt" not in raw
            file_rows = [row for row in rows[1:-1] if any(name in row for name in ["new.txt", "deep/", "docs/", "removed.txt"])]
            assert [next(name for name in ["new.txt", "deep/", "docs/", "removed.txt"] if name in row) for row in file_rows] == ["new.txt", "deep/", "docs/", "removed.txt"]
            raw_lines = raw.split("\r\n")
            removed_line = next(line for line in raw_lines[1:-1] if "removed.txt" in line)
            assert "\x1b[9m" in removed_line
            assert {GREEN, BLUE, RED} <= {color for _, _, _, color in colored_cells(raw)}
            print("PASS: exact cumulative +19/-3 counts, added-line ordering, colors, and deleted-name strikethrough")
            assert all(any(name in row for row in rows) for name in ["deep/", "path/", "change.txt"])
            assert not any("repo/" in row for row in rows[2:-2])
            print("PASS: automatically revealed full ancestor paths to changed files")
            terminal.send(b"\x1b[H\x1b[B\x1b[D")  # First file -> deep; collapse.
            terminal.frame(lambda raw, rows: "deep/" in raw and "change.txt" not in raw)
            terminal.send(b"\x1b[C")
            terminal.frame(lambda raw, rows: "change.txt" in raw)
            terminal.send(b"\x1b[C\x1b[C")
            terminal.frame(lambda raw, rows: any(row.startswith("›") and "change.txt" in row for row in rows))
            print("PASS: arrow navigation expands, collapses, and descends through folders")
            _, rows = terminal.frame()
            docs_y = next(i for i, row in enumerate(rows[1:-1], 1) if "docs/" in row)
            terminal.send(f"\x1b[<35;20;{docs_y + 1}M".encode())
            terminal.frame(lambda raw, rows: rows[docs_y].startswith("›"))
            terminal.send(f"\x1b[<0;20;{docs_y + 1}M".encode())
            terminal.frame(lambda raw, rows: "existing.txt" not in raw)
            terminal.send(b"\x1b[C")
            terminal.frame(lambda raw, rows: "existing.txt" in raw)
            print("PASS: mouse hover selects and clicking toggles folders")
            terminal.send(b"\t")
            raw, _ = terminal.frame(lambda raw, rows: mode(rows, "graph"))
            assert "5 files" in raw
            assert "untouched.txt" not in raw
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: mode(rows, "combined"))
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: mode(rows, "list") and "+19" in header_prefix(rows[0]) and "-3" in header_prefix(rows[0]))
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
            assert_terminal_restored(terminal.slave, terminal.before)
            print("PASS: clean exit restores terminal attributes")
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
