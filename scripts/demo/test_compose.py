"""Тесты сборки роликов (102, Д2): критерии 102-3, 102-5 и охрана аргументов demo.py. Без браузера и стенда.

Запуск: `python3 -I -m unittest discover -s scripts/demo -v`. Синтетический каталог прогона — synthrun.py по контракту
Д1 → Д2; временные файлы — в <корень>/.gotmp/demo-test (корень от пути скрипта).
"""

from __future__ import annotations

import json
import os
import re
import shutil
import sys
import tempfile
import types
import unittest
from pathlib import Path
from unittest import mock

import compose as cp
import demo
import synthrun
import timeline as tl
import verify as vf

FIX_SECONDS = 5.0
ROLL = (4.0, 8.0)  # бюджет длительности для коротких синтетических роликов (боевой — 25–35 с)
HERO = (2.0, 6.0)
_cache: dict[str, Path] = {}
_made: list[Path] = []  # все временные каталоги модуля — снимаются в tearDownModule


def tmp_root() -> Path:
    base = cp.GOTMP / "demo-test"
    base.mkdir(parents=True, exist_ok=True)
    d = Path(tempfile.mkdtemp(dir=base))
    _made.append(d)
    return d


def composed_run() -> Path:
    """Один раз на прогон тестов: синтетика 5 с (30 к/с захвата) и сборка через demo.main — как в бою."""
    if "run" not in _cache:
        root = tmp_root()
        run = synthrun.make_run(root / "20261022-000000", seconds=FIX_SECONDS, fps=30)
        _cache["run"] = run
        if demo.main(["compose", "--run", str(run)]) not in (0, 2):  # 2 — длительность вне боевых 25–35 с, ожидаемо
            raise AssertionError("compose упал на синтетике")
    return _cache["run"]


def setUpModule():
    composed_run()  # до патчей отдельных тестов (они подменяют бюджеты)


def tearDownModule():
    for d in _made:
        shutil.rmtree(d, ignore_errors=True)


def copy_meta_only(run: Path) -> Path:
    """Копия прогона без каталогов кадров (check_run их не читает) — для правок журналов и файлов."""
    dst = tmp_root() / run.name
    shutil.copytree(run, dst, ignore=shutil.ignore_patterns("frames"))
    return dst


def backends() -> list[str]:
    return [b for b, exe in (("gst", "gst-launch-1.0"), ("ffmpeg", "ffmpeg")) if shutil.which(exe)]


class TestTimeline(unittest.TestCase):
    F = [tl.Frame(1, 10.0), tl.Frame(2, 11.0)]

    def test_resample_holds_last_frame(self):
        # кадр 2 — последний; до конца окна (t=13) он держится
        self.assertEqual(tl.resample(self.F, 10.0, 13.0, fps=2), [1, 1, 2, 2, 2, 2])

    def test_resample_before_first_frame_takes_first(self):
        self.assertEqual(tl.resample(self.F, 9.0, 9.5, fps=2), [1])

    def test_resample_duration_matches(self):
        for dur in (0.01, 0.5, 1.0, 1.02, 3.0, 29.97, 29.99, 30.0):
            n = len(tl.resample(self.F, 10.0, 10.0 + dur, fps=30))
            self.assertEqual(n, max(1, round(dur * 30)), dur)
        # медленный режим: время при сборке делится на speed
        self.assertEqual(len(tl.resample(self.F, 10.0, 70.0, fps=30, speed=2)), 900)

    def test_resample_rejects_empty_window(self):
        for t0, t1 in ((5.0, 5.0), (6.0, 5.0)):
            with self.assertRaises(tl.JournalError):
                tl.resample(self.F, t0, t1)

    def test_cut_by_marks(self):
        marks = tl.parse_marks(
            '{"t": 1.0, "mark": "start"}\n{"t": 2.0, "mark": "a"}\n{"t": 3.0, "mark": "b"}\n{"t": 4.0, "mark": "a"}\n{"t": 5.0, "mark": "end"}\n'
        )
        self.assertEqual(tl.cut_by_marks(marks, "a", "b"), (2.0, 3.0))
        self.assertEqual(tl.cut_by_marks(marks, "start", "end"), (1.0, 5.0))
        self.assertEqual(tl.cut_by_marks(marks, "b", "end"), (3.0, 5.0))
        with self.assertRaises(tl.JournalError):
            tl.cut_by_marks(marks, "a", "нет-такой")
        with self.assertRaises(tl.JournalError):  # конец раньше начала: «b» после «b» не бывает
            tl.cut_by_marks(marks, "b", "b")

    def test_dedupe_merges_durations(self):
        got = tl.dedupe([("a", 66), ("a", 67), ("b", 66), ("a", 66), ("a", 1)])
        self.assertEqual(got, [("a", 133), ("b", 66), ("a", 67)])  # одинаковые, но не соседние — не сливаются
        self.assertEqual(tl.dedupe([]), [])
        self.assertEqual(tl.dedupe([("x", 5)]), [("x", 5)])

    def test_grid_ms_sum_is_exact(self):
        for fps, n in ((15, 7), (12, 101), (30, 900)):
            self.assertEqual(sum(tl.grid_ms(n, fps)), round(n * 1000 / fps))

    def test_frame_at_and_capture_fps(self):
        self.assertEqual(tl.frame_at(self.F, 10.99), 1)
        self.assertEqual(tl.frame_at(self.F, 99.0), 2)
        self.assertEqual(tl.frame_at(self.F, 0.0), 1)
        fr = [tl.Frame(i, i / 25) for i in range(1, 51)]
        self.assertAlmostEqual(tl.capture_fps(fr, 0, 10), 25.0, places=6)
        self.assertEqual(tl.capture_fps(fr[:1], 0, 10), 0.0)


class TestJournalParse(unittest.TestCase):
    def test_marks_rejected(self):
        bad = [
            "не json\n",
            "[1, 2]\n",
            '{"t": 1.0}\n',  # нет имени
            '{"t": true, "mark": "start"}\n',  # bool — не число
            '{"t": 1.0, "mark": "start"}\n{"t": 1.0, "mark": "end"}\n',  # время не растёт
            '{"t": 1.0, "mark": "a"}\n{"t": 2.0, "mark": "end"}\n',  # первая не start
            '{"t": 1.0, "mark": "start"}\n{"t": 2.0, "mark": "a"}\n',  # последняя не end
            '{"t": 1.0, "mark": "start"}\n',  # одна метка
            "",
        ]
        for text in bad:
            with self.assertRaises(tl.JournalError, msg=text):
                tl.parse_marks(text)

    def test_frames_rejected(self):
        for text in ("", "x\n", '{"i": 1}\n', '{"i": 1.5, "t": 1}\n', '{"i": 2, "t": 1}\n{"i": 1, "t": 2}\n', '{"i": 1, "t": 2}\n{"i": 2, "t": 1}\n'):
            with self.assertRaises(tl.JournalError, msg=text):
                tl.parse_frames(text)

    def test_valid_roundtrip_with_blank_lines(self):
        fr = tl.parse_frames('{"i": 1, "t": 1.5}\n\n{"i": 2, "t": 1.5}\n')
        self.assertEqual([(f.i, f.t) for f in fr], [(1, 1.5), (2, 1.5)])  # равное время допустимо — кадры пачкой


class TestCompose(unittest.TestCase):
    def test_webp_budget(self):
        run = cp.load_run(composed_run())
        out = run.root / "budget.webp"
        info = cp.build_webp(run, out)
        self.assertLessEqual(out.stat().st_size, cp.WEBP_MAX)
        wf = vf._webp_facts(out)
        self.assertEqual(wf["size"], (960, 600))
        self.assertGreater(wf["frames"], 1)
        want = sum(run.scenes[s].window(a, b)[1] - run.scenes[s].window(a, b)[0] for s, a, b in cp.hero_segments())
        self.assertAlmostEqual(wf["dur"], want, delta=0.1)
        self.assertEqual(info["bytes"], out.stat().st_size)
        with cp.Image.open(out) as im:
            self.assertEqual(im.info.get("loop"), 0)  # по кругу

    def test_webp_ladder_goes_down_and_refuses(self):
        run = cp.load_run(composed_run())
        out = run.root / "ladder.webp"
        steps = ((15, 960, 72), (15, 480, 72))
        big = cp.build_webp(run, out, steps=steps)["bytes"]
        small = cp.build_webp(run, out, steps=steps[1:])["bytes"]
        mid = (big + small) // 2
        got = cp.build_webp(run, out, steps=steps, limit=mid)
        self.assertEqual((got["fps"], got["width"]), (15, 480))  # первая ступень не легла — вторая
        with self.assertRaises(cp.BudgetError) as e:
            cp.build_webp(run, out, steps=steps, limit=1)
        self.assertEqual(e.exception.code, 5)

    def test_webp_idle_costs_nothing(self):
        # покой сливается: кадров заметно меньше, чем сетка 15 к/с даёт на длину заставки
        run = cp.load_run(composed_run())
        items, _ = cp.hero_items(run, cp.hero_segments(), 15)
        total = sum(ms for _, ms in items)
        self.assertLess(len(items), total / 1000 * 15)

    def test_mp4_props(self):
        if not backends():
            self.skipTest("нет gst-launch-1.0 и ffmpeg: mp4 собрать нечем")
        run = cp.load_run(composed_run())
        sc = run.scenes["A"]
        t0, t1 = sc.window()
        for b in backends():
            with self.subTest(backend=b):
                out = run.root / f"props-{b}.mp4"
                info = cp.build_mp4(sc, out, backend=b)
                pr = cp.probe_mp4(out)
                self.assertIn("264", pr["codec"])
                self.assertIn("High", pr["profile"])
                self.assertEqual((pr["w"], pr["h"]), (1600, 1000))
                self.assertAlmostEqual(pr["fps"], 30.0, places=2)
                self.assertAlmostEqual(pr["dur"], t1 - t0, delta=0.05)
                self.assertLessEqual(out.stat().st_size, cp.MP4_MAX)
                self.assertEqual(info["frames"], round((t1 - t0) * 30))
                self.assertFalse(list(run.root.glob(".seq-*")), "временная последовательность кадров осталась")

    def test_mp4_slow_mode_halves_duration(self):
        if not backends():
            self.skipTest("нет gst-launch-1.0 и ffmpeg: mp4 собрать нечем")
        run = cp.load_run(composed_run())
        sc = run.scenes["B"]
        out = run.root / "slow.mp4"
        cp.build_mp4(sc, out, slow=2.0)
        t0, t1 = sc.window()
        self.assertAlmostEqual(cp.probe_mp4(out)["dur"], (t1 - t0) / 2, delta=0.05)

    def test_mp4_ladder_raises_q(self):
        if not backends():
            self.skipTest("нет gst-launch-1.0 и ffmpeg: mp4 собрать нечем")
        run = cp.load_run(composed_run())
        sc = run.scenes["A"]
        first = cp.build_mp4(sc, run.root / "q0.mp4")
        self.assertEqual(first["q"], cp.MP4_Q[0])
        got = cp.build_mp4(sc, run.root / "q1.mp4", limit=first["bytes"] - 1)  # q=22 не лёг — следующая ступень
        self.assertGreater(got["q"], cp.MP4_Q[0])
        self.assertLess(got["bytes"], first["bytes"])

    def test_mp4_budget_refuses(self):
        if not backends():
            self.skipTest("нет gst-launch-1.0 и ffmpeg: mp4 собрать нечем")
        run = cp.load_run(composed_run())
        with self.assertRaises(cp.BudgetError) as e:
            cp.build_mp4(run.scenes["A"], run.root / "tiny.mp4", limit=1000)
        self.assertEqual(e.exception.code, 5)
        self.assertFalse(list(run.root.glob(".seq-*")))

    def test_still_and_sheet(self):
        run = cp.load_run(composed_run())
        with cp.Image.open(run.root / "screenshot.jpg") as im:
            self.assertEqual(im.size, (1600, 1000))
        self.assertLessEqual((run.root / "screenshot.jpg").stat().st_size, cp.SHOT_MAX)
        for name in ("A", "B"):
            sc = run.scenes[name]
            moments = cp.sheet_moments(sc)
            self.assertGreaterEqual(len(moments), len(sc.marks))  # кадр у каждой метки
            self.assertTrue((run.root / f"sheet-{name}.jpg").stat().st_size > 10_000)
            self.assertEqual([t for t, _ in moments], sorted(t for t, _ in moments))

    def test_still_budget_refuses(self):
        run = cp.load_run(composed_run())
        with self.assertRaises(cp.BudgetError):
            cp.build_still(run, run.root / "tiny.jpg", limit=100)

    def test_stage_frames_falls_back_to_copy(self):
        run = cp.load_run(composed_run())
        sc = run.scenes["A"]
        idx = [sc.frames[0].i, sc.frames[1].i, sc.frames[0].i]  # повтор кадра — две записи в последовательности
        dest = run.root / ".seq-test"
        with mock.patch("os.link", side_effect=OSError("разные тома")):
            cp.stage_frames(sc, idx, dest)
        names = sorted(p.name for p in dest.iterdir())
        self.assertEqual(names, ["000001.jpg", "000002.jpg", "000003.jpg"])
        self.assertEqual((dest / "000001.jpg").read_bytes(), (dest / "000003.jpg").read_bytes())
        shutil.rmtree(dest)

    def test_stage_frames_missing_file_refuses(self):
        run = cp.load_run(composed_run())
        with self.assertRaises(cp.Refuse):
            cp.stage_frames(run.scenes["A"], [999999], run.root / ".seq-miss")
        shutil.rmtree(run.root / ".seq-miss", ignore_errors=True)

    def test_hero_segments_default(self):
        with mock.patch.dict(sys.modules, {"scenes": None}):  # модуля Д1 нет — запасной список
            # бросок и раскладка, два варианта (B), листание трёх разворотов (A) — как в спеке 102
            self.assertEqual(cp.hero_segments(), (("B", "select", "variant-2"), ("A", "flip-1", "flip-4")))
        fake = types.SimpleNamespace(HERO=[("A", "start", "end")])
        with mock.patch.dict(sys.modules, {"scenes": fake}):
            self.assertEqual(cp.hero_segments(), (("A", "start", "end"),))

    def test_hero_unknown_mark_refuses(self):
        run = cp.load_run(composed_run())
        with self.assertRaises(cp.Refuse):
            cp.hero_items(run, (("A", "нет-метки", "end"),), 15)
        with self.assertRaises(cp.Refuse):
            cp.hero_items(run, (("C", "start", "end"),), 15)

    def test_load_run_rejects_broken(self):
        base = copy_meta_only(composed_run())
        (base / "B" / "marks.jsonl").write_text('{"t": 1.0, "mark": "end"}\n', encoding="utf-8")
        with self.assertRaises(cp.Refuse):
            cp.load_run(base)
        with self.assertRaises(cp.Refuse):
            cp.load_run(base / "нет-такого")
        base2 = copy_meta_only(composed_run())
        (base2 / "meta.json").write_text("{", encoding="utf-8")
        with self.assertRaises(cp.Refuse):
            cp.load_run(base2)

    def test_publish_copies_two_files(self):
        dest = tmp_root() / "media"
        got = cp.publish(composed_run(), dest)
        self.assertEqual(sorted(p.name for p in got), ["demo.webp", "screenshot.jpg"])
        self.assertTrue(all(p.stat().st_size > 0 for p in got))
        empty = tmp_root()
        with self.assertRaises(cp.Refuse):
            cp.publish(empty, tmp_root())


class TestCheck(unittest.TestCase):
    def test_clean_run_passes_with_short_budgets(self):
        rep = vf.check_run(composed_run(), roll=ROLL, hero=HERO)
        self.assertEqual(rep.problems, [])
        self.assertEqual(rep.code, 0)

    def test_default_budgets_flag_short_clips(self):
        rep = vf.check_run(composed_run())
        self.assertEqual(rep.code, 2)
        self.assertTrue(any("вне 25–35" in t for _, t in rep.problems))

    def test_privacy_is_code_3_and_beats_others(self):
        run = copy_meta_only(composed_run())
        (run / "A" / "privacy.txt").write_text("метка drop: /home/user\n", encoding="utf-8")
        (run / "B" / "console.txt").write_text("error: x\n", encoding="utf-8")
        self.assertEqual(vf.check_run(run, roll=ROLL, hero=HERO).code, 3)
        (run / "A" / "privacy.txt").write_text("", encoding="utf-8")
        self.assertEqual(vf.check_run(run, roll=ROLL, hero=HERO).code, 6)

    def test_console_noise_is_not_code_6(self):
        # метки сцен и известный шум wails dev (NOT_OURS) — не ошибка; настоящая ошибка — код 6
        run = copy_meta_only(composed_run())
        con = run / "B" / "console.txt"
        con.write_text("== scene B\npageerror: Cannot read properties of null (reading 'nodes')\n", encoding="utf-8")
        rep = vf.check_run(run, roll=ROLL, hero=HERO)
        self.assertEqual(rep.code, 0)
        self.assertIn("консоль — 0 строк", vf.render_summary(run, rep))
        con.write_text("== scene B\nerror: something real\n", encoding="utf-8")
        self.assertEqual(vf.check_run(run, roll=ROLL, hero=HERO).code, 6)

    def test_budget_overrun_is_code_5(self):
        with mock.patch.object(cp, "WEBP_MAX", 1000):
            rep = vf.check_run(composed_run(), roll=ROLL, hero=HERO)
        self.assertEqual(rep.code, 5)

    def test_missing_files_are_code_2(self):
        run = copy_meta_only(composed_run())
        for name in ("A.mp4", "demo.webp", "screenshot.jpg"):
            (run / name).unlink()
        rep = vf.check_run(run, roll=ROLL, hero=HERO)
        self.assertEqual(rep.code, 2)
        self.assertEqual(len([1 for _, t in rep.problems if t.startswith("нет ")]), 3)

    def test_clock_skew_between_marks_and_frames(self):
        run = copy_meta_only(composed_run())
        marks = [json.loads(s) for s in (run / "A" / "marks.jsonl").read_text(encoding="utf-8").splitlines()]
        for m in marks:
            m["t"] += 30.0
        (run / "A" / "marks.jsonl").write_text("".join(json.dumps(m) + "\n" for m in marks), encoding="utf-8")
        rep = vf.check_run(run, roll=ROLL, hero=HERO)
        self.assertTrue(any("часы" in t for _, t in rep.problems))

    def test_low_capture_fps_warns_but_passes(self):
        root = tmp_root()
        self.addCleanup(shutil.rmtree, root, ignore_errors=True)
        run = copy_meta_only(composed_run())
        # прореживаем кадры сцены A вдвое по журналу: медиана интервалов 1/15 с
        fr = (run / "A" / "frames.jsonl").read_text(encoding="utf-8").splitlines()
        (run / "A" / "frames.jsonl").write_text("\n".join(fr[::2]) + "\n", encoding="utf-8")
        rep = vf.check_run(run, roll=ROLL, hero=HERO)
        self.assertTrue(any("захват" in w for w in rep.warnings), rep.warnings)
        self.assertEqual(rep.code, 0)

    def test_summary_content(self):
        text = (composed_run() / "summary.md").read_text(encoding="utf-8")
        for need in ("A.mp4", "B.mp4", "demo.webp", "screenshot.jpg", "к/с", "консоль", "приватность"):
            self.assertIn(need, text)
        self.assertRegex(text, r"CRF \d+")

    def test_slow_mode_is_reported(self):
        run = copy_meta_only(composed_run())
        meta = json.loads((run / "meta.json").read_text(encoding="utf-8"))
        meta["slow"] = 2
        (run / "meta.json").write_text(json.dumps(meta), encoding="utf-8")
        rep = vf.check_run(run, roll=ROLL, hero=HERO)
        self.assertTrue(any("медленном режиме" in w for w in rep.warnings))
        self.assertIn("медленном режиме", vf.render_summary(run, rep))


class TestDemoArgs(unittest.TestCase):
    def test_refuses_owner_port(self):
        with self.assertRaises(cp.Refuse):
            demo.check_port(34115)
        for p in (0, 34116 - 1, 34200, 80):
            with self.assertRaises(cp.Refuse, msg=str(p)):
                demo.check_port(p)
        self.assertEqual(demo.check_port(34119), 34119)
        self.assertEqual(demo.main(["record", "--port", "34115"]), 2)
        self.assertEqual(demo.main(["all", "--port", "34115"]), 2)

    def test_out_inside_gotmp(self):
        default = cp.GOTMP / "demo"
        self.assertEqual(demo.resolve_dir(None, default), default)
        inside = demo.resolve_dir(str(cp.GOTMP / "demo" / "x"), default)
        self.assertTrue(inside.is_relative_to(cp.GOTMP))
        for bad in ("", "   ", "/", str(cp.ROOT), str(cp.GOTMP), str(cp.GOTMP / ".." / "docs"), "docs/media", str(Path.home()), "../x"):
            with self.assertRaises(cp.Refuse, msg=repr(bad)):
                demo.resolve_dir(bad, default)

    def test_out_symlink_escape_refused(self):
        root = tmp_root()
        self.addCleanup(shutil.rmtree, root, ignore_errors=True)
        link = root / "link"
        os.symlink(cp.ROOT / "docs", link)  # ссылка внутри .gotmp на каталог вне его
        with self.assertRaises(cp.Refuse):
            demo.resolve_dir(str(link / "media"), cp.GOTMP / "demo")

    def test_empty_values_refused_in_main(self):
        self.assertEqual(demo.main(["check", "--run", ""]), 2)
        self.assertEqual(demo.main(["compose", "--run", ""]), 2)
        self.assertEqual(demo.main(["record", "--out", ""]), 2)

    def test_latest_run_picks_newest_and_refuses_empty(self):
        base = tmp_root()
        self.addCleanup(shutil.rmtree, base, ignore_errors=True)
        with self.assertRaises(cp.Refuse):
            demo.latest_run(base)
        for n in ("20261021-101010", "20261022-090000", "мусор"):
            (base / n).mkdir()
        self.assertEqual(demo.latest_run(base).name, "20261022-090000")

    def test_record_without_module_refuses(self):
        args = demo.parser().parse_args(["record", "--port", "34119"])
        base = tmp_root()
        self.addCleanup(shutil.rmtree, base, ignore_errors=True)
        with mock.patch.dict(sys.modules, {"record": None}):
            with self.assertRaises(cp.Refuse):
                demo.do_record(args, base)
        self.assertEqual(list(base.iterdir()), [], "каталог прогона создан до проверки модуля")

    def test_record_hands_over_and_propagates_exit_code(self):
        base = tmp_root()
        self.addCleanup(shutil.rmtree, base, ignore_errors=True)
        calls = []

        def fake(code):
            def record(**kw):
                calls.append(kw)
                return code

            return types.SimpleNamespace(record=record)

        args = demo.parser().parse_args(["record", "--port", "34119", "--slow", "2", "--photos", "/x/y"])
        with mock.patch.dict(sys.modules, {"record": fake(0)}):
            run = demo.do_record(args, base)
        self.assertEqual((calls[0]["port"], calls[0]["slow"], calls[0]["photos"]), (34119, 2, Path("/x/y")))
        self.assertEqual(calls[0]["out"], run)
        self.assertTrue(re.fullmatch(r"\d{8}-\d{6}", run.name) and run.parent == base)
        other = tmp_root()  # имя прогона — секунда: второй вызов в тот же момент не должен столкнуться
        self.addCleanup(shutil.rmtree, other, ignore_errors=True)
        with mock.patch.dict(sys.modules, {"record": fake(4)}):
            with self.assertRaises(demo._Exit) as e:  # код сцены (4) идёт наружу как есть
                demo.do_record(args, other)
            self.assertEqual(e.exception.code, 4)

    def test_check_and_compose_commands(self):
        run = composed_run()
        self.assertEqual(demo.main(["check", "--run", str(run)]), 2)  # боевые 25–35 с: короткая синтетика не проходит
        self.assertTrue((run / "summary.md").is_file())
        self.assertEqual(demo.main(["compose", "--run", str(run.parent / "нет")]), 2)

    def test_publish_only_after_clean_check(self):
        run = composed_run()
        with mock.patch.object(cp, "compose_all"), mock.patch.object(cp, "publish", return_value=[]) as pub:
            self.assertEqual(demo.do_compose(run, publish=True), 2)  # боевые 25–35 с: синтетика не проходит
            pub.assert_not_called()
            with mock.patch.object(vf, "check_run", return_value=vf.Report()):
                self.assertEqual(demo.do_compose(run, publish=True), 0)
                pub.assert_called_once_with(run)
                self.assertEqual(demo.do_compose(run, publish=False), 0)
                pub.assert_called_once()  # без --publish — не вызывался второй раз


class TestCredits(unittest.TestCase):
    def rows(self, p: Path) -> dict[int, str]:
        out = {}
        # только таблица до первого «##»: ниже в stock-photos.md свой набор «Детские портреты» с номерами 01… (130),
        # и его строки затирали бы автора фото stock-01
        text = p.read_text(encoding="utf-8").split("\n## ")[0]
        for ln in text.splitlines():
            c = [x.strip() for x in ln.strip().strip("|").split("|")]
            if c and re.fullmatch(r"\d{2,3}", c[0]):
                out[int(c[0])] = c
        return out

    def test_credits_match_stock_list(self):
        qa = cp.ROOT / "docs" / "qa" / "stock-photos.md"
        if not qa.is_file():
            self.skipTest("docs/qa/stock-photos.md нет в этой копии (не публикуется)")
        credits = self.rows(cp.ROOT / "docs" / "media" / "CREDITS.md")
        stock = self.rows(qa)
        self.assertEqual(sorted(credits), sorted(stock), "в CREDITS не весь набор stock")
        for n, c in credits.items():
            self.assertEqual(c[1], stock[n][3], f"автор фото {n}")  # столбец «автор» в stock-photos.md — четвёртый
            self.assertTrue(c[2].startswith("https://www.pexels.com/photo/"))

    def test_photos_txt_numbers_in_credits(self):
        txt = cp.ROOT / "scripts" / "demo" / "photos.txt"
        if not txt.is_file():
            self.skipTest("photos.txt (Д1) ещё нет")
        credits = self.rows(cp.ROOT / "docs" / "media" / "CREDITS.md")
        for n in (int(x) for x in re.findall(r"\b\d{1,3}\b", txt.read_text(encoding="utf-8"))):
            self.assertIn(n, credits, f"номер {n} из photos.txt")


if __name__ == "__main__":
    unittest.main()
