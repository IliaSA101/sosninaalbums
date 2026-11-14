"""Регресс окна выгрузки (166): R6 повторное открытие не показывает объём прошлого раза."""

from base import RegressCase, bug

SIZE = "p.hint.size"
EXPORT_BTN = "[data-guide=topbar-export]"

# Журнал строки объёма: каждое новое значение, пока окно открывается и закрывается. Наблюдатель стоит на body:
# сама строка живёт только при открытом окне, а её первый кадр при повторном открытии и есть предмет проверки.
LOG_JS = """
(sel) => {
  const W = window; W.__reg.rec = W.__reg.rec || {};
  const log = (W.__reg.rec.size = []);
  const snap = () => {
    const e = document.querySelector(sel); if (!e) return;
    if (log[log.length - 1] !== e.textContent) log.push(e.textContent);
  };
  new MutationObserver(snap).observe(document.body, { subtree: true, childList: true, characterData: true });
  snap();
}
"""


@bug(id="R6", fix="3a71cfdf", src="окно выгрузки, 04.11", red="revert 3a71cfdf")
class ExportSize(RegressCase):
    def _open(self) -> None:
        self.q(EXPORT_BTN).click()
        self.expect(self.q(SIZE)).to_be_attached()

    def _shown(self, asked, n: int) -> str:
        """Окно спросило объём (n-й вызов) -> ждём в строке ровно ответ Go на этот же запрос; возвращает строку."""
        self.wait_js("([k, n]) => window.__reg.spies[k].calls.length >= n", ["ExportSize", n])
        want = self.call("ExportSize", asked.calls()[n - 1][0])["summary"] + "."
        self.wait_js("([s, t]) => document.querySelector(s)?.textContent === t", [SIZE, want])
        return want

    def test_reopen_no_stale_size(self):
        """Закрыли окно, альбом вырос, открыли снова: прежней цифры в строке нет ни кадра, затем — ответ Go."""
        self.album([2, 2, 2])
        asked = self.spy("ExportSize")
        n0 = asked.count()
        self._open()
        first = self._shown(asked, n0 + 1)
        self.page.keyboard.press("Escape")
        self.expect(self.q(SIZE)).to_have_count(0)

        for _ in range(3):
            self.call("NewSpread", -1, [])
        v = self.call("GetView")
        self.wait_js("(r) => window.__reg.rev >= r", v["rev"])
        self.st.ev(LOG_JS, SIZE)
        n0 = asked.count()
        self._open()
        second = self._shown(asked, n0 + 1)
        self.assertNotEqual(first, second, "альбом изменился, а объём тот же: проверка ничего не доказывает")
        seen = self.st.ev("() => window.__reg.rec.size")
        self.assertNotIn(first, seen, f"при повторном открытии мелькнул прошлый объём: {seen}")
