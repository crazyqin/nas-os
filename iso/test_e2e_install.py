"""Acceptance infrastructure regressions; no disk or VM is booted."""
import io
import json
from pathlib import Path
import socket
import subprocess
import threading
import tempfile
import time
import unittest
from unittest.mock import patch

from e2e_install import acceleration, INSTALL_DIAGNOSTICS, probe_kvm, VM


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

    def test_kvm_probe_requires_enabled_not_just_present(self):
        for enabled in (False, True):
            output = json.dumps({"return": {"present": True, "enabled": enabled}}) + "\n"
            result = subprocess.CompletedProcess([], 0, stdout=output, stderr="")
            with tempfile.TemporaryDirectory() as directory, \
                    patch("e2e_install.subprocess.run", return_value=result) as run, \
                    patch("builtins.print"):
                if enabled:
                    probe_kvm("arm64", Path(directory))
                else:
                    with self.assertRaisesRegex(RuntimeError, "could not enable KVM"):
                        probe_kvm("arm64", Path(directory))
                self.assertIn('"query-kvm"', run.call_args.kwargs["input"])
                self.assertIn("-accel", run.call_args.args[0])
                self.assertNotIn("kvm:tcg", run.call_args.args[0])

    def test_kvm_initialization_failure_is_saved_and_fails(self):
        result = subprocess.CompletedProcess([], 1, stdout="", stderr="KVM_CREATE_VM failed")
        with tempfile.TemporaryDirectory() as directory, \
                patch("e2e_install.subprocess.run", return_value=result):
            with self.assertRaisesRegex(RuntimeError, "could not enable KVM"):
                probe_kvm("arm64", Path(directory))
            self.assertIn("KVM_CREATE_VM failed", (Path(directory) / "kvm-probe.log").read_text())

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

    def test_sampler_survives_target_directory_not_created_yet(self):
        mocks = r'''
set -eu
ps() { printf 'process sample\n'; }
find() { return 1; }
df() { return 1; }
sleep() { printf 'SAMPLE_CYCLE_COMPLETED\n'; exit 0; }
'''
        result = subprocess.run(["bash", "-c", mocks + INSTALL_DIAGNOSTICS +
                                 '\nwait "$install_diagnostics_pid"'],
                                text=True, capture_output=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("SAMPLE_CYCLE_COMPLETED", result.stdout)


if __name__ == "__main__":
    unittest.main()
