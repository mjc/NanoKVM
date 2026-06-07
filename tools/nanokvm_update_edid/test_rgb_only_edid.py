import re
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
KVM_VISION = REPO_ROOT / "support/sg2002/additional/kvm/src/kvm_vision.cpp"


def load_builtin_edid():
    source = KVM_VISION.read_text()
    match = re.search(r"NanoKVM_edit\[\]\s*=\s*\{(?P<body>.*?)\};", source, re.S)
    if not match:
        raise AssertionError("NanoKVM_edit EDID array was not found")
    return bytes(int(value, 16) for value in re.findall(r"0x([0-9A-Fa-f]{2})", match.group("body")))


class BuiltinEdidTest(unittest.TestCase):
    def setUp(self):
        self.edid = load_builtin_edid()

    def test_builtin_edid_is_two_valid_blocks(self):
        self.assertEqual(len(self.edid), 256)
        self.assertEqual(self.edid[:8], b"\x00\xff\xff\xff\xff\xff\xff\x00")
        self.assertEqual(sum(self.edid[:128]) & 0xFF, 0)
        self.assertEqual(sum(self.edid[128:256]) & 0xFF, 0)
        self.assertEqual(self.edid[126], 1)

    def test_cta_extension_advertises_rgb_only(self):
        self.assertEqual(self.edid[128], 0x02)
        self.assertEqual(self.edid[129], 0x03)
        cta_flags = self.edid[131]
        self.assertEqual(cta_flags & 0x30, 0)


if __name__ == "__main__":
    unittest.main()
