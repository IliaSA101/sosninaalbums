# -*- coding: utf-8 -*-
"""
Нанесение масок и выносок (рамки, стрелки, подписи) на сырой снимок.

Запуск (локально после правки .ann.json; результат коммитится):
    python -I scripts/guide/annotate.py docs/install/assets/raw
    python -I scripts/guide/annotate.py docs/install/assets/raw/win-01-smartscreen.png

Для <raw>/<имя>.png ищет рядом <raw>/<имя>.ann.json и пишет <assets>/annot/<имя>.png
(палитра 256 цветов, без текстовых блоков и EXIF). Схема .ann.json, координаты —
в пикселях ИСХОДНОГО снимка:
{
  "masks":    [ {"rect": [x0,y0,x1,y1], "color": "#808080"} ],
  "boxes":    [ {"rect": [30,150,1150,470], "width": 3} ],
  "arrows":   [ {"from": [470,205], "to": [300,360], "width": 3} ],
  "callouts": [ {"at": [360,130], "text": "Строка 1\\nСтрока 2", "align": "left", "size": 15} ],
  "targets":  [ {"rect": [692,395,832,421], "text": "Нажмите «Все равно открыть»", "side": "auto"} ],
  "keep_clear": [ [320,81,704,455] ]
}
Маски заливаются первыми, под рамками и стрелками: личные пути и имена на снимках
владельца закрываются до любой разметки. Цвет выносок по умолчанию — фирменный красный;
переопределяется полем "color".
targets — «видим такое окно → идём сюда»: рамка цели, подпись крупным шрифтом с белой обводкой
и стрелка от подписи к рамке. Размер шрифта и линий — от ширины снимка; сторона подписи
(side: left|right|above|below|auto): при auto — где фон ровнее и нет наложения на цели и другие
подписи. Рисуются последними, поверх остальной разметки. keep_clear — области (окно диалога),
на которые подпись по возможности не ложится. Координаты целей прогон пишет в shots.json
(target.rect), place_run.py переводит их в .ann.json.
"""
import argparse
import json
import math
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont, ImageStat

RED = "#d81f26"
WHITE = "#ffffff"
MASK = "#808080"

# Жирный шрифт с кириллицей по платформе; свой — через --font.
FONTS = {
    "win32": ["C:/Windows/Fonts/arialbd.ttf"],
    "darwin": ["/System/Library/Fonts/Supplemental/Arial Bold.ttf"],
    "linux": [
        "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
        "/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
        "/usr/share/fonts/TTF/DejaVuSans-Bold.ttf",
    ],
}


def find_font(custom: str = "") -> str:
    """Путь к шрифту: --font или первый существующий для платформы."""
    if custom:
        if not Path(custom).is_file():
            raise SystemExit(f"Шрифт не найден: {custom}")
        return custom
    for p in FONTS.get(sys.platform, FONTS["linux"]):
        if Path(p).is_file():
            return p
    raise SystemExit("Нет шрифта с кириллицей: задайте --font <путь к .ttf>")


def _arrow(draw, p0, p1, width, color):
    draw.line([tuple(p0), tuple(p1)], fill=color, width=width)
    ang = math.atan2(p1[1] - p0[1], p1[0] - p0[0])
    head = max(10, width * 4)
    for a in (ang - 0.4, ang + 0.4):
        draw.line(
            [tuple(p1), (p1[0] - head * math.cos(a), p1[1] - head * math.sin(a))],
            fill=color, width=width,
        )


# ── Цели: рамка + подпись + стрелка ──────────────────────────────────────
def target_metrics(width: int) -> dict:
    """Размеры разметки от ширины снимка: на 1920 px подпись крупная, на окне 384 px — не меньше 16."""
    size = max(16, min(48, round(width / 48)))
    return {"size": size, "line": max(3, round(width / 320)), "pad": max(4, round(width / 200))}


def wrap_text(text: str, font, max_w: int) -> str:
    """Переносит по словам так, чтобы строка не шире max_w; свои переводы строк сохраняются."""
    out = []
    for para in text.split("\n"):
        line = ""
        for word in para.split():
            trial = f"{line} {word}".strip()
            if line and font.getlength(trial) > max_w:
                out.append(line)
                line = word
            else:
                line = trial
        out.append(line)
    return "\n".join(out)


def _inter(a, b) -> bool:
    return a[0] < b[2] and b[0] < a[2] and a[1] < b[3] and b[1] < a[3]


def _busyness(img: Image.Image, r) -> float:
    """Разброс яркости под подписью: чем ровнее фон, тем лучше читается и меньше закрыто."""
    box = tuple(max(0, int(v)) for v in r)
    box = (box[0], box[1], min(img.width, box[2]), min(img.height, box[3]))
    if box[2] <= box[0] or box[3] <= box[1]:
        return 999.0
    return ImageStat.Stat(img.crop(box).convert("L")).stddev[0]


_SIDES = ("below", "above", "left", "right")


def place_label(img: Image.Image, rect, text_wh, gap: int, avoid, side: str = "auto"):
    """Прямоугольник подписи [x0,y0,x1,y1] рядом с целью. Возвращает (прямоугольник, сторона).

    Подпись целиком внутри снимка, не пересекает avoid (цели и чужие подписи). Из допустимых
    сторон берётся та, где фон ровнее; если места нет нигде — сторона с наименьшим сдвигом.
    """
    tw, th = text_wh
    cx, cy = (rect[0] + rect[2]) / 2, (rect[1] + rect[3]) / 2
    margin = 6
    cands = []
    for s in ([side] if side in _SIDES else _SIDES):
        if s == "below":
            x, y = cx - tw / 2, rect[3] + gap
        elif s == "above":
            x, y = cx - tw / 2, rect[1] - gap - th
        elif s == "left":
            x, y = rect[0] - gap - tw, cy - th / 2
        else:
            x, y = rect[2] + gap, cy - th / 2
        # наехала на область (окно диалога) — выталкиваем за её край в сторону подписи
        for _ in range(3):
            r = (x, y, x + tw, y + th)
            a = next((a for a in avoid if _inter(r, a)), None)
            if a is None:
                break
            if s == "below":
                y = a[3] + 2
            elif s == "above":
                y = a[1] - th - 2
            elif s == "left":
                x = a[0] - tw - 2
            else:
                x = a[2] + 2
        # сдвиг вдоль стороны допустим (подпись смещается от края), поперёк — нет
        x = min(max(x, margin), img.width - margin - tw)
        y = min(max(y, margin), img.height - margin - th)
        r = (x, y, x + tw, y + th)
        # сторона «держится», если подпись не наехала на цель после сдвига
        hit = any(_inter(r, a) for a in avoid)
        cands.append((hit, _busyness(img, r), s, r))
    cands.sort(key=lambda c: (c[0], c[1]))
    best = cands[0]
    return best[3], best[2]


def _edge_point(r, toward):
    """Ближайшая к точке toward точка границы прямоугольника r."""
    x = min(max(toward[0], r[0]), r[2])
    y = min(max(toward[1], r[1]), r[3])
    if r[0] < x < r[2] and r[1] < y < r[3]:  # точка внутри: прижать к ближайшей стороне
        d = {"l": x - r[0], "r": r[2] - x, "t": y - r[1], "b": r[3] - y}
        k = min(d, key=d.get)
        return {"l": (r[0], y), "r": (r[2], y), "t": (x, r[1]), "b": (x, r[3])}[k]
    return (x, y)


def draw_target(img: Image.Image, d: ImageDraw.ImageDraw, tgt: dict, font_path: str, avoid: list):
    """Рамка цели, подпись с обводкой и стрелка; прямоугольники подписи добавляются в avoid."""
    m = target_metrics(img.width)
    color = tgt.get("color", RED)
    r0 = tgt["rect"]
    pad = tgt.get("pad", m["pad"])
    box = (r0[0] - pad, r0[1] - pad, r0[2] + pad, r0[3] + pad)
    font = ImageFont.truetype(font_path, tgt.get("size", m["size"]))
    text = wrap_text(tgt["text"], font, int(img.width * 0.42))
    stroke = max(2, font.size // 8)
    l, t, r, b = d.multiline_textbbox((0, 0), text, font=font, spacing=font.size // 4, stroke_width=stroke)
    tw, th = r - l, b - t
    gap = max(int(m["size"] * 1.6), 30)
    lab, _ = place_label(img, box, (tw + 2 * stroke, th + 2 * stroke), gap, [box] + avoid, tgt.get("side", "auto"))
    # рамка цели: белая подложка под красной — виден на любом фоне
    d.rectangle(box, outline=WHITE, width=m["line"] + 2)
    d.rectangle(box, outline=color, width=m["line"])
    # стрелка от подписи к рамке: от ближайшей точки подписи к ближайшей точке рамки
    centre = ((box[0] + box[2]) / 2, (box[1] + box[3]) / 2)
    p0 = _edge_point(lab, centre)
    p1 = _edge_point(box, p0)
    _arrow(d, p0, p1, m["line"] + 3, WHITE)
    _arrow(d, p0, p1, m["line"], color)
    d.multiline_text((lab[0] + stroke - l, lab[1] + stroke - t), text, font=font, fill=color,
                     spacing=font.size // 4, stroke_width=stroke, stroke_fill=WHITE)
    avoid.append(tuple(lab))


def annotate_image(img: Image.Image, spec: dict, font_path: str) -> Image.Image:
    """Рисует маски, рамки, стрелки и подписи на копии RGB-снимка."""
    img = img.convert("RGB")
    d = ImageDraw.Draw(img)
    for m in spec.get("masks", []):
        d.rectangle(m["rect"], fill=m.get("color", MASK))
    for b in spec.get("boxes", []):
        d.rectangle(b["rect"], outline=b.get("color", RED), width=b.get("width", 3))
    for a in spec.get("arrows", []):
        _arrow(d, a["from"], a["to"], a.get("width", 3), a.get("color", RED))
    for c in spec.get("callouts", []):
        d.multiline_text(
            tuple(c["at"]), c["text"],
            font=ImageFont.truetype(font_path, c.get("size", 15)),
            fill=c.get("color", RED), spacing=c.get("spacing", 6),
            align=c.get("align", "left"),
        )
    # keep_clear — окно диалога: подпись ставится рядом, а не поверх его текста
    avoid = [tuple(t["rect"]) for t in spec.get("targets", [])] + [tuple(r) for r in spec.get("keep_clear", [])]
    for t in spec.get("targets", []):
        draw_target(img, d, t, font_path, avoid)
    return img


def save_clean(img: Image.Image, out: Path):
    """Палитра 256 цветов у UI-снимков вдвое легче RGB; без pnginfo и EXIF."""
    out.parent.mkdir(parents=True, exist_ok=True)
    pal = img.quantize(colors=256, method=Image.Quantize.FASTOCTREE, dither=Image.Dither.NONE)
    pal.info.clear()
    pal.save(out, "PNG", optimize=True)


def annotate_one(img_path: Path, font: str = "") -> bool:
    ann_path = img_path.parent / (img_path.stem + ".ann.json")
    if not ann_path.is_file():
        print(f"  [skip] {img_path.name}: нет {ann_path.name}")
        return False
    spec = json.loads(ann_path.read_text(encoding="utf-8"))
    # шрифт нужен только подписям: маскам и рамкам он не требуется
    font_path = find_font(font) if spec.get("callouts") or spec.get("targets") else ""
    with Image.open(img_path) as src:
        img = annotate_image(src, spec, font_path)
    out = img_path.parent.parent / "annot" / f"{img_path.stem}.png"
    save_clean(img, out)
    print(f"  [ok] {img_path.stem} -> annot/{out.name}")
    return True


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("path", help="файл снимка или папка raw")
    ap.add_argument("--font", default="", help="свой .ttf с кириллицей")
    args = ap.parse_args(argv)

    p = Path(args.path)
    if p.is_file():
        targets = [p]
    else:
        targets = sorted(list(p.glob("*.png")) + list(p.glob("*.jpg")))
    if not targets:
        raise SystemExit(f"Не найдено снимков в {p}")
    for t in targets:
        annotate_one(t, args.font)


if __name__ == "__main__":
    for _s in (sys.stdout, sys.stderr):
        _s.reconfigure(encoding="utf-8")
    main()
