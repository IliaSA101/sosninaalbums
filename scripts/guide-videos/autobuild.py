"""Ролик «Автосборка» (статья autobuild): окно с предпросмотром, ползунок, «Собрать», листание, «Пересобрать».

Шаги и селекторы — как у сцены A роликов README (scripts/demo/scenes.py): тот же экран, другие паузы и окно 1280×760.
"""

from __future__ import annotations

import scenes
from scenes import AUTO_BTN, DIALOG, dialog_ready, flip, painted

ID = "autobuild"
LEVEL = "simple"
START = (330, 640)  # у кнопки «Автосборка»: «Альбом пуст» не висит без курсора
NOTE = "«Автосборка» ▸ ползунок числа разворотов ▸ «Собрать» ▸ листание разворотов ▸ «Пересобрать» с другим вариантом"


def prepare(st, ids: list[str]) -> None:
    """Пустой альбом, вся лента свободна: ничего сверх общей подготовки."""


def scene(p, st) -> None:
    p.mark("start")
    p.sleep(0.8)
    p.click(AUTO_BTN)
    p.wait(lambda: dialog_ready(st), "окно «Автосборка» с предпросмотром")
    p.sleep(0.9)
    rng = st.page.locator("#auto-range")
    lo, hi, v0 = float(rng.get_attribute("min") or 1), float(rng.get_attribute("max") or 1), float(rng.input_value())
    box = rng.bounding_box()
    target = min(hi, 12.0)  # заметный ход ползунка: по умолчанию там 10
    xs = lambda v: box["x"] + box["width"] * (v - lo) / (hi - lo)  # noqa: E731
    y = box["y"] + box["height"] / 2
    p.move(xs(v0), y)
    p.sleep(0.2)
    st.page.mouse.down()
    p.move(xs(target), y, dur=0.9)
    st.page.mouse.up()
    p.wait(lambda: dialog_ready(st), "предпросмотр после ползунка")
    p.sleep(0.8)
    p.click(f'{DIALOG} button:has-text("Что соберётся")')
    p.sleep(1.6)
    n = int(float(rng.input_value()))
    p.click(f'{DIALOG} button:has-text("Собрать")')
    p.wait(lambda: len(st.view()["spreads"]) == n and st.page.locator(DIALOG).count() == 0, f"сборка {n} разворотов")
    p.wait(lambda: painted(st), "разворот на холсте после сборки", timeout=15.0)
    p.sleep(1.0)
    p.to("[data-guide=canvas]", dy=-60)
    for i in range(1, 4):
        flip(p, st, i, hold=1.0)
    p.click(AUTO_BTN)
    p.wait(lambda: dialog_ready(st), "окно пересборки")
    p.sleep(0.8)
    p.click(f'{DIALOG} button[aria-label="Следующий вариант"]')
    p.wait(lambda: dialog_ready(st), "предпросмотр следующего варианта")
    p.sleep(1.2)
    p.click(f'{DIALOG} button:has-text("Пересобрать")')
    p.wait(lambda: st.page.locator(DIALOG).count() == 0 and painted(st), "пересборка", timeout=15.0)
    p.sleep(1.0)
    p.to("[data-guide=canvas]", dy=-60)
    for i in range(4, 6):
        flip(p, st, i, hold=1.0)
    p.sleep(1.0)
    p.mark("end")
