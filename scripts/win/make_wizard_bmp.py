#!/usr/bin/env python3
"""Картинки мастера установщика (installer.nsi), BMP 24 бита.

Слева на первой и последней страницах (MUI_WELCOMEFINISHPAGE_BITMAP) — значок build/appicon.png (1024 px) и слово
«sosnina albums» (internal/decor/gen/art/brand/brand-albums-black.png, 1500 px, чёрные буквы перекрашены в INK,
красные остаются) на светлом фоне. MUI2 (AspectFitHeight) сжимает картинку в контрол без сглаживания, поэтому
рисуем по одной картинке в точный размер контрола на каждом масштабе экрана и подменяем её по DPI в installer.nsi
(WizardForDpi); высоты контрола измерены на снимках CI (спека 122, design §2):
  wizard-100.bmp 164x314, wizard-125.bmp 202x386, wizard-150.bmp 239x458, wizard-175.bmp 290x555;
  wizard.bmp 328x628 (2x) — для 200 % и выше и как запасная картинка MUI2.
Значок и надпись каждый раз берутся из исходников (LANCZOS), а не из 2x. Раскладка — в долях 1x.
header.bmp (MUI_HEADERIMAGE_BITMAP, справа в шапке внутренних страниц): 114x114 = 57x57 в 2x, значок на белом.
Нужен Pillow; результат лежит в git, чтобы сборка установщика не зависела от Pillow.
Запуск: python3 -I scripts/win/make_wizard_bmp.py [папка] — все картинки в неё (по умолчанию scripts/win).
"""

import os
import sys

from PIL import Image

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.normpath(os.path.join(HERE, "..", ".."))
W1, H1 = 164, 314  # размер 1x, который ждёт NSIS для страниц приветствия и завершения
# имя файла -> высота контрола на этом масштабе (ширина — по пропорции 1x)
WIZARDS = {
    "wizard-100.bmp": 314,
    "wizard-125.bmp": 386,
    "wizard-150.bmp": 458,
    "wizard-175.bmp": 555,
    "wizard.bmp": 628,
}
HEADER_SIZE = (57 * 2, 57 * 2)  # шапка внутренних страниц: высота 57 px при 100 %
HEADER_BG = (255, 255, 255)  # шапка NSIS белая
BG = (244, 244, 247)
INK = (33, 33, 36)  # цвет значка и тёмных букв
ICON_SRC = os.path.join(ROOT, "build", "appicon.png")
MARK_SRC = os.path.join(ROOT, "internal", "decor", "gen", "art", "brand", "brand-albums-black.png")


def wordmark_source(path):
    """Надпись с прозрачностью: тёмные пиксели — в INK, красные остаются."""
    img = Image.open(path).convert("RGBA")
    px = img.load()
    for y in range(img.height):
        for x in range(img.width):
            r, g, b, a = px[x, y]
            if max(r, g, b) < 100:
                px[x, y] = (*INK, a)
    return img


def build(height, icon_src, mark_src):
    """Картинка мастера высотой height: раскладка 1x, умноженная на height/314, из исходников."""
    s = height / H1
    width = round(W1 * s)
    canvas = Image.new("RGB", (width, height), BG)
    side = round(120 * s)
    icon = icon_src.resize((side, side), Image.LANCZOS)
    canvas.paste(icon, ((width - side) // 2, round(52 * s)), icon)
    mw = round(140 * s)
    mark = mark_src.resize((mw, round(mark_src.height * mw / mark_src.width)), Image.LANCZOS)
    canvas.paste(mark, ((width - mw) // 2, round(190 * s)), mark)
    return canvas


def build_header(icon_src):
    canvas = Image.new("RGB", HEADER_SIZE, HEADER_BG)
    side = 44 * 2  # поля вокруг значка: шапка не должна упираться краями
    icon = icon_src.resize((side, side), Image.LANCZOS)
    canvas.paste(icon, ((HEADER_SIZE[0] - side) // 2, (HEADER_SIZE[1] - side) // 2), icon)
    return canvas


def main(argv):
    out = argv[1] if len(argv) > 1 else HERE
    os.makedirs(out, exist_ok=True)
    icon_src = Image.open(ICON_SRC).convert("RGBA")
    mark_src = wordmark_source(MARK_SRC)
    for name, height in WIZARDS.items():
        build(height, icon_src, mark_src).save(os.path.join(out, name), format="BMP")
    build_header(icon_src).save(os.path.join(out, "header.bmp"), format="BMP")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
