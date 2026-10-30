"""Юнит-тесты охраны big.py (089N): --cold только в наборе e2e, сверка стенда с портом, атомарная запись, метки консоли.

Без браузера: `python -m unittest discover -s scripts/e2e`. Временные файлы — в <корень>/.gotmp/e2e-test.
"""

import os
import shutil
import tempfile
import unittest
from pathlib import Path

import big
import common
from common import Refuse
from driver import MARK, Stand


class TmpCase(unittest.TestCase):
    def tmp(self) -> Path:
        base = common.ROOT / ".gotmp" / "e2e-test"
        base.mkdir(parents=True, exist_ok=True)
        d = Path(tempfile.mkdtemp(dir=base))
        self.addCleanup(shutil.rmtree, d, ignore_errors=True)
        return d


OLD = 1_000_000_000


def touch(p: Path) -> Path:
    p.write_bytes(b"x")
    os.utime(p, (OLD, OLD))
    return p


class ColdTest(TmpCase):
    def test_no_manifest_refuses(self):
        a = self.tmp() / "a"
        a.mkdir()
        f = touch(a / "e2e-001.jpg")
        with self.assertRaises(Refuse):
            big.cold([a])
        self.assertEqual(int(f.stat().st_mtime), OLD)

    def test_only_e2e_files_touched(self):
        root = self.tmp()
        (root / "manifest.txt").write_text("files 2\n", encoding="utf-8")
        a = root / "a"
        a.mkdir()
        mine, link_name = touch(a / "e2e-001.jpg"), touch(a / "s001-01-e2e-336.jpg")
        foreign = [touch(a / "mine.jpg"), touch(a / "notes.txt"), touch(a / "e2e-001.png")]
        self.assertEqual(big.cold([a]), 2)
        for f in (mine, link_name):
            self.assertGreater(f.stat().st_mtime, OLD + 1000)
        for f in foreign:
            self.assertEqual(int(f.stat().st_mtime), OLD)

    def test_symlink_skipped(self):
        root = self.tmp()
        (root / "manifest.txt").write_text("files 1\n", encoding="utf-8")
        a, out = root / "a", root / "outside"
        a.mkdir()
        out.mkdir()
        target = touch(out / "e2e-009.jpg")
        try:
            os.symlink(target, a / "e2e-009.jpg")
        except (OSError, NotImplementedError):
            self.skipTest("симлинки недоступны (нужны права)")
        self.assertEqual(big.cold([a]), 0)
        self.assertEqual(int(target.stat().st_mtime), OLD)


class StandLogTest(TmpCase):
    def log(self, name: str, head: str) -> Path:
        d = self.tmp()
        (d / f"wails-dev-{name}.log").write_text(head + "\nrest\n", encoding="utf-8")
        return d

    def test_match(self):
        d = self.log("e2e", "стенд e2e, http://localhost:34118, root")
        self.assertTrue(big.check_stand_log("e2e", 34118, d))

    def test_port_mismatch_refuses(self):
        d = self.log("e2e", "стенд e2e, http://localhost:34117, root")
        with self.assertRaises(Refuse):
            big.check_stand_log("e2e", 34118, d)

    def test_garbage_head_refuses(self):
        d = self.log("e2e", "что-то другое")
        with self.assertRaises(Refuse):
            big.check_stand_log("e2e", 34118, d)

    def test_name_mismatch_refuses(self):
        d = self.log("e2e", "стенд other, http://localhost:34118, root")
        with self.assertRaises(Refuse):
            big.check_stand_log("e2e", 34118, d)

    def test_no_log_not_verified(self):
        self.assertFalse(big.check_stand_log("e2e", 34118, self.tmp()))


class AtomicTest(TmpCase):
    def test_replace_without_leftover(self):
        p = self.tmp() / "config.json"
        p.write_text("old", encoding="utf-8")
        big.write_atomic(p, "new")
        self.assertEqual(p.read_text(encoding="utf-8"), "new")
        self.assertEqual([x.name for x in p.parent.iterdir()], ["config.json"])


class MarkOrderTest(TmpCase):
    def test_marks_split_both_parts(self):
        st = Stand(34118, self.tmp())
        st.mark("b1")
        st.console.append(("error", "first"))
        st.failed.append((404, "/media/t/1"))
        st.mark("b2")
        st.console.append(("warning", "second"))
        st.failed.append((404, "/media/p/2"))
        st.write_console()
        lines = (st.out / "console.txt").read_text(encoding="utf-8").splitlines()
        self.assertEqual(
            lines,
            ["== b1", "error: first", "== b2", "warning: second", "== b1", "http 404: /media/t/1", "== b2", "http 404: /media/p/2"],
        )
        # метки — не ошибки консоли
        self.assertEqual(st.console_problems(), ["error: first", "warning: second"])
        self.assertEqual(MARK, "==")


class _Loc:
    """Локатор-заглушка: число совпадений берёт у страницы, focus() запоминает."""

    def __init__(self, page, sel):
        self.page, self.sel = page, sel
        self.first = self

    def count(self):
        return self.page.counts(self.sel)

    def focus(self):
        self.page.focused.append(self.sel)


class _FakePage:
    """Страница без браузера: диалог стартового окна и карточка тура закрываются по Esc (карточка — только при фокусе в ней)."""

    def __init__(self, start=True, tour=True):
        self.start, self.tour, self.focused, self.presses = start, tour, [], []
        self.keyboard = self

    def counts(self, sel):
        if sel == "[role=dialog]":
            return int(self.start) + int(self.tour)
        if sel == "[data-tour-step]":
            return int(self.tour)
        return 0

    def locator(self, sel):
        return _Loc(self, sel)

    def press(self, key):
        self.presses.append(key)
        if self.start:
            self.start = False
        elif self.tour and self.focused:
            self.tour = False

    def wait_for_timeout(self, ms):
        pass


class DismissTest(TmpCase):
    def stand(self, page):
        st = Stand(34119, self.tmp())
        st.page = page
        return st

    def test_dismiss_closes_tour(self):
        page = _FakePage()
        self.stand(page).dismiss_start()
        self.assertFalse(page.start)
        self.assertFalse(page.tour)
        self.assertIn("[data-tour-step] [data-primary]", page.focused)

    def test_dismiss_without_tour(self):
        page = _FakePage(tour=False)
        self.stand(page).dismiss_start()
        self.assertFalse(page.start)
        self.assertEqual(page.focused, [])


if __name__ == "__main__":
    unittest.main()
