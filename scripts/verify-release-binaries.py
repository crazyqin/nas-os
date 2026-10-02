"""Verify release CPU/OS from executable headers, never from file names."""
import argparse
from pathlib import Path
import struct


TARGETS = {
    ("linux", "amd64"): ("ELF", 2, 62),
    ("linux", "arm64"): ("ELF", 2, 183),
    ("linux", "arm"): ("ELF", 1, 40),
    ("darwin", "amd64"): ("Mach-O", 2, 0x01000007),
    ("darwin", "arm64"): ("Mach-O", 2, 0x0100000C),
    ("windows", "amd64"): ("PE", 2, 0x8664),
}


def executable_target(path):
    with Path(path).open("rb") as stream:
        header = stream.read(64)
        if len(header) < 64:
            raise ValueError("truncated executable header")
        if header[:4] == b"\x7fELF":
            if header[5] != 1:
                raise ValueError("release ELF must use little endian")
            return "ELF", header[4], struct.unpack_from("<H", header, 18)[0]
        if header[:4] in (b"\xcf\xfa\xed\xfe", b"\xfe\xed\xfa\xcf"):
            order = "<" if header[:4] == b"\xcf\xfa\xed\xfe" else ">"
            if struct.unpack_from(order + "I", header, 12)[0] != 2:
                raise ValueError("Mach-O file is not an executable")
            return "Mach-O", 2, struct.unpack_from(order + "I", header, 4)[0]
        if header[:2] == b"MZ":
            offset = struct.unpack_from("<I", header, 60)[0]
            stream.seek(offset)
            pe = stream.read(26)
            if len(pe) < 26 or pe[:4] != b"PE\0\0" or pe[24:26] != b"\x0b\x02":
                raise ValueError("invalid PE32+ executable")
            return "PE", 2, struct.unpack_from("<H", pe, 4)[0]
        raise ValueError("unrecognized executable format")


def verify(path, os_name, arch):
    expected = TARGETS[(os_name, arch)]
    actual = executable_target(path)
    if actual != expected:
        raise ValueError(f"{path}: expected {os_name}/{arch} {expected}, found {actual}")
    return actual


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--os", required=True, choices=("linux", "darwin", "windows"))
    parser.add_argument("--arch", required=True, choices=("amd64", "arm64", "arm"))
    parser.add_argument("files", nargs="+")
    args = parser.parse_args()
    try:
        for path in args.files:
            target = verify(path, args.os, args.arch)
            print(f"{path}: verified {args.os}/{args.arch} {target[0]}")
    except (KeyError, OSError, ValueError, struct.error) as error:
        parser.exit(1, f"Binary verification failed: {error}\n")


if __name__ == "__main__":
    main()
