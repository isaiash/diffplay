#!/usr/bin/env python3
"""Exercise the workflow's publish command without touching GitHub."""
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[1]


class ReleaseTests(unittest.TestCase):
    def publish(self, exists=False, upload_fails=False):
        workflow = (ROOT / ".github/workflows/ci.yml").read_text()
        lines = workflow.split("- name: Publish release\n", 1)[1].splitlines()
        for index, line in enumerate(lines):
            if line.strip().startswith("run:"):
                command = line.strip().removeprefix("run:").strip()
                if command == "|":
                    command = textwrap.dedent("\n".join(lines[index + 1:]))
                break
        else:
            self.fail("Publish release step has no run command")
        with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache"))) as directory:
            root = Path(directory)
            (root / "dist").mkdir()
            (root / "dist/diffplay-linux-amd64").write_text("binary fixture")
            (root / "dist/diffplay-linux-amd64.sha256").write_text("checksum fixture")
            gh = root / "gh"
            gh.write_text('''#!/bin/sh
printf '%s\\n' "$*" >> "$LOG"
case "$1 $2" in
    'release view') [ "$EXISTS" = 1 ];;
    'release create')
        if [ "$EXISTS" = 1 ]; then
            echo 'a release with the same tag name already exists: v1.0.0' >&2
            exit 1
        fi;;
    'release upload') [ "$UPLOAD_FAILS" = 0 ];;
    *) exit 1;;
esac
''')
            gh.chmod(0o755)
            env = dict(os.environ, PATH=f"{root}:{os.environ['PATH']}", TAG="v1.0.0",
                       EXISTS=str(int(exists)), UPLOAD_FAILS=str(int(upload_fails)), LOG=str(root / "log"))
            result = subprocess.run(["bash", "-euo", "pipefail", "-c", command],
                                    cwd=root, env=env, text=True, capture_output=True)
            return result, (root / "log").read_text().splitlines()

    def test_existing_release_uploads_assets_instead_of_creating(self):
        result, calls = self.publish(exists=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(any(call.startswith("release upload v1.0.0 ") and "--clobber" in call for call in calls))
        self.assertFalse(any(call.startswith("release create ") for call in calls))

    def test_missing_release_is_created_with_assets(self):
        result, calls = self.publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(any(call.startswith("release create v1.0.0 dist/") and "--verify-tag" in call for call in calls))

    def test_upload_failure_is_not_hidden(self):
        result, _ = self.publish(exists=True, upload_fails=True)
        self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main(verbosity=2)
