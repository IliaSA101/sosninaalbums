"""Ролик «Варианты раскладки» (статья variants): стрелки полосы «Раскладка: Авто», Пробел и Shift+Пробел, галерея ⊞."""

from __future__ import annotations

from scenes import layout_pos

ID = "variants"
LEVEL = "simple"
START = (900, 200)
NOTE = "Полоса над разворотом ▸ стрелки «‹ ›» листают раскладки ▸ Пробел и Shift+Пробел ▸ ⊞ все раскладки ▸ выбор плитки"
NEXT = '[data-guide=layout-bar] [aria-label="Следующая раскладка разворота"]'
ALL = '[data-guide=layout-bar] [aria-label="Все раскладки разворота"]'


def prepare(st, ids: list[str]) -> None:
    """Один готовый разворот из пяти фото: у него несколько десятков раскладок."""
    st.call("NewSpread", -1, ids[:5])


def step(p, st, what: str, act, hold: float = 1.5) -> None:
    """Действие, меняющее раскладку: ждём новый номер «Раскладка N / M», пауза на кадре."""
    before = layout_pos(st)
    act()
    p.wait(lambda: layout_pos(st) != before, f"счётчик раскладок ({what})")
    p.sleep(hold)


def scene(p, st) -> None:
    p.mark("start")
    p.sleep(1.0)
    p.mark("arrows")
    p.to("[data-guide=canvas]", dy=-60)  # над холстом полоса показывает «Раскладка: Авто» со стрелками
    p.sleep(1.0)
    for i in range(1, 5):
        step(p, st, f"стрелка {i}", lambda: p.click(NEXT))
    p.mark("space")
    p.to("[data-guide=canvas]", dy=20)
    for i in range(2):
        step(p, st, f"Пробел {i + 1}", lambda: st.page.keyboard.press("Space"), hold=1.4)
    step(p, st, "Shift+Пробел", lambda: st.page.keyboard.press("Shift+Space"), hold=1.6)
    p.mark("gallery")
    p.to("[data-guide=canvas]", dy=-60)
    p.click(ALL)
    p.wait(lambda: st.page.locator("section.gallery button.card").count() > 2, "ряд плиток раскладок")
    p.sleep(0.8)
    cards = st.page.locator("section.gallery button.card")
    for k in (1, 3):
        p.to(cards.nth(k))
        p.sleep(0.5)
    p.mark("pick")
    p.click(cards.nth(3))
    p.sleep(1.4)
    if st.page.locator("section.gallery").count() > 0:  # выбор плитки галерею не закрывает — закрываем как пользователь
        p.click('[aria-label="Закрыть галерею раскладок"]')
    p.wait(lambda: st.page.locator("section.gallery").count() == 0, "закрытие галереи")
    p.sleep(1.6)
    p.mark("end")
