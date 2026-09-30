"""Non-destructive installer regressions: mock disk and mount commands."""
import os
from pathlib import Path
import re
import subprocess
import unittest

SOURCE = (Path(__file__).parent / "live-config/rootfs/usr/sbin/nasos-install").read_text()


def function(name):
    match = re.search(r"^" + name + r"\(\) \{\n.*?^\}", SOURCE, re.M | re.S)
    if not match:
        raise AssertionError(f"missing function {name}")
    return match.group()


def run(name, arch="amd64", fail=""):
    mocks = r'''
sgdisk() { printf 'sgdisk %s\n' "$*"; }
partprobe() { printf 'partprobe %s\n' "$*"; }
udevadm() { printf 'udevadm %s\n' "$*"; }
dpkg() { echo "$ARCH"; }
mkdir() { printf 'mkdir %s\n' "$*"; }
mount() { printf 'mount %s\n' "$*"; [[ "$*" != "$FAIL" ]]; }
DISK=/dev/testdisk
TARGET_MNT=/test-target
'''
    return subprocess.run(["bash", "-euo", "pipefail", "-c", mocks + function(name) + "\n" + name + " || exit 9"],
                          env={**os.environ, "ARCH": arch, "FAIL": fail},
                          capture_output=True, text=True)


class InstallerTests(unittest.TestCase):
    def test_live_medium_excludes_whole_disk_partition_and_mapper_backing_disks(self):
        for topology, expected in (
            ("sda disk\n", "sda\n"),
            ("nvme0n1p1 part\nnvme0n1 disk\n", "nvme0n1\n"),
            ("ventoy dm\nsda1 part\nsda disk\nsdb disk\n", "sda\nsdb\n"),
            ("sr0 rom\n", ""),
        ):
            with self.subTest(topology=topology):
                script = 'lsblk() { printf "%s" "$TOPOLOGY"; }\n' + function("find_medium_disks")
                result = subprocess.run(["bash", "-euo", "pipefail", "-c", script +
                                         '\nfind_medium_disks /dev/source'],
                                        env={**os.environ, "TOPOLOGY": topology},
                                        capture_output=True, text=True)
                self.assertEqual(result.returncode, 0)
                self.assertEqual(result.stdout, expected)

    def test_live_medium_lookup_failure_is_not_hidden_by_pipeline(self):
        script = 'lsblk() { return 1; }\n' + function("find_medium_disks")
        result = subprocess.run(["bash", "-euo", "pipefail", "-c", script +
                                 '\nfind_medium_disks /dev/source'], capture_output=True)
        self.assertNotEqual(result.returncode, 0)

    def test_bios_embedding_partition_precedes_esp_and_root(self):
        result = run("partition_disk")
        self.assertEqual(result.returncode, 0, result.stderr)
        lines = result.stdout.splitlines()
        self.assertEqual(lines[:4], [
            "sgdisk --zap-all /dev/testdisk",
            "sgdisk -n 3:2048:4095 -t 3:ef02 -c 3:BIOS-GRUB /dev/testdisk",
            "sgdisk -n 1:0:+512M -t 1:ef00 -c 1:ESP /dev/testdisk",
            "sgdisk -n 2:0:0 -t 2:8300 -c 2:ROOT /dev/testdisk"])

    def test_arm64_uses_only_esp_and_root(self):
        result = run("partition_disk", arch="arm64")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn("ef02", result.stdout)
        self.assertIn("-t 1:ef00", result.stdout)
        self.assertIn("-t 2:8300", result.stdout)

    def test_chroot_mounts_are_recursive_and_slave(self):
        result = run("prepare_chroot")
        self.assertEqual(result.returncode, 0, result.stderr)
        mounts = [line for line in result.stdout.splitlines() if line.startswith("mount ")]
        self.assertEqual(mounts, [line for fs in ("dev", "proc", "sys", "run") for line in (
            f"mount --rbind /{fs} /test-target/{fs}",
            f"mount --make-rslave /test-target/{fs}")])

    def test_failed_mount_stops_before_chroot(self):
        result = run("prepare_chroot", fail="--rbind /proc /test-target/proc")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("--rbind /sys", result.stdout)


if __name__ == "__main__":
    unittest.main()
