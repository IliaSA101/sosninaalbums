"""Тесты macshots.py: python -m unittest discover -s scripts/mac (Pillow — для части тестов)."""

import contextlib
import io
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import macshots as m  # noqa: E402

try:
    from PIL import Image, ImageDraw
except ImportError:
    Image = None

WINS = [
    {"id": 7, "pid": 100, "owner": "Dock", "rect": [0, 1040, 1920, 40]},
    {"id": 9, "pid": 200, "owner": "CoreServicesUIAgent", "rect": [700, 300, 260, 220]},
    {"id": 3, "pid": 300, "owner": "Sosnina Albums", "rect": [100, 50, 1280, 760]},
    {"id": 4, "pid": 300, "owner": "Sosnina Albums", "rect": [10, 10, 30, 30]},
    {"id": 5, "pid": 300, "owner": "Sosnina Albums", "rect": [200, 200, 300, 200]},
]


class TestPure(unittest.TestCase):
    def test_pick_by_pid_largest(self):
        self.assertEqual(m.pick_window(WINS, {300}, "", "largest")["id"], 3)

    def test_pick_front_skips_tiny(self):
        # окно 30×30 — служебное; первое годное спереди — id 3
        self.assertEqual(m.pick_window(WINS, {300}, "", "front")["id"], 3)

    def test_pick_by_owner_regex(self):
        self.assertEqual(m.pick_window(WINS, set(), "coreservices|securityagent", "front")["id"], 9)

    def test_pick_none(self):
        self.assertIsNone(m.pick_window(WINS, {999}, "", "largest"))
        self.assertIsNone(m.pick_window(WINS, set(), "", "largest"))

    def test_parse(self):
        self.assertEqual(m.parse_pids(" 12 x 34 "), {12, 34})
        self.assertEqual(m.parse_rect("812 640 160 28"), [812, 640, 160, 28])
        self.assertEqual(m.parse_rect("1,2,3,4"), [1, 2, 3, 4])
        self.assertIsNone(m.parse_rect("1 2 3"))
        self.assertIsNone(m.parse_rect(""))

    def test_scale_and_clamp(self):
        self.assertEqual(m.scale_rect([10.4, 20, 30, 40], 2), [21, 40, 60, 80])
        self.assertEqual(m.clamp_rect([-10, 5, 50, 50], (30, 30)), [0, 5, 30, 25])
        self.assertIsNone(m.clamp_rect([40, 40, 10, 10], (30, 30)))
        self.assertEqual(m.snapshot_scale(3840, 1920, 1), 2)
        self.assertEqual(m.snapshot_scale(0, 1920, 2), 2)

    def test_decide_ok(self):
        self.assertEqual(m.decide_ok(True, 0.3, True, True), (True, ""))
        self.assertFalse(m.decide_ok(False, None, False, False)[0])
        ok, why = m.decide_ok(True, 0.001, True, True)
        self.assertFalse(ok)
        self.assertIn("обои", why)
        self.assertFalse(m.decide_ok(True, None, True, False)[0])
        # без Pillow решает только список окон
        self.assertTrue(m.decide_ok(True, None, True, True)[0])

    def test_upsert_replaces(self):
        s = m.upsert([{"name": "a", "ok": False}, {"name": "b"}], {"name": "a", "ok": True})
        self.assertEqual([x["name"] for x in s], ["b", "a"])
        self.assertTrue(s[1]["ok"])

    def test_moved_share(self):
        self.assertEqual(m.moved_share([]), 0)
        self.assertAlmostEqual(m.moved_share([0.0, 0.2, 0.001, 0.05]), 0.5)


@unittest.skipIf(Image is None, "нет Pillow")
class TestImages(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.out = self.tmp.name
        os.makedirs(os.path.join(self.out, "png"))
        os.makedirs(os.path.join(self.out, "logs"))

    def tearDown(self):
        self.tmp.cleanup()

    def desk(self, with_window):
        im = Image.new("RGB", (1920, 1080), (90, 90, 95))
        d = ImageDraw.Draw(im)
        d.rectangle([0, 0, 1919, 24], fill=(240, 240, 240))  # строка меню
        if with_window:
            d.rectangle([200, 100, 1479, 859], fill=(30, 30, 30))
            d.rectangle([220, 120, 600, 200], fill=(220, 40, 40))
        return im

    def write_windows(self, name):
        path = os.path.join(self.out, "logs", f"windows-{name}.json")
        doc = {"screen": [1920, 1080], "scale": 1.0, "windows": [
            {"id": 3, "pid": 300, "owner": "Sosnina Albums", "rect": [200, 100, 1280, 760]}]}
        with open(path, "w", encoding="utf-8") as f:
            json.dump(doc, f)
        return path

    def record(self, name, *extra):
        wj = self.write_windows(name)
        argv = ["record", "--out", self.out, "--name", name, "--windows", wj,
                "--pid", "300", "--macos", "26.0", "--lang", "ru", *extra]
        with contextlib.redirect_stdout(io.StringIO()):
            return m.main(argv)

    def shots(self):
        with open(os.path.join(self.out, "shots.json"), encoding="utf-8") as f:
            return {s["name"]: s for s in json.load(f)}

    def test_changed_fraction(self):
        a, b = self.desk(False), self.desk(True)
        self.assertLess(m.changed_fraction(a, a), 0.001)
        self.assertGreater(m.changed_fraction(a, b), 0.3)

    def test_is_uniform(self):
        self.assertTrue(m.is_uniform(Image.new("RGB", (50, 50), (0, 0, 0))))
        self.assertFalse(m.is_uniform(self.desk(True)))

    def test_record_window_shot(self):
        self.desk(False).save(os.path.join(self.out, "png", "01-desktop.png"))
        self.desk(True).save(os.path.join(self.out, "png", "02-app-window.png"))
        self.assertEqual(self.record("02-app-window"), 0)
        s = self.shots()["02-app-window"]
        self.assertTrue(s["ok"], s["why"])
        self.assertGreater(s["changed"], 0.3)
        self.assertEqual(s["screen"], [1920, 1080])
        self.assertEqual(s["target"]["rect"], [200, 100, 1280, 760])
        # снимка окна не было — вырезан из полного
        self.assertEqual(s["win"], "png/02-app-window-win.png")
        with Image.open(os.path.join(self.out, s["win"])) as w:
            self.assertEqual(w.size, (1280, 760))

    def test_record_wallpaper_only_fails(self):
        self.desk(False).save(os.path.join(self.out, "png", "01-desktop.png"))
        self.desk(False).save(os.path.join(self.out, "png", "03-gatekeeper.png"))
        self.record("03-gatekeeper")
        s = self.shots()["03-gatekeeper"]
        self.assertFalse(s["ok"])
        self.assertIn("обои", s["why"])

    def test_record_label_rect_retina(self):
        big = self.desk(True).resize((3840, 2160))
        self.desk(False).resize((3840, 2160)).save(os.path.join(self.out, "png", "01-desktop.png"))
        big.save(os.path.join(self.out, "png", "05-open-anyway.png"))
        self.record("05-open-anyway", "--label", "Открыть всё равно", "--rect", "812 640 160 28")
        s = self.shots()["05-open-anyway"]
        self.assertEqual(s["scale"], 2)
        self.assertEqual(s["target"], {"label": "Открыть всё равно", "rect": [1624, 1280, 320, 56]})

    def test_record_need_rect_and_get(self):
        self.desk(False).save(os.path.join(self.out, "png", "01-desktop.png"))
        self.desk(True).save(os.path.join(self.out, "png", "05-open-anyway.png"))
        self.record("05-open-anyway", "--label", "Open Anyway", "--need-rect")
        s = self.shots()["05-open-anyway"]
        self.assertFalse(s["ok"])
        self.assertIn("Open Anyway", s["why"])
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            m.main(["get", "--out", self.out, "--name", "05-open-anyway", "--field", "ok"])
        self.assertEqual(buf.getvalue().strip(), "false")


if __name__ == "__main__":
    unittest.main()
