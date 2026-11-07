"""Регресс инспектора (166): R5 «Ручная» на пустом развороте, R8 ряд выравнивания надписи в карточке 174 px."""

from base import RegressCase, bug

MODE_GROUP = "Раскладка разворота"
EMPTY_TIP = "Сначала положите фото на разворот"
ALIGN, STYLE = "Выравнивание", "Начертание"

# Геометрия ряда «Выравнивание · Начертание»: его прямоугольник (родитель групп) и пункты внутри групп.
ROW_JS = """
(names) => {
  const rect = (e) => { const r = e.getBoundingClientRect(); return { l: r.left, t: r.top, r: r.right, w: r.width } };
  const groups = names.map((n) => document.querySelector(`[role=group][aria-label="${n}"]`));
  if (groups.some((g) => !g)) return null;
  const row = groups[0].parentElement;
  return {
    row: { ...rect(row), sw: row.scrollWidth, cw: row.clientWidth, same: groups.every((g) => g.parentElement === row) },
    groups: groups.map((g) => ({ ...rect(g), btn: [...g.querySelectorAll('button')].map(rect) })),
  };
}
"""


@bug(id="R5", fix="2e21cba6", src="QA Seg 04.11", red="revert 2e21cba6")
class SpreadMode(RegressCase):
    def _mode(self):
        return self.page.get_by_role("group", name=MODE_GROUP).get_by_role("button", name="Ручная")

    def _open(self, sid: str) -> None:
        self.q(f"[data-guide=timeline] [data-spread='{sid}']").first.click()

    def test_manual_off_on_empty_spread(self):
        """Пустой разворот: пункт «Ручная» серый с причиной, щелчок не зовёт Go и не меняет вид; у разворота с фото — живой."""
        self.album([2])
        self.call("NewSpread", -1, [])
        v = self.call("GetView")
        full, empty = v["spreads"][0]["id"], v["spreads"][1]["id"]
        self.wait_js("(r) => window.__reg.rev >= r", v["rev"])
        self._open(empty)
        manual = self._mode()
        self.expect(manual).to_have_attribute("aria-disabled", "true")
        self.expect(manual).to_have_attribute("title", EMPTY_TIP)
        spy = self.spy("SetGrid")
        manual.click(force=True)  # aria-disabled не мешает настоящему щелчку: проверяем, что его глушит сам пункт
        self.settle()
        self.assertEqual(spy.count(), 0, "щелчок по серой «Ручной» дошёл до Go")
        after = self.call("GetView")
        self.assertEqual(after["rev"], v["rev"])
        self.assertFalse(after["spreads"][1]["grid"])
        # Парная проверка: серый только пустой разворот, у разворота с фото пункт не заблокирован.
        self._open(full)
        self.expect(self._mode()).not_to_have_attribute("aria-disabled", "true")


@bug(id="R8", fix="10a798b2", src="сверка Seg ч. 2 04.11", red="revert 10a798b2")
class TextSeg(RegressCase):
    def _select_text(self) -> None:
        sid = self.album([1])["spreads"][0]["id"]
        self.call("AddText", sid, 0, "e2e")
        self.q(".hit.block").first.click()
        self.expect(self.page.get_by_role("group", name=ALIGN)).to_be_visible()

    def _check(self, tag: str) -> None:
        g = self.st.ev(ROW_JS, [ALIGN, STYLE])
        self.assertIsNotNone(g, tag)
        row, groups = g["row"], g["groups"]
        self.assertTrue(row["same"], f"{tag}: группы не в одном ряду-карточке")
        self.assertLessEqual(row["sw"], row["cw"], f"{tag}: ряд шире карточки ({row['sw']} > {row['cw']})")
        for n, grp in zip((ALIGN, STYLE), groups):
            self.assertGreaterEqual(grp["l"], row["l"] - 0.5, f"{tag}: «{n}» выступает слева")
            self.assertLessEqual(grp["r"], row["r"] + 0.5, f"{tag}: «{n}» выступает справа")
            ws = [b["w"] for b in grp["btn"]]
            self.assertLessEqual(max(ws) - min(ws), 1, f"{tag}: «{n}», ширины пунктов {ws}")
            for b in grp["btn"]:
                self.assertGreaterEqual(b["l"], grp["l"] - 0.5, f"{tag}: пункт «{n}» вне группы")
                self.assertLessEqual(b["r"], grp["r"] + 0.5, f"{tag}: пункт «{n}» вне группы")
        self.assertLessEqual(abs(groups[0]["t"] - groups[1]["t"]), 0.5, f"{tag}: группы в разных строках")

    def test_align_row_fits_card(self):
        """Выравнивание и Ж/К лежат в одну строку внутри карточки; пункты поровну. Профили A (×1) и C (×1,5)."""
        for prof in ("A", "C"):
            with self.subTest(profile=prof):
                self.reopen(prof)
                self._select_text()
                self.stable_rect(f"[role=group][aria-label={ALIGN}]")
                self._check(prof)
