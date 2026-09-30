#!/usr/bin/env python3
"""Install the produced ISO into a disposable VM disk, then boot without ISO.

Only whiptail answers are automated. The guest runs the unmodified installer.
No host disks are passed through. The user-mode network blocks guest outbound
connections, so the installation cannot fetch packages from the Internet.
"""
import argparse
import base64
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import threading
import time
import urllib.request


class VM:
    def __init__(self, args, disk, logdir, phase, with_iso):
        self.logdir = logdir
        self.phase = phase
        self.buffer = bytearray()
        self.sockpath = logdir / (phase + ".sock")
        network = ("user,id=n0,restrict=on,hostfwd=tcp:127.0.0.1:8080-:8080,"
                   "hostfwd=tcp:127.0.0.1:2222-:22")
        cmd = ["qemu-system-x86_64" if args.arch == "amd64" else "qemu-system-aarch64",
               "-m", "3072", "-smp", "2", "-display", "none",
               "-serial", f"unix:{self.sockpath},server=on,wait=off",
               "-monitor", "none", "-netdev", network]
        if args.arch == "amd64":
            if os.access("/dev/kvm", os.W_OK):
                cmd += ["-enable-kvm", "-cpu", "host"]
            cmd += ["-device", "e1000,netdev=n0",
                    "-drive", f"file={disk},format=qcow2,if=virtio"]
            if args.firmware == "uefi":
                cmd += ["-drive", "if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE_4M.fd",
                        "-drive", f"if=pflash,format=raw,file={logdir / 'vars.fd'}"]
            if with_iso:
                cmd += ["-cdrom", str(args.iso), "-boot", "d"]
            else:
                cmd += ["-boot", "c"]
        else:
            cmd += ["-M", "virt", "-cpu", "cortex-a72",
                    "-bios", "/usr/share/qemu-efi-aarch64/QEMU_EFI.fd",
                    "-device", "virtio-net-pci,netdev=n0",
                    "-drive", f"file={disk},format=qcow2,if=none,id=root",
                    "-device", "virtio-blk-pci,drive=root"]
            if with_iso:
                cmd += ["-drive", f"file={args.iso},media=cdrom,if=none,id=cd,readonly=on",
                        "-device", "virtio-scsi-pci", "-device", "scsi-cd,drive=cd"]
        self.stderr = (logdir / (phase + "-qemu.log")).open("wb")
        self.serial = (logdir / (phase + "-serial.log")).open("wb")
        self.proc = subprocess.Popen(cmd, stdout=self.stderr, stderr=self.stderr)
        deadline = time.monotonic() + 30
        self.socket = socket.socket(socket.AF_UNIX)
        while True:
            try:
                self.socket.connect(str(self.sockpath))
                break
            except (FileNotFoundError, ConnectionRefusedError):
                if self.proc.poll() is not None or time.monotonic() > deadline:
                    raise RuntimeError("QEMU did not create serial socket")
                time.sleep(0.2)
        self.reader = threading.Thread(target=self.read_serial, daemon=True)
        self.reader.start()

    def read_serial(self):
        try:
            while True:
                data = self.socket.recv(65536)
                if not data:
                    break
                self.buffer.extend(data)
                self.serial.write(data)
                self.serial.flush()
        except OSError:
            pass

    def send(self, command):
        self.socket.sendall(command.encode() + b"\n")

    def marker(self, marker, timeout):
        deadline = time.monotonic() + timeout
        last_report = time.monotonic()
        while marker.encode() not in self.buffer:
            if self.proc.poll() is not None or time.monotonic() > deadline:
                raise RuntimeError(f"{self.phase}: missing serial marker {marker}")
            if time.monotonic() - last_report > 45:
                print(f"{self.phase}: waiting for {marker}; QEMU running", flush=True)
                last_report = time.monotonic()
            time.sleep(1)

    def script(self, text):
        # Small lines avoid the terminal's canonical input limit.
        self.send("stty -echo; : > /tmp/nasos-e2e.b64")
        encoded = base64.b64encode(text.encode()).decode()
        for start in range(0, len(encoded), 500):
            self.send(f"printf '%s' '{encoded[start:start+500]}' >> /tmp/nasos-e2e.b64")
            time.sleep(0.05)
        self.send("base64 -d /tmp/nasos-e2e.b64 > /tmp/nasos-e2e.sh; bash /tmp/nasos-e2e.sh")

    def stop(self):
        if self.proc.poll() is None:
            self.proc.terminate()
            try:
                self.proc.wait(timeout=20)
            except subprocess.TimeoutExpired:
                self.proc.kill()
                self.proc.wait()
        self.socket.close()
        self.reader.join(timeout=3)
        self.stderr.close()
        self.serial.close()


def request(path, data=None, token=None):
    headers = {}
    if data is not None:
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request("http://127.0.0.1:8080" + path,
                                 data=None if data is None else json.dumps(data).encode(),
                                 headers=headers)
    with urllib.request.urlopen(req, timeout=10) as response:
        return response.read()


def wait_health(vm, timeout):
    deadline = time.monotonic() + timeout
    last_report = 0
    while time.monotonic() < deadline:
        if vm.proc.poll() is not None:
            raise RuntimeError("QEMU exited before health check")
        try:
            result = json.loads(request("/api/v1/system/health"))
            if result["data"]["status"] == "healthy":
                print(f"{vm.phase}: healthy", flush=True)
                return result
        except (OSError, ValueError, KeyError):
            pass
        if time.monotonic() - last_report > 45:
            print(f"{vm.phase}: waiting for healthy API", flush=True)
            last_report = time.monotonic()
        time.sleep(5)
    raise RuntimeError(f"{vm.phase}: health timeout")


def ssh(logdir, command):
    return subprocess.check_output([
        "ssh", "-i", str(logdir / "key"), "-p", "2222",
        "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
        "-o", "LogLevel=ERROR", "-o", "ConnectTimeout=10", "-o", "BatchMode=yes",
        "root@127.0.0.1", command], text=True, timeout=60)


def verify_installed(vm, logdir):
    try:
        health = wait_health(vm, 600)
    except RuntimeError:
        # Use the test root account to collect guest diagnostics after boot failure.
        vm.send("root")
        time.sleep(3)
        vm.send("Nasos-Test-Only-8392!")
        time.sleep(3)
        vm.script("ip -br addr; ip route; systemctl --no-pager --failed; systemctl --no-pager status nas-os systemd-networkd; journalctl -b -u nas-os --no-pager -n 100; ss -lntp; curl -s http://127.0.0.1:8080/api/v1/system/health; printf '\\nE2E_BOOT_DIAGNOSTICS_DONE\\n'\n")
        try:
            vm.marker("E2E_BOOT_DIAGNOSTICS_DONE", 60)
        except RuntimeError:
            pass
        raise
    # Explicitly assert the root is the installed disk, not squashfs/overlay/live.
    diagnostics = ssh(logdir, "set -eu; test ! -d /run/live/medium; "
                      "test \"$(findmnt -n -o FSTYPE /)\" = btrfs; "
                      "findmnt -n -o SOURCE,FSTYPE /; "
                      "systemctl is-enabled nas-os; systemctl is-active nas-os; "
                      "test ! -e /etc/systemd/system/serial-getty@.service.d/autologin.conf; "
                      "hostname; cat /etc/fstab")
    (logdir / (vm.phase + "-system.txt")).write_text(diagnostics)
    for path in ("/", "/pages/login.html"):
        content = request(path)
        if len(content) < 100 or b"<html" not in content.lower():
            raise RuntimeError(f"{path}: not an HTML page")
    return health


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--arch", choices=["amd64", "arm64"], required=True)
    parser.add_argument("--firmware", choices=["bios", "uefi"], required=True)
    parser.add_argument("--iso", type=Path, required=True)
    parser.add_argument("--output", type=Path, default=Path("e2e-results"))
    args = parser.parse_args()
    args.iso = args.iso.resolve()
    args.output.mkdir(parents=True, exist_ok=True)
    logdir = args.output.resolve()
    disk = logdir / "installed.qcow2"
    subprocess.run(["qemu-img", "create", "-f", "qcow2", str(disk), "12G"], check=True)
    subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(logdir / "key")], check=True)
    if args.arch == "amd64" and args.firmware == "uefi":
        shutil.copyfile("/usr/share/OVMF/OVMF_VARS_4M.fd", logdir / "vars.fd")
    pubkey = (logdir / "key.pub").read_text().strip()
    results = {"arch": args.arch, "firmware": args.firmware, "status": "running"}
    vm = None
    try:
        vm = VM(args, disk, logdir, "live-install", True)
        wait_health(vm, 1800)
        # Answer only the UI prompts. /dev/vda is the one newly created test disk.
        # Keep the real disk selection, partitioning, copying and chroot untouched.
        script = r'''set -eu
whiptail() {
  case "$2" in
    '选择目标磁盘') printf vda >&2 ;;
    '主机名') printf nasos-e2e >&2 ;;
    'root 密码'|'确认密码') printf 'Nasos-Test-Only-8392!' >&2 ;;
    '重启') return 1 ;;
    *) return 0 ;;
  esac
}
export -f whiptail
sha256sum /usr/sbin/nasos-install
if /usr/sbin/nasos-install; then
  mkdir -p /mnt/e2e-target
  mount /dev/vda2 /mnt/e2e-target
  printf '\n=== INSTALLER LOG ===\n'
  cat /tmp/nasos-install.log
  printf '\n=== TARGET BOOT DIAGNOSTICS ===\n'
  ls -la /mnt/e2e-target/boot /mnt/e2e-target/boot/grub
  find /mnt/e2e-target/boot/efi -type f -exec sha256sum {} \;
  cat /mnt/e2e-target/etc/default/grub /mnt/e2e-target/boot/grub/grub.cfg 2>/dev/null || true
  find /mnt/e2e-target/etc/grub.d -maxdepth 1 -type f -exec ls -l {} \;
  printf '\n=== END BOOT DIAGNOSTICS ===\n'
  install -d -m 700 /mnt/e2e-target/root/.ssh
  printf '%s\n' '__PUBKEY__' > /mnt/e2e-target/root/.ssh/authorized_keys
  chmod 600 /mnt/e2e-target/root/.ssh/authorized_keys
  umount /mnt/e2e-target
  sync
  printf '\nE2E_INSTALL_OK\n'
else
  cat /tmp/nasos-install.log
  printf '\nE2E_INSTALL_FAILED\n'
fi
'''.replace("__PUBKEY__", pubkey)
        vm.script("printf '\nE2E_SERIAL_READY\n'\n")
        vm.marker("E2E_SERIAL_READY", 120)
        vm.script(script)
        deadline = time.monotonic() + 1800
        while b"E2E_INSTALL_OK" not in vm.buffer:
            if b"E2E_INSTALL_FAILED" in vm.buffer:
                raise RuntimeError("real installer failed; see live-install-serial.log")
            if time.monotonic() > deadline or vm.proc.poll() is not None:
                raise RuntimeError("installer timed out; see live-install-serial.log")
            time.sleep(5)
        print("Real offline installation completed", flush=True)
        vm.stop()
        vm = VM(args, disk, logdir, "disk-boot", False)
        results["installed_health"] = verify_installed(vm, logdir)
        bootstrap = ssh(logdir, "cat /etc/nas-os/.admin_password").strip()
        login = json.loads(request("/api/v1/auth/login", {"username": "admin", "password": bootstrap}))["data"]
        if not login.get("must_change_password"):
            raise RuntimeError("bootstrap login did not require password rotation")
        changed = "Nasos-Web-Test-Only-48392!"
        rotated = json.loads(request("/api/v1/me/password", {"old_password": bootstrap, "new_password": changed}, login["token"]))
        if rotated.get("code") != 0:
            raise RuntimeError("bootstrap password rotation failed")
        print("Admin login and first-login password rotation passed", flush=True)
        ssh(logdir, "sync")
        vm.stop()
        # Cold boot once more, with no installation media attached.
        vm = VM(args, disk, logdir, "second-disk-boot", False)
        results["second_boot_health"] = verify_installed(vm, logdir)
        login = json.loads(request("/api/v1/auth/login", {"username": "admin", "password": changed}))["data"]
        if login.get("must_change_password") or not login.get("token"):
            raise RuntimeError("changed password was not persisted across reboot")
        results["status"] = "passed"
        results["checks"] = ["offline real installation", "boot without ISO", "btrfs root",
                             "enabled active service", "WebUI HTML", "bootstrap login",
                             "password rotation", "second cold boot", "password persistence"]
        print(json.dumps(results), flush=True)
    except Exception as error:
        results["status"] = "failed"
        results["error"] = str(error)
        raise
    finally:
        if vm:
            vm.stop()
        (logdir / "result.json").write_text(json.dumps(results, indent=2))


if __name__ == "__main__":
    main()
