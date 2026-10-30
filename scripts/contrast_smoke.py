#!/usr/bin/env python3
"""Check readable structure and selected-only white folder branches via PTY."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from list_smoke import ANSI, Terminal, assert_terminal_restored, divider, mode


def cells(raw):
    fg = bg = None
    dim = reverse = hidden = False
    x = y = 0
    result = {}
    pos = 0
    for match in ANSI.finditer(raw + "\x1b[0m"):
        for ch in raw[pos:match.start()]:
            if ch == "\r":
                x = 0
            elif ch == "\n":
                y += 1
            else:
                result[x, y] = (ch, fg, bg, dim, reverse, hidden)
                x += 1
        code = match.group()
        if code.endswith("m"):
            values = [int(n or 0) for n in code[2:-1].split(";")]
            i = 0
            while i < len(values):
                n = values[i]
                if n == 0:
                    fg = bg = None
                    dim = reverse = hidden = False
                elif n in (1, 2):
                    dim = True
                elif n == 22:
                    dim = False
                elif n == 7:
                    reverse = True
                elif n == 27:
                    reverse = False
                elif n == 8:
                    hidden = True
                elif n == 28:
                    hidden = False
                elif n == 39:
                    fg = None
                elif n == 49:
                    bg = None
                elif 30 <= n <= 37 or 90 <= n <= 97:
                    fg = n
                elif 40 <= n <= 47 or 100 <= n <= 107:
                    bg = n
                elif n in (38, 48):
                    count = 2 if values[i+1] == 5 else 4
                    color = tuple(values[i:i+count+1])
                    if n == 38:
                        fg = color
                    else:
                        bg = color
                    i += count
                i += 1
        pos = match.end()
    return result


def parser_check():
    sample = "\x1b[2;90;100;7;8mX\r\n\x1b[22;39;49;27;28mY\x1b[0mZ\x1b[38;5;8mW"
    got = cells(sample)
    assert got[0, 0] == ("X", 90, 100, True, True, True)
    assert got[0, 1] == ("Y", None, None, False, False, False)
    assert got[1, 1] == ("Z", None, None, False, False, False)
    assert got[2, 1][1] == (38, 5, 8)


def check(raw, rows, view):
    decoded = cells(raw)
    def check_cell(x, y, glyph):
        got = decoded[x, y]
        assert got == (glyph, None, None, False, False, False), f"{view} ({x},{y}): {got}"

    nested_y = next(i for i, row in enumerate(rows) if "nested/" in row)
    nested = rows[nested_y]
    assert not nested.startswith("›"), nested
    prefix = nested.index("├─")
    icon = nested.index("▾", prefix)
    for x in (prefix, prefix + 1, icon):
        check_cell(x, nested_y, nested[x])
    file_y = next(i for i, row in enumerate(rows) if "change.txt" in row)
    row = rows[file_y]
    assert not row.startswith("›"), row
    center = row.index("│")
    assert row[center-1] == "█" and row[center+1] == "█", row
    check_cell(center, file_y, "│")
    tree = row.index("│", center+1)
    check_cell(tree, file_y, "│")
    split = rows[1].rfind("│")
    assert split > tree and all(rows[y][split] == "│" for y in range(1, 5)), rows[1:5]
    check_cell(split, 4, "│")
    if view == "combined":
        y = divider(rows)
        assert y is not None
        for x in (0, len(rows[y])-1):
            check_cell(x, y, "─")
        corners = [(x, line) for line in range(y+1, len(rows)-1)
                   for x, ch in enumerate(rows[line]) if ch == "┌"]
        assert corners, "missing graph folder outlines"
        for x, line in corners:
            assert decoded[x, line] == ("┌", (38, 2, 128, 128, 128), None, False, False, False), decoded[x, line]


def main():
    parser_check()
    binary = str(Path(sys.argv[1]).resolve())
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as temp:
        base = Path(temp)
        root = base / "repo"
        root.mkdir()
        home = base / "home"
        home.mkdir()
        cache = home / "tmp"
        cache.mkdir()
        os.environ.update(HOME=str(home), XDG_CONFIG_HOME=str(home / ".config"), TMPDIR=str(cache))
        subprocess.run(["git", "init", "-q", str(root)], check=True)
        terminal = Terminal(binary, root, "list")
        try:
            terminal.resize(140, 40)
            terminal.frame(lambda raw, rows: mode(rows, "list"))
            (root / "selected.txt").write_text("one\n")
            terminal.frame(lambda raw, rows: "selected.txt" in raw)
            (root / "dir/nested").mkdir(parents=True)
            (root / "dir/nested/change.txt").write_text("new\n" * 8)
            terminal.frame(lambda raw, rows: "change.txt" in raw)
            (root / "dir/nested/change.txt").write_text("new\n" * 4 + "changed\n" * 4)
            terminal.frame(lambda raw, rows: any("█│█" in row for row in rows))
            (root / "other").mkdir()
            (root / "other/sibling.txt").write_text("sibling\n")
            _, rows = terminal.frame(lambda raw, rows: "sibling.txt" in raw)
            selected_y = next(y for y, row in enumerate(rows[2:], 2) if "selected.txt" in row)
            terminal.send(f"\x1b[<35;20;{selected_y + 1}M".encode())
            raw, rows = terminal.frame(lambda raw, rows: mode(rows, "list") and "change.txt" in raw and "✦" not in raw and any("█│█" in row for row in rows) and any(row.startswith("›") and "selected.txt" in row for row in rows), timeout=12)
            check(raw, rows, "list")
            terminal.send(b"\t\t")
            raw, rows = terminal.frame(lambda raw, rows: mode(rows, "combined") and "change.txt" in raw and "✦" not in raw and any("█│█" in row for row in rows), timeout=12)
            check(raw, rows, "combined")
            top = divider(rows)
            assert top is not None
            selected_y = next(y for y, row in enumerate(rows[2:top], 2) if "change.txt" in row)
            terminal.send(f"\x1b[<35;30;{selected_y + 1}M".encode())
            def branch_colors(raw, rows):
                top = divider(rows)
                return [fg for (x, y), (char, fg, *_rest) in cells(raw).items() if top is not None and y > top and char == "┌"]
            terminal.frame(lambda raw, rows: branch_colors(raw, rows).count(97) == 1 and branch_colors(raw, rows).count((38, 2, 128, 128, 128)) == 1)
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0
            assert_terminal_restored(terminal.slave, terminal.before)
            print("PASS: readable default structure, medium-gray folders and white selected ancestors only")
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
