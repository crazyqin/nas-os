"""Regression checks without installing services or contacting GitHub."""
import hashlib
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parent


class InstallReleaseTests(unittest.TestCase):
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
            self.assertEqual(run().returncode, 0)
            self.assertEqual(run("true").returncode, 0)
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
