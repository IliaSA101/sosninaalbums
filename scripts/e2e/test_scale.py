"""Юнит-тесты правил S3/S4 масштаба (089): «не открылось» — не тихий 0, а отдельный gate; уровень «Полный» на время шага.

Без браузера и стенда: стенд подменён заглушкой. `python -m unittest discover -s scripts/e2e`.
"""

import unittest
from pathlib import Path
from unittest import mock

import scale
import scalejs
from report import GATE, Report


def metric(rep: Report, id: str):
    return next(m for m in rep.metrics if m.id == id)


def fake_stand(*, opens: bool = True, level: str | None = "simple", ev=None):
    """Стенд-заглушка: opens=False — любой щелчок по странице падает по таймауту (как в прогоне 20.10)."""
    st = mock.MagicMock()
    st.set_level.return_value = level
    st.wait_until.return_value = opens
    if not opens:
        st.page.locator.return_value.first.click.side_effect = RuntimeError("Locator.click: Timeout 5000ms exceeded.")
        st.menu_item.side_effect = RuntimeError("Locator.click: Timeout 5000ms exceeded.")
    st.page.locator.return_value.count.return_value = 0  # clear(): окон и меню нет
    st.ev.side_effect = ev or (lambda js, arg=None: None)
    return st


def runner(st) -> tuple[scale.Scale, Report]:
    rep = Report("scale", "2026-10-20T10:00:00")
    r = scale.Scale(mock.MagicMock(), [Path(".")], Path("."), rep, st)
    r.a_profiles = ["A"]
    return r, rep


FITS = {"fits": True, "reach": True, "rect": {}}


class MenuVerdictTest(unittest.TestCase):
    def test_verdicts(self):
        box = {"top": 10, "bottom": 300, "left": 10, "right": 200}
        self.assertEqual(scalejs.menu_verdict(None), "unopened")
        self.assertEqual(scalejs.menu_verdict({"menu": None, "vw": 800, "vh": 600}), "unopened")
        self.assertEqual(scalejs.menu_verdict({"menu": box, "vw": 800, "vh": 600}), "ok")
        self.assertEqual(scalejs.menu_verdict({"menu": box, "vw": 800, "vh": 250}), "overflow")
        self.assertEqual(scalejs.menu_verdict({"menu": {**box, "right": 900}, "vw": 800, "vh": 600}), "overflow")
        self.assertEqual(scalejs.menu_verdict({"menu": {**box, "left": -5}, "vw": 800, "vh": 600}), "overflow")


class S3Test(unittest.TestCase):
    def test_nothing_opens_is_gate_fail_not_zero(self):
        r, rep = runner(fake_stand(opens=False))
        r.s3("A")
        self.assertEqual(metric(rep, "s3_dialog_overflow.A").value, 0)
        m = metric(rep, "s3_dialog_unopened.A")
        self.assertEqual((m.value, m.kind, m.ok), (len(scale.WINDOWS), GATE, False))
        self.assertIn("s3_dialog_unopened.A", [x.id for x in rep.failed()])

    def test_window_not_shown_counts_as_unopened(self):
        r, rep = runner(fake_stand(opens=True))
        r.st.wait_until.return_value = False  # щелчок прошёл, окна нет
        r.s3("A")
        self.assertEqual(metric(rep, "s3_dialog_unopened.A").value, len(scale.WINDOWS))

    def test_all_open_and_fit(self):
        r, rep = runner(fake_stand(ev=lambda js, arg=None: FITS))
        r.s3("A")
        self.assertEqual(metric(rep, "s3_dialog_unopened.A").value, 0)
        self.assertEqual(metric(rep, "s3_dialog_overflow.A").value, 0)
        self.assertEqual(rep.failed(), [])

    def test_overflow_counted_separately(self):
        r, rep = runner(fake_stand(ev=lambda js, arg=None: {"fits": False, "reach": True, "rect": {"w": 9}}))
        r.s3("A")
        self.assertEqual(metric(rep, "s3_dialog_overflow.A").value, len(scale.WINDOWS))
        self.assertEqual(metric(rep, "s3_dialog_unopened.A").value, 0)

    def test_level_full_then_restored(self):
        r, _ = runner(fake_stand(level="simple", ev=lambda js, arg=None: FITS))
        r.s3("A")
        self.assertEqual([c.args[0] for c in r.st.set_level.call_args_list], ["full", "simple"])

    def test_level_already_full_not_touched_again(self):
        r, _ = runner(fake_stand(level="full", ev=lambda js, arg=None: FITS))
        r.s3("A")
        self.assertEqual([c.args[0] for c in r.st.set_level.call_args_list], ["full"])

    def test_level_restored_after_failure(self):
        r, _ = runner(fake_stand(opens=False, level="standard"))
        r.s3("A")
        self.assertEqual([c.args[0] for c in r.st.set_level.call_args_list], ["full", "standard"])


class S4Test(unittest.TestCase):
    BOX = {"top": 10, "bottom": 300, "left": 10, "right": 200, "vw": 800, "vh": 600}

    def ev(self, menu=BOX, corner="ok"):
        def f(js, arg=None):
            if js == scalejs.MENU_RECT_JS:
                return menu
            if corner == "no-target":
                return None
            if corner == "no-menu":
                return {"menu": None, "vw": 800, "vh": 600}
            return {"menu": self.BOX, "vw": 800, "vh": 600}

        return f

    def test_all_open(self):
        r, rep = runner(fake_stand(ev=self.ev()))
        r.s4("A")
        self.assertEqual(metric(rep, "s4_menu_unopened.A").value, 0)
        self.assertEqual(metric(rep, "s4_menu_overflow.A").value, 0)

    def test_photo_menu_not_opened_is_gate_fail(self):
        r, rep = runner(fake_stand(ev=self.ev(corner="no-menu")))
        r.s4("A")
        m = metric(rep, "s4_menu_unopened.A")
        self.assertEqual((m.value, m.ok), (2, False))  # разворот и фото
        self.assertIn("фото", m.note)
        self.assertEqual(metric(rep, "s4_menu_overflow.A").value, 0)

    def test_no_target_is_unopened(self):
        r, rep = runner(fake_stand(ev=self.ev(corner="no-target")))
        r.s4("A")
        self.assertEqual(metric(rep, "s4_menu_unopened.A").value, 2)

    def test_menubar_menu_missing(self):
        r, rep = runner(fake_stand(ev=self.ev(menu=None)))
        r.s4("A")
        self.assertEqual(metric(rep, "s4_menu_unopened.A").value, len(scale.MENUS))

    def test_overflow_counted_separately(self):
        r, rep = runner(fake_stand(ev=self.ev(menu={**self.BOX, "bottom": 700})))
        r.s4("A")
        self.assertEqual(metric(rep, "s4_menu_overflow.A").value, len(scale.MENUS))
        self.assertEqual(metric(rep, "s4_menu_unopened.A").value, 0)

    def test_level_full_then_restored(self):
        r, _ = runner(fake_stand(level="simple", ev=self.ev()))
        r.s4("A")
        self.assertEqual([c.args[0] for c in r.st.set_level.call_args_list], ["full", "simple"])


if __name__ == "__main__":
    unittest.main()
