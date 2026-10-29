"""Общие действия сценариев F1 над страницей стенда (поверх pilot.Pilot): листание карточки колесом, ожидание «Собираю…».

Чистая проверка «элемент виден в окне» — gvlib.inside (тест test_gvlib.py); здесь только то, что нужно браузеру.
"""

from __future__ import annotations

import gvlib

CARD = "section.insp"  # прокручиваемая карточка справа (Inspector)


def box_of(st, sel: str) -> dict | None:
    loc = st.page.locator(sel).first
    return loc.bounding_box() if loc.count() > 0 else None


def reveal_in_card(p, st, sel: str, step: float = 70.0) -> None:
    """Карточка узкая: кнопка ниже её края. Курсор — на карточку, колесо (вниз; вверх, если элемент выше), пока кнопка не окажется внутри.

    Так в ролике виден жест пользователя, а не мгновенный скачок прокрутки.
    """
    p.wait(lambda: box_of(st, sel) is not None, f"элемент {sel} в карточке")
    p.to(CARD, dy=-20)

    def seen() -> bool:
        b, c = box_of(st, sel), box_of(st, CARD)
        return bool(b and c and gvlib.inside(b, c, pad=4.0))

    b, c = box_of(st, sel), box_of(st, CARD)
    up = bool(b and c and b["y"] < c["y"])  # элемент выше видимой части (карточка уже прокручена) — колесо вверх
    p.wheel_until(seen, f"прокрутка карточки до {sel}", dy=-step if up else step, every=0.05)
    p.sleep(0.4)


def settled(st) -> bool:
    """«Собираю…» в тексте страницы нет: промежуточный кадр с пустым холстом (1–2 с) прошёл."""
    return "Собираю" not in st.ev("() => document.body.innerText || ''")


def drag_from(p, st, x: float, y: float, to: tuple[float, float], rest: float = 0.4) -> None:
    """Перенос от точки (x, y), а не от центра элемента: центр ячейки занят ✥ сдвига кадра (FrameControls)."""
    import motion

    p.move(x, y)
    p.sleep(motion.CLICK_REST)
    st.page.mouse.down()
    for sx, sy in motion.drag_start_steps((p.x, p.y)):
        st.page.mouse.move(sx, sy)
        st.page.wait_for_timeout(16)
        p.x, p.y = sx, sy
    p.move(*to)
    p.sleep(rest)
    st.page.mouse.up()
