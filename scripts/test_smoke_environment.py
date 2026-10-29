#!/usr/bin/env python3
"""Check smoke-test isolation when TMPDIR is absent, as on CI."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class SmokeEnvironmentTests(unittest.TestCase):
    def test_missing_tmpdir_creates_isolated_fixture_directory(self):
        scratch = os.environ.get("TMPDIR", str(Path.home() / ".cache"))
        with tempfile.TemporaryDirectory(dir=scratch) as temp:
            root = Path(temp)
            tools = root / "tools"
            tools.mkdir()
            for name, body in {
                "go": "exit 0\n",
                "python3": 'test -d "$TMPDIR" || exit 1\ncase "$TMPDIR" in "$HOME"/*) ;; *) exit 2 ;; esac\nprintf "%s\\n" "$1" >> "$SMOKE_LOG"\n',
            }.items():
                tool = tools / name
                tool.write_text("#!/bin/sh\n" + body)
                tool.chmod(0o755)
            log = root / "calls"
            env = dict(os.environ, HOME=str(root), PATH=f"{tools}:{os.environ['PATH']}", SMOKE_LOG=str(log))
            env.pop("TMPDIR", None)
            result = subprocess.run(["make", "smoke"], cwd=ROOT, env=env, text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(len(log.read_text().splitlines()), 9)


if __name__ == "__main__":
    unittest.main(verbosity=2)
