"""Ролик «Макеты альбома» (статья looks): «Файл ▸ Макеты альбома…», плитка, шаги ‹ ›, «Применить» на пустом холсте.

Нужен установленный набор с макетом: стенд (SA_DEMO) примера не кладёт, поэтому prepare ставит zip примера из
internal/demopack через AddPackPath и ConfirmPack — без диалога выбора файла и без сети.
"""

from __future__ import annotations

from pathlib import Path

import gvlib

ID = "looks"
LEVEL = "simple"
START = (900, 200)
NOTE = "«Файл ▸ Макеты альбома…» ▸ плитка макета ▸ шаги ‹ › ▸ «Применить» ▸ листание готовых разворотов"
DIALOG = "[role=dialog]"
TILE = f"{DIALOG} button.tile"
NEXT = f'{DIALOG} [aria-label="Следующий шаг"]'
APPLY = f'{DIALOG} button:has-text("Применить")'
DEMO_ZIP = gvlib.ROOT / "internal" / "demopack" / "wedding-navy-watercolor-demo-1.zip"


def prepare(st, ids: list[str]) -> None:
    """Пустой альбом; набор примера в библиотеке (повторный запуск — «уже стоит»: токена нет, это не ошибка)."""
    if not Path(DEMO_ZIP).is_file():
        raise gvlib.Refuse(f"нет {DEMO_ZIP}")
    add = st.call("AddPackPath", str(DEMO_ZIP))
    if add and add.get("token"):
        st.call("ConfirmPack", add["token"])


def scene(p, st) -> None:
    p.mark("start")
    p.sleep(1.2)
    p.mark("open")
    st.menu_item("Файл", "looks")
    p.wait(lambda: st.page.locator(TILE).count() > 0, "плитки макетов в окне", timeout=20.0)
    p.sleep(1.4)
    p.mark("tile")
    p.click(st.page.locator(TILE).first)
    p.wait(lambda: st.page.locator(NEXT).count() > 0, "предпросмотр с шагами макета")
    p.sleep(1.6)
    p.mark("steps")
    for _ in range(3):  # шаг виден по смене листа справа; признака в состоянии нет — держим паузу
        p.click(NEXT)
        p.sleep(1.6)
    p.mark("apply")
    p.click(APPLY)
    p.wait(lambda: len(st.view()["spreads"]) > 0 and st.page.locator(DIALOG).count() == 0, "макет применён", timeout=30.0)
    p.sleep(1.6)
    p.to("[data-guide=canvas]", dy=-60)
    for i in range(1, 3):
        p.click(f'[data-guide=timeline] .group[data-index="{i}"]', dx=-60)
        p.sleep(1.6)
    p.mark("end")
