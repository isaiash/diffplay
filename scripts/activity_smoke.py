#!/usr/bin/env python3
"""Real-PTY regression checks for live follow, HEAD previews and durable counts."""
import codecs
import os
from pathlib import Path
import pty
import re
import subprocess
import sys
import tempfile
import termios
import time

from list_smoke import ANSI, WHITE, Terminal, assert_terminal_restored, divider, mode, sgr_colors, sgr_inverse


def effective_cells(raw):
    """Decode native/default colors and inverse without losing state at newlines."""
    fg = bg = None
    inverse = False
    cells = {}
    for y, line in enumerate(raw.split("\r\n")):
        x = offset = 0
        for match in ANSI.finditer(line):
            for char in line[offset:match.start()]:
                cells[x, y] = (char, bg if inverse else fg)
                x += 1
            code = match.group()
            fg, bg = sgr_colors(code, fg, bg)
            inverse = sgr_inverse(code, inverse)
            offset = match.end()
        for char in line[offset:]:
            cells[x, y] = (char, bg if inverse else fg)
            x += 1
    return cells


def parser_check():
    for index in range(16):
        fg = 30 + index if index < 8 else 90 + index - 8
        assert effective_cells(f"\x1b[{fg}mA")[0, 0] == ("A", index)
        assert effective_cells(f"\x1b[{fg + 10};7mA")[0, 0] == ("A", index)
    cells = effective_cells("\x1b[97;44mA\x1b[7mB\r\nC\x1b[27mD\x1b[39mE\x1b[7;49mF\x1b[0mG")
    assert [cells[x, 0][1] for x in range(2)] == [WHITE, 4]
    assert [cells[x, 1][1] for x in range(5)] == [4, WHITE, None, None, None]
    assert effective_cells("\x1b[38;2;7;27;0mA")[0, 0] == ("A", (7, 27, 0))
    assert effective_cells("\x1b[7;48;5;0mA")[0, 0] == ("A", 0)
    decoder = codecs.getincrementaldecoder("utf-8")("replace")
    encoded = "›✦".encode()
    assert "".join(decoder.decode(bytes([byte])) for byte in encoded) == "›✦"


class ActivityTerminal(Terminal):
    def __init__(self, binary, root, view, env):
        self.master, self.slave = pty.openpty()
        try:
            self.before = termios.tcgetattr(self.slave)
            self.resize(160, 36, notify=False)
            self.proc = subprocess.Popen(
                [binary, "--interval", "50ms", "--view", view], cwd=root,
                env=env, stdin=self.slave, stdout=self.slave, stderr=self.slave,
                start_new_session=True)
        except BaseException:
            os.close(self.master)
            os.close(self.slave)
            raise
        self.buffer = self.tail = ""
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")

    def quit(self):
        self.send(b"q")
        self.proc.wait(timeout=5)
        assert self.proc.returncode == 0, self.tail[-2000:]
        assert_terminal_restored(self.slave, self.before)

    def close(self):
        try:
            if self.proc.poll() is None:
                self.quit()
        finally:
            try:
                if self.proc.poll() is None:
                    self.proc.kill()
                    self.proc.wait(timeout=5)
            finally:
                termios.tcsetattr(self.slave, termios.TCSANOW, self.before)
                os.close(self.master)
                os.close(self.slave)


def body(rows):
    return rows[1:divider(rows) if divider(rows) is not None else -1]


def file_y(rows, name):
    width = list_width(rows)
    return next((y for y, row in enumerate(body(rows), 1) if row[:width].rstrip().endswith(name)), None)


def list_width(rows):
    return body(rows)[0].rfind("│")


def preview(rows):
    width = list_width(rows)
    return "\n".join(row[width + 1:] for row in body(rows))


def selected(rows, name):
    y = file_y(rows, name)
    return y is not None and rows[y].startswith("›")


def assert_count(raw, rows, name, expected):
    y = file_y(rows, name)
    assert y is not None, f"{name} not visible"
    prefix = rows[y][:list_width(rows)]
    match = re.match(r"^.{2}(\d*) +(\+\d+)", prefix)
    assert match, f"missing count/ADDED columns: {prefix!r}"
    assert match[1] == (str(expected) if expected else ""), prefix
    cells = effective_cells(raw)
    if expected:
        assert all(cells[x, y][1] == WHITE for x in range(2, 2 + len(str(expected)))), "counter must be effective white 15"
        assert 2 + len(str(expected)) < match.start(2), "counter must precede ADDED"
    else:
        assert cells[2, y][0] == " ", "zero counter must be blank, not text"


def inspect(terminal, name):
    terminal.send(b"\x1b[H")
    raw, rows = terminal.frame()
    for _ in range(64):
        if selected(rows, name):
            return raw, rows
        previous = rows
        terminal.send(b"\x1b[B")
        raw, rows = terminal.frame(lambda raw, rows: rows != previous)
    raise AssertionError(f"could not navigate to {name}")


def scenario(binary, parent, view):
    root = parent / "repo"
    (root / "src").mkdir(parents=True)
    home = parent / "home"
    config = parent / "config"
    home.mkdir()
    config.mkdir()
    env = {**os.environ, "HOME": str(home), "XDG_CONFIG_HOME": str(config),
           "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
           "TERM": "xterm-256color", "LC_ALL": "C.UTF-8"}

    def git(*args):
        return subprocess.check_output(["git", "-C", str(root), *args], env=env, stderr=subprocess.STDOUT)

    def commit(message, *paths):
        git("add", "--", *paths)
        git("-c", "user.name=Activity Smoke", "-c", "user.email=activity@example.invalid", "commit", "-qm", message)

    baseline = "old first\nold second\n"
    for i in range(48):
        (root / "src" / f"file{i:02d}.txt").write_text(baseline)
    keeper = root / "keeper.txt"
    keeper.write_text("keeper baseline\n")
    git("init", "-q")
    commit("baseline", ".")
    for i in range(48):
        (root / "src" / f"file{i:02d}.txt").write_text(baseline + f"seed {i}\n" * (48 - i))
    keeper.write_text("keeper baseline\n" + "keeper seed\n" * 40)
    target = root / "src/file47.txt"
    terminal = ActivityTerminal(binary, root, view, env)
    try:
        raw, rows = terminal.frame(lambda raw, rows: mode(rows, view) and any("src/" in row[:80] for row in body(rows)))
        assert file_y(rows, target.name) is None, "fixture target must begin offscreen"
        assert "✦" not in raw, "startup must not synthesize bursts"
        folder_y = next(y for y, row in enumerate(rows) if "src/" in row[:80])
        terminal.send(f"\x1b[<0;35;{folder_y + 1}M".encode())
        terminal.frame(lambda raw, rows: any("▸" in row[:80] and "src/" in row[:80] for row in body(rows)))

        target.write_text("first live change\nold second\nseed 47\n")
        raw, rows = terminal.frame(lambda raw, rows: selected(rows, target.name) and "+first live change" in preview(rows))
        assert_count(raw, rows, target.name, 1)
        assert "src/file47.txt" in preview(rows), "revealed file must retain its full preview path"
        target.write_text("first live change\nsecond live change\nseed 47\n")
        raw, rows = terminal.frame(lambda raw, rows: selected(rows, target.name) and "+second live change" in preview(rows))
        assert_count(raw, rows, target.name, 2)
        assert "+first live change" in preview(rows), "preview must be HEAD-relative, not last interval"
        print(f"PASS [{view}]: collapsed/offscreen live follow, white 1/2 before ADDED, cumulative HEAD preview", flush=True)

        keeper.write_text(keeper.read_text() + "keeper live\n")
        raw, rows = terminal.frame(lambda raw, rows: selected(rows, keeper.name) and "+keeper live" in raw)
        assert_count(raw, rows, keeper.name, 1)
        terminal.quit()
    finally:
        terminal.close()

    terminal = ActivityTerminal(binary, root, view, env)
    try:
        raw, rows = terminal.frame(lambda raw, rows: mode(rows, view))
        assert "✦" not in raw, "restart must not synthesize a burst"
        raw, rows = inspect(terminal, target.name)
        assert_count(raw, rows, target.name, 2)
        until = time.monotonic() + .4
        while time.monotonic() < until:
            raw, rows = terminal.frame()
            assert "✦" not in raw, "idle restart synthesized a burst"
            assert_count(raw, rows, target.name, 2)
        print(f"PASS [{view}]: restart retains counter 2 without startup bursts", flush=True)

        before = target.read_bytes()
        commit("partial target commit", "src/file47.txt")
        assert target.read_bytes() == before, "commit fixture unexpectedly changed working-tree bytes"
        raw, rows = terminal.frame(lambda raw, rows: file_y(rows, target.name) is not None and rows[file_y(rows, target.name)][2] == " ")
        assert_count(raw, rows, target.name, 0)
        raw, rows = inspect(terminal, keeper.name)
        assert_count(raw, rows, keeper.name, 1)
        assert git("diff", "--name-only", "HEAD").decode().splitlines(), "partial commit must leave uncommitted files"
        target.write_text(target.read_text() + "after commit live\n")
        raw, rows = terminal.frame(lambda raw, rows: selected(rows, target.name) and "+after commit live" in raw)
        assert_count(raw, rows, target.name, 1)
        assert "+first live change" not in preview(rows), "preview did not advance to new HEAD"
        terminal.quit()
        print(f"PASS [{view}]: Git-only partial commit blanks zero, preserves other count, next edit starts 1; terminal restored", flush=True)
    finally:
        terminal.close()


def main():
    parser_check()
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    scratch = os.environ.get("TMPDIR", str(Path.home() / ".hermes/cache/scratch"))
    with tempfile.TemporaryDirectory(prefix="diffplay-activity-", dir=scratch) as temp:
        for view in ("combined", "list"):
            parent = Path(temp) / view
            parent.mkdir()
            scenario(binary, parent, view)
    print("PASS: activity smoke (combined/list), parser self-checks and isolated fixture cleanup")


if __name__ == "__main__":
    main()
