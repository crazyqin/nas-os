"""Executable-format regressions, including amd64 files renamed as ARM."""
import importlib.util
from pathlib import Path
import struct
import tempfile
import unittest


SPEC = importlib.util.spec_from_file_location("verify_binaries", Path(__file__).with_name("verify-release-binaries.py"))
VERIFY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(VERIFY)


def elf(bits, machine):
    data = bytearray(64)
    data[:6] = b"\x7fELF" + bytes((bits, 1))
    struct.pack_into("<H", data, 18, machine)
    return data


def macho(cpu):
    data = bytearray(64)
    data[:4] = b"\xcf\xfa\xed\xfe"
    struct.pack_into("<I", data, 4, cpu)
    struct.pack_into("<I", data, 12, 2)
    return data


def pe():
    data = bytearray(154)
    data[:2] = b"MZ"
    struct.pack_into("<I", data, 60, 128)
    data[128:132] = b"PE\0\0"
    struct.pack_into("<H", data, 132, 0x8664)
    data[152:154] = b"\x0b\x02"
    return data


class BinaryTests(unittest.TestCase):
    def test_all_supported_targets_and_cross_target_rejection(self):
        fixtures = {
            ("linux", "amd64"): elf(2, 62),
            ("linux", "arm64"): elf(2, 183),
            ("linux", "arm"): elf(1, 40),
            ("darwin", "amd64"): macho(0x01000007),
            ("darwin", "arm64"): macho(0x0100000C),
            ("windows", "amd64"): pe(),
        }
        with tempfile.TemporaryDirectory() as tmp:
            # A misleading filename must never override the executable header.
            path = Path(tmp) / "nasd-linux-arm64"
            for actual, data in fixtures.items():
                path.write_bytes(data)
                for requested in fixtures:
                    with self.subTest(actual=actual, requested=requested):
                        if actual == requested:
                            VERIFY.verify(path, *requested)
                        else:
                            with self.assertRaises(ValueError):
                                VERIFY.verify(path, *requested)

    def test_corrupt_and_truncated_headers_fail(self):
        invalid = [b"", b"binary fixture", b"\0" * 64, elf(1, 62)]
        endian = elf(2, 62)
        endian[5] = 2
        invalid.append(endian)
        bad_mach = macho(0x01000007)
        struct.pack_into("<I", bad_mach, 12, 1)
        invalid.append(bad_mach)
        bad_pe = pe()
        bad_pe[152:154] = b"\x0b\x01"
        invalid.extend((bad_pe, pe()[:132]))
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "binary"
            for data in invalid:
                path.write_bytes(data)
                with self.subTest(data=bytes(data[:6])):
                    with self.assertRaises(ValueError):
                        VERIFY.verify(path, "linux", "amd64")


if __name__ == "__main__":
    unittest.main()
