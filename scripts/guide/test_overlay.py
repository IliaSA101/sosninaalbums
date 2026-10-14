# -*- coding: utf-8 -*-
"""Тесты монтажа ролика-инструкции. Запуск: python -I -m unittest discover -s scripts/guide -v

TestJournal/TestPlan/TestChecks — чистая логика, без ffmpeg. TestRender собирает синтетику
через ffmpeg lavfi (320x180, 10 с) и пропускается, если ffmpeg нет в PATH.
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

import annotate
import overlay
import overlay_draw
import overlay_plan as op

HAVE_FFMPEG = bool(shutil.which("ffmpeg") and shutil.which("ffprobe"))
COLOR = "#d81f26"
BG = (0x20, 0x30, 0x40)


def jsonl(events: list) -> str:
    return "\n".join(json.dumps(e, ensure_ascii=False) for e in events) + "\n"


def meta(space="pixels", screen=(1920, 1080), scale=1):
    return {"ev": "meta", "os": "test", "space": space, "screen": list(screen), "scale": scale}


def sync(t=0.0, dur=0.8):
    return {"ev": "sync", "t": t, "kind": "flash", "color": COLOR, "dur": dur}


def hold(t, caption="Нажмите «Готово»", rect=None, keys=None):
    return {"ev": "hold", "t": t, "rect": rect, "caption": caption, "keys": keys}


def parse(events: list, **kw) -> op.Journal:
    return op.parse_journal(jsonl(events), **kw)


def sample_events() -> list:
    """Журнал на 320x180: глава, шаг, два стоп-кадра, клавиши, простой. Ожидаемая длина — 12,05 с."""
    return [
        meta(screen=(320, 180)), sync(0.0, 0.8),
        {"ev": "chapter", "t": 1.0, "title": "Первый запуск"},
        {"ev": "step", "t": 1.1, "n": 1, "of": 2, "title": "Откройте программу"},
        hold(2.0, "Нажмите «Готово»", [100, 60, 60, 30]),
        {"ev": "move", "t0": 2.05, "t1": 2.5},
        {"ev": "keys", "t0": 3.0, "t1": 3.3, "keys": ["⌘", "Пробел"]},
        {"ev": "idle", "t0": 4.0, "t1": 7.0},
        hold(7.2, "Подпись без стрелки", None, ["⌘", "V"]),
        {"ev": "end", "t": 8.5},
    ]


class TestJournal(unittest.TestCase):
    def test_parse_ok(self):
        j = parse(sample_events())
        self.assertEqual(j.meta["space"], "pixels")
        self.assertEqual(len(j.events), 9)
        self.assertEqual(j.events[0]["line"], 2)  # meta — строка 1
        self.assertEqual(j.sync()["color"], COLOR)

    def test_parse_rejects_bad_lines(self):
        base = [meta(), sync(), hold(2.0, "Нажмите", [10, 10, 50, 50]), {"ev": "end", "t": 5.0}]

        def with_line(i, **upd):
            ev = [dict(e) for e in base]
            ev[i] = {**ev[i], **upd}
            return ev

        cases = {
            "убывающее t": ([meta(), sync(2.0), hold(1.0), {"ev": "end", "t": 5.0}], 3),
            "пустая подпись": (with_line(2, caption="  "), 3),
            "длинная подпись": (with_line(2, caption="я" * 81), 3),
            "рамка вне экрана": (with_line(2, rect=[1900, 10, 50, 50]), 3),
            "отрицательная рамка": (with_line(2, rect=[-5, 10, 50, 50]), 3),
            "meta не первой": ([sync(), meta(), {"ev": "end", "t": 5.0}], 1),
            "meta дважды": ([meta(), sync(), meta(), {"ev": "end", "t": 5.0}], 3),
            "неизвестное событие": ([meta(), sync(), {"ev": "boom", "t": 1.0}, {"ev": "end", "t": 5.0}], 3),
            "idle пересекает move": (
                [meta(), sync(), {"ev": "move", "t0": 1.0, "t1": 3.0}, {"ev": "idle", "t0": 2.0, "t1": 4.0},
                 {"ev": "end", "t": 5.0}], 3),
            "hold внутри idle": (
                [meta(), sync(), {"ev": "idle", "t0": 1.0, "t1": 4.0}, hold(2.0), {"ev": "end", "t": 5.0}], 4),
            "нет end": ([meta(), sync(), hold(2.0)], 3),
            "событие после end": ([meta(), sync(), {"ev": "end", "t": 5.0}, hold(6.0)], 4),
            "нет sync": ([meta(), hold(2.0), {"ev": "end", "t": 5.0}], 3),
            "плохой цвет": ([meta(), {**sync(), "color": "red"}, {"ev": "end", "t": 5.0}], 2),
            "step.n больше of": ([meta(), sync(), {"ev": "step", "t": 1.0, "n": 3, "of": 2, "title": "x"},
                                  {"ev": "end", "t": 5.0}], 3),
        }
        for name, (events, line) in cases.items():
            with self.subTest(name):
                with self.assertRaises(ValueError) as cm:
                    parse(events)
                self.assertIn(f"строка {line}:", str(cm.exception))

    def test_parse_bad_json_and_empty(self):
        with self.assertRaisesRegex(ValueError, "строка 2: не JSON"):
            op.parse_journal(jsonl([meta()]) + "{oops\n")
        with self.assertRaisesRegex(ValueError, "журнал пуст"):
            op.parse_journal("\n\n")

    def test_manual_journal_without_sync(self):
        # запись владельца: метки нет, смещение задаётся --offset
        j = parse([meta(), hold(2.0), {"ev": "end", "t": 5.0}], require_sync=False)
        self.assertIsNone(j.sync())

    def test_rect_scaling_hidpi(self):
        m = meta("points", (1440, 900), 2)
        m["scale"] = 2
        # k из design §2: пиксель экрана (точки × 2 = 2880) -> пиксель ролика 1920
        self.assertAlmostEqual(op.px_scale(m, 1920), 1920 / 2880)
        # точки -> пиксели (× 2) -> ролик (× 0,6667): итог 1920 / 1440
        x0, y0, x1, y1 = op.rect_to_frame([300, 150, 90, 60], m, 1920)
        self.assertAlmostEqual(x0, 400.0)
        self.assertAlmostEqual(y0, 200.0)
        self.assertAlmostEqual(x1 - x0, 120.0)
        self.assertAlmostEqual(y1 - y0, 80.0)
        # hidpi 1440x900 (16:10) -> запись 1920x1200: по обеим осям один множитель, 1080 не предполагается
        _, y0, _, y1 = op.rect_to_frame([0, 450, 10, 450], m, 1920)
        self.assertAlmostEqual(y0, 600.0)
        self.assertAlmostEqual(y1, 1200.0)  # низ экрана = низ кадра 1920x1200
        # Windows: пиксели, scale не участвует
        w = meta("pixels", (1280, 720), 2)
        self.assertEqual(op.rect_to_frame([100, 50, 200, 40], w, 1920), (150.0, 75.0, 450.0, 135.0))

    def test_find_flash_frame(self):
        gray, red = (32, 48, 64), (216, 31, 38)
        self.assertEqual(op.find_flash_frame([gray] * 5 + [red] * 3 + [gray], COLOR), 5)
        # переход: первый кадр, ближе к красному, чем к исходному
        self.assertEqual(op.find_flash_frame([gray, gray, (150, 40, 50), red], COLOR), 2)
        self.assertIsNone(op.find_flash_frame([gray] * 10, COLOR))  # вспышки нет
        self.assertIsNone(op.find_flash_frame([red] * 10, COLOR))  # с первого кадра — не отличить
        self.assertIsNone(op.find_flash_frame([], COLOR))


class TestPlan(unittest.TestCase):
    def plan(self, events, off=0.0, w=1920, h=1080):
        return op.build_plan(parse(events), off, w, h)

    def test_hold_before_each_action(self):
        events = [
            meta(), sync(0.0, 0.8), {"ev": "step", "t": 1.0, "n": 1, "of": 2, "title": "Шаг"},
            hold(2.0, "Первое", [10, 10, 50, 50]), {"ev": "move", "t0": 2.05, "t1": 2.5},
            hold(4.0, "Второе", [300, 300, 50, 50]), {"ev": "keys", "t0": 4.5, "t1": 4.8, "keys": ["⌘", "V"]},
            hold(6.0, "Третье", None), {"ev": "end", "t": 8.0},
        ]
        p = self.plan(events)
        holds = [(i, s) for i, s in enumerate(p.segs) if s.kind == "hold"]
        self.assertEqual(len(holds), 3)
        for i, s in holds:
            self.assertEqual(s.src0, s.src1)  # один кадр записи
            before, after = p.segs[i - 1], p.segs[i + 1]
            self.assertEqual(before.kind, "live")
            self.assertAlmostEqual(before.src1, s.src0)  # живой отрезок кончается на кадре стоп-кадра...
            self.assertEqual(after.kind, "live")
            self.assertAlmostEqual(after.src0, s.src0)  # ...и следующий начинается с него же
        self.assertEqual([s.src0 for _, s in holds], [2.0, 4.0, 6.0])

    def test_hold_duration_by_caption(self):
        self.assertEqual(op.hold_duration("я" * 10), 2.0)
        self.assertEqual(op.hold_duration("я" * 40), 3.5)
        self.assertEqual(op.hold_duration("я"), 2.0)  # нижний предел
        self.assertEqual(op.hold_duration("я" * 80), 3.5)  # верхний предел
        p = self.plan([meta(), sync(), hold(2.0, "я" * 10), hold(5.0, "ж" * 40), {"ev": "end", "t": 8.0}])
        self.assertEqual([s.dur for s in p.holds()], [2.0, 3.5])
        self.assertEqual([s.frames for s in p.holds()], [30, 53])

    def test_idle_speedup_capped(self):
        # живой промежуток до idle короче SETTLE_KEEP + IDLE_CAP — остаётся живьём
        p = self.plan([meta(), sync(), {"ev": "idle", "t0": 2.0, "t1": 11.0}, {"ev": "end", "t": 12.0}])
        fast = [s for s in p.segs if s.kind == "fast"]
        self.assertEqual(len(fast), 1)
        self.assertEqual(fast[0].dur, 0.6)
        self.assertEqual((fast[0].src0, fast[0].src1), (2.0, 11.0))
        # live 1,2 (после 0,8 вспышки) + 0,6 + live 1,0
        self.assertAlmostEqual(p.duration, 2.8, delta=1 / 15)
        # короткое ожидание не режется: отрезка fast нет, время идёт как есть
        q = self.plan([meta(), sync(), {"ev": "idle", "t0": 1.0, "t1": 1.3}, {"ev": "end", "t": 2.0}])
        self.assertEqual([s.kind for s in q.segs], ["live"])
        self.assertAlmostEqual(q.duration, 1.2, delta=1 / 15)

    def test_overlays_avoid_targets(self):
        step = {"ev": "step", "t": 1.0, "n": 1, "of": 6, "title": "Откройте меню Apple"}
        m = meta("points", (1440, 900), 2)
        # меню Apple в углу: плашка шага уходит вправо
        p = self.plan([m, sync(), step, hold(2.0, "Откройте меню Apple", [24, 12, 20, 20]),
                       {"ev": "end", "t": 4.0}])
        self.assertEqual(p.holds()[0].plates[0].anchor, "tr")
        live = [s for s in p.segs if s.kind == "live" and s.plates]
        self.assertTrue(all(s.plates[0].anchor == "tr" for s in live))  # один якорь на весь шаг
        # цель в центре: плашка остаётся слева
        q = self.plan([m, sync(), step, hold(2.0, "Нажмите", [700, 400, 100, 40]), {"ev": "end", "t": 4.0}])
        self.assertEqual(q.holds()[0].plates[0].anchor, "tl")
        # цель внизу по центру: плашка клавиш уходит вверх
        r = self.plan([m, sync(), step, hold(2.0, "Нажмите ⌘ + V", [600, 700, 200, 60], ["⌘", "V"]),
                       {"ev": "end", "t": 4.0}])
        keys = [pl for pl in r.holds()[0].plates if pl.kind == "keys"]
        self.assertEqual([k.anchor for k in keys], ["tc"])
        # цели нет — подпись внизу, плашка клавиш тоже уходит вверх
        s = self.plan([m, sync(), hold(2.0, "Если спросит пароль", None, ["⌘", "V"]), {"ev": "end", "t": 4.0}])
        self.assertEqual(s.holds()[0].plates[0].anchor, "tc")
        # цель свободна — плашка клавиш внизу
        t = self.plan([m, sync(), hold(2.0, "Нажмите", [700, 100, 100, 40], ["⌘", "V"]), {"ev": "end", "t": 4.0}])
        self.assertEqual(t.holds()[0].plates[0].anchor, "bc")

    def test_keys_window_on_live(self):
        p = self.plan([meta(), sync(), {"ev": "keys", "t0": 3.0, "t1": 3.3, "keys": ["⌘", "Пробел"]},
                       {"ev": "type", "t0": 6.0, "t1": 6.5, "text": "Терминал"}, {"ev": "end", "t": 9.0}])
        kseg = [s for s in p.segs if any(pl.kind == "keys" for pl in s.plates)]
        self.assertEqual([pl.text for s in kseg for pl in s.plates], ["⌘ + Пробел", "Ввод: Терминал"])
        self.assertAlmostEqual(kseg[0].src0, 3.0 - op.KEYS_PRE)
        self.assertAlmostEqual(kseg[0].src1, 3.3 + op.KEYS_POST)
        self.assertAlmostEqual(kseg[1].src0, 6.0 - op.KEYS_PRE)
        self.assertAlmostEqual(kseg[1].src1, 6.5 + op.KEYS_POST)

    def test_chapter_card_and_step_reset(self):
        p = self.plan(sample_events(), off=0.5, w=320, h=180)
        cards = [s for s in p.segs if s.kind == "card"]
        self.assertEqual([(c.title, c.dur) for c in cards], [("Первый запуск", 2.0)])
        self.assertEqual(p.segs[0].plates, ())  # до шага плашки шага нет
        steps = [pl for s in p.segs for pl in s.plates if pl.kind == "step"]
        self.assertTrue(steps)
        self.assertEqual({pl.text for pl in steps}, {"Шаг 1 из 2. Откройте программу"})

    def test_duration_sums_without_drift(self):
        # ручной расчёт: 0,2 + 2 (заставка) + 0,1 + 0,9 + 2,3 + 2,0 + 0,6 + 0,2 + 2,45 + 1,3
        p = self.plan(sample_events(), off=0.5, w=320, h=180)
        self.assertAlmostEqual(p.duration, 12.05, delta=1 / 15)
        self.assertEqual(len(p.holds()), 2)
        self.assertIsNotNone(p.holds()[0].target.rect)
        self.assertIsNone(p.holds()[1].target.rect)

    def test_many_short_segments_keep_total(self):
        # 40 коротких окон клавиш: потери на округлении каждого отрезка не копятся
        ev = [meta(), sync(0.0, 0.1)]
        for i in range(40):
            ev.append({"ev": "keys", "t0": 1.0 + i * 0.43, "t1": 1.01 + i * 0.43, "keys": ["A"]})
        ev.append({"ev": "end", "t": 20.0})
        p = self.plan(ev)
        self.assertGreater(len(p.segs), 40)
        self.assertAlmostEqual(p.duration, 20.0 - 0.1, delta=1 / 15)

    def test_hold_inside_sync_not_rewound(self):
        # стоп-кадр до конца вспышки не должен отматывать курсор на вспышку
        p = self.plan([meta(), sync(0.0, 2.0), hold(0.5), {"ev": "end", "t": 4.0}])
        nxt = p.segs[p.segs.index(p.holds()[0]) + 1]
        self.assertGreaterEqual(nxt.src0, 2.0)


class TestChecks(unittest.TestCase):
    def test_duration_limits(self):
        self.assertEqual(overlay.check_duration(60.0), [])
        self.assertEqual(len(overlay.check_duration(29.0)), 1)
        self.assertEqual(len(overlay.check_duration(100.0)), 1)
        with self.assertRaises(overlay.Fail) as cm:
            overlay.check_duration(121.0)
        self.assertEqual(cm.exception.code, 3)

    def test_uniform_and_diff(self):
        flat = Image.new("RGB", (40, 40), (10, 20, 30))
        noisy = flat.copy()
        noisy.paste((255, 255, 255), (0, 0, 10, 10))
        self.assertTrue(overlay.is_uniform(flat))
        self.assertFalse(overlay.is_uniform(noisy))
        self.assertEqual(overlay.diff_ratio(flat, flat), 0.0)
        self.assertGreater(overlay.diff_ratio(flat, noisy), 0.05)


@unittest.skipUnless(HAVE_FFMPEG, "нет ffmpeg/ffprobe в PATH")
class TestRender(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = Path(tempfile.mkdtemp(prefix="overlay-test-"))
        # вспышка в записи на 0,5-1,3 с; зелёная полоса появляется на 4,5 с (действие)
        cls.raw = cls.make_raw(cls.tmp / "raw.mp4", flash=(0.5, 1.3), change=4.5)
        cls.raw_late = cls.make_raw(cls.tmp / "raw-late.mp4", flash=(1.2, 2.0), change=9.0)
        cls.out = cls.tmp / "out.mp4"
        cls.frames = cls.tmp / "frames"
        cls.code = cls.run_overlay(sample_events(), cls.raw, ["--out", str(cls.out), "--frames", str(cls.frames)])
        cls.setup_log = cls.last_output

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.tmp, ignore_errors=True)

    @staticmethod
    def make_raw(path: Path, flash, change, dur=10):
        src = (f"color=c=0x203040:s=320x180:r=15:d={dur},"
               f"drawbox=x=0:y=0:w=iw:h=ih:color=0xd81f26:t=fill:enable='between(t,{flash[0]},{flash[1]})',"
               f"drawbox=x=0:y=60:w=iw:h=60:color=0x00c000:t=fill:enable='gte(t,{change})'")
        subprocess.run(["ffmpeg", "-nostdin", "-y", "-loglevel", "error", "-f", "lavfi", "-i", src,
                        "-c:v", "libx264", "-pix_fmt", "yuv420p", str(path)], check=True)
        return path

    @classmethod
    def run_overlay(cls, events, raw, extra, name="steps.jsonl"):
        jp = cls.tmp / name
        jp.write_text(jsonl(events), encoding="utf-8")
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(buf):
            code = overlay.main([str(jp), "--raw", str(raw), "--width", "320", *extra])
        cls.last_output = buf.getvalue()
        return code

    @staticmethod
    def probe_duration(p: Path) -> float:
        return overlay.probe(p)["duration"]

    def test_duration_matches_plan(self):
        self.assertEqual(self.code, 0, self.setup_log)
        plan = op.build_plan(parse(sample_events()), 0.5, 320, 180)
        self.assertAlmostEqual(self.probe_duration(self.out), plan.duration, delta=1 / 15 + 0.02)
        self.assertAlmostEqual(plan.duration, 12.05, delta=1 / 15)  # и ручной расчёт

    def test_output_format(self):
        p = overlay.probe(self.out)
        self.assertEqual((p["codec"], p["pix_fmt"], p["w"], p["h"]), ("h264", "yuv420p", 320, 180))
        self.assertTrue(overlay.is_faststart(self.out))
        self.assertLessEqual(p["size"], overlay.MAX_MB * 1024 * 1024)
        self.assertNotIn("ПРЕДУПРЕЖДЕНИЕ: формат", self.last_output)

    def test_frames_saved_for_each_hold(self):
        files = sorted(f.name for f in self.frames.glob("*.png"))
        self.assertEqual(files, ["hold-01.png", "hold-02.png"])
        img = Image.open(self.frames / "hold-01.png")
        self.assertEqual(img.size, (320, 180))
        # красная рамка цели нарисована: в кадре есть красные пиксели, которых нет в записи
        px = img.convert("RGB").load()
        reds = sum(1 for y in range(img.height) for x in range(img.width)
                   if px[x, y][0] > 190 and px[x, y][1] < 70 and px[x, y][2] < 80)
        self.assertGreater(reds, 30)

    def test_hold_frame_is_pre_action(self):
        events = [meta(screen=(320, 180)), sync(0.0, 0.8), hold(3.9, "До"), hold(4.1, "После"),
                  {"ev": "end", "t": 6.0}]
        d = self.tmp / "pre"
        code = self.run_overlay(events, self.raw, ["--out", str(d / "o.mp4"), "--frames", str(d), "--no-layers"],
                                "pre.jsonl")
        self.assertEqual(code, 0, self.last_output)
        before = Image.open(d / "hold-01.png").convert("RGB").getpixel((160, 90))
        after = Image.open(d / "hold-02.png").convert("RGB").getpixel((160, 90))
        self.assertLess(max(abs(a - b) for a, b in zip(before, BG)), 25, before)  # серый фон: действия ещё нет
        self.assertGreater(after[1], 150, after)  # зелёная полоса: действие уже случилось
        self.assertLess(after[0], 60, after)

    def test_sync_offset_detected(self):
        off = overlay.detect_offset(self.raw_late, {"t": 0.0, "color": COLOR})
        self.assertAlmostEqual(off, 1.20, delta=1 / 15)
        off = overlay.detect_offset(self.raw_late, {"t": 0.3, "color": COLOR})
        self.assertAlmostEqual(off, 0.90, delta=1 / 15)

    def test_no_sync_mark_is_refused(self):
        # запись без красной вспышки: отказ с кодом 2, файл не создаётся
        raw = self.make_raw(self.tmp / "noflash.mp4", flash=(50, 51), change=9.0)
        out = self.tmp / "refused.mp4"
        code = self.run_overlay(sample_events(), raw, ["--out", str(out)], "nosync.jsonl")
        self.assertEqual(code, 2)
        self.assertFalse(out.exists())
        self.assertIn("Метка синхронизации", self.last_output)

    def test_check_only_builds_nothing(self):
        out = self.tmp / "check.mp4"
        code = self.run_overlay(sample_events(), self.raw, ["--check", "--out", str(out)], "check.jsonl")
        self.assertEqual(code, 0, self.last_output)
        self.assertFalse(out.exists())
        self.assertIn("Стоп-кадров 2", self.last_output)
        self.assertIn("без стрелки", self.last_output)

    def test_journal_longer_than_raw_is_refused(self):
        # хвост журнала за концом записи терпим (overlay_fit), если не теряет шагов; шаг за концом — отказ
        ev = sample_events()[:-1] + [{"ev": "step", "t": 20.0, "n": 2, "of": 2, "title": "Поздний"},
                                     {"ev": "end", "t": 30.0}]
        code = self.run_overlay(ev, self.raw, ["--check"], "long.jsonl")
        self.assertEqual(code, 4)
        self.assertIn("потерян шаг без снимка", self.last_output)
        ev = sample_events()[:-1] + [{"ev": "end", "t": 30.0}]
        code = self.run_overlay(ev, self.raw, ["--check"], "long2.jsonl")
        self.assertEqual(code, 0, self.last_output)
        self.assertIn("запись короче журнала", self.last_output)

    def test_manual_offset_without_sync(self):
        ev = [meta(screen=(320, 180)), hold(3.0, "Нажмите"), {"ev": "end", "t": 5.0}]
        out = self.tmp / "manual.mp4"
        code = self.run_overlay(ev, self.raw, ["--out", str(out), "--offset", "0.5"], "manual.jsonl")
        self.assertEqual(code, 0, self.last_output)
        self.assertTrue(out.is_file())

    def test_mask_covers_frame(self):
        d = self.tmp / "mask"
        events = [meta(screen=(320, 180)), sync(0.0, 0.8), hold(2.0, "Маска"), {"ev": "end", "t": 3.0}]
        code = self.run_overlay(events, self.raw, ["--check", "--frames", str(d), "--no-layers",
                                                    "--mask", "0,0,40,40"], "mask.jsonl")
        self.assertEqual(code, 0, self.last_output)
        px = Image.open(d / "hold-01.png").convert("RGB")
        self.assertEqual(px.getpixel((10, 10)), (128, 128, 128))
        self.assertLess(max(abs(a - b) for a, b in zip(px.getpixel((200, 20)), BG)), 25)

    def test_keycap_glyphs_render(self):
        path = overlay_draw.find_sym_font()
        if not path:
            self.skipTest("нет шрифта символов (Segoe UI Symbol / DejaVu Sans / Apple Symbols)")
        sym = overlay_draw.load_font(path, 40)
        main = overlay_draw.load_font(annotate.find_font(), 40)
        self.assertTrue(overlay_draw.has_glyph(sym, "⌘"))  # ⌘ есть в шрифте символов
        self.assertFalse(overlay_draw.has_glyph(main, "⌘"))  # а в основном (Arial) — нет
        self.assertFalse(overlay_draw.has_glyph(sym, "￿"))
        # смесь шрифтов: ⌘ рисуется шрифтом символов, кириллица — любым, где она есть
        runs = overlay_draw.glyph_runs("⌘ + Пробел", [main, sym])
        self.assertIs(runs[0][0], sym)
        self.assertIs(runs[-1][0], main)
        self.assertTrue(all(overlay_draw.has_glyph(f, ch) for f, s in runs for ch in s))
        layer, rects = overlay_draw.Painter(1920, 1080).plates_layer((op.Plate("keys", "⌘ + V", "bc"),))
        self.assertEqual(len(rects), 1)
        self.assertIsNotNone(layer.getchannel("A").getbbox())


class TestCaptionGlyphs(unittest.TestCase):
    """Подпись цели с «⌘» рисуется со сменой шрифта; обычные подписи — прежним draw_target."""

    CMD = "Нажмите \u2318 + Пробел"

    def setUp(self):
        self.p = overlay_draw.Painter(1920, 1080)
        if not self.p.sym_path:
            self.skipTest("нет шрифта символов (Segoe UI Symbol / DejaVu Sans / Apple Symbols)")
        self.base = Image.new("RGB", (1920, 1080), (40, 60, 80))

    def frame(self, painter, caption, rect=(900, 500, 1000, 560)):
        seg = op.Seg("hold", 0, 0, 2.0, 30, plates=(), target=op.Target(rect, caption))
        return painter.hold_frame(self.base, seg, [])

    def test_symbol_caption_uses_symbol_font(self):
        self.assertTrue(self.p.needs_sym(self.CMD))
        self.assertFalse(self.p.needs_sym("Нажмите «Готово»"))

    def test_target_caption_has_no_tofu(self):
        with_sym = self.frame(self.p, self.CMD)
        # тот же кадр без шрифта символов: draw_target рисует «квадратик» вместо ⌘
        bare = overlay_draw.Painter(1920, 1080)
        bare.sym_path = ""
        tofu = self.frame(bare, self.CMD)
        self.assertNotEqual(with_sym.tobytes(), tofu.tobytes())
        # подмена: ⌘ берётся из шрифта символов, каждый знак подписи есть в выбранном шрифте
        runs = overlay_draw.glyph_runs(self.CMD, self.p.caption_fonts())
        self.assertTrue(all(overlay_draw.has_glyph(f, ch) for f, s in runs for ch in s))
        self.assertEqual(len(runs), 3)  # «Нажмите », «⌘», « + Пробел»
        # красная подпись и рамка нарисованы
        px = with_sym.load()
        reds = sum(1 for y in range(1080) for x in range(0, 1000, 2) if px[x, y][0] > 190 and px[x, y][1] < 70)
        self.assertGreater(reds, 200)

    def test_plain_caption_matches_annotate_exactly(self):
        # без особых знаков подпись рисует сам annotate.draw_target — вид снимков docx не меняется
        caption = "Нажмите «Готово»"
        got = self.frame(self.p, caption)
        want = self.base.copy()
        annotate.draw_target(want, overlay_draw.ImageDraw.Draw(want),
                             {"rect": [900, 500, 1000, 560], "text": caption}, self.p.font_path, [])
        self.assertEqual(got.tobytes(), want.tobytes())

    def test_long_symbol_caption_wraps(self):
        caption = ("Нажмите \u2318 + Пробел и потом \u2318 + V " * 3).strip()
        fonts = self.p.caption_fonts()
        lines = self.p._wrap_runs(caption, fonts, 1920 * 0.42)
        self.assertGreater(len(lines), 1)
        self.assertTrue(all(overlay_draw.runs_width(overlay_draw.glyph_runs(ln, fonts)) <= 1920 * 0.42 for ln in lines))
        self.assertEqual(self.frame(self.p, caption, (60, 500, 160, 560)).size, (1920, 1080))


if __name__ == "__main__":
    unittest.main()
