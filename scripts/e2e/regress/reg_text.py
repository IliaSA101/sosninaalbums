"""Надписи на холсте: вход в правку (R1, 156). Признак — прямоугольники строк в DOM, настоящий шрифт и настоящий Go."""

from base import RegressCase, bug

WORDS = ["Первая", "вторая", "третья"]
# Имя гарнитуры в CSS (internal/text/families.go): по нему ждём загрузки шрифта до замера.
FONT_CSS = {"badscript": "SA Bad Script", "oldstandard": "SA Old Standard TT"}
TOL_EDIT = 1.0  # px: допуск редактора против строк холста (156 design §6 п. 3)
TOL_BACK = 0.5  # px: после Esc строки холста там же

# Центры строк <text> холста.
TEXT_CENTERS_JS = """
() => [...document.querySelectorAll('text')].map((e) => {
  const r = e.getBoundingClientRect(); return [r.x + r.width / 2, r.y + r.height / 2];
})
"""

# Центры строк редактора: прямоугольники диапазона по всему содержимому; нулевой ширины (знак переноса) и повторы по центру отбрасываем.
EDITOR_LINES_JS = """
() => {
  const ed = document.querySelector('.editor'); const rg = document.createRange(); rg.selectNodeContents(ed);
  const out = [];
  for (const r of rg.getClientRects()) {
    if (!(r.width > 0)) continue;
    const c = [r.x + r.width / 2, r.y + r.height / 2];
    if (!out.some((o) => Math.abs(o[0] - c[0]) < 0.5 && Math.abs(o[1] - c[1]) < 0.5)) out.push(c);
  }
  return out;
}
"""


@bug(id="R1", fix="2b2c911d", src="156-5: QA руками на стенде; бэклог К1", red="revert 2b2c911d (Bad Script прыгал на 16,8 px при DPR 1)")
class TextEditJump(RegressCase):
    def test_handwritten_stays(self):
        # Шрифт × число строк × масштаб экрана (профиль A — DPR 1, B — 1,25): прыжок зависит от метрик шрифта и округления браузера.
        for font in FONT_CSS:
            for lines in (1, 3):
                for profile in ("A", "B"):
                    with self.subTest(font=font, lines=lines, profile=profile):
                        self.reopen(profile)
                        self.check_stays(font, lines)

    def check_stays(self, font: str, lines: int) -> None:
        sid = self.album([1])["spreads"][0]["id"]
        self.call("AddText", sid, 0, "e2e")
        tid = self.call("GetView")["spreads"][0]["texts"][0]["id"]
        self.call("SetText", tid, "\n".join(WORDS[:lines]), "e2e")
        style = dict(self.call("GetView")["spreads"][0]["texts"][0]["style"], font=font, size=80)
        self.call("SetTextStyle", tid, style)
        # Строки нарисованы и шрифт загружен (проверка загрузки, а не пауза), затем прямоугольник устойчив.
        self.wait_js(
            "([n, css]) => document.querySelectorAll('text').length === n && document.fonts.check(`80px \"${css}\"`, 'Первая')",
            [lines, FONT_CSS[font]],
        )
        self.fonts_ready()
        self.stable_rect("text")
        before = self.st.ev(TEXT_CENTERS_JS)
        self.assertEqual(len(before), lines)
        rev0 = self.rev()

        # Двойной щелчок по средней строке открывает правку.
        x, y = before[lines // 2]
        self.page.mouse.dblclick(x, y)
        self.expect(self.q(".editor")).to_be_visible(timeout=self.lim(10) * 1000)
        self.fonts_ready()
        self.stable_rect(".editor")
        edit = self.st.ev(EDITOR_LINES_JS)
        self.assertEqual(len(edit), lines, f"строк редактора {edit}")
        for i, (a, b) in enumerate(zip(before, edit)):
            self.assertLessEqual(abs(a[1] - b[1]), TOL_EDIT, f"строка {i + 1}: редактор сдвинут на {b[1] - a[1]:.2f} px от строки холста")

        # Esc: правка закрыта, строки холста на прежнем месте, Go ничего не получил.
        self.page.keyboard.press("Escape")
        self.expect(self.q(".editor")).to_have_count(0, timeout=self.lim(10) * 1000)
        self.wait_js("(n) => document.querySelectorAll('text').length === n", lines)
        self.stable_rect("text")
        after = self.st.ev(TEXT_CENTERS_JS)
        self.assertEqual(len(after), lines)
        for i, (a, b) in enumerate(zip(before, after)):
            self.assertLessEqual(abs(a[1] - b[1]), TOL_BACK, f"строка {i + 1} холста сдвинулась на {b[1] - a[1]:.2f} px после Esc")
            self.assertLessEqual(abs(a[0] - b[0]), TOL_BACK)
        self.settle()
        self.assertEqual(self.rev(), rev0, "вход в правку и Esc без ввода не должны менять проект")
