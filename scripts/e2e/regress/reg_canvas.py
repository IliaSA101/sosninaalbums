"""Холст разворота: мини-панель действий (R4; owner-ux §1 п. 5). Признак — что лежит под точкой (elementFromPoint) и что ушло в Go."""

import re

from base import RegressCase, bug

CELLS = ".stage .cell[data-photo]"
PANEL = "[data-guide=mini-panel]"
FIRST = PANEL + " button:first-of-type"  # «Зеркало ↔»: корзина стоит ниже фото (столбец выше самой ячейки)
DEL = PANEL + " button.del"

# Что под точкой: панель или лист разворота (ячейки сами мышь не ловят: фото выбирает обработчик листа по геометрии).
UNDER_JS = """
([x, y]) => {
  const e = document.elementFromPoint(x, y); if (!e) return null;
  return { panel: !!e.closest('.minipanel'), spread: !!e.closest('.spread') };
}
"""

BOXES_JS = """
(sel) => [...document.querySelectorAll(sel)].map((e) => {
  const r = e.getBoundingClientRect();
  return { x: r.left + r.width / 2, y: r.top + r.height / 2, left: r.left, right: r.right, top: r.top, bottom: r.bottom };
})
"""

CENTER_JS = """
(sel) => { const r = document.querySelector(sel).getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; }
"""


@bug(id="R4", fix="10bee2b4", src="QA 08.10: Shift+щелчок у мини-панели попадал в корзину (owner-ux §1 п. 5)", red="pointer-events: none убрать у .minipanel.through")
class MiniPanel(RegressCase):
    def test_shift_click_passes_to_photo(self):
        self.album([2])
        self.wait_js("(sel) => document.querySelectorAll(sel).length === 2", CELLS)
        self.stable_rect(CELLS)
        a, b = self.st.ev(BOXES_JS, CELLS)
        # Выбрано первое фото: столбец панели встаёт справа от него — на второе фото (owner-ux §1 п. 5).
        self.page.mouse.click(a["x"], a["y"])
        self.expect(self.q(PANEL)).to_be_visible(timeout=self.lim(10) * 1000)
        self.stable_rect(PANEL)
        btn = self.st.ev(CENTER_JS, FIRST)
        self.assertTrue(b["left"] < btn["x"] < b["right"] and b["top"] < btn["y"] < b["bottom"], "кнопка панели не лежит на втором фото")
        rev0 = self.rev()

        # Без модификатора под кнопкой — панель.
        self.page.mouse.move(btn["x"], btn["y"])
        under = self.st.ev(UNDER_JS, [btn["x"], btn["y"]])
        self.assertTrue(under["panel"], f"под кнопкой панели не панель: {under}")

        # Shift зажат: панель сквозная, под той же точкой — лист со вторым фото.
        self.page.keyboard.down("Shift")
        self.page.mouse.move(btn["x"] + 1, btn["y"])
        self.expect(self.q(PANEL)).to_have_class(re.compile(r"\bthrough\b"), timeout=self.lim(10) * 1000)
        under = self.st.ev(UNDER_JS, [btn["x"], btn["y"]])
        self.assertEqual(under, {"panel": False, "spread": True}, "с Shift панель не пропустила мышь к листу под ней")

        # Shift+щелчок: второе фото добавлено в выделение (на кнопке «убрать» значок 2), в Go ничего не ушло.
        self.page.mouse.click(btn["x"], btn["y"])
        self.expect(self.q(DEL + " .badge")).to_have_text("2", timeout=self.lim(10) * 1000)
        self.page.keyboard.up("Shift")
        view = self.call("GetView")
        self.assertEqual(view["rev"], rev0, "Shift+щелчок у панели изменил проект")
        self.assertEqual(len(view["spreads"][0]["photos"]), 2, "фото убрано с разворота")
