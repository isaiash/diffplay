#!/usr/bin/env python3
"""Real-PTY coverage of the default vertically combined changes/Hex view."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import termios

from list_smoke import GREEN, WHITE, Terminal, braille_cells, colored_cells, divider, header_prefix, mode


def geometry(rows):
    height, width = len(rows), len(rows[0])
    midpoint = divider(rows)
    assert midpoint is not None
    assert "new lines" in rows[0]
    assert all("q quit" not in row and "Tab view" not in row for row in rows[:-1])
    assert "q quit" in rows[-1] and "Tab view" in rows[-1]
    assert not any("Diffplay  " in row for row in rows)
    assert all(len(row) == width for row in rows)
    return midpoint


def selected_row(rows, name, midpoint):
    return any(row.startswith("›") and name in row for row in rows[1:midpoint])


def graph_glyphs(rows, midpoint):
    return {(x, y, char) for y, row in enumerate(rows) if midpoint + 1 <= y < len(rows) - 1
            for x, char in enumerate(row) if "\u2801" <= char <= "\u28ff"}


def full_circle_selection(binary, parent):
    root = parent / "single-cell-repo"
    root.mkdir()
    subprocess.check_call(["git", "-C", str(root), "init", "-q"])
    (root / "only.txt").write_text("old\n" * 10)
    subprocess.check_call(["git", "-C", str(root), "add", "."])
    subprocess.check_call(["git", "-C", str(root), "-c", "user.name=PTY Test", "-c", "user.email=pty@example.invalid", "commit", "-qm", "baseline"])
    (root / "only.txt").write_text("new\n" * 20)
    terminal = Terminal(binary, root, view=None)
    try:
        terminal.resize(160, 48)
        terminal.frame(lambda raw, rows: len(rows) == 48 and mode(rows, "combined") and "only.txt" in raw)
        terminal.send(b"\x1b[<35;1;26M")  # Clear the automatically selected first row.
        raw, rows = terminal.frame(lambda raw, rows: len(rows) == 48 and mode(rows, "combined") and any(y > divider(rows) and color != WHITE for _, y, _, color in braille_cells(raw)))
        midpoint = geometry(rows)
        before = {(x, y): (char, color) for x, y, char, color in braille_cells(raw) if y > midpoint}
        assert before and any(color != WHITE for _, color in before.values())
        y = next(y for y, row in enumerate(rows[1:midpoint], 1) if "only.txt" in row)
        terminal.send(f"\x1b[<35;30;{y + 1}M".encode())
        raw, rows = terminal.frame(lambda raw, rows: selected_row(rows, "only.txt", midpoint))
        after = {(x, y): (char, color) for x, y, char, color in braille_cells(raw) if y > midpoint}
        assert set(before) == set(after), "selection changed circle geometry"
        assert all(after[pos] == (char, WHITE) for pos, (char, _) in before.items()), "entire selected circle must be white with identical masks"
        preview = [row[80:] for row in rows[1:midpoint]]
        terminal.send(b"JJJJ")
        terminal.frame(lambda raw, rows: [row[80:] for row in rows[1:midpoint]] != preview)
        terminal.send(b"KKKK")
        terminal.frame(lambda raw, rows: [row[80:] for row in rows[1:midpoint]] == preview)
        print("PASS: upper diff preview scrolls and returns with J/K")
        terminal.send(b"q")
        terminal.proc.wait(timeout=5)
        assert terminal.proc.returncode == 0 and termios.tcgetattr(terminal.slave) == terminal.before
        print(f"PASS: all {len(before)} glyphs of the isolated selected circle turn white without mask or position changes")
    finally:
        terminal.close()


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    scratch = os.environ.get("TMPDIR", str(Path.home() / ".cache"))
    with tempfile.TemporaryDirectory(prefix="diffplay-unified-", dir=scratch) as temp:
        full_circle_selection(binary, Path(temp))
        root = Path(temp) / "repo"
        (root / "active/nested").mkdir(parents=True)
        (root / "inactive/deep").mkdir(parents=True)
        (root / "active/nested/changed.txt").write_text("old changed\n")
        (root / "active/nested/other.txt").write_text("old other\n")
        (root / "active/nested/context.txt").write_text("quiet context\n" * 6)
        (root / "inactive/deep/hidden.txt").write_text("inactive context\n")
        (root / "root-context.txt").write_text("root context\n")
        subprocess.check_call(["git", "-C", str(root), "init", "-q"])
        subprocess.check_call(["git", "-C", str(root), "add", "."])
        subprocess.check_call(["git", "-C", str(root), "-c", "user.name=PTY Test", "-c", "user.email=pty@example.invalid", "commit", "-qm", "baseline"])
        (root / "active/nested/changed.txt").write_text("new changed\n" * 3)
        (root / "active/nested/other.txt").write_text("new other\n" * 2)
        terminal = Terminal(binary, root, view=None)  # Test the actual CLI default.
        try:
            terminal.resize(160, 48)
            raw, rows = terminal.frame(lambda raw, rows: len(rows) == 48 and mode(rows, "combined") and "changed.txt" in raw and "other.txt" in raw)
            midpoint = geometry(rows)
            assert midpoint == (48 - 2) // 2
            assert "inactive/" not in raw and "deep/" not in raw and "./" not in raw
            assert "nested/" in raw and "active/nested/" not in "\n".join(rows[midpoint:])
            assert "✦" not in raw and "�" not in raw
            totals = header_prefix(rows[0])
            assert totals == "repo +5 -2 23.08% new lines"
            assert rows[0].endswith("13 total lines") and str(root) not in rows[0]
            print("PASS: default changes/diff top, Hex bottom, exact half split and quiet startup")

            # Selection must recolor existing circle glyphs without altering their masks.
            before_cells = {(x, y): (char, color) for x, y, char, color in braille_cells(raw) if y > midpoint}
            # Select a list file using full-terminal mouse coordinates.
            y = next(y for y, row in enumerate(rows[1:midpoint], 1) if "changed.txt" in row)
            terminal.send(f"\x1b[<35;30;{y + 1}M".encode())
            raw, rows = terminal.frame(lambda raw, rows: selected_row(rows, "changed.txt", midpoint) and "+new changed" in "\n".join(rows[:midpoint]))
            assert any("+new changed" in row and row.index("+new changed") > len(row) // 2 for row in rows[:midpoint]), "diff must be in the upper right pane"
            white = [(x, y, char) for x, y, char, color in braille_cells(raw) if y > midpoint and color == WHITE]
            assert white, "selected file must have white Braille cells"
            assert all((x, y) in before_cells and before_cells[x, y][0] == char for x, y, char in white), "selection must not change circle symbols/masks"
            assert any(before_cells[x, y][1] != WHITE for x, y, _ in white), "selection must recolor existing cells"
            white_frames = [(x, y, char) for x, y, char, color in colored_cells(raw) if y > midpoint and char in "┌┐└┘─│" and color == WHITE]
            assert sum(char == "┌" for _, _, char in white_frames) == 2, "both ancestor frames must turn white"
            evidence = Path(scratch, "diffplay-combined-frame.ansi")
            evidence.write_text(raw)
            print(f"Captured combined frame: {evidence}")
            print("PASS: list selection drives upper-right diff, white Braille cell and both ancestor borders")

            # Search real painted graph cells; no fabricated node coordinates.
            def hover_file(name, color):
                nonlocal raw, rows
                points = [(x, y) for x, y, _, fg in braille_cells(raw) if y > midpoint + 2 and fg == color]
                for x, y in points:
                    terminal.send(f"\x1b[<35;{x + 1};{y + 1}M".encode())
                    try:
                        raw, rows = terminal.frame(lambda raw, rows: name in "\n".join(rows[:midpoint]) and (selected_row(rows, name, midpoint) if name == "other.txt" else "unchanged" in "\n".join(rows[:midpoint])), timeout=.4)
                        return
                    except AssertionError:
                        continue
                raise AssertionError(f"could not inspect {name} from its lower-pane graph glyphs")

            hover_file("other.txt", GREEN)
            assert "+new other" in "\n".join(rows[:midpoint])
            assert header_prefix(rows[0]) == totals
            print("PASS: lower graph hover synchronizes changed list row and upper diff")
            hover_file("context.txt", None)
            assert header_prefix(rows[0]) == totals, "context inspection must not alter history totals"
            assert selected_row(rows, "context.txt", midpoint), "context inspection must temporarily reveal its list row"
            terminal.send(f"\x1b[<35;1;{midpoint + 3}M".encode())
            raw, rows = terminal.frame(lambda raw, rows: "active/nested/context.txt" not in "\n".join(rows[:midpoint]))
            assert header_prefix(rows[0]) == totals
            assert not selected_row(rows, "context.txt", midpoint), "temporary context row must disappear when deselected"
            print("PASS: context hover is temporary and does not enter list/history totals")

            # Clear hover before exact raster comparisons: halos are selection UI.
            original = graph_glyphs(rows, midpoint)
            assert original
            terminal.send(f"\x1b[<0;2;{midpoint + 4}M\x1b[<32;5;{midpoint + 6}M\x1b[<0;5;{midpoint + 6}m".encode())
            expected = {(x + 3, y + 2, char) for x, y, char in original if x + 3 < 160 and y + 2 < 47}
            raw, rows = terminal.frame(lambda raw, rows: graph_glyphs(rows, midpoint) == expected)
            terminal.send(b"r")
            terminal.frame(lambda raw, rows: graph_glyphs(rows, midpoint) == original)
            terminal.send(f"\x1b[<64;40;{midpoint + 10}M".encode())
            terminal.frame(lambda raw, rows: graph_glyphs(rows, midpoint) != original)
            terminal.send(b"r")
            terminal.frame(lambda raw, rows: graph_glyphs(rows, midpoint) == original)
            print("PASS: lower-pane drag translates exact raster; pointer wheel zoom and reset work")

            # Real SGR press, motion and release, moving the divider both ways.
            for target in (15, 31):
                terminal.send(f"\x1b[<0;5;{midpoint + 1}M\x1b[<32;5;{target + 1}M\x1b[<0;5;{target + 1}m".encode())
                raw, rows = terminal.frame(lambda raw, rows: divider(rows) == target)
                midpoint = geometry(rows)
                assert header_prefix(rows[0]) == totals
                assert graph_glyphs(rows, midpoint), "resized lower pane must still render graph"
                y = next(y for y, row in enumerate(rows[1:midpoint], 1) if "changed.txt" in row[:79])
                terminal.send(f"\x1b[<35;30;{y + 1}M".encode())
                raw, rows = terminal.frame(lambda raw, rows: divider(rows) == target and selected_row(rows, "changed.txt", target) and "+new changed" in "\n".join(rows[:target]))
            print("PASS: real mouse drag moves the divider up and down without losing totals or graph")
            for width, height in [(100, 30), (80, 24), (40, 12), (160, 48)]:
                terminal.resize(width, height)
                expected = max(3, min(height - 5, int(31 / 48 * height + .5)))
                raw, rows = terminal.frame(lambda raw, rows: len(rows) == height and all(len(row) == width for row in rows) and divider(rows) == expected)
                geometry(rows)
                assert "+5" in header_prefix(rows[0]) and "-2" in header_prefix(rows[0])
            print("PASS: divider proportion and pane geometry persist across real SIGWINCH")
            for title in ["list", "graph", "combined"]:
                terminal.send(b"\t")
                raw, rows = terminal.frame(lambda raw, rows: mode(rows, title))
            geometry(rows)
            assert divider(rows) == 31
            assert header_prefix(rows[0]) == totals
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0 and termios.tcgetattr(terminal.slave) == terminal.before
            print("PASS: Tab combined/list/graph cycle preserves history; clean exit restores terminal")
        finally:
            terminal.close()


if __name__ == "__main__":
    main()
