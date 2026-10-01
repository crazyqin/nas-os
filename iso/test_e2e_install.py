"""Acceptance infrastructure regressions; no disk or VM is booted."""
import io
import json
from pathlib import Path
import socket
import subprocess
import threading
import time
import unittest
from unittest.mock import patch

from e2e_install import acceleration, INSTALL_DIAGNOSTICS, VM


class AcceptanceTests(unittest.TestCase):
    def test_native_arm_kvm_is_selected(self):
        with patch("e2e_install.platform.machine", return_value="aarch64"), \
                patch("e2e_install.os.access", return_value=True):
            self.assertEqual(acceleration("arm64", require_kvm=True), "kvm")

    def test_missing_kvm_fails_required_gate(self):
        with patch("e2e_install.platform.machine", return_value="aarch64"), \
                patch("e2e_install.os.access", return_value=False):
            with self.assertRaisesRegex(RuntimeError, "TCG cannot replace this gate"):
                acceleration("arm64", require_kvm=True)
            self.assertEqual(acceleration("arm64"), "tcg")

    def test_cross_arch_kvm_cannot_satisfy_gate(self):
        with patch("e2e_install.platform.machine", return_value="x86_64"), \
                patch("e2e_install.os.access", return_value=True):
            with self.assertRaisesRegex(RuntimeError, "KVM required"):
                acceleration("arm64", require_kvm=True)

    def test_serial_timeline_preserves_bytes_and_times_receipt(self):
        vm = VM.__new__(VM)
        vm.buffer = bytearray()
        vm.serial = io.BytesIO()
        vm.timeline = io.StringIO()
        vm.started = time.monotonic()
        vm.socket, sender = socket.socketpair()
        reader = threading.Thread(target=vm.read_serial)
        reader.start()
        try:
            data = b"update-initramfs: Generating\r\nCPU sampling\n"
            sender.sendall(data)
            sender.shutdown(socket.SHUT_WR)
            reader.join(timeout=2)
            self.assertFalse(reader.is_alive())
            records = [json.loads(line) for line in vm.timeline.getvalue().splitlines()]
            self.assertEqual("".join(row["serial"] for row in records), data.decode())
            self.assertEqual(vm.serial.getvalue(), data)
            self.assertTrue(all(row["elapsed_seconds"] >= 0 and "+00:00" in row["utc"]
                                for row in records))
        finally:
            sender.close()
            vm.socket.close()

    def test_diagnostics_shell_syntax(self):
        result = subprocess.run(["bash", "-n"], input=INSTALL_DIAGNOSTICS,
                                text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
