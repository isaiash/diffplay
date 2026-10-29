#!/usr/bin/env python3
"""Verify Git-seeded activity, ignore filtering, and the revised tree in a PTY."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

from list_smoke import BLUE, GREEN, RED, Terminal, header_backgrounds, header_prefix, mode


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], stderr=subprocess.PIPE)


def deletion_check(binary, parent):
    root = parent / "deletion-repo"
    root.mkdir()
    git(root, "init", "-q")
    (root / "tracked.txt").write_text("tracked first\ntracked second\n")
    git(root, "add", "tracked.txt")
    git(root, "-c", "user.name=PTY Test", "-c", "user.email=pty@example.invalid", "commit", "-qm", "baseline")
    terminal = Terminal(binary, root)
    try:
        terminal.resize(160, 48)
        _, rows = terminal.frame(lambda raw, rows: len(rows) == 48 and mode(rows, "list") and "No changes observed" in raw)
        clean = header_prefix(rows[0])
        tree = root / "untracked-tree" / "nested"
        tree.mkdir(parents=True)
        (tree / "one.txt").write_text("untracked one\n" * 3)
        (tree / "two.txt").write_text("untracked two\n" * 2)
        terminal.frame(lambda raw, rows: "one.txt" in raw and "two.txt" in raw and "+5" in header_prefix(rows[0]))
        shutil.rmtree(root / "untracked-tree")
        raw, rows = terminal.frame(lambda raw, rows: header_prefix(rows[0]) == clean and "No changes observed" in raw and "untracked-tree" not in raw and "one.txt" not in raw and "two.txt" not in raw)
        assert "deleted" not in raw and "-5" not in header_prefix(rows[0])
        assert git(root, "status", "--porcelain") == b""
        print("PASS: recursively deleting an untracked tree clears rows and totals without deletion history")
        (root / "tracked.txt").unlink()
        raw, rows = terminal.frame(lambda raw, rows: "tracked.txt" in raw and "-2" in header_prefix(rows[0]) and "-tracked first" in raw and "-tracked second" in raw)
        assert "deleted" in raw and b" D tracked.txt" in git(root, "status", "--porcelain")
        print("PASS: deleting a tracked file shows its removed lines and Git deletion")
        terminal.send(b"q")
        terminal.proc.wait(timeout=5)
        assert terminal.proc.returncode == 0
    finally:
        terminal.close()


def header_integration(binary, parent):
    """A full Git corpus, including a file beyond the graph traversal limit."""
    root = parent / "header-repo"
    root.mkdir()
    git(root, "init", "-q")
    deep = root / "one/two/three/four/five/six"
    deep.mkdir(parents=True)
    (deep / "context.txt").write_text("deep context\n" * 100)
    edited = root / "edited.txt"
    edited.write_text("keep0\nkeep1\nkeep2\nold\n")
    (root / "removed.txt").write_text("gone\n" * 3)
    git(root, "add", ".")
    git(root, "-c", "user.name=PTY Test", "-c", "user.email=pty@example.invalid", "commit", "-qm", "baseline")
    edited.write_text("keep0\nkeep1\nkeep2\nnew0\nnew1\n")
    (root / "removed.txt").unlink()
    (root / "new.txt").write_text("fresh\n" * 4)
    terminal = Terminal(binary, root)
    colors = [BLUE, GREEN, RED, None]

    def expect(label, blue, green, red, denominator):
        added = blue + green
        net = added - red
        metrics = (f" +{added}" if added else "") + (f" -{red}" if red else "")
        expected = f"{root.name}{metrics} {100 * net / denominator:.2f}% new lines"
        raw, rows = terminal.frame(lambda raw, rows: mode(rows, "list") and header_prefix(rows[0]) == expected)
        assert rows[0].endswith(f"{denominator} total lines"), (label, rows[0])
        assert str(root) not in rows[0], (label, rows[0])
        assert "|" not in rows[0], rows[0]
        width = len(rows[0])
        positive_net = max(net, 0)
        scale = max(added, 1)
        neutral = max(denominator - positive_net - red, 0)
        weights = [positive_net * blue, positive_net * green, red * scale, neutral * scale]
        quotients = [divmod(width * count, sum(weights)) for count in weights]
        lengths = [q for q, _ in quotients]
        order = sorted(range(4), key=lambda i: (-quotients[i][1], i))
        for i in order[:width - sum(lengths)]:
            lengths[i] += 1
        expected_bg = [color for color, n in zip(colors, lengths) for _ in range(n)]
        actual = header_backgrounds(raw)
        assert actual == expected_bg, (label, lengths, actual)
        print(f"PASS: {label}: {expected}, repository lines={denominator}, background widths={lengths}, total right-aligned")
        return raw

    try:
        terminal.resize(160, 48)
        raw = expect("startup", 4, 2, 4, 109)
        evidence = Path(os.environ.get("TMPDIR", str(Path.home() / ".cache"))) / "diffplay-header-frame.ansi"
        evidence.write_text(raw)
        print(f"Captured terminal frame: {evidence}")
        (root / "live.txt").write_text("live\n" * 2)
        expect("create", 6, 2, 4, 111)
        edited.write_text("keep0\nkeep1\nkeep2\nnew0\nnew1\nnew2\nnew3\n")
        expect("update", 6, 4, 4, 113)
        (root / "new.txt").unlink()
        expect("delete untracked", 2, 4, 4, 109)
        (deep / "context.txt").write_text("deep context\n" * 120)
        expect("deep update beyond graph traversal", 2, 4, 4, 129)
        (deep / "context.txt").write_text("deep context\n" * 100)
        expect("deep restore beyond graph traversal", 2, 4, 4, 109)
        (root / "live.txt").unlink()
        edited.write_text("keep0\nkeep1\nkeep2\n")
        expect("negative net", 0, 4, 8, 103)
        terminal.send(b"q")
        terminal.proc.wait(timeout=5)
        assert terminal.proc.returncode == 0
    finally:
        terminal.close()


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
            raw, rows = terminal.frame(lambda raw, rows: all(name in raw for name in ["existing.txt", "staged.txt", "deleted.txt", "new.txt", "real.txt"]) and f"+{expected_added}" in header_prefix(rows[0]) and f"-{expected_removed}" in header_prefix(rows[0]))
            assert "✦" not in raw, "startup Git data must not flash"
            assert not any("repo/" in row for row in rows[2:-2]), rows
            assert not any("secret.txt" in row or "scratch.cache" in row for row in rows)
            assert not any("+0" in row or "-0" in row for row in rows[2:-2]), rows
            print("PASS: startup seeds staged + unstaged + untracked + deleted changes without double counting")
            print("PASS: zero cells are blank and the tree starts at depth one")
            # An ignored write must not create an activity row or change totals.
            (root / "ignored/secret.txt").write_text("ignored new content\n" * 30)
            terminal.frame(lambda raw, rows: f"+{expected_added}" in header_prefix(rows[0]) and "secret.txt" not in raw)
            # A previously visible untracked file becomes ignored while running.
            (root / "new.txt").write_text("live addition\n" * 5)
            terminal.frame(lambda raw, rows: "✦" in raw and "new.txt" in raw)
            ignore = root / ".gitignore"
            ignore.write_text(ignore.read_text() + "new.txt\n")
            terminal.frame(lambda raw, rows: "new.txt" not in raw and ".gitignore" in raw)
            print("PASS: live ignore-rule edits remove activity without false deletion rows")
            terminal.send(b"\t")
            raw, rows = terminal.frame(lambda raw, rows: mode(rows, "graph") and "6 files" in rows[-1])
            assert "clean.txt" not in raw
            # Leave automatic live inspection; n/p must include clean context.
            terminal.send(b"\x1b[<35;1;1M")
            terminal.frame(lambda raw, rows: ".gitignore" not in raw and "clean.txt" not in raw)
            terminal.send(b"n")
            terminal.frame(lambda raw, rows: ".gitignore" in raw)
            terminal.send(b"n")
            terminal.frame(lambda raw, rows: "clean.txt" in raw and "unchanged" in rows[3])
            terminal.send(b"p")
            terminal.frame(lambda raw, rows: ".gitignore" in raw and "clean.txt" not in raw)
            print("PASS: HEX includes unchanged Git context, hidden names, and n/p inspection")
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: mode(rows, "combined"))
            terminal.send(b"\t")
            terminal.frame(lambda raw, rows: mode(rows, "list") and "staged.txt" in raw)
            terminal.send(b"q")
            terminal.proc.wait(timeout=5)
            assert terminal.proc.returncode == 0
            print("PASS: Git-seeded state survives view switching and clean exit")
        finally:
            terminal.close()
        deletion_check(binary, Path(temp))
        header_integration(binary, Path(temp))


if __name__ == "__main__":
    main()
