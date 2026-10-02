#!/usr/bin/env python3
"""Validate the stable amd64 ISO against a tag's immutable source commit."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import tempfile


def verify(directory, version, source_commit):
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9.]+)?", version):
        raise ValueError("invalid release version")
    if not re.fullmatch(r"[0-9a-f]{40}", source_commit):
        raise ValueError("expected a full immutable source commit")
    name = f"nas-os-{version}-amd64.iso"
    iso = directory / name
    if not iso.is_file() or iso.stat().st_size == 0:
        raise ValueError("missing or empty amd64 ISO")
    if list(directory.glob("*arm64.iso*")):
        raise ValueError("experimental ARM ISO cannot enter stable release assets")
    expected_files = {name, name + ".sha256", name + ".source.json"}
    if {file.name for file in directory.glob("*.iso*")} != expected_files:
        raise ValueError("unexpected or missing ISO release asset")
    # Exactly one digest and the expected basename; no checksum-controlled paths.
    checksum = (directory / (name + ".sha256")).read_text()
    match = re.fullmatch(r"([0-9a-f]{64}) [ *]" + re.escape(name) + r"\n?", checksum)
    if not match:
        raise ValueError("invalid ISO checksum record")
    with iso.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    if digest != match[1]:
        raise ValueError("ISO checksum mismatch")
    expected = {"version": version, "source_commit": source_commit, "architecture": "amd64"}
    source = json.loads((directory / (name + ".source.json")).read_text())
    if source != expected:
        raise ValueError("ISO source record does not match release tag/commit/architecture")
    # Read from the actual ISO, rather than trusting a renamed sidecar alone.
    with tempfile.TemporaryDirectory() as tmp:
        extracted = Path(tmp) / "source.json"
        subprocess.run(["xorriso", "-osirrox", "on", "-indev", str(iso),
                        "-extract", "/nas-os-source.json", str(extracted)],
                       check=True, capture_output=True, text=True)
        if json.loads(extracted.read_text()) != expected:
            raise ValueError("embedded ISO source does not match release tag/commit/architecture")
    print(f"Release ISO verified: {name}, source={source_commit}, sha256={digest}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--source-commit", required=True)
    args = parser.parse_args()
    try:
        verify(args.directory, args.version, args.source_commit)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"Release ISO verification failed: {error}\n")


if __name__ == "__main__":
    main()
