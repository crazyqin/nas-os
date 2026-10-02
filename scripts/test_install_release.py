"""Regression checks without installing services or contacting GitHub."""
import hashlib
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import textwrap
import unittest

SCRIPTS = Path(__file__).resolve().parent


class InstallReleaseTests(unittest.TestCase):
    def test_readme_latest_and_explicit_installer_urls(self):
        readme = (SCRIPTS.parent / "README.md").read_text()
        block = readme.split("### 裸机安装\n```bash\n", 1)[1].split("```", 1)[0]
        with tempfile.TemporaryDirectory() as tmp:
            calls = Path(tmp) / "calls"
            mock = r'''
curl() {
    printf 'curl %s\n' "$*" >> "$CALLS"
    if [[ "$2" == */latest/* && "${MISSING_LATEST:-false}" == true ]]; then return 22; fi
    if [[ "$3" == --output ]]; then printf ':\n' > "$4"; else echo ':'; fi
}
sudo() {
    printf 'sudo %s\n' "$*" >> "$CALLS"
    if [[ "$1" == bash && "$#" == 2 ]]; then bash "$2"; else cat >/dev/null; fi
}
'''
            result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c",
                                     mock + block.replace("VERSION=vX.Y.Z", "VERSION=v3.25.0")],
                                    env={**os.environ, "CALLS": str(calls)},
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            lines = calls.read_text().splitlines()
            self.assertEqual(len(lines), 4)
            self.assertTrue(lines[0].startswith("curl -fsSL https://github.com/crazyqin/nas-os/releases/latest/download/install.sh --output "))
            installer = lines[0].split(" --output ", 1)[1]
            self.assertIn("sudo bash " + installer, lines)
            self.assertFalse(Path(installer).exists(), "Temporary installer was not removed")
            self.assertIn("curl -fsSL https://github.com/crazyqin/nas-os/releases/download/v3.25.0/install.sh", lines)
            self.assertIn("sudo env NAS_OS_VERSION=v3.25.0 bash", lines)

            calls.unlink()
            result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", mock + block],
                                    env={**os.environ, "CALLS": str(calls), "MISSING_LATEST": "true"},
                                    capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("当前 latest Release 尚无可用安装器", result.stderr)
            self.assertNotIn("sudo ", calls.read_text())
            self.assertFalse(Path(calls.read_text().strip().split(" --output ", 1)[1]).exists())

    def package_installer(self, root, version):
        workflow = (SCRIPTS.parent / ".github/workflows/release.yml").read_text()
        job = workflow.split("  create-release:\n", 1)[1].split("  verify-release:\n", 1)[0]
        self.assertIn("ref: ${{ needs.prepare-release.outputs.version }}", job)
        upload = job.split("uses: softprops/action-gh-release@v2", 1)[1]
        self.assertIn("            install.sh\n", upload)
        self.assertIn("            install.sh.sha256\n", upload)
        step = job.split("      - name: 打包与目标 tag 绑定的安装器\n", 1)[1]
        command = textwrap.dedent(step.split("        run: |\n", 1)[1]
                                  .split("      - name:", 1)[0])
        (root / "scripts").mkdir(exist_ok=True)
        (root / "scripts/install.sh").write_text((SCRIPTS / "install.sh").read_text())
        result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", command],
                                cwd=root, env={**os.environ, "RELEASE_VERSION": version},
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        result = subprocess.run(["sha256sum", "--check", "install.sh.sha256"],
                                cwd=root, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return (root / "install.sh").read_text()

    def test_latest_and_explicit_version_download_urls(self):
        source = (SCRIPTS / "install.sh").read_text()
        with tempfile.TemporaryDirectory() as tmp:
            published = self.package_installer(Path(tmp), "v3.24.6")
        mock = r'''
uname() { echo "$TEST_ARCH"; }
curl() { echo "$2"; }
chmod() { :; }
mkdir() { :; }
rm() { :; }
tar() { :; }
download_binary
download_webui
'''
        cases = [(source, None, "latest/download"),
                 (source, "latest", "latest/download"),
                 (source, "v3.24.6", "download/v3.24.6"),
                 (source, "v3.25.0-rc.1", "download/v3.25.0-rc.1"),
                 (published, None, "download/v3.24.6"),
                 (published, "latest", "download/v3.24.6"),
                 (published, "v3.24.6", "download/v3.24.6")]
        for script, version, path in cases:
            for host, arch in (("x86_64", "amd64"), ("aarch64", "arm64"), ("armv7l", "arm")):
                with self.subTest(published=script == published, version=version, host=host):
                    env = {k: v for k, v in os.environ.items() if k != "NAS_OS_VERSION"}
                    env["TEST_ARCH"] = host
                    if version is not None:
                        env["NAS_OS_VERSION"] = version
                    result = subprocess.run(["bash", "-c", script.replace('main "$@"', ':') + mock],
                                            env=env, capture_output=True, text=True)
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    urls = [line for line in result.stdout.splitlines() if line.startswith("https://")]
                    base = f"https://github.com/crazyqin/nas-os/releases/{path}"
                    self.assertEqual(urls, [f"{base}/nasd-linux-{arch}", f"{base}/webui.tar.gz"])

        # Reject a stale installer before main can change the host or download assets.
        result = subprocess.run(["bash", "-c", published.replace('main "$@"', 'echo MAIN_CALLED')],
                                env={**os.environ, "NAS_OS_VERSION": "v3.25.0"},
                                capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("MAIN_CALLED", result.stdout)
        self.assertIn("/releases/download/v3.25.0/install.sh", result.stderr)

    def test_service_failures_print_journal_even_when_status_fails(self):
        source = (SCRIPTS / "install.sh").read_text().replace('main "$@"', ':')
        for mode in ("start", "inactive", "unhealthy"):
            with self.subTest(mode=mode):
                mock = r'''
systemctl() {
    case "$1" in
        start) test "$MODE" != start ;;
        is-active) test "$MODE" = unhealthy ;;
        status) echo STATUS_PRINTED; return 3 ;;
        *) return 0 ;;
    esac
}
journalctl() { echo JOURNAL_PRINTED "$*"; }
sleep() { :; }
curl() { return 1; }
enable_service
'''
                result = subprocess.run(["bash", "-c", source + mock],
                                        env={**os.environ, "MODE": mode},
                                        capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("STATUS_PRINTED", result.stdout)
                self.assertIn("JOURNAL_PRINTED -u nas-os -b -n 100", result.stdout)

    def test_release_assets_reject_missing_corrupt_and_wrong_layout(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            assets = root / "assets"
            assets.mkdir()
            binary_names = ["nasd-linux-amd64", "nasd-linux-arm64", "nasd-linux-arm"]
            for name in binary_names:
                (assets / name).write_bytes(b"binary fixture: " + name.encode())
            (assets / "checksums.txt").write_text("".join(
                f"{hashlib.sha256((assets / n).read_bytes()).hexdigest()}  {n}\n"
                for n in binary_names))
            installer = self.package_installer(root, "v3.24.6")
            (assets / "install.sh").write_text(installer)

            def installer_checksum():
                digest = hashlib.sha256((assets / "install.sh").read_bytes()).hexdigest()
                (assets / "install.sh.sha256").write_text(f"{digest}  install.sh\n")

            installer_checksum()

            def archive(valid=True):
                ui = root / "webui"
                (ui / "pages").mkdir(parents=True, exist_ok=True)
                (ui / "index.html").write_text("index")
                (ui / "pages/login.html").write_text("login")
                with tarfile.open(assets / "webui.tar.gz", "w:gz") as tar:
                    tar.add(ui, arcname="webui" if valid else "wrong-root")
                digest = hashlib.sha256((assets / "webui.tar.gz").read_bytes()).hexdigest()
                (assets / "webui.tar.gz.sha256").write_text(f"{digest}  webui.tar.gz\n")

            mocks = root / "bin"
            mocks.mkdir()
            (mocks / "gh").write_text('''#!/bin/bash
if [[ "$2" == view ]]; then
    printf '{"isDraft":%s}\\n' "${DRAFT:-false}"
else
    while (($#)); do
        if [[ "$1" == --pattern ]]; then name=$2; break; fi
        shift
    done
    cp "$FIXTURES/$name" "$name"
fi
''')
            (mocks / "curl").write_text('''#!/bin/bash
while (($#)); do
    if [[ "$1" == --output ]]; then name=$2; break; fi
    shift
done
cp "$FIXTURES/$name" "$name"
''')
            for path in mocks.iterdir():
                path.chmod(0o755)

            def run(draft="false"):
                return subprocess.run(["bash", str(SCRIPTS / "verify-release-assets.sh")],
                    env={**os.environ, "PATH": f"{mocks}:{os.environ['PATH']}",
                         "FIXTURES": str(assets), "DRAFT": draft,
                         "RELEASE_REPOSITORY": "crazyqin/nas-os", "RELEASE_VERSION": "v3.24.6"},
                    capture_output=True, text=True)

            archive()
            for draft in ("false", "true"):
                result = run(draft)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                for name in ("install.sh", "install.sh.sha256"):
                    with self.subTest(draft=draft, missing=name):
                        saved = (assets / name).read_bytes()
                        (assets / name).unlink()
                        self.assertNotEqual(run(draft).returncode, 0)
                        (assets / name).write_bytes(saved)
                with self.subTest(draft=draft, corrupt="install.sh"):
                    (assets / "install.sh").write_text(installer + "# corrupt\n")
                    self.assertNotEqual(run(draft).returncode, 0)
                for invalid in (installer + "if\n",
                                installer.replace('NAS_OS_RELEASE_VERSION="v3.24.6"',
                                                  'NAS_OS_RELEASE_VERSION="v3.25.0"'),
                                (SCRIPTS / "install.sh").read_text()):
                    with self.subTest(draft=draft, invalid=invalid[:40]):
                        (assets / "install.sh").write_text(invalid)
                        installer_checksum()
                        self.assertNotEqual(run(draft).returncode, 0)
                (assets / "install.sh").write_text(installer)
                installer_checksum()
            archive_file = assets / "webui.tar.gz"
            saved = archive_file.read_bytes()
            archive_file.unlink()
            self.assertNotEqual(run().returncode, 0)
            archive_file.write_bytes(saved + b"corrupt")
            self.assertNotEqual(run().returncode, 0)
            archive(False)
            self.assertNotEqual(run().returncode, 0)
            archive()
            (assets / "nasd-linux-arm").unlink()
            self.assertNotEqual(run().returncode, 0)


if __name__ == "__main__":
    unittest.main()
