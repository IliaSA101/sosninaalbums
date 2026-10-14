"""Тесты macshots.py: python -m unittest discover -s scripts/mac (Pillow — для части тестов)."""

import contextlib
import io
import json
import os
import sys
import struct
import tempfile
import time
import unittest
import zlib

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

    def test_moved_count(self):
        self.assertEqual(m.moved_count([]), 0)
        self.assertEqual(m.moved_count([0.0, 0.2, 0.001, 0.05]), 2)

    def test_rect_contains_centre(self):
        dialog = [360, 100, 300, 300]
        self.assertTrue(m.rect_contains_centre(dialog, [376, 372, 272, 28]))  # кнопка в окне
        self.assertFalse(m.rect_contains_centre(dialog, [830, 430, 124, 14]))  # та же подпись под окном

    def test_screen_line(self):
        self.assertEqual(m.screen_line([1440, 900], 2.0, [2880, 1800]), "1440×900 точек, масштаб 2, снимок 2880×1800 px")
        self.assertEqual(m.screen_line([1920, 1080], 1, None), "1920×1080 точек, масштаб 1")
        self.assertEqual(m.screen_line(None, 1, None), "")

    def test_inrect_leftmost(self):
        # «Конфиденциальность и безопасность» — и в боковой колонке, и в заголовке окна
        hits = "495 68 284 20\tКонфиденциальность и безопасность\n170 560 250 18\tКонфиденциальность и безопасность\n"
        stdin, buf = sys.stdin, io.StringIO()
        sys.stdin = io.StringIO(hits)
        try:
            with contextlib.redirect_stdout(buf):
                rc = m.main(["inrect", "--rect", "154 53 835 625", "--leftmost"])
        finally:
            sys.stdin = stdin
        self.assertEqual(rc, 0)
        self.assertTrue(buf.getvalue().startswith("170 560"))

    def test_inrect_picks_hit_inside_window(self):
        # OCR видит «Все равно открыть» дважды: в окне подтверждения и в «Настройках» за ним
        hits = "830 430 124 14\tВсе равно открыть\n376 372 272 28\tВсе равно открыть\n"
        stdin, buf = sys.stdin, io.StringIO()
        sys.stdin = io.StringIO(hits)
        try:
            with contextlib.redirect_stdout(buf):
                rc = m.main(["inrect", "--rect", "360 100 300 300"])
        finally:
            sys.stdin = stdin
        self.assertEqual(rc, 0)
        self.assertTrue(buf.getvalue().startswith("376 372 272 28"))
        sys.stdin = io.StringIO(hits)
        try:
            self.assertEqual(m.main(["inrect", "--rect", "0 0 10 10"]), 1)
        finally:
            sys.stdin = stdin


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

    def test_motion_with_count(self):
        d = os.path.join(self.out, "frames")
        os.makedirs(d)
        # четыре кадра: стол, стол, окно, окно — изменилась одна пара из трёх
        for i, win in enumerate((False, False, True, True)):
            self.desk(win).save(os.path.join(d, f"{i:04d}.png"))
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            self.assertEqual(m.main(["motion", d, "--with-count"]), 0)
        self.assertEqual(buf.getvalue().split(), ["0.333", "1"])

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
        self.assertEqual(s["window"], [200, 100, 1280, 760])
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
        # кнопка — цель, окно записано отдельно (крупный план считается по нему)
        self.assertEqual(s["window"], [400, 200, 2560, 1520])

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


class TestJournal(unittest.TestCase):
    """video/steps.jsonl — контракт с overlay.py (036 design §2)."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.file = os.path.join(self.tmp.name, "steps.jsonl")
        self.clock = os.path.join(self.tmp.name, "clock")
        win = os.path.join(self.tmp.name, "w.json")
        with open(win, "w", encoding="utf-8") as f:
            json.dump({"screen": [1440, 900], "scale": 2.0, "windows": []}, f)
        self.jl("meta", "os=macos", "ver=15.7", "lang=ru", "recorder=avfoundation", windows=win)

    def tearDown(self):
        self.tmp.cleanup()

    def jl(self, ev, *fields, windows=""):
        argv = ["journal", "--file", self.file, "--clock", self.clock]
        if windows:
            argv += ["--windows", windows]
        with contextlib.redirect_stderr(io.StringIO()):
            return m.main(argv + [ev, *fields])

    def lines(self):
        with open(self.file, encoding="utf-8") as f:
            return [json.loads(x) for x in f.read().splitlines()]

    def test_meta_first_with_screen_from_windows(self):
        meta = self.lines()[0]
        self.assertEqual(meta["ev"], "meta")
        self.assertEqual((meta["space"], meta["screen"], meta["scale"]), ("points", [1440, 900], 2.0))
        self.assertEqual(meta["lang"], "ru")

    def test_append_escapes_and_cyrillic(self):
        cap = 'Нажмите «Готово» — не "Корзину"\\ \nвторая строка'
        self.assertEqual(self.jl("hold", "t=1.5", "rect=612 388 96 110", "caption=" + cap, 'keys:=["⌘","Пробел"]'), 0)
        with open(self.file, encoding="utf-8") as f:
            raw = f.read().splitlines()
        # одна строка на событие; кириллица не превращается в \uXXXX
        self.assertEqual(len(raw), 2)
        self.assertIn("«Готово»", raw[1])
        self.assertNotIn("\\u04", raw[1])
        hold = json.loads(raw[1])
        self.assertEqual(hold["caption"], 'Нажмите «Готово» — не "Корзину"\\ вторая строка')
        self.assertEqual(hold["rect"], [612, 388, 96, 110])
        self.assertEqual(hold["keys"], ["⌘", "Пробел"])
        self.assertEqual(list(hold)[:2], ["ev", "t"])

    def test_times_monotonic(self):
        self.assertEqual(self.jl("hold", "t=5", "caption=Первый"), 0)
        # запись «из прошлого» и отрезок с t1 < t0 не должны пятить время
        self.assertEqual(self.jl("hold", "t=3", "caption=Второй"), 0)
        self.assertEqual(self.jl("move", "t0:=2", "t1:=1"), 0)
        self.assertEqual(self.jl("keys", "t0:=6", "t1:=6.25", 'keys:=["Return"]'), 0)
        got = self.lines()[1:]
        keys = [x.get("t", x.get("t0")) for x in got]
        self.assertEqual(keys, [5, 5, 5, 6])
        self.assertLessEqual(got[2]["t0"], got[2]["t1"])

    def test_now_and_end_tokens(self):
        with open(self.clock, "w", encoding="utf-8") as f:
            f.write(repr(time.time() - 10))
        self.assertEqual(self.jl("hold", "t=@now", "caption=Сейчас"), 0)
        t = self.lines()[1]["t"]
        self.assertAlmostEqual(t, 10, delta=2)
        self.assertEqual(self.jl("move", "t0=@end", "t1=@now"), 0)
        mv = self.lines()[2]
        self.assertEqual(mv["t0"], t)
        self.assertGreaterEqual(mv["t1"], mv["t0"])

    def test_short_idle_dropped_long_kept(self):
        self.assertEqual(self.jl("idle", "t0:=4.0", "t1:=4.3"), 0)
        self.assertEqual(self.jl("idle", "t0:=4.5", "t1:=9.1"), 0)
        got = self.lines()[1:]
        self.assertEqual([(x["ev"], x["t0"], x["t1"]) for x in got], [("idle", 4.5, 9.1)])

    def test_note_inside_idle_keeps_idle_start(self):
        # заметка записана раньше, чем закрыт простой, внутри которого стоит: idle встаёт перед ней, начало цело
        self.assertEqual(self.jl("hold", "t=1", "caption=Шаг"), 0)
        self.assertEqual(self.jl("note", "t=5", "text=внутри простоя"), 0)
        self.assertEqual(self.jl("idle", "t0:=3", "t1:=9"), 0)
        got = self.lines()[1:]
        self.assertEqual([x["ev"] for x in got], ["hold", "idle", "note"])
        self.assertEqual((got[1]["t0"], got[1]["t1"]), (3, 9))
        self.assertEqual(got[2]["t"], 5)
        # время ключей не убывает: контракт с overlay_plan
        keys = [x.get("t", x.get("t0")) for x in got]
        self.assertEqual(keys, sorted(keys))

    def test_idle_after_notes_before_it_keeps_order(self):
        # несколько заметок: те, что раньше начала простоя, остаются до него; позже — после
        self.assertEqual(self.jl("note", "t=2", "text=до"), 0)
        self.assertEqual(self.jl("note", "t=6", "text=внутри"), 0)
        self.assertEqual(self.jl("note", "t=7", "text=ещё внутри"), 0)
        self.assertEqual(self.jl("idle", "t0:=4", "t1:=9"), 0)
        got = self.lines()[1:]
        self.assertEqual([(x["ev"], x.get("t", x.get("t0"))) for x in got], [("note", 2), ("idle", 4), ("note", 6), ("note", 7)])

    def test_short_idle_with_notes_leaves_file_untouched(self):
        self.assertEqual(self.jl("note", "t=5", "text=x"), 0)
        with open(self.file, encoding="utf-8") as f:
            before = f.read()
        self.assertEqual(self.jl("idle", "t0:=4.0", "t1:=4.2"), 0)
        with open(self.file, encoding="utf-8") as f:
            self.assertEqual(f.read(), before)

    def test_hold_caption_limit_and_rect_clamp(self):
        self.assertEqual(self.jl("hold", "t=1", "rect:=[1400,880,200,100]", "caption=" + "я" * 120), 0)
        h = self.lines()[1]
        self.assertEqual(len(h["caption"]), 80)
        self.assertTrue(h["caption"].endswith("…"))
        self.assertEqual(h["rect"], [1400, 880, 40, 20])
        # цель вне экрана или не найдена — без стрелки, но стоп-кадр есть
        self.assertEqual(self.jl("hold", "t=2", "rect:=[3000,10,5,5]", "caption=Вне экрана"), 0)
        self.assertEqual(self.jl("hold", "t=3", "rect=", "caption=Нет цели"), 0)
        self.assertEqual([x["rect"] for x in self.lines()[2:]], [None, None])
        self.assertIsNone(self.lines()[3]["keys"])

    def test_rejects_bad_input(self):
        self.assertEqual(self.jl("hold", "t=1", "caption=   "), 2)
        self.assertEqual(self.jl("лишнее", "t=1"), 2)
        self.assertEqual(self.jl("step", "t=abc"), 2)
        self.assertEqual(self.jl("hold", "t=1", "без_равно"), 2)
        self.assertEqual(len(self.lines()), 1)  # одна meta: плохие строки не записаны

    def test_requires_meta_and_clock(self):
        os.remove(self.file)
        os.remove(self.clock)
        self.assertEqual(self.jl("note", "t=1", "text=x"), 2)
        self.assertEqual(self.jl("now"), 2)

    def test_stats(self):
        self.jl("hold", "t=1", "rect:=[10,10,50,20]", "caption=Со стрелкой")
        self.jl("hold", "t=2", "caption=Без стрелки")
        self.jl("hold", "t=3", "rect:=[20,20,50,20]", "caption=Ещё со стрелкой")
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            self.assertEqual(m.main(["jstat", "--file", self.file]), 0)
        self.assertEqual(buf.getvalue().strip(), "шагов 3, со стрелкой 2")

    def test_solid_png(self):
        out = os.path.join(self.tmp.name, "red.png")
        self.assertEqual(m.main(["solid", "--color", "#d81f26", "--size", "6x4", "--out", out]), 0)
        with open(out, "rb") as f:
            data = f.read()
        self.assertEqual(data[:8], b"\x89PNG\r\n\x1a\n")
        self.assertEqual(struct.unpack(">II", data[16:24]), (6, 4))
        # IDAT: 4 строки по (фильтр + 6 пикселей RGB)
        i = data.index(b"IDAT")
        n = struct.unpack(">I", data[i - 4 : i])[0]
        raw = zlib.decompress(data[i + 4 : i + 4 + n])
        self.assertEqual(raw, (b"\x00" + bytes([0xD8, 0x1F, 0x26]) * 6) * 4)
        self.assertEqual(m.main(["solid", "--color", "красный", "--size", "6x4", "--out", out]), 2)
        self.assertEqual(m.main(["solid", "--color", "#d81f26", "--size", "0x4", "--out", out]), 2)


if __name__ == "__main__":
    unittest.main()
