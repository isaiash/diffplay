#!/usr/bin/env python3
"""Exercise the piped installer without changing host packages or using the network."""
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
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
        for tool in ("bash", "sh", "mkdir", "mktemp", "rm", "install", "chmod", "sha256sum", "shasum"):
            location = shutil.which(tool)
            if location:
                (self.bin / tool).symlink_to(location)
        self.env = dict(os.environ, HOME=str(self.root), PATH=str(self.bin),
                        TMPDIR=str(self.root), PREFIX=str(self.root / "prefix with spaces"),
                        TEST_ROOT=str(self.root), TEST_BIN=str(self.bin), TEST_OS="Linux", TEST_ARCH="x86_64")
        self.executable("uname", 'case "$1" in -s) echo "$TEST_OS";; -m) echo "$TEST_ARCH";; esac\n')
        self.executable("id", "echo 1000\n")
        self.executable("git", "echo 'git version 2.49.0'\n")
        self.executable("curl", '''
url= output=
while [ "$#" -gt 0 ]; do
    case "$1" in -o|--output) shift; output=$1;; https://*) url=$1;; esac
    shift
done
printf '%s\n' "$url" >> "$TEST_ROOT/downloads"
case "$url" in
    *.sha256)
        [ "${TEST_DOWNLOAD_FAILURE:-}" != 1 ] || exit 22
        if [ "${TEST_BAD_CHECKSUM:-}" = 1 ]; then
            printf '%s\n' '1111111111111111111111111111111111111111111111111111111111111111' > "$output"
        else
            printf '%s\n' "$TEST_SHA256" > "$output"
        fi;;
    */releases/latest/download/diffplay-*) install "$TEST_ROOT/diffplay" "$output";;
    *) echo "Unexpected download: $url" >&2; exit 1;;
esac
''')
        binary = self.root / "diffplay"
        binary.write_text('#!/bin/sh\necho "diffplay 0.6.0"\n')
        self.env["TEST_SHA256"] = hashlib.sha256(binary.read_bytes()).hexdigest() + "  diffplay-linux-amd64"

    def executable(self, name, content):
        path = self.bin / name
        if path.is_symlink():
            path.unlink()
        path.write_text("#!/bin/sh\nset -eu\n" + content)
        path.chmod(0o755)
        return path

    def invoke(self):
        # Feed through stdin, exactly like curl | bash (not bash filename).
        script = INSTALLER.read_text() if INSTALLER.exists() else "echo 'quickinstall.sh is missing' >&2; exit 1\n"
        return subprocess.run([str(self.bin / "bash")], input=script, env=self.env, text=True, capture_output=True)

    def assert_installed(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        target = Path(self.env["PREFIX"]) / "bin/diffplay"
        self.assertTrue(os.access(target, os.X_OK), str(target))
        self.assertEqual(subprocess.check_output([str(target)], text=True).strip(), "diffplay 0.6.0")
        self.assertFalse(list(self.root.glob("diffplay-install.*")), "temporary downloads were not cleaned up")

    def test_linux_installs_from_pipe_with_existing_dependencies(self):
        self.assert_installed(self.invoke())
        self.assertNotIn("VERSION", (self.root / "downloads").read_text())

    def test_latest_release_download_without_go_or_source_build(self):
        for system, os_name in (("Linux", "linux"), ("Darwin", "darwin")):
            for machine, arch in (("x86_64", "amd64"), ("arm64", "arm64")):
                with self.subTest(system=system, machine=machine):
                    self.env.update(TEST_OS=system, TEST_ARCH=machine)
                    self.assert_installed(self.invoke())
                    downloads = (self.root / "downloads").read_text().splitlines()
                    asset = f"diffplay-{os_name}-{arch}"
                    self.assertEqual(downloads[-2:], [
                        f"https://github.com/isaiash/diffplay/releases/latest/download/{asset}.sha256",
                        f"https://github.com/isaiash/diffplay/releases/latest/download/{asset}",
                    ])

    def test_macos_installs_from_pipe_with_existing_dependencies(self):
        self.env.update(TEST_OS="Darwin", TEST_ARCH="arm64")
        self.assert_installed(self.invoke())

    def test_checksum_failure_preserves_existing_install(self):
        target = Path(self.env["PREFIX"]) / "bin/diffplay"
        target.parent.mkdir(parents=True)
        target.write_text("existing installation")
        self.env["TEST_BAD_CHECKSUM"] = "1"
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("checksum", result.stderr.lower())
        self.assertEqual(target.read_text(), "existing installation")
        self.assertFalse(list(self.root.glob("diffplay-install.*")))

    def test_missing_release_fails_without_installing(self):
        self.env["TEST_DOWNLOAD_FAILURE"] = "1"
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((Path(self.env["PREFIX"]) / "bin/diffplay").exists())

    def test_invalid_checksum_fails_before_binary_download(self):
        self.env["TEST_SHA256"] = "invalid metadata"
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Invalid release checksum", result.stderr)
        self.assertEqual(len((self.root / "downloads").read_text().splitlines()), 1)

    def test_default_install_prefix(self):
        del self.env["PREFIX"]
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertTrue(os.access(self.root / ".local/bin/diffplay", os.X_OK))

    def test_shasum_fallback(self):
        (self.bin / "sha256sum").unlink(missing_ok=True)
        if not (self.bin / "shasum").exists():
            self.skipTest("shasum is not installed")
        self.assert_installed(self.invoke())

    def test_unsupported_os_fails_before_downloads(self):
        self.env["TEST_OS"] = "FreeBSD"
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("macOS and Linux", result.stderr)
        self.assertFalse((self.root / "downloads").exists())

    def test_unsupported_architecture_fails_before_downloads(self):
        self.env["TEST_ARCH"] = "riscv64"
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Supported architectures", result.stderr)
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
