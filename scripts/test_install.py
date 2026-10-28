#!/usr/bin/env python3
"""Installer regression tests with isolated compiler/tool-manager fixtures."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

INSTALLER = Path(__file__).resolve().parents[1] / "install.sh"


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache")))
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.tool = self.root / "installed go"
        (self.tool / "bin").mkdir(parents=True)
        self.env = dict(os.environ, PATH=f"{self.bin}:/usr/bin:/bin", PREFIX=str(self.root / "prefix with spaces"),
                        TMPDIR=str(self.root), TEST_GO_ROOT=str(self.tool), TEST_LOG=str(self.root / "calls"))

    def executable(self, path, content):
        path.write_text("#!/bin/sh\nset -eu\n" + content)
        path.chmod(0o755)

    def compiler(self, path):
        self.executable(path, '''
if [ "$1" = version ]; then printf 'go version go1.27.3 linux/amd64\n'; exit 0; fi
printf 'build\n' >> "$TEST_LOG"
while [ "$#" -gt 0 ]; do
    if [ "$1" = -o ]; then shift; printf '#!/bin/sh\\nprintf "fixture executable\\\\n"\\n' > "$1"; exit 0; fi
    shift
done
exit 1
''')

    def invoke(self):
        return subprocess.run(["/bin/sh", str(INSTALLER)], env=self.env, text=True, capture_output=True)

    def assert_installed(self, result):
        self.assertEqual(result.returncode, 0, result.stderr)
        target = Path(self.env["PREFIX"]) / "bin/diffplay"
        self.assertTrue(os.access(target, os.X_OK))
        self.assertEqual((self.root / "calls").read_text(), "build\n")

    def test_working_go_does_not_consult_mise(self):
        self.compiler(self.bin / "go")
        self.executable(self.bin / "mise", "printf 'unexpected mise call' >&2; exit 1\n")
        self.assert_installed(self.invoke())

    def test_inactive_go_shim_uses_installed_mise_tool(self):
        self.executable(self.bin / "go", "printf 'No version is set for shim: go' >&2; exit 1\n")
        self.compiler(self.tool / "bin/go")
        self.executable(self.bin / "mise", '''
case "$1" in
 latest) [ "$2" = --installed ] && [ "$3" = go ]; printf '1.27.3\n' ;;
 where) [ "$2" = go@1.27.3 ]; printf '%s\n' "$TEST_GO_ROOT" ;;
 *) printf 'must not install tools or change configuration' >&2; exit 1 ;;
esac
''')
        self.assert_installed(self.invoke())

    def test_unusable_go_reports_actionable_error(self):
        self.executable(self.bin / "go", "exit 1\n")
        self.executable(self.bin / "mise", "exit 1\n")
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Go 1.24 or newer", result.stderr)
        self.assertFalse((self.root / "calls").exists())


if __name__ == "__main__":
    unittest.main(verbosity=2)
