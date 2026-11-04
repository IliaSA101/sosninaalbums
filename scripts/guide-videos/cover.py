"""Ролик «Обложка» (статья cover): фото на плитку «Обложка», раскладки «‹ ▦ ›» над лицевой и задней сторонкой."""

from __future__ import annotations

import json

ID = "cover"
LEVEL = "simple"  # «‹ ▦ ›» обложки — с уровня «Варианты» (как у страниц)
START = (900, 200)
NOTE = "Фото на плитку «Обложка» ▸ «‹ ▦ ›» над лицевой сторонкой листают раскладки ▸ то же над задней ▸ ▦ галерея"
DROP = "[data-drop=cover]"
FRONT = '[data-guide=layout-bar] [aria-label="Следующая раскладка лицевой"]'
BACK = '[data-guide=layout-bar] [aria-label="Следующая раскладка задней"]'
G_FRONT = '[data-guide=layout-bar] [aria-label="Лицевая сторонка"]'  # группа «‹ ▦ ›»: стрелки видны при наведении
G_BACK = '[data-guide=layout-bar] [aria-label="Задняя сторонка"]'
FRONT_GRID = '[data-guide=layout-bar] [aria-label="Раскладки лицевой"]'
TILE = "section.gallery button.card"
PHOTO = {"id": ""}  # фото для броска на обложку: ID первого фото ленты (в развороты оно не входит)


def prepare(st, ids: list[str]) -> None:
    """Три готовых разворота (фото с 7-го): обложка — отдельная плитка, фото для неё — из первого ряда ленты."""
    PHOTO["id"] = ids[0]
    for k in range(3):
        st.call("NewSpread", -1, ids[6 + 5 * k : 11 + 5 * k])


def tiles(st) -> int:
    return st.page.locator(TILE).count()


def cover(st) -> dict | None:
    return st.view().get("cover")


def sig(st) -> str:
    """Подпись раскладки обложки: места листа; меняется при каждом шаге ‹ ›."""
    c = cover(st)
    return json.dumps(c["sheet"], sort_keys=True) if c else ""


def step(p, st, what: str, group: str, sel: str, hold: float = 1.5) -> None:
    before = sig(st)
    p.to(group)  # курсор на группе: стрелки появляются, только пока она под ним
    p.sleep(0.3)
    p.click(sel)
    p.wait(lambda: sig(st) != before, f"раскладка обложки ({what})")
    p.sleep(hold)


def scene(p, st) -> None:
    p.mark("start")
    p.sleep(1.2)
    p.mark("drop")
    p.drag(f'[data-thumb="{PHOTO["id"]}"]', DROP, dst_rest=0.6)
    p.wait(lambda: cover(st) is not None, "обложка появилась")
    p.sleep(1.4)
    p.mark("front")
    p.to("[data-guide=canvas]", dy=-60)  # группы «‹ ▦ ›» появляются при наведении на обложку
    p.wait(lambda: st.page.locator(G_FRONT).count() > 0, "группа «‹ ▦ ›» лицевой сторонки")
    p.sleep(0.8)
    p.to(G_FRONT)  # у сторонки без мест › открывает галерею (с одним местом): выбрать плитку
    p.sleep(0.3)
    p.click(FRONT)
    p.wait(lambda: tiles(st) > 1, "плитки раскладок лицевой", timeout=15.0)
    p.sleep(1.2)
    p.click('section.gallery button:text-is("3")')  # вкладка «Число мест»: три места — у раскладок есть из чего листать
    p.sleep(1.0)
    before = sig(st)
    p.click(st.page.locator(TILE).nth(1))
    p.wait(lambda: sig(st) != before, "места лицевой сторонки")
    p.wait(lambda: st.page.locator("section.gallery").count() == 0, "галерея закрылась после выбора")
    p.sleep(0.8)
    p.mark("fill")
    p.click('button:has-text("Заполнить места")')  # пустые места занимают следующие фото ленты
    p.wait(lambda: "+ фото" not in st.page.locator("[data-guide=canvas]").first.inner_text(), "фото встали в места")
    p.sleep(1.4)
    p.mark("arrows")
    p.to("[data-guide=canvas]", dy=-60)  # курсор ушёл в ленту: к обложке — полоса показывает группы снова
    p.wait(lambda: st.page.locator(G_FRONT).count() > 0, "группа «‹ ▦ ›» лицевой сторонки")
    p.sleep(0.5)
    for i in range(1, 4):
        step(p, st, f"лицевая {i}", G_FRONT, FRONT)
    p.mark("gallery")
    p.to(G_FRONT)
    p.click(FRONT_GRID)
    p.wait(lambda: tiles(st) > 1, "галерея раскладок обложки", timeout=15.0)
    p.sleep(1.6)
    p.click('[aria-label="Закрыть галерею раскладок"]')
    p.wait(lambda: st.page.locator("section.gallery").count() == 0, "закрытие галереи")
    p.sleep(1.8)
    p.mark("end")
