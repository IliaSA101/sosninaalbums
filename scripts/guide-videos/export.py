"""Ролик «Экспорт в JPEG» (статья export): окно «Файл ▸ Экспорт…», диапазон «2-3, 5п», подсказка под полем, виды.

«Выгрузить» открыл бы системный диалог выбора папки — на стенде он не вызывается: ролик кончается на наведении
курсора на кнопку и закрытии окна («Отмена»), выгрузка не запускается. Показаны группы окна: развороты (все / текущий /
диапазон с подсказкой и ошибкой), вид, качество, dpi.
"""

from __future__ import annotations

ID = "export"
LEVEL = "simple"
START = (900, 200)
NOTE = "«Файл ▸ Экспорт…» ▸ «Все» / «Текущий» ▸ диапазон «2-3, 5п» с подсказкой и ошибкой ▸ вид ▸ качество ▸ dpi ▸ курсор у «Выгрузить» ▸ «Отмена» (диалога папки нет)"
DIALOG = "[role=dialog]"
RANGE = '[aria-label="Диапазон экспорта"]'
HINT = f"{DIALOG} p.hint[role=status]"
ERR = f"{DIALOG} p.err"
SCOPE_ALL = f'{DIALOG} label:has-text("Все (")'
SCOPE_CUR = f'{DIALOG} label:has-text("Текущий (")'
PAGES = f'{DIALOG} label:has-text("По страницам")'
SPREADS = f'{DIALOG} label:has-text("Разворотами")'
QUALITY = f'{DIALOG} [aria-label="Качество"] button:has(b:text-is("100"))'
DPI = f'{DIALOG} [aria-label="Разрешение"] button:has(b:text-is("300"))'


def prepare(st, ids: list[str]) -> None:
    """Пять готовых разворотов по четыре фото: есть что выбирать в диапазоне."""
    for k in range(5):
        st.call("NewSpread", -1, ids[4 * k : 4 * k + 4])


def hint(st) -> str:
    loc = st.page.locator(HINT)
    return loc.first.inner_text() if loc.count() > 0 else ""


def checked(st, sel: str) -> bool:
    return st.page.locator(f"{sel} input").first.is_checked()


def pressed(st, sel: str) -> bool:
    return st.page.locator(sel).first.get_attribute("aria-checked") == "true"


def scene(p, st) -> None:
    p.mark("start")
    p.sleep(1.2)
    p.mark("open")
    st.menu_item("Файл", "export")  # меню — через data-item; курсор ролика при этом не двигается
    p.wait(lambda: st.page.locator(DIALOG).count() > 0 and st.page.locator(RANGE).count() > 0, "окно экспорта")
    p.sleep(1.2)
    p.mark("scope")
    p.click(SCOPE_CUR)
    p.wait(lambda: checked(st, SCOPE_CUR), "выбран «Текущий»")
    p.sleep(1.2)
    p.click(SCOPE_ALL)
    p.wait(lambda: checked(st, SCOPE_ALL), "выбран «Все»")
    p.sleep(1.4)
    p.mark("range")
    p.click(RANGE)  # фокус в поле переключает «Диапазон»
    st.page.keyboard.type("2-3", delay=160)
    p.wait(lambda: "2" in hint(st), "подсказка под полем диапазона")
    p.sleep(1.4)
    st.page.keyboard.type(", 5п", delay=160)
    p.sleep(1.6)
    p.mark("error")
    st.page.keyboard.type(", 99", delay=160)  # такого разворота нет: вместо подсказки — ошибка
    p.wait(lambda: st.page.locator(ERR).count() > 0, "ошибка диапазона")
    p.sleep(1.4)
    for _ in range(4):
        st.page.keyboard.press("Backspace")
        st.page.wait_for_timeout(160)
    p.wait(lambda: "2" in hint(st), "подсказка вернулась")
    p.sleep(1.0)
    p.mark("kind")
    p.click(PAGES)
    p.wait(lambda: checked(st, PAGES), "вид «По страницам»")
    p.sleep(1.4)
    p.click(SPREADS)
    p.wait(lambda: checked(st, SPREADS), "вид «Разворотами»")
    p.sleep(1.2)
    p.mark("quality")
    p.click(QUALITY)
    p.wait(lambda: pressed(st, QUALITY), "качество 100")
    p.sleep(1.2)
    p.click(DPI)
    p.wait(lambda: pressed(st, DPI), "300 dpi")
    p.sleep(1.4)
    p.mark("run")
    p.to(f"{DIALOG} button.primary")  # к «Выгрузить»: нажатие открыло бы диалог папки — не нажимать
    p.sleep(1.8)
    p.mark("close")
    p.click(f'{DIALOG} button:has-text("Отмена")')
    p.wait(lambda: st.page.locator(DIALOG).count() == 0, "окно экспорта закрылось")
    p.sleep(1.6)
    p.mark("end")
