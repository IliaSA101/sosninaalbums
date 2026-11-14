# Фикстуры internal/text/testdata (082, design §15): CFF-шрифт (OTF) из Montserrat,
# он же только с латиницей и «символьный» без букв. Запуск руками, результат коммитится:
#   python scripts/fonts-testdata.py internal/text/fonts/Montserrat-Regular.ttf internal/text/testdata
# Нужен fontTools (pip install fonttools). Montserrat — OFL без зарезервированного
# имени, поэтому имя гарнитуры своё («SA Test Cff»), лицензия — рядом (OFL-montserrat.txt).
import os
import sys

from fontTools import subset
from fontTools.fontBuilder import FontBuilder
from fontTools.pens.t2CharStringPen import T2CharStringPen
from fontTools.ttLib import TTFont


def build(src, dst, unicodes, family, ps_name):
    opts = subset.Options()
    opts.layout_features = ["kern"]
    opts.hinting = False
    f = TTFont(src)
    sub = subset.Subsetter(opts)
    sub.populate(unicodes=unicodes)
    sub.subset(f)
    gs, order = f.getGlyphSet(), f.getGlyphOrder()
    cs = {}
    for g in order:
        pen = T2CharStringPen(gs[g].width, gs)
        gs[g].draw(pen)
        cs[g] = pen.getCharString()
    fb = FontBuilder(f["head"].unitsPerEm, isTTF=False)
    fb.setupGlyphOrder(order)
    fb.setupCharacterMap(f.getBestCmap())
    fb.setupCFF(ps_name, {"FullName": family}, cs, {})
    fb.setupHorizontalMetrics({g: f["hmtx"][g] for g in order})
    fb.setupHorizontalHeader(ascent=f["hhea"].ascent, descent=f["hhea"].descent)
    fb.setupNameTable({"familyName": family, "styleName": "Regular"})
    o = f["OS/2"]
    fb.setupOS2(sTypoAscender=o.sTypoAscender, usWinAscent=o.usWinAscent,
                usWinDescent=o.usWinDescent, fsSelection=0x40)
    fb.setupPost()
    if "GPOS" in f:
        fb.font["GPOS"] = f["GPOS"]
    fb.save(dst)


def main():
    src, out = sys.argv[1], sys.argv[2]
    # латиница и кириллица А–я: кернинг и ширины — как у исходного TTF
    build(src, os.path.join(out, "sa-test-cff.otf"),
          list(range(0x20, 0x7F)) + list(range(0x410, 0x450)), "SA Test Cff", "SATestCff")
    # только латиница — шрифт без русских букв (NoCyr)
    build(src, os.path.join(out, "sa-test-latin.otf"),
          list(range(0x20, 0x7F)), "SA Test Latin", "SATestLatin")
    # только цифры и знаки — для отказа «нет букв»
    build(src, os.path.join(out, "sa-test-symbol.otf"),
          list(range(0x20, 0x41)), "SA Test Symbol", "SATestSymbol")


if __name__ == "__main__":
    main()
