"""Ролик «Ручная раскладка» (статья manual): «Ручная» в карточке разворота, перенос места, размер за угол, «Вернуть авто»."""

from __future__ import annotations

from gvact import drag_from, reveal_in_card

ID = "manual"
LEVEL = "full"  # «Ручная раскладка» открывается с уровня «Полный» (levels.ts)
START = (900, 200)
NOTE = "Карточка разворота ▸ «Ручная» ▸ перенос места ▸ размер за угол ▸ «Вернуть авто»"
MODE = "[data-guide=mode-toggle]"
PLACE = "[data-guide=canvas] .cell[data-drop=place]"
DIALOG = "[role=dialog]"
CORNER = "[data-guide=canvas] .h.se"  # правый нижний угол выделенного места


def prepare(st, ids: list[str]) -> None:
    """Один готовый разворот из пяти фото: раскладка авто, места ещё не заданы."""
    st.call("NewSpread", -1, ids[:5])


def scene(p, st) -> None:
    p.mark("start")
    p.sleep(1.2)
    p.mark("grid-on")
    reveal_in_card(p, st, MODE)
    p.click(f'{MODE} button:has-text("Ручная")')
    p.wait(lambda: st.view()["spreads"][0].get("grid"), "разворот стал ручным")
    p.sleep(1.0)
    p.to("[data-guide=canvas]", dy=-60)  # над холстом полоса показывает «Раскладка: Ручная · Вернуть авто»
    p.sleep(1.6)
    p.mark("select")
    p.click(PLACE)
    p.wait(lambda: st.page.locator(CORNER).count() > 0, "ручки выделенного места")
    p.sleep(1.2)
    p.mark("resize")
    b = st.page.locator(CORNER).first.bounding_box()
    p.drag(CORNER, (b["x"] + b["width"] / 2 - 90, b["y"] + b["height"] / 2 - 70), dst_rest=0.5)
    p.sleep(1.4)
    p.mark("move")
    first = st.page.locator(PLACE).first
    c = first.bounding_box()
    x, y = c["x"] + c["width"] * 0.25, c["y"] + c["height"] * 0.3  # не центр: там ✥ сдвига кадра
    drag_from(p, st, x, y, (x + 50, y + 70), rest=0.5)
    p.sleep(1.6)
    p.mark("back-auto")
    st.page.keyboard.press("Escape")  # снять выделение: вместо «Выровнять» снова полоса раскладки
    p.to("[data-guide=canvas]", dy=-60)
    p.click('[data-guide=layout-bar] button:has-text("Вернуть авто")')
    p.wait(lambda: st.page.locator(DIALOG).count() > 0, "вопрос «Вернуть авто?»")
    p.sleep(1.2)
    p.click(f'{DIALOG} button:has-text("Вернуть авто")')
    p.wait(lambda: not st.view()["spreads"][0].get("grid") and st.page.locator(DIALOG).count() == 0, "раскладка снова авто")
    p.sleep(2.2)
    p.mark("end")
