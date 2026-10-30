#!/usr/bin/env python3
"""Git startup and live monitoring must include deeply nested source paths."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from list_smoke import Terminal, assert_terminal_restored, divider, mode

JAVA = "CentralControlManager/src/astri/cc/CentralControlManagerImpl/CentralControlManagerImpl.java"
DEEP = "deep/one/two/three/four/five/six/seven/eight/nine/deep-live.txt"


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args])


def main():
    binary = str(Path(sys.argv[1] if len(sys.argv) > 1 else "./bin/diffplay").resolve())
    with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as temp:
        root = Path(temp)
        baseline = {
            JAVA: "package astri.cc;\n// old implementation\n",
            "CentralControlManager/test/README_TESTS.md": "tests\nold instructions\n",
            DEEP: "old deep contents\n",
            "deep/one/two/three/four/five/six/seven/eight/nine/deleted.txt": "gone\n",
            ".gitignore": "ignored/\n",
        }
        for name, content in baseline.items():
            target = root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(content)
        git(root, "init", "-q")
        git(root, "add", ".")
        git(root, "-c", "user.name=Depth Test", "-c", "user.email=depth@example.invalid", "commit", "-qm", "baseline")
        (root / JAVA).write_text("package astri.cc;\n// new implementation\n")
        (root / "CentralControlManager/test/README_TESTS.md").write_text("tests\nnew instructions\n")
        (root / DEEP).write_text("new deep contents\n")
        (root / "deep/one/two/three/four/five/six/seven/eight/nine/deleted.txt").unlink()
        for name in ("CentralControlManager/test/astri/offline/OfflineTest.java", "CentralControlManager/test/run-offline-tests.py", "ignored/deep/secret.txt"):
            target = root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text("untracked\n")
        snapshot = json.loads(subprocess.check_output([binary, "--snapshot"], cwd=root))
        paths = {entry["path"] for entry in snapshot}
        assert JAVA in paths and DEEP in paths, "snapshot silently omitted deep Git paths"
        assert not any(name.startswith("ignored/") or name.startswith(".git/") for name in paths)
        print("PASS: full-depth Git snapshot includes Java/deeper source paths and prunes ignored metadata")
        for view in ("list", "combined"):
            terminal = Terminal(binary, root, view)
            terminal.resize(260, 80)
            try:
                raw, rows = terminal.frame(lambda raw, rows: mode(rows, view) and all(name in raw for name in ("CentralControlManagerImpl.java", "README_TESTS.md", "OfflineTest.java", "run-offline-tests.py", "deep-live.txt", "deleted.txt")))
                assert "✦" not in raw, "deep startup changes must remain quiet"
                assert "secret.txt" not in raw
                end = divider(rows) if view == "combined" else len(rows) - 1
                width = rows[1].rfind("│")
                assert width > 0
                y = next(y for y, row in enumerate(rows[2:end], 2) if "CentralControlManagerImpl.java" in row[:width])
                terminal.send(f"\x1b[<35;30;{y + 1}M".encode())
                terminal.frame(lambda raw, rows: "+// new implementation" in raw)
                print(f"PASS [{view}]: deep Java and shallow test changes both appear at startup with HEAD preview")
                marker = f"// live beyond depth five in {view}"
                with (root / JAVA).open("a") as output:
                    output.write(marker + "\n")
                terminal.frame(lambda raw, rows: "+" + marker in raw and "✦" in raw)
                newly_created = root / "deep/one/two/three/four/five/six/seven/eight/nine/new-deep.txt"
                newly_created.write_text("created live\n")
                terminal.frame(lambda raw, rows: "new-deep.txt" in raw and "+created live" in raw)
                newly_created.unlink()
                terminal.frame(lambda raw, rows: "new-deep.txt" not in raw)
                (root / DEEP).unlink()
                terminal.frame(lambda raw, rows: "deep-live.txt" in raw and "-old deep contents" in raw)
                terminal.send(b"q")
                terminal.proc.wait(timeout=5)
                assert terminal.proc.returncode == 0
                assert_terminal_restored(terminal.slave, terminal.before)
                print(f"PASS [{view}]: deep live edit/create/untracked removal/tracked deletion and terminal restoration")
            finally:
                terminal.close()
                (root / DEEP).write_text("new deep contents\n")


if __name__ == "__main__":
    main()
