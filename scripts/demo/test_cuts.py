"""Тесты вырезок ожидания и защиты от перезагрузки страницы (102, доправка Д3): чистые части, без браузера."""

from __future__ import annotations

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import compose as cp  # noqa: E402
import pilot  # noqa: E402
import scenes  # noqa: E402
import timeline as tl  # noqa: E402


def frames(n=100, step=0.1, t0=1000.0):
    return [tl.Frame(i + 1, t0 + i * step) for i in range(n)]


def marks(pairs):
    return [tl.Mark(t, name) for name, t in pairs]


class TestKeptSpans(unittest.TestCase):
    def test_basic_and_edges(self):
        self.assertEqual(tl.kept_spans(0, 10, []), [(0, 10)])
        self.assertEqual(tl.kept_spans(0, 10, [(3, 5)]), [(0, 3), (5, 10)])
        self.assertEqual(tl.kept_spans(0, 10, [(0, 4)]), [(4, 10)])  # вырезка с начала
        self.assertEqual(tl.kept_spans(0, 10, [(6, 10)]), [(0, 6)])  # до конца
        self.assertEqual(tl.kept_spans(0, 10, [(-5, 20)]), [])  # съела всё
        self.assertEqual(tl.kept_spans(0, 10, [(3, 3), (5, 4)]), [(0, 10)])  # пустые и перевёрнутые не считаются

    def test_many_cuts_overlap_and_order(self):
        got = tl.kept_spans(0, 20, [(12, 15), (2, 5), (4, 8), (30, 40)])
        self.assertEqual(got, [(0, 2), (8, 12), (15, 20)])  # пересечение сведено, порядок не важен, вне окна — мимо

    def test_resample_cuts_joins_and_fails_when_empty(self):
        fr = frames()
        a = tl.resample(fr, 1000.0, 1002.0, 30)
        b = tl.resample(fr, 1005.0, 1008.0, 30)
        self.assertEqual(tl.resample_cuts(fr, 1000.0, 1008.0, [(1002.0, 1005.0)], 30), a + b)
        self.assertEqual(len(tl.resample_cuts(fr, 1000.0, 1008.0, [(1002.0, 1005.0)], 30, 2.0)), len(a + b) // 2)
        with self.assertRaises(tl.JournalError):
            tl.resample_cuts(fr, 1000.0, 1008.0, [(900.0, 2000.0)], 30)


class TestSceneCuts(unittest.TestCase):
    def scene(self, name, pairs):
        return cp.Scene(name, Path("/x"), frames(), marks(pairs))

    def test_cut_rules_from_scenes(self):
        self.assertEqual(cp.cut_rules()["A"], tuple(tuple(r) for r in scenes.CUTS["A"]))
        self.assertIn("build-click", [r[0] for r in scenes.CUTS["A"]])

    def test_cuts_by_marks(self):
        a = self.scene("A", [("start", 1000.0), ("build-click", 1002.0), ("built", 1006.0), ("rebuild-click", 1007.0), ("rebuilt", 1008.0), ("end", 1009.0)])
        self.assertEqual(a.cuts(), [(1002.5, 1006.0), (1007.5, 1008.0)])
        self.assertAlmostEqual(a.kept(), 9.0 - 3.5 - 0.5)
        self.assertAlmostEqual(a.kept(2.0), (9.0 - 3.5 - 0.5) / 2)
        # без вырезок: сцена B, сцена без меток результата
        self.assertEqual(self.scene("B", [("start", 1000.0), ("end", 1009.0)]).cuts(), [])
        self.assertEqual(self.scene("A", [("start", 1000.0), ("end", 1009.0)]).cuts(), [])
        self.assertAlmostEqual(self.scene("A", [("start", 1000.0), ("end", 1009.0)]).kept(), 9.0)

    def test_wait_shorter_than_keep_is_not_cut(self):
        a = self.scene("A", [("start", 1000.0), ("build-click", 1002.0), ("built", 1002.3), ("end", 1005.0)])
        self.assertEqual(a.cuts(), [])


class TestStaleFrames(unittest.TestCase):
    """Метка результата стоит по DOM, а кадры ещё секунды показывают прежнее окно: вырезка идёт до первого кадра с новой картинкой."""

    def make(self, d: Path, sizes):
        (d / "frames").mkdir()
        fr = []
        for i, sz in enumerate(sizes, 1):
            (d / "frames" / f"{i:06d}.jpg").write_bytes(b"x" * sz)
            fr.append(tl.Frame(i, 1000.0 + i * 0.1))
        return fr

    def test_cut_extends_to_new_picture(self):
        import tempfile

        with tempfile.TemporaryDirectory() as t:
            d = Path(t)
            # 0–3,0 с окно (40 КБ), с 3,1 с — развороты (150 КБ); метка «built» на 1,5 с
            fr = self.make(d, [40_000] * 30 + [150_000] * 20)
            sc = cp.Scene("A", d, fr, marks([("start", 1000.0), ("build-click", 1000.5), ("built", 1001.5), ("end", 1005.0)]))
            (a, b), = sc.cuts()
            self.assertAlmostEqual(a, 1001.0)
            self.assertAlmostEqual(b, 1003.1)  # первый кадр нового вида, а не метка
            self.assertAlmostEqual(sc.kept(), 5.0 - 2.1)

    def test_no_jump_or_no_files_keeps_mark(self):
        import tempfile

        with tempfile.TemporaryDirectory() as t:
            d = Path(t)
            fr = self.make(d, [40_000] * 50)
            sc = cp.Scene("A", d, fr, marks([("start", 1000.0), ("build-click", 1000.5), ("built", 1002.0), ("end", 1005.0)]))
            self.assertEqual(sc.cuts(), [(1001.0, 1002.0)])  # скачка нет — по метке
        sc = cp.Scene("A", Path("/нет/такой"), frames(), marks([("start", 1000.0), ("build-click", 1000.5), ("built", 1002.0), ("end", 1005.0)]))
        self.assertEqual(sc.cuts(), [(1001.0, 1002.0)])  # кадров на диске нет — по метке


class FakePage:
    def __init__(self, loads):
        self.loads = loads

    def evaluate(self, js, *a):
        if isinstance(self.loads, Exception):
            raise self.loads
        return self.loads


class FakeStand:
    def __init__(self, page):
        self.page = page


class TestReloadGuard(unittest.TestCase):
    def pilot(self, loads):
        return pilot.Pilot(FakeStand(FakePage(loads)))

    def test_check_alive(self):
        p = self.pilot(1)
        p.check_alive()  # загрузок столько же — живы
        p.page.loads = 2  # страница перезагрузилась
        with self.assertRaises(pilot.SceneFail) as e:
            p.check_alive()
        self.assertIn("перезагрузилась", str(e.exception))
        q = self.pilot(RuntimeError("нет контекста"))
        self.assertEqual(q.loads, -1)  # нет счётчика — защита не включается, но и не падает
        q.check_alive()

    def test_guarded_converts_navigation_errors(self):
        p = self.pilot(1)

        def boom(_p, msg):
            raise RuntimeError(msg)

        def fail(_p):
            raise pilot.SceneFail("m", "w")

        with self.assertRaises(pilot.SceneFail):
            pilot.guarded(p, boom, "Execution context was destroyed, most likely because of a navigation")
        with self.assertRaises(RuntimeError):  # прочие ошибки идут как есть
            pilot.guarded(p, boom, "что-то другое")
        pilot.guarded(p, lambda _p: None)
        p.page.loads = 2  # счётчик загрузок вырос: таймаут Playwright на белой странице — перезагрузка, сцену можно повторить
        with self.assertRaises(pilot.SceneFail) as e:
            pilot.guarded(p, boom, "Timeout 30000ms exceeded")
        self.assertTrue(e.exception.reload)
        p.page.loads = 1
        with self.assertRaises(pilot.SceneFail):
            pilot.guarded(p, fail)


if __name__ == "__main__":
    unittest.main()
