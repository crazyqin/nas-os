"""ISO release provenance and checksum failure-path regressions."""

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("verify_iso", ROOT / "scripts/verify-release-iso.py")
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ReleaseISOTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.version = "v3.25.0"
        self.commit = "a" * 40
        self.name = f"nas-os-{self.version}-amd64.iso"
        self.iso = self.root / self.name
        self.iso.write_bytes(b"ISO test payload")
        self.source = {"version": self.version, "source_commit": self.commit, "architecture": "amd64"}
        self.sidecar = self.root / (self.name + ".source.json")
        self.sidecar.write_text(json.dumps(self.source))
        self.embedded = self.root / "embedded.json"
        self.embedded.write_text(json.dumps(self.source))
        self.checksum = self.root / (self.name + ".sha256")
        self.refresh_checksum()
        self.bin = self.root / "bin"
        self.bin.mkdir()
        mock = self.bin / "xorriso"
        mock.write_text('#!/bin/bash\ncp "$ISO_EMBEDDED_FIXTURE" "${@: -1}"\n')
        mock.chmod(0o755)

    def refresh_checksum(self):
        digest = hashlib.sha256(self.iso.read_bytes()).hexdigest()
        self.checksum.write_text(f"{digest}  {self.name}\n")

    def run_verify(self):
        return subprocess.run(["python3", str(ROOT / "scripts/verify-release-iso.py"),
                               "--directory", str(self.root), "--version", self.version,
                               "--source-commit", self.commit],
                              env={**os.environ, "PATH": str(self.bin) + ":" + os.environ["PATH"],
                                   "ISO_EMBEDDED_FIXTURE": str(self.embedded)},
                              capture_output=True, text=True)

    def test_valid_artifact_and_missing_assets(self):
        result = self.run_verify()
        self.assertEqual(result.returncode, 0, result.stderr)
        for file in (self.iso, self.checksum, self.sidecar, self.embedded):
            with self.subTest(missing=file.name):
                saved = file.read_bytes()
                file.unlink()
                self.assertNotEqual(self.run_verify().returncode, 0)
                file.write_bytes(saved)

    def test_corruption_and_unsafe_checksum_records(self):
        self.iso.write_bytes(b"modified ISO")
        self.assertIn("checksum mismatch", self.run_verify().stderr)
        self.refresh_checksum()
        valid = self.checksum.read_text()
        for invalid in (valid + valid, valid.replace(self.name, "../" + self.name),
                        valid.replace(self.name, "other.iso"), "not a digest\n"):
            self.checksum.write_text(invalid)
            self.assertIn("invalid ISO checksum record", self.run_verify().stderr)

    def test_sidecar_and_embedded_source_must_both_match(self):
        for key, value in (("version", "v3.24.8"), ("source_commit", "b" * 40),
                           ("architecture", "arm64")):
            for file in (self.sidecar, self.embedded):
                with self.subTest(key=key, file=file.name):
                    file.write_text(json.dumps({**self.source, key: value}))
                    self.assertNotEqual(self.run_verify().returncode, 0)
                    file.write_text(json.dumps(self.source))
        self.commit = "a" * 7
        self.assertIn("full immutable source commit", self.run_verify().stderr)

    def test_experimental_arm_iso_is_rejected(self):
        (self.root / f"nas-os-{self.version}-arm64.iso").write_bytes(b"experimental")
        self.assertIn("experimental ARM ISO", self.run_verify().stderr)

    def test_extra_versioned_iso_cannot_be_uploaded(self):
        (self.root / "nas-os-v3.24.8-amd64.iso").write_bytes(b"stale")
        self.assertIn("unexpected or missing ISO release asset", self.run_verify().stderr)

    @unittest.skipUnless(shutil.which("xorriso"), "real ISO extraction requires xorriso")
    def test_real_iso_embedded_record(self):
        image_root = self.root / "image-root"
        image_root.mkdir()
        embedded = image_root / "nas-os-source.json"
        embedded.write_text(json.dumps(self.source))
        def build():
            subprocess.run(["xorriso", "-as", "mkisofs", "-o", str(self.iso), str(image_root)],
                           check=True, capture_output=True)
            self.refresh_checksum()
        build()
        module.verify(self.root, self.version, self.commit)
        embedded.write_text(json.dumps({**self.source, "source_commit": "b" * 40}))
        build()
        with self.assertRaisesRegex(ValueError, "embedded ISO source"):
            module.verify(self.root, self.version, self.commit)


if __name__ == "__main__":
    unittest.main()
