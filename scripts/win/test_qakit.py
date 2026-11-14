"""Набор QA 0.25 (qa-kit.sh) в режиме --fake: раскладка, лента, испорченные ленты, безопасность папки. Только stdlib.

Без wails и makensis: заглушки вместо exe и установщиков, но подпись и самопроверка лент настоящие
(release-sign.mjs --from), поэтому нужны bash и node.
"""

import json
import os
import shutil
import subprocess
import unittest
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
KIT = os.path.join(HERE, "qa-kit.sh")
UPDATE = os.path.join(ROOT, "docs", "qa", "update.md")
SETUP = "SosninaAlbums-0.25.1-windows-x64-setup.exe"


def have(tool):
    return shutil.which(tool) is not None


@unittest.skipUnless(os.name == "posix" and have("bash") and have("node") and have("git"), "нужны Linux или macOS, bash, node и git")
class QaKitFake(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.out = os.path.join(ROOT, ".gotmp", "qa-kit-test-%d" % os.getpid())
        r = subprocess.run(["bash", KIT, "--fake", "--out", cls.out], capture_output=True, text=True, timeout=120)
        cls.res = r

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.out, ignore_errors=True)

    def p(self, *parts):
        return os.path.join(self.out, *parts)

    def test_ok_and_selfcheck(self):
        self.assertEqual(self.res.returncode, 0, self.res.stderr)
        self.assertIn("подписи и хеши как задумано", self.res.stderr)

    def test_layout(self):
        names = ["README.txt", "pub.txt", "run-feed.cmd", "start-0.25.0.cmd", "start-installed.cmd",
                 "installer-check.ps1", "installer-check-lib.ps1", "kit-sha256.txt", "zip/SosninaAlbums-0.25.0-rel-windows-x64.zip",
                 "setup/SosninaAlbums-0.25.0-windows-x64-setup.exe", "setup/" + SETUP]
        # update.md кладётся из docs/qa, а в публичном дереве и ветке winqa/* его нет (qa-kit.sh это допускает)
        if os.path.isfile(UPDATE):
            names.append("update.md")
        for name in names:
            self.assertTrue(os.path.isfile(self.p(*name.split("/"))), name)
        for feed in ("feed", "feed-bad-byte", "feed-wrong-key", "feed-old-setup"):
            self.assertTrue(os.path.isfile(self.p(feed, "SosninaAlbums-update.json")), feed)
            self.assertTrue(os.path.isfile(self.p(feed, "v0.25.1", SETUP)), feed)

    def test_kit_has_albums_tool(self):
        self.assertTrue(os.path.isfile(self.p("tools", "qa-albums.exe")))
        with open(self.p("kit-sha256.txt"), encoding="utf-8") as f:
            self.assertIn("tools/qa-albums.exe", f.read())

    def test_zip_has_versioned_exe(self):
        with zipfile.ZipFile(self.p("zip", "SosninaAlbums-0.25.1-qa-windows-x64.zip")) as z:
            self.assertEqual(z.namelist(), ["Sosnina Albums 0.25.1-qa/SosninaAlbums-0.25.1.exe"])

    def test_newer_albums_are_format_99(self):
        for name in ("NewerTest-0.26.salbum", "NewerTest-0.25.1.salbum", "NewerTest-unknown.salbum"):
            with open(self.p("albums", name), encoding="utf-8") as f:
                self.assertEqual(json.load(f)["version"], 99)

    def test_cmd_files_crlf_and_pub_matches(self):
        with open(self.p("run-feed.cmd"), "rb") as f:
            data = f.read()
        self.assertIn(b"\r\n", data)
        self.assertNotIn(b"\n", data.replace(b"\r\n", b""))
        with open(self.p("pub.txt"), encoding="ascii") as f:
            self.assertEqual(len(f.read().strip()), 44)  # base64 от 32 байт

    def test_no_private_key_in_kit(self):
        for dirpath, _, names in os.walk(self.out):
            for n in names:
                self.assertFalse(n.endswith(".pem"), n)

    def test_out_outside_gotmp_refused(self):
        r = subprocess.run(["bash", KIT, "--fake", "--out", "/tmp/qa-kit-nope"], capture_output=True, text=True, timeout=60)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn(".gotmp", r.stderr)
        self.assertFalse(os.path.exists("/tmp/qa-kit-nope"))


if __name__ == "__main__":
    unittest.main()
