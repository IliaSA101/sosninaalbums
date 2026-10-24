#!/usr/bin/env python3
"""Картинки мастера установщика (installer.nsi), BMP 24 бита, в 2x: NSIS при DPI 125-200 % сжимает их
в контрол (AspectFit), а не растягивает 1x — иначе логотип размыт.

wizard.bmp (MUI_WELCOMEFINISHPAGE_BITMAP): 328x628 = 164x314 в 2x. Значок build/appicon.png и слово
«sosnina albums» (frontend/src/assets/logo.png, белые буквы перекрашены в тёмные) на светлом фоне.
header.bmp (MUI_HEADERIMAGE_BITMAP, справа в шапке внутренних страниц): 114x114 = 57x57 в 2x, значок на белом.
Нужен Pillow; результат лежит в git, чтобы сборка установщика не зависела от Pillow.
Запуск: python3 -I scripts/win/make_wizard_bmp.py [wizard.bmp [header.bmp]].
"""

import os
import sys

from PIL import Image

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.normpath(os.path.join(HERE, "..", ".."))
K = 2  # во сколько раз крупнее 1x
SIZE = (164 * K, 314 * K)  # 164x314 — размер 1x, который ждёт NSIS для страниц приветствия и завершения
HEADER_SIZE = (57 * K, 57 * K)  # шапка внутренних страниц: высота 57 px при 100 %
HEADER_BG = (255, 255, 255)  # шапка NSIS белая
BG = (244, 244, 247)
INK = (33, 33, 36)  # цвет значка


def wordmark(path, width):
    """Логотип с прозрачностью; светлые (белые) пиксели — в тёмный, красные остаются."""
    img = Image.open(path).convert("RGBA")
    px = img.load()
    for y in range(img.height):
        for x in range(img.width):
            r, g, b, a = px[x, y]
            if min(r, g, b) > 150:
                px[x, y] = (*INK, a)
    return img.resize((width, round(img.height * width / img.width)), Image.LANCZOS)


def build():
    canvas = Image.new("RGB", SIZE, BG)
    side = 120 * K
    icon = Image.open(os.path.join(ROOT, "build", "appicon.png")).convert("RGBA").resize((side, side), Image.LANCZOS)
    canvas.paste(icon, ((SIZE[0] - side) // 2, 52 * K), icon)
    mark = wordmark(os.path.join(ROOT, "frontend", "src", "assets", "logo.png"), 140 * K)
    canvas.paste(mark, ((SIZE[0] - mark.width) // 2, 190 * K), mark)
    return canvas


def build_header():
    canvas = Image.new("RGB", HEADER_SIZE, HEADER_BG)
    side = 44 * K  # поля вокруг значка: шапка не должна упираться краями
    icon = Image.open(os.path.join(ROOT, "build", "appicon.png")).convert("RGBA").resize((side, side), Image.LANCZOS)
    canvas.paste(icon, ((HEADER_SIZE[0] - side) // 2, (HEADER_SIZE[1] - side) // 2), icon)
    return canvas


def main(argv):
    out = argv[1] if len(argv) > 1 else os.path.join(HERE, "wizard.bmp")
    head = argv[2] if len(argv) > 2 else os.path.join(HERE, "header.bmp")
    build().save(out, format="BMP")
    build_header().save(head, format="BMP")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
