#!/usr/bin/env python3
"""Exercise the piped installer without changing host packages or using the network."""
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
INSTALLER = ROOT / "quickinstall.sh"


class QuickInstallTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR", str(Path.home() / ".cache")))
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        for tool in ("bash", "sh", "mkdir", "mktemp", "rm", "tar", "gzip", "install", "chmod", "dirname", "sha256sum", "shasum"):
            location = shutil.which(tool)
            if location:
                (self.bin / tool).symlink_to(location)
        self.env = dict(os.environ, HOME=str(self.root), PATH=str(self.bin),
                        TMPDIR=str(self.root), PREFIX=str(self.root / "prefix with spaces"),
                        TEST_ROOT=str(self.root), TEST_BIN=str(self.bin), TEST_OS="Linux", TEST_ARCH="x86_64")
        self.executable("uname", 'case "$1" in -s) echo "$TEST_OS";; -m) echo "$TEST_ARCH";; esac\n')
        self.executable("id", "echo 1000\n")
        self.executable("git", "echo 'git version 2.49.0'\n")
        self.compiler(self.bin / "go")
        self.executable("curl", '''
url= output=
while [ "$#" -gt 0 ]; do
    case "$1" in -o|--output) shift; output=$1;; https://*) url=$1;; esac
    shift
done
printf '%s\n' "$url" >> "$TEST_ROOT/downloads"
case "$url" in
    *VERSION*) printf 'go1.27.3\ntime 2026-10-19T00:00:00Z\n';;
    *.sha256)
        if [ "${TEST_BAD_CHECKSUM:-}" = 1 ]; then
            echo '1111111111111111111111111111111111111111111111111111111111111111'
        else
            printf '%s\n' "$TEST_GO_SHA256"
        fi;;
    *go1.27.3.*.tar.gz) install "$TEST_ROOT/go.tar.gz" "$output";;
    *diffplay*/tar.gz/*) install "$TEST_ROOT/source.tar.gz" "$output";;
    *) echo "Unexpected download: $url" >&2; exit 1;;
esac
''')
        self.source_archive()
        self.go_archive()

    def executable(self, name, content):
        path = self.bin / name
        if path.is_symlink():
            path.unlink()
        path.write_text("#!/bin/sh\nset -eu\n" + content)
        path.chmod(0o755)
        return path

    def compiler(self, path, version="1.27.3"):
        path.write_text('''#!/bin/sh
set -eu
if [ "$1" = version ]; then echo 'go version goVERSION linux/amd64'; exit 0; fi
while [ "$#" -gt 0 ]; do
    if [ "$1" = -o ]; then
        shift
        printf '#!/bin/sh\necho "diffplay 0.6.0"\n' > "$1"
        exit 0
    fi
    shift
done
exit 1
'''.replace("VERSION", version))
        path.chmod(0o755)

    def source_archive(self):
        with tarfile.open(self.root / "source.tar.gz", "w:gz") as archive:
            archive.add(ROOT / "install.sh", arcname="diffplay-master/install.sh")
            archive.add(ROOT / "go.mod", arcname="diffplay-master/go.mod")

    def go_archive(self):
        compiler = self.root / "downloaded-go"
        self.compiler(compiler)
        with tarfile.open(self.root / "go.tar.gz", "w:gz") as archive:
            archive.add(compiler, arcname="go/bin/go")
        self.env["TEST_GO_SHA256"] = hashlib.sha256((self.root / "go.tar.gz").read_bytes()).hexdigest()

    def invoke(self):
        # Feed through stdin, exactly like curl | bash (not bash filename).
        script = INSTALLER.read_text() if INSTALLER.exists() else "echo 'quickinstall.sh is missing' >&2; exit 1\n"
        return subprocess.run([str(self.bin / "bash")], input=script, env=self.env, text=True, capture_output=True)

    def assert_installed(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        target = Path(self.env["PREFIX"]) / "bin/diffplay"
        self.assertTrue(os.access(target, os.X_OK), str(target))
        self.assertEqual(subprocess.check_output([str(target)], text=True).strip(), "diffplay 0.6.0")
        self.assertFalse(list(self.root.glob("diffplay-install.*")), "temporary checkout was not cleaned up")

    def test_linux_installs_from_pipe_with_existing_dependencies(self):
        self.assert_installed(self.invoke())
        self.assertNotIn("VERSION", (self.root / "downloads").read_text())

    def test_source_download_uses_main_branch(self):
        self.assert_installed(self.invoke())
        downloads = (self.root / "downloads").read_text().splitlines()
        self.assertIn("https://codeload.github.com/isaiash/diffplay/tar.gz/refs/heads/main", downloads)
        self.assertFalse(any("refs/heads/master" in url for url in downloads))

    def test_macos_installs_from_pipe_with_existing_dependencies(self):
        self.env.update(TEST_OS="Darwin", TEST_ARCH="arm64")
        self.assert_installed(self.invoke())

    def test_missing_go_downloads_verified_toolchain(self):
        (self.bin / "go").unlink()
        self.assert_installed(self.invoke())
        downloads = (self.root / "downloads").read_text()
        self.assertIn("go1.27.3.linux-amd64.tar.gz", downloads)
        self.assertIn(".sha256", downloads)

    def test_old_go_downloads_supported_toolchain(self):
        self.compiler(self.bin / "go", version="1.20.0")
        self.assert_installed(self.invoke())
        self.assertIn("VERSION", (self.root / "downloads").read_text())

    def test_macos_downloads_arm_toolchain(self):
        self.env.update(TEST_OS="Darwin", TEST_ARCH="arm64")
        (self.bin / "go").unlink()
        self.assert_installed(self.invoke())
        self.assertIn("go1.27.3.darwin-arm64.tar.gz", (self.root / "downloads").read_text())

    def test_checksum_failure_stops_before_build(self):
        (self.bin / "go").unlink()
        self.env["TEST_BAD_CHECKSUM"] = "1"
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("checksum", result.stderr.lower())
        self.assertFalse((Path(self.env["PREFIX"]) / "bin/diffplay").exists())

    def test_unsupported_os_fails_before_downloads(self):
        self.env["TEST_OS"] = "FreeBSD"
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("macOS and Linux", result.stderr)
        self.assertFalse((self.root / "downloads").exists())

    def package_manager(self, name):
        self.executable("sudo", 'printf "sudo %s\\n" "$*" >> "$TEST_ROOT/packages"; exec "$@"\n')
        self.executable(name, '''
printf '%s %s\n' "$(basename "$0")" "$*" >> "$TEST_ROOT/packages"
printf '#!/bin/sh\necho "git version 2.49.0"\n' > "$TEST_BIN/git"
chmod +x "$TEST_BIN/git"
''')
        location = shutil.which("basename")
        if location and not (self.bin / "basename").exists():
            (self.bin / "basename").symlink_to(location)

    def test_linux_installs_missing_git_using_package_manager(self):
        for manager in ("apt-get", "dnf", "yum", "pacman", "zypper", "apk"):
            with self.subTest(manager=manager):
                (self.bin / "git").unlink(missing_ok=True)
                self.package_manager(manager)
                try:
                    self.assert_installed(self.invoke())
                    self.assertIn(manager, (self.root / "packages").read_text())
                    self.assertIn("sudo", (self.root / "packages").read_text())
                finally:
                    (self.bin / manager).unlink()

    def test_macos_installs_missing_git_with_homebrew(self):
        self.env["TEST_OS"] = "Darwin"
        (self.bin / "git").unlink()
        self.package_manager("brew")
        self.assert_installed(self.invoke())
        self.assertIn("brew install", (self.root / "packages").read_text())
        self.assertNotIn("sudo", (self.root / "packages").read_text())

    def test_linux_missing_package_manager_fails_clearly(self):
        (self.bin / "git").unlink()
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("package manager", result.stderr.lower())


if __name__ == "__main__":
    unittest.main(verbosity=2)
