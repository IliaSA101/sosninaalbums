"""Ролик «Фото на фоне» (статья bg-photo): фото из ленты брошено в «Фото фона» карточки, прозрачность, «Править», «Готово»."""

from __future__ import annotations

from gvact import CARD, reveal_in_card

ID = "bg-photo"  # модуль — bg_photo.py (gv.py заменяет «-» на «_»)
LEVEL = "standard"  # фото на фоне открывается со «Стандартного» (levels.ts)
START = (900, 200)
NOTE = "Карточка разворота ▸ «Фото фона»: фото из ленты ▸ «Прозрачность» ▸ «Готово» ▸ «Править»: сдвиг в рамке ▸ «Готово»"
ZONE = "[data-drop=backdrop]"
DETAILS = "[data-guide=topbar-details]"
FADE = '[aria-label="Прозрачность фото фона, %"]'
EDIT = f'{ZONE} button:has-text("Править")'
PAN = "[data-guide=canvas] button.pan"  # ✥ сдвига фото в рамке
DONE = 'button:has-text("Готово")'
PHOTO = {"id": ""}
SWAP_PHOTO = (4, "big-srgb-4-portrait.jpg")  # IMG_0005 → 3000×4000: на 60×30 см ≈ 127 dpi, ни красной метки, ни «!»


def prepare(st, ids: list[str]) -> None:
    """Один разворот из четырёх фото; для фона берётся IMG_0005 (первый ряд ленты, не в развороте); gv.py на время статьи подменяет его крупным фото (SWAP_PHOTO)."""
    PHOTO["id"] = ids[4]
    st.call("NewSpread", -1, ids[:4])


def has_bg(st) -> bool:
    sp = st.view()["spreads"][0]
    return bool(sp.get("backdrops"))


def scene(p, st) -> None:
    p.mark("start")
    p.sleep(1.0)
    if st.page.locator(DETAILS).first.get_attribute("aria-pressed") != "true":  # на «Стандартном» карточка закрыта
        p.click(DETAILS)
    p.wait(lambda: st.page.locator(CARD).count() > 0, "карточка справа")
    p.sleep(0.8)
    p.mark("zone")
    reveal_in_card(p, st, ZONE)
    p.sleep(0.8)
    p.mark("drop")
    p.drag(f'[data-thumb="{PHOTO["id"]}"]', ZONE, dst_rest=0.6)
    p.wait(lambda: has_bg(st), "фото легло на фон разворота")
    p.sleep(1.6)
    p.mark("fade")
    reveal_in_card(p, st, FADE)
    box = st.page.locator(FADE).first.bounding_box()
    y = box["y"] + box["height"] / 2
    p.move(box["x"] + box["width"] * 0.05, y)
    p.sleep(0.3)
    st.page.mouse.down()
    p.move(box["x"] + box["width"] * 0.3, y, dur=1.0)  # 30 %: фон явно читается на полях
    st.page.mouse.up()
    p.sleep(1.6)
    p.mark("auto")  # бросок сразу включает режим фона: «Готово» выходит из него, «Править» возвращает
    p.wait(lambda: st.page.locator(PAN).count() > 0, "рамка фона на холсте после броска")
    p.click(f"[role=toolbar] {DONE}")
    p.wait(lambda: st.page.locator(PAN).count() == 0, "выход из режима фона")
    p.sleep(1.4)
    p.mark("edit")
    reveal_in_card(p, st, EDIT)
    p.click(EDIT)
    p.wait(lambda: st.page.locator(PAN).count() > 0, "рамка фона на холсте")
    p.sleep(1.2)
    b = st.page.locator(PAN).first.bounding_box()
    # Только вертикально: фото 3000×4000 выше рамки (вбок — была бы пустота); 120 px уже оставляли белую полосу сверху, 70 — нет
    p.drag(PAN, (b["x"] + b["width"] / 2, b["y"] + b["height"] / 2 + 70), dst_rest=0.5)
    p.sleep(1.6)
    p.mark("done")
    p.click(f"[role=toolbar] {DONE}")
    p.wait(lambda: st.page.locator(PAN).count() == 0, "выход из режима фона")
    p.sleep(2.0)
    p.mark("end")
