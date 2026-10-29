#!/usr/bin/env python3
"""Regression tests for workspace terminal restoration checks."""
import copy
import os
import pty
import select
import tempfile
import termios
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import workspace_smoke


# XNU's sys/termios.h value, used only to simulate its transient state.
PENDIN = 0x20000000


class WorkspaceTerminalTest(unittest.TestCase):
    def test_darwin_reprocesses_pending_input_before_exact_comparison(self):
        self.run_workspace()

    def test_darwin_rejects_unrelated_terminal_changes(self):
        for index, label in ((0, "selected workspace"), (2, "picker")):
            with self.subTest(index=index), self.assertRaisesRegex(AssertionError, label + " terminal state"):
                self.run_workspace(changed_index=index)

    def test_darwin_rejects_pendin_if_readiness_does_not_clear_it(self):
        with self.assertRaisesRegex(AssertionError, "selected workspace terminal state"):
            self.run_workspace(reprocess=False)

    def test_exit_failure_reports_late_pty_stderr(self):
        for index, label in ((0, "selected workspace"), (1, "focused workspace"), (2, "picker cancellation")):
            with self.subTest(index=index), self.assertRaisesRegex(AssertionError, "(?s)" + label + ".*status 7.*late child error"):
                self.run_workspace(fail_index=index)

    def test_linux_does_not_trigger_readiness_check(self):
        self.run_workspace(platform="linux")

    def run_workspace(self, fail_index=None, changed_index=None, platform="darwin", reprocess=True):
        terminals = []
        real_tcgetattr = termios.tcgetattr
        real_select = select.select
        selects = []

        class Terminal:
            def __init__(self, binary, root):
                self.master, self.slave = pty.openpty()
                self.before = real_tcgetattr(self.slave)
                self.tail = "child stderr from pty"
                self.index = len(terminals)
                self.root = root
                self.frames = 0
                self.pending = platform == "darwin"
                terminals.append(self)
                def wait(timeout):
                    if self.index == fail_index:
                        os.write(self.slave, b"late child error\n")
                self.proc = SimpleNamespace(returncode=7 if self.index == fail_index else 0, wait=wait)
                os.write(self.master, b"pending input\n")
                if not real_select([self.slave], [], [], 2)[0]:
                    raise AssertionError("test input did not reach the PTY")

            def frame(self, predicate):
                self.frames += 1
                if self.index == 0:
                    frames = [
                        ("SELECT REPOSITORIES alpha beta 2 repositories found", [""]),
                        ("alpha/ beta/", [f"{Path(self.root).name} +112 100.00% new lines", "alpha/same.txt ████", "beta/same.txt ████", "112 total lines"]),
                        ("✦", [f"{Path(self.root).name} +113 100.00% new lines", "alpha/same.txt ✦", "beta/same.txt", "113 total lines"]),
                        ("second.txt", []),
                        ("", []),
                        ("beta/second.txt", []),
                        ("", []),
                        ("alpha/ beta/", []),
                    ]
                    # The smoke check expects the total at the end of the first row.
                    raw, rows = frames[self.frames - 1]
                    if self.frames in (2, 3):
                        rows[0] += " " + rows[-1]
                    return raw, rows
                if self.index == 1 and self.frames == 2:
                    return "beta/", []
                return "SELECT REPOSITORIES", []

            def send(self, data):
                pass

            def resize(self, width, height):
                pass

            def close(self):
                self.input_after = os.read(self.slave, 1024) if real_select([self.slave], [], [], 0)[0] else None
                os.close(self.master)
                os.close(self.slave)

        def get_attrs(fd):
            terminal = next(t for t in reversed(terminals) if t.slave == fd)
            result = copy.deepcopy(terminal.before)
            if terminal.pending:
                result[3] |= PENDIN
            if terminal.index == changed_index:
                result[3] ^= termios.ECHO
            return result

        def readiness(read, write, error, timeout):
            self.assertEqual((write, error, timeout), ([], [], 0))
            self.assertEqual(len(read), 1)
            if read[0] == terminals[-1].master:
                return real_select(read, write, error, timeout)
            terminal = next(t for t in reversed(terminals) if t.slave == read[0])
            selects.append(terminal.index)
            if reprocess:
                terminal.pending = False
            return real_select(read, write, error, timeout)

        def make_repo(root, count):
            root.mkdir(parents=True)
            (root / "same.txt").write_text("new\n" * count)

        def header(row):
            return row.split(" 112 total lines")[0].split(" 113 total lines")[0]

        with tempfile.TemporaryDirectory() as root, patch.dict(os.environ, {"TMPDIR": root}), \
             patch.object(workspace_smoke.sys, "platform", platform), \
             patch.object(workspace_smoke.sys, "argv", ["workspace_smoke.py"]), \
             patch.object(workspace_smoke, "Terminal", Terminal), \
             patch.object(workspace_smoke, "make_repo", make_repo), \
             patch.object(workspace_smoke, "header_prefix", header), \
             patch.object(workspace_smoke.subprocess, "run", return_value=SimpleNamespace(returncode=1, stderr="open the application inside a Git repository")), \
             patch.object(termios, "tcgetattr", side_effect=get_attrs), \
             patch.object(select, "select", side_effect=readiness):
            workspace_smoke.main()
        self.assertEqual(selects, [0, 2] if platform == "darwin" else [])
        self.assertEqual([t.input_after for t in terminals], [b"pending input\n"] * 3)


if __name__ == "__main__":
    unittest.main()
