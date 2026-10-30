#!/usr/bin/env python3
"""Exercise the real terminal executable through a Linux pseudo-terminal."""
import codecs
import fcntl
import json
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

from list_smoke import BLUE, GREEN, RED, WHITE, assert_terminal_restored, braille_cells, mode

ANSI = re.compile(r"\x1b\[[0-?]*[ -/]*[@-~]")


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    scratch = os.environ.get("TMPDIR", str(Path.home() / ".cache"))
    with tempfile.TemporaryDirectory(prefix="diffplay-smoke-", dir=scratch) as directory:
        root = Path(directory)
        (root / "tracked.txt").write_text("old content\n")
        (root / ".hidden").write_text("visible\n")
        deep = root / "one/two/three/four"
        deep.mkdir(parents=True)
        (deep / "level-five.txt").write_text("included\n")
        (deep / "five").mkdir()
        (deep / "five/level-six.txt").write_text("excluded\n")
        subprocess.check_call(["git","-C",str(root),"init","-q"])
        subprocess.check_call(["git","-C",str(root),"add","."])
        subprocess.check_call(["git","-C",str(root),"-c","user.name=Graph Test","-c","user.email=graph@example.invalid","commit","-qm","baseline"])
        listing = json.loads(subprocess.check_output([binary, "--snapshot"], cwd=root))
        names = {entry["path"] for entry in listing}
        assert ".hidden" in names and "one/two/three/four/level-five.txt" in names
        assert "one/two/three/four/five/level-six.txt" in names
        print("PASS: current-directory root, hidden files, full-depth Git traversal")

        master, slave = pty.openpty()
        before = termios.tcgetattr(slave)
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 36, 120, 0, 0))
        proc = subprocess.Popen([binary, "--view", "graph", "--interval", "100ms"], cwd=root,
                                stdin=slave, stdout=slave, stderr=slave, start_new_session=True)
        buffer = ""
        all_tail = ""
        last_frame = None
        decoder = codecs.getincrementaldecoder("utf-8")("replace")

        def frame(predicate=lambda raw, rows: True, timeout=8):
            nonlocal buffer, all_tail, last_frame
            deadline = time.monotonic() + timeout
            while time.monotonic() < deadline:
                ready, _, _ = select.select([master], [], [], 0.1)
                if ready:
                    chunk = decoder.decode(os.read(master, 262144))
                    buffer += chunk
                    all_tail = (all_tail + chunk)[-300000:]
                    while "\x1b[H" in buffer:
                        start = buffer.index("\x1b[H")
                        reset_start = start + len("\x1b[H")
                        if buffer.startswith("\x1b[0m", reset_start):
                            reset_start += len("\x1b[0m")
                        end = buffer.find("\x1b[0m", reset_start)
                        if end < 0:
                            buffer = buffer[start:]
                            break
                        raw = buffer[start:end + 4]
                        buffer = buffer[end + 4:]
                        rows = ANSI.sub("", raw).split("\r\n")
                        last_frame = raw, rows
                        if predicate(raw, rows):
                            return raw, rows
                if proc.poll() is not None:
                    raise AssertionError(f"application exited early: {proc.returncode}: {all_tail[-2000:]}")
                if last_frame is not None and predicate(*last_frame):
                    return last_frame
            raise AssertionError("timed out waiting for expected terminal frame: " + ANSI.sub("", all_tail[-5000:]))

        def send(data):
            os.write(master, data)

        try:
            raw, _ = frame(lambda raw, rows: mode(rows, "graph") and "tracked.txt" not in raw and "2 files" in rows[-1])
            assert "one/" not in raw and "./" not in raw, "inactive nested folders and implicit root must be absent"
            (root / "new.txt").write_text("new file\n")
            frame(lambda raw, rows: "new.txt" in raw and "3 files" in rows[-1] and any(
                color == WHITE
                for _, _, _, color in braille_cells(raw)))
            print("PASS: live inspection selects white graph glyphs")
            raw, rows = frame(lambda raw, rows: "3 files" in rows[-1] and any(color == BLUE for _, _, _, color in braille_cells(raw)))
            assert "new.txt" in raw, "a live burst must select its file for inspection"
            print("PASS: new files render fixed blue graph glyphs")
            (root / "tracked.txt").write_text("new content\n")
            frame(lambda raw, rows: "3 files" in rows[-1] and any(color == GREEN for _, _, _, color in braille_cells(raw)))
            send(b"\x1b[<35;1;1M")  # Leave automatic inspection before hit testing.
            frame(lambda raw, rows: "new.txt" not in raw and "tracked.txt" not in raw)
            send(b" ")  # Freeze positions for repeatable hit testing.
            raw, rows = frame(lambda raw, rows: "PAUSED" in rows[-1] and "3 files" in rows[-1] and any("\u2801" <= char <= "\u28ff" for char in raw))
            assert "new.txt" not in raw and "tracked.txt" not in raw
            assert "●" not in raw and not any(char in raw for char in "█▀▄")
            centers = [(x, y) for y, row in enumerate(rows[2:-2], 2) for x, char in enumerate(row) if "\u2801" <= char <= "\u28ff"]
            inspected = False
            for x, y in centers:
                send(b"\x1b[<35;1;1M")
                frame(lambda raw, rows: "new.txt" not in raw and "tracked.txt" not in raw)
                send(f"\x1b[<35;{x + 1};{y + 1}M".encode())
                # Use a short timeout on edge cells whose center may lie just
                # outside the circular hit radius; keep trying dotted cells.
                try:
                    raw, _ = frame(lambda raw, rows: "new.txt" in raw or "tracked.txt" in raw, timeout=.5)
                except AssertionError:
                    continue
                if "-old content" in raw and "+new content" in raw:
                    inspected = True
                    break
            assert inspected, "hover must show the edited file's actual diff"
            print("PASS: mouse hover displays the actual previous-version diff")
            (root / "tracked.txt").unlink()
            send(b"\x1b[<35;1;1M")
            frame(lambda raw, rows: "3 files" in rows[-1] and any(color == RED for _, _, _, color in braille_cells(raw)))
            print("PASS: deleted files become red dotted cells")
            (root / "new").mkdir()
            (root / "new/child.txt").write_text("child\n")
            raw, _ = frame(lambda raw, rows: "new/" in raw and "4 files" in rows[-1])
            frame(lambda raw, rows: "child.txt" in raw and "4 files" in rows[-1])
            (root / "new/child.txt").unlink()
            (root / "new").rmdir()
            frame(lambda raw, rows: "3 files" in rows[-1] and "new/" not in raw)
            print("PASS: directory creation and recursive deletion update nodes")
            for width, height in [(80, 24), (160, 48), (20, 8), (120, 36)]:
                fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
                proc.send_signal(signal.SIGWINCH)
                _, rows = frame(lambda raw, rows: len(rows) == height and all(len(row) == width for row in rows))
                assert len(rows) == height
            print("PASS: graph resizes live to 80x24, 160x48, 20x8, and 120x36")
            # Drain frames until change-triggered bursts have finished.
            deadline = time.monotonic() + 4.2
            while time.monotonic() < deadline:
                frame()
            for title in ["combined", "list", "graph"]:
                send(b"\t")
                raw, _ = frame(lambda raw, rows: mode(rows, title))
                if title == "graph":
                    # The preceding Hex check can generate a live edit burst.
                    deadline = time.monotonic() + 4.2
                    while time.monotonic() < deadline:
                        frame()
                    def glyphs(rows):
                        return {(x, y, char) for y, row in enumerate(rows[2:-2], 2)
                                for x, char in enumerate(row)
                                if "\u2801" <= char <= "\u28ff"}

                    # Shared list selection can carry a halo/inspector into the
                    # full graph. Clear it before testing camera-only motion.
                    send(b"\x1b[<35;1;1M")
                    baseline_raw, baseline = frame(lambda raw, rows: mode(rows, title) and not any(name in raw for name in [".hidden", "new.txt", "tracked.txt", "child.txt"]))
                    original = glyphs(baseline)
                    assert original, "expected graph glyphs"
                    send(b"\x1b[<0;2;3M\x1b[<32;5;5M\x1b[<0;5;5m")
                    expected = {(x + 3, y + 2, char) for x, y, char in original
                                if 2 <= y + 2 < len(baseline) - 2 and x + 3 < 120}
                    try:
                        frame(lambda raw, rows: mode(rows, title) and glyphs(rows) == expected)
                    except AssertionError:
                        actual_raw, actual_rows = frame()
                        Path(scratch, "diffplay-pan-baseline.ansi").write_text(baseline_raw)
                        Path(scratch, "diffplay-pan-actual.ansi").write_text(actual_raw)
                        actual = glyphs(actual_rows)
                        print("Pan raster mismatch: missing", sorted(expected - actual), "extra", sorted(actual - expected))
                        raise
                    send(b"r")
                    frame(lambda raw, rows: mode(rows, title) and glyphs(rows) == original)
                    send(b"\x1b[<64;30;12M")
                    frame(lambda raw, rows: mode(rows, title) and glyphs(rows) != original)
                    send(b"r")
                    frame(lambda raw, rows: mode(rows, title) and glyphs(rows) == original)
                    # Exercise fragmented wheel input and reset through the real PTY.
                    send(b"\x1b[<64;30;")
                    send(b"12M\x1b[<65;30;12M")
                    raw, _ = frame(lambda raw, rows: mode(rows, title) and glyphs(rows) == original)
                    print(f"PASS: {title} mouse pan, reset, and fragmented wheel input")
                if title == "graph":
                    assert "new.txt" not in raw and "tracked.txt" not in raw
                    stable, _ = frame(lambda raw, rows: mode(rows, title))
                    assert stable == raw, "idle clusters must be stationary"
                    send(b"nn")  # .hidden, then new.txt in stable path order
                    raw, _ = frame(lambda raw, rows: mode(rows, title) and "new.txt" in raw)
                    assert "+new file" in raw
                    if title == "graph":
                        assert "1 dot" in raw
                        (root / "new.txt").write_text("live cluster edit\n")
                        frame(lambda raw, rows: mode(rows, title) and "+live cluster edit" in raw)
                    send(b"\x1b[<35;1;1M")
                    frame(lambda raw, rows: mode(rows, title) and "new.txt" not in raw)
            print("PASS: Tab cycles graph/combined/list; idle positions and live diff inspection work")
            send(b"q")
            proc.wait(timeout=5)
            while select.select([master], [], [], 0.1)[0]:
                all_tail += decoder.decode(os.read(master, 262144))
            assert proc.returncode == 0
            assert "\x1b[?1006l" in all_tail and "\x1b[?1049l" in all_tail
            assert_terminal_restored(slave, before)
            print("PASS: clean exit restores terminal attributes, mouse, and alternate screen")
        finally:
            if proc.poll() is None:
                proc.kill()
                proc.wait()
            os.close(master)
            os.close(slave)


if __name__ == "__main__":
    main()
