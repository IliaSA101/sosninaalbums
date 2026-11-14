"""Ролик «Виньетка» (статья vignette): «Сделать виньеткой» в карточке разворота, шаблоны полосы, «Обычный разворот»."""

from __future__ import annotations

from gvact import CARD, reveal_in_card, settled

ID = "vignette"
LEVEL = "standard"  # виньетка открывается с «Стандартного» (levels.ts)
START = (900, 200)
NOTE = "Карточка разворота ▸ «Сделать виньеткой» ▸ шаблоны виньетки ‹ › и ⊞ ▸ «Обычный разворот» в «⋯» группы"
MAKE = "[data-guide=vig-make]"
NEXT = '[data-guide=layout-bar] [aria-label="Следующий шаблон виньетки"]'
GRID = '[data-guide=layout-bar] [aria-label="Все шаблоны виньетки"]'
DETAILS = "[data-guide=topbar-details]"  # значок справа от «Формат»: карточка справа
MORE = '[aria-label="Ещё: Виньетка"]'  # «⋯» группы «Виньетка»
ITEM = '[role=menuitem]:has-text("Обычный разворот")'
CONFIRM = '[role=dialog] button:has-text("Обычный разворот")'


def prepare(st, ids: list[str]) -> None:
    """Один обычный разворот из восьми фото: он и станет виньеткой."""
    st.call("NewSpread", -1, ids[:8])


def is_vig(st) -> bool:
    return bool(st.view()["spreads"][0].get("vig"))


def bar_name(st) -> str:
    """Подпись полосы («Виньетка: имя»): меняется при каждом шаге ‹ ›."""
    loc = st.page.locator("[data-guide=layout-bar] .tag.name")
    return loc.first.inner_text() if loc.count() > 0 else ""


def gallery_cards(st) -> int:
    return st.page.locator("section.gallery button.tile").count()


def scene(p, st) -> None:
    p.mark("start")
    p.sleep(1.0)
    p.mark("card")
    if st.page.locator(DETAILS).first.get_attribute("aria-pressed") != "true":  # на «Стандартном» карточка закрыта
        p.click(DETAILS)
    p.wait(lambda: st.page.locator(CARD).count() > 0, "карточка справа")
    p.sleep(0.8)
    p.mark("make")
    reveal_in_card(p, st, MAKE)
    p.sleep(0.4)
    p.click(MAKE)
    p.wait(lambda: is_vig(st), "разворот стал виньеткой")
    p.wait(lambda: settled(st), "перерисовка после «Сделать виньеткой»", timeout=15.0)
    p.sleep(1.8)
    p.mark("arrows")
    st.page.keyboard.press("Escape")  # выделение места заменяет полосу раскладки панелью «Выровнять»
    p.to("[data-guide=canvas]", dy=-60)  # над холстом — полоса «Виньетка: имя» со стрелками
    p.wait(lambda: st.page.locator(NEXT).count() > 0, "стрелки шаблонов виньетки")
    p.sleep(0.8)
    for i in range(2):
        name = bar_name(st)
        p.click(NEXT)
        p.wait(lambda: bar_name(st) != name, f"новый шаблон виньетки ({i + 1})")
        p.wait(lambda: settled(st), "перерисовка шаблона", timeout=15.0)
        p.sleep(1.4)
    p.mark("gallery")
    p.click(GRID)
    p.wait(lambda: gallery_cards(st) > 2, "ряд плиток шаблонов виньетки", timeout=15.0)
    p.sleep(1.4)
    p.click('[aria-label="Закрыть галерею раскладок"]')
    p.wait(lambda: st.page.locator("section.gallery").count() == 0, "закрытие галереи")
    p.sleep(0.8)
    p.mark("clear")
    reveal_in_card(p, st, MORE)
    p.click(MORE)
    p.wait(lambda: st.page.locator(ITEM).count() > 0, "пункт «Обычный разворот»")
    p.sleep(1.0)
    p.click(ITEM)
    p.sleep(0.8)
    if st.page.locator(CONFIRM).count() > 0:  # может спросить подтверждение
        p.sleep(0.8)
        p.click(CONFIRM)
    p.wait(lambda: not is_vig(st), "разворот снова обычный")
    p.wait(lambda: settled(st), "перерисовка после «Обычный разворот»", timeout=15.0)
    p.sleep(2.0)
    p.mark("end")
