#!/usr/bin/env python3
"""Regression tests for terminal restoration in PTY smoke tests."""
import copy
import os
import pty
import select
import sys
import termios
import unittest
from unittest.mock import patch

import list_smoke


class TerminalRestorationTests(unittest.TestCase):
    def test_darwin_reprocesses_before_comparing_exact_state(self):
        before = [1, 2, 3, 4, 5, 6, [b"x"]]
        pending = copy.deepcopy(before)
        pending[3] |= 0x20000000  # Darwin PENDIN
        state = [pending]
        calls = []

        def reprocess(read, write, error, timeout):
            calls.append((read, write, error, timeout))
            state[0] = before
            return [], [], []

        with patch.object(sys, "platform", "darwin"), \
             patch.object(select, "select", side_effect=reprocess), \
             patch.object(termios, "tcgetattr", side_effect=lambda fd: state[0]):
            list_smoke.assert_terminal_restored(12, before)
        self.assertEqual(calls, [([12], [], [], 0)])

    def test_darwin_still_rejects_unrestored_flags(self):
        before = [1, 2, 3, 4, 5, 6, [b"x"]]
        after = copy.deepcopy(before)
        after[3] ^= termios.ECHO
        with patch.object(sys, "platform", "darwin"), \
             patch.object(select, "select", return_value=([], [], [])), \
             patch.object(termios, "tcgetattr", return_value=after):
            with self.assertRaisesRegex(AssertionError, "terminal state"):
                list_smoke.assert_terminal_restored(12, before)

    def test_linux_does_not_reprocess(self):
        before = [1, 2, 3, 4, 5, 6, [b"x"]]
        with patch.object(sys, "platform", "linux"), \
             patch.object(select, "select") as readiness, \
             patch.object(termios, "tcgetattr", return_value=before):
            list_smoke.assert_terminal_restored(12, before)
            readiness.assert_not_called()

    def test_readiness_does_not_consume_pending_input(self):
        master, slave = pty.openpty()
        try:
            before = termios.tcgetattr(slave)
            os.write(master, b"pending input\n")
            self.assertTrue(select.select([slave], [], [], 2)[0])
            with patch.object(sys, "platform", "darwin"):
                list_smoke.assert_terminal_restored(slave, before)
            self.assertEqual(os.read(slave, 100), b"pending input\n")
        finally:
            os.close(master)
            os.close(slave)


if __name__ == "__main__":
    unittest.main(verbosity=2)
