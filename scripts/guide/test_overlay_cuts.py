# -*- coding: utf-8 -*-
"""Тесты ускоряемых зон и деления по главам (решение 132). Запуск: python -I -m unittest discover -s scripts/guide -v"""
import argparse
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
import overlay_cut
import overlay_draw
import overlay_plan as op
from test_overlay import hold, meta, parse, sync


class TestCuts(unittest.TestCase):
    def plan(self, events, off=0.0):
        return op.build_plan(parse(events), off, 1920, 1080)

    def kinds(self, p):
        return [(s.kind, round(s.dur, 2)) for s in p.segs]

    def test_uncovered_live_gap_compressed(self):
        # 30 с без событий: 1,2 живьём, остальное ускоряется до 0,6
        p = self.plan([meta(), sync(0.0, 0.8), {"ev": "end", "t": 30.8}])
        self.assertEqual(self.kinds(p), [("live", 1.2), ("fast", 0.6)])
        self.assertAlmostEqual(p.segs[1].src0, 0.8 + op.SETTLE_KEEP)
        self.assertAlmostEqual(p.segs[1].src1, 30.8)
        # край: 1,8 с — ровно SETTLE_KEEP + IDLE_CAP — ещё не режется
        q = self.plan([meta(), sync(0.0, 0.8), {"ev": "end", "t": 2.6}])
        self.assertEqual([k for k, _ in self.kinds(q)], ["live"])

    def test_gap_stops_at_hold_and_move(self):
        # стоп-кадр делит промежуток: до него свой live+fast, после — короткий move живьём и простой
        p = self.plan([meta(), sync(0.0, 0.8), hold(20.0, "Нажмите", [10, 10, 50, 50]),
                       {"ev": "move", "t0": 20.0, "t1": 21.0}, {"ev": "end", "t": 40.0}])
        self.assertEqual([s.kind for s in p.segs], ["live", "fast", "hold", "live", "fast"])
        self.assertEqual((p.segs[1].src0, p.segs[1].src1), (0.8 + op.SETTLE_KEEP, 20.0))
        # move (1 с) живой, плюс SETTLE_KEEP после него; дальше простой ускорен
        self.assertEqual((p.segs[3].src0, round(p.segs[3].src1, 3)), (20.0, 21.0 + op.SETTLE_KEEP))
        self.assertEqual((round(p.segs[4].src0, 3), p.segs[4].src1), (21.0 + op.SETTLE_KEEP, 40.0))

    def test_note_inside_gap_keeps_compression(self):
        base = [meta(), sync(0.0, 0.8)]
        with_note = self.plan(base + [{"ev": "note", "t": 15.0, "text": "ждём"}, {"ev": "end", "t": 30.8}])
        no_note = self.plan(base + [{"ev": "end", "t": 30.8}])
        self.assertEqual(self.kinds(with_note), self.kinds(no_note))
        self.assertEqual(self.kinds(with_note), [("live", 1.2), ("fast", 0.6)])

    def test_gap_not_cut_inside_keys_window(self):
        # окно плашки клавиш (с KEYS_POST) живое, простой режется по обе стороны от него
        p = self.plan([meta(), sync(0.0, 0.8), {"ev": "keys", "t0": 10.0, "t1": 10.3, "keys": ["⌘", "V"]},
                       {"ev": "end", "t": 30.0}])
        keyed = [s for s in p.segs if any(pl.kind == "keys" for pl in s.plates)]
        self.assertEqual([s.kind for s in keyed], ["live"])
        self.assertAlmostEqual(keyed[0].src1 - keyed[0].src0, 0.3 + op.KEYS_PRE + op.KEYS_POST)
        self.assertEqual(sum(1 for s in p.segs if s.kind == "fast"), 2)

    def test_long_move_capped(self):
        p = self.plan([meta(), sync(0.0, 0.8), hold(1.0, "Нажмите", [10, 10, 50, 50]),
                       {"ev": "move", "t0": 1.0, "t1": 21.0}, {"ev": "end", "t": 21.5}])
        fast = [s for s in p.segs if s.kind == "fast"]
        self.assertEqual([(s.src0, s.src1, s.dur) for s in fast], [(1.0, 21.0, op.MOVE_CAP)])
        # move ровно MOVE_CAP — не режется (край)
        q = self.plan([meta(), sync(0.0, 0.8), {"ev": "move", "t0": 1.0, "t1": 1.0 + op.MOVE_CAP},
                       {"ev": "end", "t": 4.0}])
        self.assertFalse([s for s in q.segs if s.kind == "fast"])

    def test_long_moves_each_capped(self):
        # несколько длинных move: у каждого своя зона
        ev = [meta(), sync(0.0, 0.8)]
        for i in range(3):
            ev.append({"ev": "move", "t0": 2.0 + i * 12, "t1": 12.0 + i * 12})
            ev.append(hold(13.0 + i * 12, f"Шаг {i}", None))
        ev.append({"ev": "end", "t": 40.0})
        fast = [s for s in self.plan(ev).segs if s.kind == "fast" and s.dur == op.MOVE_CAP]
        self.assertEqual(len(fast), 3)

    def test_move_split_by_hold_shares_cap(self):
        # стоп-кадр внутри длинного move: зона делится пропорционально, суммарно MOVE_CAP
        p = self.plan([meta(), sync(0.0, 0.8), {"ev": "move", "t0": 1.0, "t1": 11.0}, hold(6.0, "Середина", None),
                       {"ev": "end", "t": 11.5}])
        fast = [s for s in p.segs if s.kind == "fast"]
        self.assertEqual(len(fast), 2)
        self.assertAlmostEqual(sum(s.dur for s in fast), op.MOVE_CAP)

    def test_type_capped(self):
        p = self.plan([meta(), sync(0.0, 0.8), {"ev": "type", "t0": 2.0, "t1": 12.0, "text": "xattr"},
                       {"ev": "end", "t": 13.5}])
        fast = [s for s in p.segs if s.kind == "fast"]
        self.assertEqual([(s.src0, s.src1, s.dur) for s in fast], [(2.0, 12.0, op.TYPE_CAP)])
        # плашка набора видна и на ускоренной части
        self.assertEqual([pl.text for pl in fast[0].plates if pl.kind == "keys"], ["Ввод: xattr"])

    def test_adjacent_fast_merged(self):
        # idle сразу после длинного move: один ускоренный отрезок, а не два
        p = self.plan([meta(), sync(0.0, 0.8), {"ev": "move", "t0": 1.0, "t1": 9.0},
                       {"ev": "idle", "t0": 9.0, "t1": 15.0}, {"ev": "end", "t": 15.0}])
        fast = [s for s in p.segs if s.kind == "fast"]
        self.assertEqual(len(fast), 1)
        self.assertEqual((fast[0].src0, fast[0].src1), (1.0, 15.0))
        self.assertEqual(fast[0].dur, op.MOVE_CAP)

    def test_merge_fast_keeps_different_plates(self):
        a = {"kind": "fast", "src0": 0.0, "src1": 5.0, "dur": 0.6, "kw": {"plates": ()}}
        b = {"kind": "fast", "src0": 5.0, "src1": 9.0, "dur": 0.6, "kw": {"plates": ("x",)}}
        self.assertEqual(len(overlay_cut.merge_fast([a, b])), 2)
        self.assertEqual(overlay_cut.merge_fast([]), [])

    def chapters(self):
        return [
            meta(), sync(0.0, 0.8),
            {"ev": "chapter", "t": 2.0, "title": "Первая"},
            {"ev": "step", "t": 2.1, "n": 1, "of": 4, "title": "Раз"},
            hold(3.0, "Один", [10, 10, 50, 50]),
            {"ev": "step", "t": 5.0, "n": 2, "of": 4, "title": "Два"},
            {"ev": "chapter", "t": 7.0, "title": "Вторая"},
            {"ev": "step", "t": 7.1, "n": 3, "of": 4, "title": "Три"},  # сквозная нумерация журнала
            hold(8.0, "Два", None),
            {"ev": "end", "t": 10.0},
        ]

    def test_plan_per_chapter(self):
        parts = op.split_chapters(parse(self.chapters()))
        self.assertEqual(len(parts), 2)
        plans = [op.build_plan(x, 0.0, 1920, 1080) for x in parts]
        for p, title in zip(plans, ("Первая", "Вторая")):
            self.assertEqual(p.segs[0].kind, "card")  # у каждой главы своя заставка, первой
            self.assertEqual([s.title for s in p.segs if s.kind == "card"], [title])
        steps = [{pl.text for s in p.segs for pl in s.plates if pl.kind == "step"} for p in plans]
        self.assertEqual(steps[0], {"Шаг 1 из 2. Раз", "Шаг 2 из 2. Два"})
        self.assertEqual(steps[1], {"Шаг 1 из 1. Три"})  # нумерация внутри главы
        # подготовка до первой главы (0,8..2,0) отброшена, главы не накладываются
        self.assertEqual(plans[0].segs[1].src0, 2.0)
        # заставка 2 + 0,1 + 0,9 + стоп-кадр 2 + два простоя по 1,2+0,6
        self.assertAlmostEqual(plans[0].duration, 8.6, delta=1 / 15)
        self.assertEqual([len(p.holds()) for p in plans], [1, 1])
        # края: без глав — пусто; одна глава — один ролик
        self.assertEqual(op.split_chapters(parse([meta(), sync(), {"ev": "end", "t": 3.0}])), [])
        one = op.split_chapters(parse([meta(), sync(), {"ev": "chapter", "t": 1.0, "title": "Одна"},
                                       {"ev": "end", "t": 3.0}]))
        self.assertEqual(len(one), 1)

    def test_split_does_not_mutate_journal(self):
        j = parse(self.chapters())
        op.split_chapters(j)
        self.assertEqual([(e["n"], e["of"]) for e in j.events if e["ev"] == "step"], [(1, 4), (2, 4), (3, 4)])

    def test_chapter_out_names(self):
        base = Path("x/mac-15.mp4")
        # имя по названию главы, а не по номеру: в 0.11 первая глава — распаковка
        cases = [("Распакуйте архив", "unzip"), ("Первый запуск", "start"), ("Первый запуск на macOS 15", "start"),
                 ("Если пишет «повреждено»", "damaged"), ("Если запустили прямо из архива", "fromzip"),
                 ("Если кнопки «Выполнить в любом случае» нет", "unblock"), ("Что-то новое", "ch5"), ("", "ch5")]
        for title, suffix in cases:
            with self.subTest(title=title):
                self.assertEqual(overlay.chapter_out(base, 5, title), Path(f"x/mac-15-{suffix}.mp4"))
        self.assertEqual(overlay.chapter_out(base, 3), Path("x/mac-15-ch3.mp4"))

    def test_jobs_for_names_by_title_and_dedupes(self):
        titles = ["Распакуйте архив", "Первый запуск на macOS 15", "Если пишет «повреждено»", "Первый запуск"]
        evs = [meta(), sync()]
        for i, t in enumerate(titles):
            evs.append({"ev": "chapter", "t": 1.0 + i, "title": t})
        evs.append({"ev": "end", "t": 9.0})
        args = argparse.Namespace(split=True, chapter=0)
        jobs = overlay.jobs_for(parse(evs), args, "x/mac-15.mp4")
        names = [Path(o).name for _, o, _ in jobs]
        # повтор названия «Первый запуск» не затирает первый ролик start
        self.assertEqual(names, ["mac-15-unzip.mp4", "mac-15-start.mp4", "mac-15-damaged.mp4", "mac-15-ch4.mp4"])
        self.assertEqual(len(set(names)), 4)


class TestCaptionKeep(unittest.TestCase):
    """Подпись второго стоп-кадра шага не закрывает цель первого («... в «Программы» слева» и значок программы)."""

    def holds(self, events):
        return op.build_plan(parse(events), 0.0, 1920, 1080).holds()

    def test_keep_collects_earlier_targets_of_same_step(self):
        step = {"ev": "step", "t": 1.0, "n": 1, "of": 2, "title": "Перенесите"}
        step2 = {"ev": "step", "t": 6.0, "n": 2, "of": 2, "title": "Дальше"}
        hs = self.holds([meta(), sync(), step,
                         hold(2.0, "Перетащите", [949, 257, 64, 64]),
                         hold(3.0, "в Программы", [539, 290, 131, 28]),
                         hold(4.0, "без цели", None),
                         hold(5.0, "третий", [10, 10, 20, 20]),
                         step2, hold(7.0, "новый шаг", [300, 300, 40, 40]), {"ev": "end", "t": 9.0}])
        keeps = [h.target.keep for h in hs]
        self.assertEqual(keeps[0], ())  # первый кадр шага — ничего не избегает
        self.assertEqual(keeps[1], ((949, 257, 1013, 321),))
        self.assertEqual(len(keeps[2]), 2)  # кадр без цели ничего не добавил
        self.assertEqual(len(keeps[3]), 2)
        self.assertEqual(keeps[4], ())  # следующий шаг — с чистого листа

    def test_caption_does_not_cover_kept_target(self):
        painter = overlay_draw.Painter(1920, 1080)
        base = Image.new("RGB", (1920, 1080), (60, 60, 60))
        rect, keep = (539, 290, 670, 318), (600, 400, 664, 464)  # там, где подпись встала бы сама (на ровном фоне — ниже цели): значок источника
        seen = []
        real = overlay_draw.annotate.place_label

        def spy(img, box, wh, gap, avoid, side="auto"):
            lab, s = real(img, box, wh, gap, avoid, side)
            seen.append(lab)
            return lab, s

        def inter(a, b):
            return a[0] < b[2] and b[0] < a[2] and a[1] < b[3] and b[1] < a[3]

        for kept in ((), (keep,)):
            seg = op.Seg("hold", 0, 0, 2.0, 30, plates=(), target=op.Target(rect, "... в «Программы» слева", kept))
            overlay_draw.annotate.place_label = spy
            try:
                painter.hold_frame(base, seg, [])
            finally:
                overlay_draw.annotate.place_label = real
        self.assertTrue(inter(seen[0], keep), "без keep подпись ложится на значок: проверка осмысленна")
        self.assertFalse(inter(seen[1], keep))


@unittest.skipUnless(shutil.which("ffmpeg") and shutil.which("ffprobe"), "нет ffmpeg/ffprobe в PATH")
class TestRenderFrames(unittest.TestCase):
    def render(self, events):
        tmp = Path(tempfile.mkdtemp(prefix="overlay-cut-"))
        self.addCleanup(shutil.rmtree, tmp, ignore_errors=True)
        raw = tmp / "raw.mp4"
        src = ("color=c=0x203040:s=320x180:r=15:d=10,"
               "drawbox=x=0:y=0:w=iw:h=ih:color=0xd81f26:t=fill:enable='between(t,0.5,1.3)'")
        subprocess.run(["ffmpeg", "-nostdin", "-y", "-loglevel", "error", "-f", "lavfi", "-i", src,
                        "-c:v", "libx264", "-pix_fmt", "yuv420p", str(raw)], check=True)
        jp = tmp / "steps.jsonl"
        jp.write_text("\n".join(json.dumps(e, ensure_ascii=False) for e in events) + "\n", encoding="utf-8")
        out = tmp / "o.mp4"
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(buf):
            code = overlay.main([str(jp), "--raw", str(raw), "--width", "320", "--out", str(out)])
        self.assertEqual(code, 0, buf.getvalue())
        plan = op.build_plan(parse(events), 0.5, 320, 180)
        self.assertAlmostEqual(overlay.probe(out)["duration"], plan.duration, delta=0.01)
        return plan

    def test_subframe_segment_not_lost(self):
        # отрезок в 0,05 с (короче кадра) не должен выпасть: окно trim расширяется до кадра
        events = [meta(screen=(320, 180)), sync(0.0, 0.78), hold(2.04, "Нажмите"),
                  {"ev": "keys", "t0": 2.29, "t1": 2.29, "keys": ["A"]}, {"ev": "end", "t": 5.0}]
        plan = self.render(events)
        self.assertTrue(any(s.frames == 1 and s.dur < 1 / 15 - 0.01 for s in plan.segs))

    def test_one_frame_segments_keep_length(self):
        # окна плашек начинаются через кадр: два однокадровых отрезка подряд, concat терял кадры
        events = [meta(screen=(320, 180)), sync(0.0, 0.8),
                  {"ev": "keys", "t0": 3.0, "t1": 3.0, "keys": ["A"]},
                  {"ev": "keys", "t0": 3.0667, "t1": 3.0667, "keys": ["B"]},
                  {"ev": "keys", "t0": 3.1333, "t1": 3.1333, "keys": ["C"]}, {"ev": "end", "t": 7.0}]
        plan = self.render(events)
        ones = [i for i, s in enumerate(plan.segs) if s.frames == 1 and plan.segs[i - 1].frames == 1]
        self.assertTrue(ones)  # однокадровые отрезки подряд есть


if __name__ == "__main__":
    unittest.main()
