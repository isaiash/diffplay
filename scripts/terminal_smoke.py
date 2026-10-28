#!/usr/bin/env python3
"""Exercise the real terminal executable through a Linux pseudo-terminal."""
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
        assert "one/two/three/four/five/level-six.txt" not in names
        print("PASS: current-directory root, hidden files, depth-five traversal")

        master, slave = pty.openpty()
        before = termios.tcgetattr(slave)
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 36, 120, 0, 0))
        proc = subprocess.Popen([binary, "--view", "graph", "--interval", "100ms"], cwd=root,
                                stdin=slave, stdout=slave, stderr=slave, start_new_session=True)
        buffer = ""
        all_tail = ""

        def frame(predicate=lambda raw, rows: True, timeout=8):
            nonlocal buffer, all_tail
            deadline = time.monotonic() + timeout
            while time.monotonic() < deadline:
                ready, _, _ = select.select([master], [], [], 0.1)
                if ready:
                    chunk = os.read(master, 262144).decode("utf-8", "replace")
                    buffer += chunk
                    all_tail = (all_tail + chunk)[-300000:]
                    while "\x1b[H" in buffer:
                        start = buffer.index("\x1b[H")
                        end = buffer.find("\x1b[0m", start)
                        if end < 0:
                            buffer = buffer[start:]
                            break
                        raw = buffer[start:end + 4]
                        buffer = buffer[end + 4:]
                        rows = ANSI.sub("", raw).split("\r\n")
                        if predicate(raw, rows):
                            return raw, rows
                if proc.poll() is not None:
                    raise AssertionError(f"application exited early: {proc.returncode}: {all_tail[-2000:]}")
            raise AssertionError("timed out waiting for expected terminal frame: " + ANSI.sub("", all_tail[-5000:]))

        def send(data):
            os.write(master, data)

        try:
            frame(lambda raw, rows: "Diffplay" in raw)
            frame(lambda raw, rows: "tracked.txt" not in raw and "CLUSTERS" in raw)
            (root / "new.txt").write_text("new file\n")
            raw, rows = frame(lambda raw, rows: "1 files" in raw and "38;2;139;202;255" in raw)
            assert "new.txt" not in raw
            print("PASS: new files appear with a light-blue change burst")
            (root / "tracked.txt").write_text("new content\n")
            frame(lambda raw, rows: "2 files" in raw and "38;2;115;211;153" in raw)
            send(b" ")  # Freeze positions for repeatable hit testing.
            raw, rows = frame(lambda raw, rows: "LAYOUT PAUSED" in raw and "2 files" in raw and any(char in raw for char in "█▀▄"))
            assert "new.txt" not in raw and "tracked.txt" not in raw
            assert "●" not in raw and "38;2;51;51;61" not in raw
            centers = [(x, y) for y, row in enumerate(rows[2:-2], 2) for x, char in enumerate(row) if char in "█▀▄"]
            inspected = False
            for x, y in centers:
                send(b"\x1b[<35;1;1M")
                frame(lambda raw, rows: "new.txt" not in raw and "tracked.txt" not in raw)
                send(f"\x1b[<35;{x + 1};{y + 1}M".encode())
                # Use a short timeout on edge cells whose center may lie just
                # outside the circular hit radius; keep trying solid pixels.
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
            frame(lambda raw, rows: "1 deleted" in raw and "38;2;244;92;105" in raw)
            print("PASS: deleted files become solid red bubbles")
            (root / "new-directory").mkdir()
            (root / "new-directory/child.txt").write_text("child\n")
            raw, _ = frame(lambda raw, rows: "new-directory/" in raw and "3 files" in raw)
            assert "child.txt" not in raw
            (root / "new-directory/child.txt").unlink()
            (root / "new-directory").rmdir()
            frame(lambda raw, rows: "2 deleted" in raw)
            print("PASS: directory creation and recursive deletion update nodes")
            for width, height in [(80, 24), (160, 48), (20, 8), (120, 36)]:
                fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
                proc.send_signal(signal.SIGWINCH)
                _, rows = frame(lambda raw, rows: len(rows) == height and all(len(row) == width for row in rows))
                assert len(rows) == height
            print("PASS: graph resizes live to 80x24, 160x48, 20x8, and 120x36")
            send(b"q")
            proc.wait(timeout=5)
            while select.select([master], [], [], 0.1)[0]:
                all_tail += os.read(master, 262144).decode("utf-8", "replace")
            assert proc.returncode == 0
            assert "\x1b[?1006l" in all_tail and "\x1b[?1049l" in all_tail
            assert termios.tcgetattr(slave) == before
            print("PASS: clean exit restores terminal attributes, mouse, and alternate screen")
        finally:
            if proc.poll() is None:
                proc.kill()
                proc.wait()
            os.close(master)
            os.close(slave)


if __name__ == "__main__":
    main()
