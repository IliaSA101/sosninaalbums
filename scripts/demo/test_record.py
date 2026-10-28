"""Тесты записи роликов README (102-1, 102-2, 102-4, 102-6, 102-7, 102-8): чистые части, без браузера и стенда.

Запуск: python3 -I -m unittest discover -s scripts/demo -v
"""

from __future__ import annotations

import datetime as dt
import math
import re
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import journal  # noqa: E402
import motion  # noqa: E402
import privacy  # noqa: E402
import record  # noqa: E402  (добавляет scripts/e2e в sys.path)
from common import Refuse  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]


def line(t=1.0, mark="start", note=""):
    import json

    return json.dumps({"t": t, "mark": mark, "note": note})


class TestJournal(unittest.TestCase):
    def test_parse_rejects_bad_lines(self):
        bad = [
            "не json",
            "[1, 2]",
            '{"mark": "start"}',  # нет t
            '{"t": "1", "mark": "start"}',  # t — строка
            '{"t": true, "mark": "start"}',  # bool — не число
            '{"t": 0, "mark": "start"}',
            '{"t": -5, "mark": "start"}',
            '{"t": 1, "mark": ""}',
            '{"t": 1, "mark": 7}',
            '{"t": 1, "mark": "x", "note": 5}',
            '{"t": NaN, "mark": "x"}',
        ]
        for s in bad:
            with self.subTest(s=s), self.assertRaises(journal.JournalError):
                journal.parse_line(s)
        m = journal.parse_line(line(2.5, "drop", "5 фото"))
        self.assertEqual((m.t, m.mark, m.note), (2.5, "drop", "5 фото"))
        with self.assertRaises(journal.JournalError) as e:  # номер строки в сообщении
            journal.parse_marks(line(1, "start") + "\nмусор\n")
        self.assertIn("строка 2", str(e.exception))

    def test_marks_monotonic(self):
        ok = "\n".join([line(1, "start"), "", line(2, "drop"), line(3, "end")]) + "\n"
        self.assertEqual([m.mark for m in journal.parse_marks(ok)], ["start", "drop", "end"])
        for text in (
            "",  # пусто
            line(1, "drop") + "\n" + line(2, "end"),  # нет start
            line(1, "start") + "\n" + line(2, "drop"),  # нет end
            line(1, "start") + "\n" + line(1, "drop") + "\n" + line(2, "end"),  # время не растёт
            line(2, "start") + "\n" + line(1, "end"),
        ):
            with self.subTest(text=text), self.assertRaises(journal.JournalError):
                journal.parse_marks(text)

    def test_writer_strictly_increases_and_roundtrips(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "marks.jsonl"
            j = journal.Journal(p, clock=lambda: 100.0)  # часы стоят: рост всё равно строгий
            for n in ("start", "a", "end"):
                j.mark(n, "ё")
            j.close()
            got = journal.load(p)  # круг запись → чтение
            self.assertEqual([m.mark for m in got], ["start", "a", "end"])
            self.assertTrue(all(b.t > a.t for a, b in zip(got, got[1:])))
            self.assertEqual(got[0].note, "ё")
            with self.assertRaises(journal.JournalError):
                j.mark("late")


class TestMotion(unittest.TestCase):
    def test_ease_endpoints_exact(self):
        self.assertEqual(motion.ease(0.0), 0.0)
        self.assertEqual(motion.ease(1.0), 1.0)
        self.assertEqual(motion.ease(-3.0), 0.0)
        self.assertEqual(motion.ease(7.0), 1.0)
        self.assertAlmostEqual(motion.ease(0.5), 0.5)
        ys = [motion.ease(i / 100) for i in range(101)]
        self.assertTrue(all(b >= a for a, b in zip(ys, ys[1:])))  # без возвратов
        a, b = (10.0, 20.0), (910.0, 420.0)
        pts = motion.path(a, b, 0.8)
        self.assertEqual((pts[0].x, pts[0].y), a)
        self.assertEqual((pts[-1].x, pts[-1].y), b)
        self.assertEqual(motion.path(a, a), [motion.Step(0.0, 10.0, 20.0)])  # нулевой путь — одна точка

    def test_steps_by_duration(self):
        a, b = (100.0, 100.0), (300.0, 150.0)
        for dur in (0.5, 1.0, 2.0):
            pts = motion.path(a, b, dur, fps=60)
            self.assertEqual(len(pts), math.ceil(dur * 60) + 1)
            self.assertAlmostEqual(pts[-1].t, dur)
            self.assertTrue(all(y.t > x.t for x, y in zip(pts, pts[1:])))
        # длинный путь с коротким сроком растягивается: шаги не длиннее предела
        long = motion.path((0, 0), (1500.0, 900.0), 0.2)
        self.assertGreater(long[-1].t, 0.2)
        self.assertAlmostEqual(motion.default_duration(1000), 0.8)
        for bad in (dict(dur=-1.0), dict(dur=1.0, fps=0)):
            with self.assertRaises(ValueError):
                motion.path(a, b, **bad)

    def test_no_teleport(self):
        cases = [((0, 0), (1600, 1000)), ((1600, 0), (0, 1000)), ((800, 500), (810, 500)), ((5, 5), (5, 900)), ((300, 300), (1450, 400))]
        for a, b in cases:
            for dur in (None, 0.1, 0.45, 2.0):
                pts = motion.path(a, b, dur)
                steps = [math.hypot(q.x - p.x, q.y - p.y) for p, q in zip(pts, pts[1:])]
                with self.subTest(a=a, b=b, dur=dur):
                    self.assertLessEqual(max(steps), motion.MAX_STEP + 1e-9)
        # дуга: середина отклонена от прямой, концы — нет
        pts = motion.path((0.0, 0.0), (1000.0, 0.0), 1.0)
        self.assertGreater(max(abs(p.y) for p in pts), 30.0)
        self.assertEqual(pts[-1].y, 0.0)
        for x, y in motion.drag_start_steps((100.0, 100.0)):  # три шага по ~3 px проходят порог 5 px
            self.assertLess(math.hypot(x - 100, y - 100), 20)


class TestPrivacy(unittest.TestCase):
    HOME, USER = "/home/ilya", "ilya"

    def test_finds_home_user_paths(self):
        cases = {
            "/home/ilya/Фото/свадьба": 1,
            "Папка: /home/someone/x": 1,
            "/media/ilya/disk/IMG": 1,
            "/run/media/ilya/x": 1,
            "C:\\Users\\Name\\Pictures": 1,
            "/work/.gotmp/demo/photos": 1,
            "SosninaAlbums-stand-demo": 1,
            "стенд stand demo": 1,
            "Автор: Ilya": 1,  # имя пользователя без учёта регистра
        }
        for text, n in cases.items():
            with self.subTest(text=text):
                self.assertGreaterEqual(len(privacy.scan([text], self.HOME, self.USER)), n)
        self.assertIn("[drop]", privacy.findings_text("drop", privacy.scan(["/home/x"], self.HOME, self.USER)))
        # несколько текстов сразу: находка из второго не теряется
        self.assertTrue(privacy.scan(["чисто", "тут /home/q"], "", ""))

    def test_clean_text_passes(self):
        clean = [
            "Разворот 3 из 12 · Раскладка 2 / 31 · IMG_0007.jpg · Папки 1 · Автосборка · Что соберётся 12",
            "Понимаю, стандартный уровень",  # «stand» не внутри слова
            "30×30 см · 300 dpi · Черновик",
            "",
        ]
        self.assertEqual(privacy.scan(clean, self.HOME, self.USER), [])
        self.assertEqual(privacy.scan(["пользователь ilyas"], self.HOME, self.USER), [])  # имя — только словом целиком
        self.assertEqual(privacy.scan(["путь /"], "/", ""), [])  # «/» не домашний каталог
        self.assertEqual(privacy.findings_text("x", []), "")


class TestArgs(unittest.TestCase):
    def test_refuses_owner_port(self):
        with self.assertRaises(Refuse):
            record.check_port(34115)
        for bad in (0, 34114, 34200, 80):
            with self.assertRaises(Refuse):
                record.check_port(bad)
        self.assertEqual(record.check_port(34119), 34119)
        self.assertEqual(record.check_port(34116), 34116)

    def test_out_inside_repo(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            base = (root / ".gotmp").resolve()
            now = dt.datetime(2026, 10, 22, 12, 30, 5)
            self.assertEqual(record.run_dir(None, now, root), base / "demo" / "20261022-123005")
            self.assertEqual(record.run_dir(str(base / "demo-runs" / "x"), root=root), base / "demo-runs" / "x")
            for bad in ("", "   ", "relative/path", str(base), str(root), str(root / "docs"), str(base / ".." / "docs"), "/", "/tmp/x"):
                with self.subTest(bad=bad), self.assertRaises(Refuse):
                    record.run_dir(bad, now, root)


class TestPhotos(unittest.TestCase):
    def test_parse(self):
        self.assertEqual(record.parse_photos("# c\nA 1 2\nB 3 # x\nA 4\n"), {"A": [1, 2, 4], "B": [3]})
        self.assertEqual(record.parse_photos(""), {})
        for bad in ("C 1", "A x", "A 1 1", "A 1\nA 1"):
            with self.subTest(bad=bad), self.assertRaises(Refuse):
                record.parse_photos(bad)

    def test_counts_and_subset(self):
        nums = record.load_photos()
        self.assertEqual(len(nums["A"]), 40)
        self.assertEqual(len(nums["B"]), 14)
        self.assertTrue(set(nums["B"]) <= set(nums["A"]))
        self.assertTrue(all(1 <= n <= 100 for n in nums["A"]))

    def test_stage_names_and_missing(self):
        with tempfile.TemporaryDirectory() as d:
            src, dst = Path(d) / "src", Path(d) / "dst"
            src.mkdir()
            for n in (3, 5, 7):
                (src / f"stock-{n:02d}.jpg").write_bytes(b"x%d" % n)
            dirs = record.stage_photos({"A": [5, 3], "B": [7]}, src, dst)
            self.assertEqual(sorted(p.name for p in dirs["A"].iterdir()), ["IMG_0001.jpg", "IMG_0002.jpg"])
            self.assertEqual((dirs["A"] / "IMG_0001.jpg").read_bytes(), b"x5")  # порядок — как в списке
            self.assertEqual(dirs["B"].name, "Свадьба-B")
            with self.assertRaises(Refuse):
                record.stage_photos({"A": [99]}, src, dst)

    def test_list_in_credits(self):
        qa = (ROOT / "docs" / "qa" / "stock-photos.md").read_text(encoding="utf-8")
        in_qa = {int(m) for m in re.findall(r"^\|\s*(\d+)\s*\|", qa, re.M)}
        allnums = {n for v in record.load_photos().values() for n in v}
        self.assertFalse(allnums - in_qa, "номера photos.txt без записи в docs/qa/stock-photos.md")
        credits = ROOT / "docs" / "media" / "CREDITS.md"
        if not credits.is_file():
            self.skipTest("docs/media/CREDITS.md пишет Д2 (спека 102): сверка с ним — после слияния")
        text = credits.read_text(encoding="utf-8")
        for n in sorted(allnums):
            self.assertRegex(text, rf"(?<!\d)0?{n}(?!\d)|stock-{n:02d}", f"номер {n} не упомянут в CREDITS.md")


class TestRecordCode(unittest.TestCase):
    """record.record(port, out, photos, slow) -> код выхода поверх record_all (контракт demo.py, design §7)."""

    def run_with(self, fake, console="", slow=1):
        import contextlib
        import io

        import pilot

        orig = record.record_all
        record.record_all = fake
        try:
            with tempfile.TemporaryDirectory() as d:
                out = Path(d)
                (out / "console.txt").write_text(console, encoding="utf-8")
                with contextlib.redirect_stderr(io.StringIO()):
                    return record.record(port=34119, out=out, photos=Path(d), slow=slow)
        finally:
            record.record_all = orig

    def test_codes(self):
        import pilot

        ok = {"scenes": {"A": {"privacy": 0}, "B": {"privacy": 0}}}

        def fail(exc):
            def f(*a, **k):
                raise exc

            return f

        self.assertEqual(self.run_with(lambda *a, **k: ok), 0)
        self.assertEqual(self.run_with(fail(Refuse("нет стенда"))), 2)
        self.assertEqual(self.run_with(fail(pilot.SceneFail("drop", "разворот"))), 4)
        # находка приватности в одной из нескольких сцен — 3, даже если вторая чистая
        self.assertEqual(self.run_with(lambda *a, **k: {"scenes": {"A": {"privacy": 0}, "B": {"privacy": 2}}}), 3)
        self.assertEqual(self.run_with(lambda *a, **k: ok, console="== scene A\nerror: boom\n"), 6)
        # «не наше» и метки этапов консоль не портят
        self.assertEqual(self.run_with(lambda *a, **k: ok, console="== scene A\nerror: Cannot read properties of null (reading 'nodes')\n"), 0)
        self.assertEqual(self.run_with(lambda *a, **k: ok, slow=2), 0)  # медленный режим доходит до record_all

    def test_args_passed(self):
        seen = {}

        def fake(port, out, photos, slow, *a, **k):
            seen.update(port=port, photos=photos, slow=slow)
            return {"scenes": {}}

        self.run_with(fake, slow=2)
        self.assertEqual((seen["port"], seen["slow"]), (34119, 2.0))


class TestStandInit(unittest.TestCase):
    """102-8: точка расширения Stand(init_scripts=…); без неё — прежнее поведение."""

    def test_init_scripts_stored(self):
        from driver import Stand

        self.assertEqual(Stand(34119, Path("/x")).init_scripts, ())
        self.assertEqual(Stand(34119, Path("/x"), init_scripts=("a", "b")).init_scripts, ("a", "b"))

    def test_cursor_script_present(self):
        js = (Path(__file__).parent / "cursor.js").read_text(encoding="utf-8")
        for s in ("__demo-cursor", "pointer-events:none", "2147483647", "pointerdown"):
            self.assertIn(s, js)


if __name__ == "__main__":
    unittest.main()
