# -*- coding: utf-8 -*-
"""Тесты: журнал длиннее записи, снимки шагов, нижняя безопасная зона подписей (spec 036).

Запуск: python -I -m unittest discover -s scripts/guide -v. Чистая логика — без ffmpeg;
TestFitRender собирает синтетику 320x180 через ffmpeg и пропускается, если его нет в PATH.
"""
import contextlib
import io
import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

from PIL import Image

import overlay
import overlay_draw
import overlay_fit
import overlay_plan as op
from test_overlay import hold, jsonl, meta, parse, sync

HAVE_FFMPEG = bool(shutil.which("ffmpeg") and shutil.which("ffprobe"))


def shot(name, rect=None, path="x.png"):
    return {"name": name, "path": path, "label": "", "rect": rect}


def long_events(end=16.0, with_step2=True):
    """Журнал, у которого шаг 2 начинается за концом записи в 10 с."""
    ev = [meta(screen=(320, 180)), sync(0.0, 0.8),
          {"ev": "chapter", "t": 1.0, "title": "Глава"},
          {"ev": "step", "t": 1.1, "n": 1, "of": 2 if with_step2 else 1, "title": "Один"},
          hold(2.0, "Первое", [100, 60, 60, 30])]
    if with_step2:
        ev += [{"ev": "step", "t": 12.0, "n": 2, "of": 2, "title": "Два"},
               hold(12.5, "Второе", [10, 10, 50, 50]), {**hold(14.0, "Третье"), "shot": "s3"}]
    return ev + [{"ev": "end", "t": end}]


class TestFit(unittest.TestCase):
    def plan(self, events, rec_end=10.0, shots=(), off=0.0):
        j = parse(events)
        plan = op.build_plan(j, off, 320, 180)
        before = plan.duration
        used = set()
        plan, warns = overlay_fit.fit_to_record(plan, j, off, rec_end, list(shots), used)
        return plan, warns, before, used

    def test_record_long_enough_is_untouched(self):
        plan, warns, before, _ = self.plan(long_events(end=9.0, with_step2=False), rec_end=12.0)
        self.assertEqual(warns, [])
        self.assertEqual(plan.duration, before)
        self.assertEqual(plan.rec_end, 12.0)

    def test_short_tail_freezes_last_frame(self):
        # журнал на 0,9 с длиннее записи: прощается, длина плана та же, окна у конца записи
        ev = long_events(end=10.9, with_step2=False)
        plan, warns, before, _ = self.plan(ev, rec_end=10.0)
        self.assertEqual(plan.duration, before)
        self.assertEqual(len(warns), 1)
        self.assertIn("последний кадр удержан", warns[0])
        raw = [s for s in plan.segs if s.kind in ("live", "fast")]
        self.assertTrue(all(s.src1 <= 10.0 + 1e-9 for s in raw))
        self.assertTrue(all(s.src0 <= 10.0 - 2 / plan.fps + 1e-9 for s in raw if s.src1 >= 10.0 - 1e-9))

    def test_tail_limit_is_inclusive(self):
        # ровно FREEZE_MAX — ещё заморозка (все живые отрезки сохранены), чуть больше — режется
        at = self.plan(long_events(end=10.0 + overlay_fit.FREEZE_MAX, with_step2=False))
        self.assertEqual(at[2], at[0].duration)
        over = self.plan(long_events(end=10.0 + overlay_fit.FREEZE_MAX + 0.2, with_step2=False))
        self.assertLess(over[0].duration, over[2])
        self.assertIn("отброшено", over[1][0])

    def test_tail_cut_drops_live_and_keeps_shots(self):
        shots = [shot("s2", [10, 10, 50, 50], path="b.png"), shot("s3", path="c.png")]
        plan, warns, before, used = self.plan(long_events(), shots=shots)
        raw = [s for s in plan.segs if s.kind in ("live", "fast")]
        self.assertTrue(raw)
        self.assertTrue(all(s.src0 < 10.0 and s.src1 <= 10.0 + 1e-9 for s in raw))
        holds = plan.holds()
        self.assertEqual([h.target.caption for h in holds], ["Первое", "Второе", "Третье"])
        self.assertEqual([h.shot for h in holds], [None, "b.png", "c.png"])  # два стоп-кадра за концом — по снимкам
        self.assertEqual(used, {"s2", "s3"})
        self.assertLess(plan.duration, before)
        self.assertIn("стоп-кадров по снимкам 2", warns[0])
        self.assertIn("стоп-кадров без снимка 0", warns[0])
        self.assertRegex(warns[0], r"отброшено [1-9]\d*")  # живое за концом ушло
        # шаг 2 жив: его плашка есть на стоп-кадрах за концом
        self.assertTrue(any(p.text.startswith("Шаг 2 из 2") for h in holds[1:] for p in h.plates))

    def test_step_without_shot_is_refused(self):
        with self.assertRaises(overlay_fit.RecordShort) as cm:
            self.plan(long_events(), shots=[])
        self.assertIn("Шаг 2 из 2. Два", str(cm.exception))
        # снимок есть у одного стоп-кадра шага — шаг не потерян, второй стоп-кадр отброшен с учётом в отчёте
        plan, warns, _, _ = self.plan(long_events(), shots=[shot("s2", [10, 10, 50, 50], path="b.png")])
        self.assertEqual([h.target.caption for h in plan.holds()], ["Первое", "Второе"])
        self.assertIn("стоп-кадров без снимка 1", warns[0])

    def test_many_lost_steps_all_named(self):
        ev = long_events()
        ev.insert(-1, {"ev": "step", "t": 15.0, "n": 3, "of": 3, "title": "Три"})
        ev = [({**e, "of": 3} if e["ev"] == "step" else e) for e in ev]
        with self.assertRaises(overlay_fit.RecordShort) as cm:
            self.plan(ev, shots=[])
        self.assertIn("Шаг 2 из 3", str(cm.exception))
        self.assertIn("Шаг 3 из 3", str(cm.exception))

    def test_trailing_card_dropped(self):
        # глава целиком за концом записи: заставка без содержимого не остаётся, но шаги — отказ
        ev = [meta(screen=(320, 180)), sync(0.0, 0.8), {"ev": "chapter", "t": 1.0, "title": "Первая"},
              {"ev": "step", "t": 1.1, "n": 1, "of": 1, "title": "Один"}, hold(2.0, "А", [10, 10, 50, 50]),
              {"ev": "chapter", "t": 12.0, "title": "Вторая"},
              {"ev": "step", "t": 12.1, "n": 2, "of": 2, "title": "Два"}, {"ev": "end", "t": 16.0}]
        with self.assertRaises(overlay_fit.RecordShort):
            self.plan(ev)
        # без шагов в главе — ролик собирается, заставка за концом убрана
        ev2 = ev[:-2] + [{"ev": "end", "t": 16.0}]
        plan, _, _, _ = self.plan(ev2)
        self.assertEqual([s.title for s in plan.segs if s.kind == "card"], ["Первая"])

    def test_shot_for_hold(self):
        shots = [shot("a", [1, 2, 3, 4]), shot("b", [5, 6, 7, 8]), shot("c")]
        self.assertEqual(overlay_fit.shot_for_hold({"rect": [5, 6, 7, 8]}, shots, set())["name"], "b")
        self.assertIsNone(overlay_fit.shot_for_hold({"rect": [5, 6, 7, 8]}, shots, {"b"}))  # уже занят
        self.assertEqual(overlay_fit.shot_for_hold({"shot": "c", "rect": None}, shots, set())["name"], "c")
        self.assertIsNone(overlay_fit.shot_for_hold({"shot": "нет", "rect": [1, 2, 3, 4]}, shots, set()))
        self.assertIsNone(overlay_fit.shot_for_hold({"rect": None}, shots, set()))  # без рамки и имени
        self.assertIsNone(overlay_fit.shot_for_hold({"rect": [9, 9, 9, 9]}, shots, set()))
        self.assertIsNone(overlay_fit.shot_for_hold({"rect": [1, 2, 3, 4]}, [], set()))

    def test_shot_for_target_needs_quoted_label(self):
        def lab(name, label, rect=(1, 2, 3, 4)):
            return {"name": name, "path": name, "label": label, "rect": list(rect) if rect else None}

        shots = [lab("a", "•Конфиденциальность и безопасность"), lab("b", "Все равно открыть"),
                 lab("c", "Готово"), lab("d", "Системные настройки..."), lab("e", "Цель", None)]
        f = overlay_fit.shot_for_target
        self.assertEqual(f("Выберите «Конфиденциальность и безопасность»", shots, set())["name"], "a")
        self.assertEqual(f("Нажмите «Все равно открыть» в этом окне", shots, set())["name"], "b")
        self.assertEqual(f("Выберите «Системные настройки…»", shots, set())["name"], "d")  # многоточие, регистр
        self.assertIsNone(f("Нажмите «Все равно открыть»", shots, {"b"}))  # снимок занят другим стоп-кадром
        self.assertIsNone(f("Программа открылась — готово", shots, set()))  # слово без кавычек — не метка
        self.assertIsNone(f("Нажмите «Цель»", shots, set()))  # у снимка нет рамки
        self.assertIsNone(f("Нажмите «Готово» или «Всё»", [], set()))
        self.assertEqual(f("Нажмите «Готово» — не «Переместить в Корзину»", shots, set())["name"], "c")

    def test_parse_shots_filters(self):
        tmp = Path(tempfile.mkdtemp(prefix="overlay-shots-"))
        self.addCleanup(shutil.rmtree, tmp, ignore_errors=True)
        (tmp / "png").mkdir()
        (tmp / "png" / "a.png").write_bytes(b"x")
        (tmp / "png" / "b.png").write_bytes(b"x")
        data = [
            {"name": "a", "ok": True, "file": "png/a.png", "target": {"label": "Кнопка", "rect": [1, 2, 3, 4]}},
            {"name": "b", "ok": True, "file": "png/b.png", "target": None},
            {"name": "c", "ok": False, "file": "png/a.png"},  # снимок не удался
            {"name": "d", "ok": True, "file": "png/нет.png"},  # файла нет
            {"name": "e", "ok": True, "file": 5},
            "мусор",
        ]
        got = overlay_fit.parse_shots(data, tmp)
        self.assertEqual([s["name"] for s in got], ["a", "b"])
        self.assertEqual(got[0]["rect"], [1, 2, 3, 4])
        self.assertIsNone(got[1]["rect"])
        self.assertEqual(overlay_fit.parse_shots({"не": "список"}, tmp), [])
        self.assertEqual(overlay_fit.parse_shots([], tmp), [])

    def test_hold_shot_field_validated(self):
        base = [meta(), sync(), {**hold(2.0), "shot": "s1"}, {"ev": "end", "t": 5.0}]
        self.assertEqual(parse(base).events[1]["shot"], "s1")
        for bad in ("", 5, ["s"]):
            with self.assertRaisesRegex(ValueError, "строка 3: hold.shot"):
                parse([meta(), sync(), {**hold(2.0), "shot": bad}, {"ev": "end", "t": 5.0}])
        parse([meta(), sync(), {**hold(2.0), "shot": None}, {"ev": "end", "t": 5.0}])  # null — можно


class TestTargetMatches(unittest.TestCase):
    def pic(self, button=True):
        img = Image.new("RGB", (320, 180), (0x20, 0x30, 0x40))
        if button:
            img.paste((255, 255, 255), (100, 60, 160, 90))
            img.paste((0x30, 0x30, 0x30), (110, 70, 130, 80))
        return img

    def test_same_picture_matches(self):
        self.assertTrue(overlay.target_matches(self.pic(), self.pic(), (100, 60, 160, 90)))

    def test_different_picture_or_empty_area_rejected(self):
        self.assertFalse(overlay.target_matches(self.pic(), self.pic(False), (100, 60, 160, 90)))  # на снимке пусто
        # пустой фон в обеих картинках совпадает «сам собой» — не подтверждение
        self.assertFalse(overlay.target_matches(self.pic(), self.pic(), (200, 100, 260, 140)))

    def test_rect_outside_frame_rejected(self):
        for rect in ((-5, 60, 40, 90), (300, 60, 340, 90), (100, 60, 100, 90), (100, 170, 160, 200)):
            with self.subTest(rect):
                self.assertFalse(overlay.target_matches(self.pic(), self.pic(), rect))


class TestSafeZone(unittest.TestCase):
    def test_safe_bottom_scales(self):
        self.assertEqual(op.Metrics(1920, 1080).safe_bottom, 120)
        self.assertEqual(op.Metrics(1280, 720).safe_bottom, 80)
        self.assertEqual(op.Metrics(320, 180).safe_bottom, 20)

    def bottom_ink(self, caption, w=1920, h=1080):
        """Самая нижняя строка пикселей, тронутых подписью без цели (на чёрном кадре)."""
        painter = overlay_draw.Painter(w, h)
        seg = op.Seg("hold", 0, 0, 2.0, frames=30, plates=(), target=op.Target(None, caption))
        img = painter.hold_frame(Image.new("RGB", (w, h)), seg, [])
        bbox = img.convert("L").point(lambda v: 255 if v > 0 else 0).getbbox()
        return bbox

    def test_bottom_caption_above_dock(self):
        for caption in ("Нажмите «Готово»", "Программа открыта. Дальше она открывается сразу " * 3):
            with self.subTest(len(caption)):
                bbox = self.bottom_ink(caption)
                self.assertIsNotNone(bbox)
                self.assertLessEqual(bbox[3], 1080 - 120, bbox)  # Dock 1010-1075 не закрыт
        small = self.bottom_ink("Нажмите «Готово»", 960, 540)
        self.assertLessEqual(small[3], 540 - 60, small)  # масштаб по высоте кадра

    def test_keys_plate_above_dock(self):
        m = op.Metrics(1920, 1080)
        plate = op.Plate("keys", "⌘ + Пробел", "bc")
        layer, rects = overlay_draw.Painter(1920, 1080).plates_layer((plate,))
        self.assertLessEqual(rects[0][3], 1080 - 120)
        self.assertLessEqual(layer.getchannel("A").getbbox()[3], 1080 - 120)
        self.assertEqual(m.plate_rect("tc", "keys", 100, 50)[1], m.margin + m.plate_h("step") + m.margin // 2)  # верх не тронут


@unittest.skipUnless(HAVE_FFMPEG, "нет ffmpeg/ffprobe в PATH")
class TestFitRender(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = Path(tempfile.mkdtemp(prefix="overlay-fit-"))
        src = ("color=c=0x203040:s=320x180:r=15:d=10,"
               "drawbox=x=0:y=0:w=iw:h=ih:color=0xd81f26:t=fill:enable='between(t,0.5,1.3)'")
        cls.raw = cls.tmp / "raw.mp4"
        subprocess.run(["ffmpeg", "-nostdin", "-y", "-loglevel", "error", "-f", "lavfi", "-i", src,
                        "-c:v", "libx264", "-pix_fmt", "yuv420p", str(cls.raw)], check=True)

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.tmp, ignore_errors=True)

    def artifact(self, name, events, shots=()):
        """Папка артефакта: video/steps.jsonl + raw.mp4, png/ и shots.json со снимками (имя, цвет)."""
        art = self.tmp / name
        (art / "video").mkdir(parents=True)
        (art / "png").mkdir()
        shutil.copyfile(self.raw, art / "video" / "raw.mp4")
        (art / "video" / "steps.jsonl").write_text(jsonl(events), encoding="utf-8")
        listing = []
        for nm, color in shots:
            Image.new("RGB", (320, 180), color).save(art / "png" / f"{nm}.png")
            listing.append({"name": nm, "ok": True, "file": f"png/{nm}.png", "target": None})
        (art / "shots.json").write_text(json.dumps(listing), encoding="utf-8")
        return art

    def run_overlay(self, art, *extra):
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(buf):
            code = overlay.main([str(art), "--width", "320", *extra])
        return code, buf.getvalue()

    def test_short_tail_freezes_and_builds(self):
        art = self.artifact("freeze", long_events(end=10.4, with_step2=False))
        out = self.tmp / "freeze.mp4"
        code, text = self.run_overlay(art, "--out", str(out))
        self.assertEqual(code, 0, text)
        self.assertIn("последний кадр удержан", text)
        plan = op.build_plan(parse(long_events(end=10.4, with_step2=False)), 0.5, 320, 180)
        self.assertAlmostEqual(overlay.probe(out)["duration"], plan.duration, delta=2 / 15)

    def test_long_tail_uses_shots(self):
        art = self.artifact("shots", long_events(), [("s2", (0, 0, 200)), ("s3", (0, 200, 0))])
        # снимок s2 найдётся по рамке стоп-кадра, s3 — по имени в журнале
        listing = json.loads((art / "shots.json").read_text(encoding="utf-8"))
        listing[0]["target"] = {"label": "x", "rect": [10, 10, 50, 50]}
        (art / "shots.json").write_text(json.dumps(listing), encoding="utf-8")
        frames, out = self.tmp / "frames", self.tmp / "shots.mp4"
        code, text = self.run_overlay(art, "--out", str(out), "--frames", str(frames), "--no-layers")
        self.assertEqual(code, 0, text)
        self.assertIn("стоп-кадров по снимкам 2", text)
        corner = [Image.open(frames / f"hold-{n:02d}.png").convert("RGB").getpixel((3, 176)) for n in (2, 3)]
        self.assertGreater(corner[0][2], 150, corner)  # второй стоп-кадр — синий снимок
        self.assertGreater(corner[1][1], 150, corner)  # третий — зелёный
        plan = overlay.probe(out)
        self.assertGreater(plan["duration"], 5)

    def test_long_tail_without_shots_is_refused(self):
        art = self.artifact("noshots", long_events())
        out = self.tmp / "noshots.mp4"
        code, text = self.run_overlay(art, "--out", str(out))
        self.assertEqual(code, 4, text)
        self.assertIn("потерян шаг без снимка", text)
        self.assertFalse(out.exists())

    def target_artifact(self, name, label, shot_color):
        """Запись с белой кнопкой (внутри тёмная метка) и снимок: shot_color=None — снимок с той же кнопкой."""
        src = ("color=c=0x203040:s=320x180:r=15:d=10,"
               "drawbox=x=0:y=0:w=iw:h=ih:color=0xd81f26:t=fill:enable='between(t,0.5,1.3)',"
               "drawbox=x=100:y=60:w=60:h=30:color=white:t=fill:enable='gte(t,1.4)',"
               "drawbox=x=110:y=70:w=20:h=10:color=0x303030:t=fill:enable='gte(t,1.4)'")
        art = self.tmp / name
        (art / "video").mkdir(parents=True)
        (art / "png").mkdir()
        subprocess.run(["ffmpeg", "-nostdin", "-y", "-loglevel", "error", "-f", "lavfi", "-i", src,
                        "-c:v", "libx264", "-pix_fmt", "yuv420p", str(art / "video" / "raw.mp4")], check=True)
        ev = [meta(screen=(320, 180)), sync(0.0, 0.8), hold(4.0, f"Нажмите «{label}»"), {"ev": "end", "t": 6.0}]
        (art / "video" / "steps.jsonl").write_text(jsonl(ev), encoding="utf-8")
        img = Image.new("RGB", (320, 180), shot_color or (0x20, 0x30, 0x40))
        if shot_color is None:
            img.paste((255, 255, 255), (100, 60, 160, 90))
            img.paste((0x30, 0x30, 0x30), (110, 70, 130, 80))
        img.save(art / "png" / "s.png")
        (art / "shots.json").write_text(json.dumps([{"name": "s", "ok": True, "file": "png/s.png",
                                                     "target": {"label": label, "rect": [100, 60, 60, 30]}}]),
                                        encoding="utf-8")
        return art

    def test_missing_target_taken_from_shot(self):
        art = self.target_artifact("tgt-ok", "Кнопка", None)
        code, text = self.run_overlay(art, "--check")
        self.assertEqual(code, 0, text)
        self.assertIn("Стоп-кадров 1, со стрелкой 1", text)
        self.assertNotIn("стоп-кадр без цели", text)

    def test_target_from_shot_rejected_when_screen_differs(self):
        # на снимке другой экран: рамка в записи не подтверждена — подпись без стрелки и предупреждение
        art = self.target_artifact("tgt-bad", "Кнопка", (0, 0, 255))
        code, text = self.run_overlay(art, "--check")
        self.assertEqual(code, 0, text)
        self.assertIn("со стрелкой 0", text)
        self.assertIn("стоп-кадр без цели", text)
        # метка не в кавычках подписи — снимок не берётся вообще
        art = self.target_artifact("tgt-noquote", "Кнопка", None)
        ev = [meta(screen=(320, 180)), sync(0.0, 0.8), hold(4.0, "Кнопка тут"), {"ev": "end", "t": 6.0}]
        (art / "video" / "steps.jsonl").write_text(jsonl(ev), encoding="utf-8")
        code, text = self.run_overlay(art, "--check")
        self.assertIn("со стрелкой 0", text)

    def test_hold_without_target_is_warned(self):
        # стоп-кадр без рамки и без клавиш — предупреждение; с клавишами подпись без стрелки нормальна
        ev = [meta(screen=(320, 180)), sync(0.0, 0.8), hold(2.0, "Без цели"),
              hold(4.0, "Нажмите ⌘ + V", None, ["⌘", "V"]), {"ev": "end", "t": 6.0}]
        art = self.artifact("notarget", ev)
        code, text = self.run_overlay(art, "--check")
        self.assertEqual(code, 0, text)
        self.assertIn("стоп-кадр без цели", text)
        self.assertEqual(text.count("стоп-кадр без цели"), 1)  # только первый
        self.assertIn("стоп-кадр 1 ", text)


if __name__ == "__main__":
    unittest.main()
