"""Сцены роликов README (102, design §4.3): A «Автосборка и листание», B «Раскладки: бросок и перебор».

Сцена — функция `(p: Pilot, st: Stand, ids: list[str])`: действия мышью, ожидание результата по состоянию
(`GetView`, текст шапки, `[role=dialog]`), метки журнала через `p.mark`. Метки: A — start, open-auto, slider, build,
built, build-click, built, flip-1…5, again, rebuild-click, rebuilt, flip-6…8, end; B — start, select, drag, drop, variant-1…4, gallery, pick, add-one,
swap, still, end. Прогрев-проход (без журнала) идёт той же функцией. Селекторы — по aria-label, data-* и тексту,
не по координатам (кроме слота «Новый разворот»: он виден лишь краем, брошенное фото кладётся в его видимую часть).
"""

from __future__ import annotations

import re

from pilot import Pilot

# Отрезки заставки: (сцена, метка-от, метка-до). Между ними склейка встык на кадре с полным разворотом (design §4.3).
HERO: list[tuple[str, str, str]] = [("B", "select", "variant-2"), ("A", "flip-1", "flip-4")]

# Вырезки ожидания (compose.Scene.cuts): (метка щелчка, метка результата, сколько секунд оставить после щелчка).
# Пока Go считает сборку и страница рисует развороты (~4 с на wails dev), окно просто висит — в ролике его нет.
CUTS: dict[str, list[tuple[str, str, float]]] = {"A": [("build-click", "built", 0.5), ("rebuild-click", "rebuilt", 0.5)]}

# Где стоит курсор в начале записи: у кнопки сцены, чтобы «Альбом пуст» не висел (A — у «Автосборки»).
START: dict[str, tuple[int, int]] = {"A": (330, 700), "B": (800, 520)}

AUTO_BTN = '[data-guide=library] button:has-text("Автосборка")'
DIALOG = "[role=dialog]"
SPREAD_RX = re.compile(r"Разворот\s+(\d+)\s+из\s+(\d+)")
LAYOUT_RX = re.compile(r"Раскладка\s+(\d+)\s*/\s*(\d+)")


def spread_pos(st) -> tuple[int, int]:
    """(номер, всего) из шапки «Разворот N из M»; (0, 0) — пока шапки нет."""
    m = SPREAD_RX.search(st.ev("() => document.body.innerText"))
    return (int(m.group(1)), int(m.group(2))) if m else (0, 0)


def layout_pos(st) -> tuple[int, int]:
    m = LAYOUT_RX.search(st.ev("() => document.body.innerText"))
    return (int(m.group(1)), int(m.group(2))) if m else (0, 0)


def dialog_ready(st) -> bool:
    """Окно открыто, и «подбираем раскладки…» в нём нет (предпросмотр готов)."""
    return st.page.locator(DIALOG).count() > 0 and "подбираем" not in st.ev("() => document.querySelector('[role=dialog]')?.innerText || ''")


def ensure_spread(p: Pilot, st, n: int, total: int | None = None) -> None:
    """Текущий разворот — n (и всего total, если задано): иначе щелчок по нему в ленте и ожидание «Разворот n из …».

    Страница сама может сменить текущий разворот (сверка папок по фокусу, перерисовка вида): сцена не должна листать раскладки
    не того разворота. Попытки не бесконечны: дальше — SceneFail с меткой.
    """
    for _ in range(3):
        cur = spread_pos(st)
        if cur[0] == n and (total is None or cur[1] == total):
            return
        p.click(f'[data-guide=timeline] .group[data-index="{n - 1}"]', dx=-60)
        p.sleep(0.8)
    p.wait(lambda: spread_pos(st)[0] == n, f"текущий разворот {n}")


def painted(st) -> bool:
    """На холсте есть фото разворота: окно закрыто, а «Альбом пуст» ещё не сменился картинкой — кадр был бы пустым."""
    return st.page.locator("[data-guide=canvas] [data-drop=cell]").count() > 0 and st.page.locator("[role=dialog]").count() == 0


def flip(p: Pilot, st, n: int, hold: float = 1.1) -> None:
    """Один шаг колесом по ряду разворотов (одно «деление» = один разворот) и пауза на нём."""
    p.mark(f"flip-{n}")
    before = spread_pos(st)[0]
    p.wheel_until(lambda: spread_pos(st)[0] != before, f"разворот не сменился (flip-{n})", every=0.05)
    p.sleep(hold)


def scene_a(p: Pilot, st, ids: list[str]) -> None:
    """A: лента полна, холст пуст → окно «Автосборка» (ползунок) → «Собрать» → листание → «Пересобрать» → листание."""
    p.mark("start")
    p.sleep(0.5)
    p.mark("open-auto")
    p.click(AUTO_BTN)
    p.wait(lambda: dialog_ready(st), "окно «Автосборка» с предпросмотром")
    p.sleep(0.6)
    p.mark("slider")
    rng = st.page.locator("#auto-range")
    lo, hi, v0 = float(rng.get_attribute("min") or 1), float(rng.get_attribute("max") or 1), float(rng.input_value())
    box = rng.bounding_box()
    target = min(hi, max(v0, 12.0))  # по спеке 102: 10 → 12 разворотов
    # Ползунок стоит от края до края дорожки: x значения v — линейно по [min, max].
    xs = lambda v: box["x"] + box["width"] * (v - lo) / (hi - lo)  # noqa: E731
    y = box["y"] + box["height"] / 2
    p.move(xs(v0), y)
    p.sleep(0.15)
    st.page.mouse.down()
    p.move(xs(target), y, dur=0.9)
    st.page.mouse.up()
    p.wait(lambda: dialog_ready(st), "предпросмотр после ползунка")
    p.click(f'{DIALOG} button:has-text("Что соберётся")')
    p.sleep(1.8)
    n = int(float(rng.input_value()))
    p.mark("build", f"{n} разворотов")
    p.click(f'{DIALOG} button:has-text("Собрать")')
    p.mark("build-click")
    p.wait(lambda: len(st.view()["spreads"]) == n and st.page.locator(DIALOG).count() == 0, f"сборка {n} разворотов")
    p.wait(lambda: painted(st), "разворот на холсте после сборки", timeout=15.0)
    p.mark("built")
    p.sleep(1.2)
    p.to("[data-guide=canvas]", dy=-60)
    for i in range(1, 6):
        flip(p, st, i)
    p.mark("again")
    p.click(AUTO_BTN)
    p.wait(lambda: dialog_ready(st), "окно пересборки")
    p.click(f'{DIALOG} button[aria-label="Следующий вариант"]')
    p.wait(lambda: dialog_ready(st), "предпросмотр следующего варианта")
    p.sleep(1.0)
    p.click(f'{DIALOG} button:has-text("Пересобрать")')
    p.mark("rebuild-click")
    p.wait(lambda: st.page.locator(DIALOG).count() == 0 and painted(st), "пересборка", timeout=15.0)
    p.mark("rebuilt")
    p.sleep(1.2)
    p.to("[data-guide=canvas]", dy=-60)
    for i in range(6, 9):
        flip(p, st, i)
    p.sleep(1.5)
    p.mark("end")


def thumb(ids: list[str], i: int) -> str:
    return f'[data-thumb="{ids[i]}"]'


def scene_b(p: Pilot, st, ids: list[str]) -> None:
    """B: два разворота → выделение 5 фото → бросок на «Новый разворот» → перебор раскладок → «Варианты» → ещё фото → обмен."""
    p.mark("start")
    st.page.keyboard.press("Escape")  # чистое состояние: без выделения и окон от прежнего прохода
    p.wait(lambda: len(st.view()["spreads"]) == 2, "два готовых разворота")
    ensure_spread(p, st, 2, 2)
    p.sleep(1.2)
    p.mark("select")
    p.click(thumb(ids, 0))
    p.sleep(0.3)
    p.click(thumb(ids, 4), shift=True)
    p.sleep(0.6)
    p.mark("drag", "5 фото")
    n0 = len(st.view()["spreads"])
    p.drag(thumb(ids, 2), (1450, 400))
    p.wait(lambda: len(st.view()["spreads"]) == n0 + 1, "новый разворот после броска")
    p.wait(lambda: spread_pos(st) == (n0 + 1, n0 + 1), "холст на новом развороте после броска")
    p.mark("drop")
    p.sleep(1.2)
    p.to("[data-guide=canvas]", dy=-60)
    nxt = '[aria-label="Следующая раскладка"]'
    for i in range(1, 5):
        ensure_spread(p, st, n0 + 1)
        before = layout_pos(st)
        p.mark(f"variant-{i}")
        p.click(nxt)
        p.wait(lambda: layout_pos(st) != before, f"счётчик раскладок (variant-{i})")
        p.sleep(1.3)
    p.mark("gallery")
    ensure_spread(p, st, n0 + 1)
    p.to("[data-guide=canvas]", dy=-250)
    p.click('[aria-label="Все раскладки разворота"]')
    p.wait(lambda: st.page.locator("section.gallery button.card").count() > 2, "ряд плиток «Варианты»")
    p.sleep(0.6)
    cards = st.page.locator("section.gallery button.card")
    for k in (1, 2):
        p.to(cards.nth(k))
        p.sleep(0.5)
    p.mark("pick")
    p.click(cards.nth(2))
    p.sleep(1.0)
    if st.page.locator("section.gallery").count() > 0:  # выбор плитки галерею не закрывает — закрываем как пользователь
        p.click('[aria-label="Закрыть галерею раскладок"]')
    p.wait(lambda: st.page.locator("section.gallery").count() == 0, "закрытие галереи")
    p.sleep(0.6)
    p.mark("add-one")
    ensure_spread(p, st, n0 + 1)
    p.drag(thumb(ids, 5), (800, 160))  # белое поле текущего разворота над фото: цель «разворот», не «ячейка»
    p.wait(lambda: len(st.view()["spreads"][-1]["photos"]) == 6, "шестое фото на развороте")
    p.sleep(1.4)
    p.mark("swap")
    ensure_spread(p, st, n0 + 1)
    cells = st.page.locator("[data-guide=canvas] [data-drop=cell]")
    cells.first.wait_for()
    p.drag(cells.nth(0), cells.nth(2))
    p.sleep(1.2)
    p.to("[data-guide=canvas]", dy=260)
    st.page.evaluate("() => window.__demoCursor && window.__demoCursor.hide()")  # снимок README — без курсора
    p.sleep(0.4)
    p.mark("still")
    p.sleep(1.5)
    p.mark("end")
