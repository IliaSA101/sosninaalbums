# -*- coding: utf-8 -*-
"""
Рисование кадров ролика-инструкции (spec 036, design §3.3): плашки «Шаг N из M» и клавиш,
стоп-кадры со стрелкой (annotate.draw_target), заставки глав, серые маски.

Расчёт, что и где (план), — в overlay_plan.py; здесь только Pillow. Красный и шрифт
подписей те же, что у рисунков docx (annotate), чтобы ролик и инструкция выглядели одинаково.
"""
from functools import lru_cache
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

import annotate
from overlay_plan import Metrics

STEP_FILL = (216, 31, 38, 235)  # annotate.RED
KEYS_FILL = (20, 20, 20, 220)
CARD_BG = (22, 24, 28)
MASK_GRAY = (128, 128, 128)

# Шрифт символов клавиш: в Arial нет «⌘» (U+2318). Порядок из design §3.3, свой — --font-sym.
SYM_FONTS = (
    "C:/Windows/Fonts/seguisym.ttf",  # Segoe UI Symbol
    "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
    "/usr/share/fonts/dejavu/DejaVuSans.ttf",
    "/usr/share/fonts/TTF/DejaVuSans.ttf",
    "/System/Library/Fonts/Apple Symbols.ttf",
)


def find_sym_font(custom: str = "") -> str:
    """Путь к шрифту символов или пустая строка (тогда рисуется основным шрифтом)."""
    if custom:
        if not Path(custom).is_file():
            raise SystemExit(f"Шрифт не найден: {custom}")
        return custom
    return next((p for p in SYM_FONTS if Path(p).is_file()), "")


@lru_cache(maxsize=32)
def load_font(path: str, size: int) -> ImageFont.FreeTypeFont:
    return ImageFont.truetype(path, size)


def _ink(font, ch: str) -> bytes:
    im = Image.new("L", (int(font.size) * 3, int(font.size) * 3))
    ImageDraw.Draw(im).text((font.size // 2, font.size // 2), ch, font=font, fill=255)
    return im.tobytes()


def has_glyph(font, ch: str) -> bool:
    """Есть ли в шрифте глиф: рисунок символа отличается от рисунка «пустого» U+FFFF."""
    return ch.isspace() or _ink(font, ch) != _ink(font, "\uffff")


def glyph_runs(text: str, fonts: list) -> list:
    """Текст -> [(шрифт, строка)]: каждый знак — первым шрифтом, где он есть."""
    runs = []
    for ch in text:
        f = next((f for f in fonts if has_glyph(f, ch)), fonts[0])
        if runs and runs[-1][0] is f:
            runs[-1] = (f, runs[-1][1] + ch)
        else:
            runs.append((f, ch))
    return runs


def runs_width(runs: list) -> float:
    return sum(f.getlength(s) for f, s in runs)


def draw_runs(d: ImageDraw.ImageDraw, x: float, y_mid: float, runs: list, fill):
    for f, s in runs:
        d.text((x, y_mid), s, font=f, fill=fill, anchor="lm")
        x += f.getlength(s)


def parse_mask(s: str) -> tuple:
    """«x,y,w,h» (пиксели кадра) -> (x0, y0, x1, y1)."""
    try:
        x, y, w, h = (int(v) for v in s.split(","))
    except ValueError:
        raise SystemExit(f"--mask: нужно x,y,w,h (целые), получено {s!r}") from None
    if w <= 0 or h <= 0:
        raise SystemExit(f"--mask: ширина и высота должны быть больше 0: {s!r}")
    return (x, y, x + w, y + h)


def apply_masks(img: Image.Image, masks: list) -> Image.Image:
    if not masks:
        return img
    img = img.convert("RGB")
    d = ImageDraw.Draw(img)
    for r in masks:
        d.rectangle(r, fill=MASK_GRAY)
    return img


class Painter:
    def __init__(self, out_w: int, out_h: int, font: str = "", font_sym: str = ""):
        self.m = Metrics(out_w, out_h)
        self.font_path = annotate.find_font(font)
        self.sym_path = find_sym_font(font_sym)

    def _fonts(self, kind: str) -> list:
        size = self.m.font_size(kind)
        main = load_font(self.font_path, size)
        if kind == "keys" and self.sym_path:
            return [main, load_font(self.sym_path, size)]  # символы — запасной шрифт: буквы одного вида
        return [main]

    def plates_layer(self, plates) -> tuple:
        """Прозрачный слой кадра с плашками и их прямоугольники (для keep_clear подписи)."""
        layer = Image.new("RGBA", (self.m.out_w, self.m.out_h), (0, 0, 0, 0))
        d = ImageDraw.Draw(layer)
        rects = []
        for p in plates:
            runs = glyph_runs(p.text, self._fonts(p.kind))
            pad = self.m.pad(p.kind)
            w, h = round(runs_width(runs)) + 2 * pad, self.m.plate_h(p.kind)
            r = self.m.plate_rect(p.anchor, p.kind, w, h)
            d.rounded_rectangle(r, radius=pad, fill=STEP_FILL if p.kind == "step" else KEYS_FILL)
            draw_runs(d, r[0] + pad, (r[1] + r[3]) / 2, runs, "white")
            rects.append(r)
        return layer, rects

    def hold_frame(self, base: Image.Image, seg, masks: list, layers: bool = True) -> Image.Image:
        """Стоп-кадр: кадр записи + маски + плашки + рамка/стрелка/подпись цели."""
        img = apply_masks(base.convert("RGB"), masks)
        if not layers:
            return img
        layer, rects = self.plates_layer(seg.plates)
        img = Image.alpha_composite(img.convert("RGBA"), layer).convert("RGB")
        d = ImageDraw.Draw(img)
        t = seg.target
        if t.rect:
            avoid = [tuple(r) for r in rects] + [tuple(r) for r in t.keep]
            if self.needs_sym(t.caption):
                self._target_mixed(img, d, t.rect, t.caption, avoid)
            else:  # обычная подпись — тот же draw_target, что у рисунков docx
                annotate.draw_target(img, d, {"rect": list(t.rect), "text": t.caption}, self.font_path, avoid)
        else:
            self._bottom_caption(d, t.caption)
        return img

    def needs_sym(self, text: str) -> bool:
        """В основном шрифте нет знака подписи (например «⌘»), а шрифт символов есть."""
        if not self.sym_path:
            return False
        main = load_font(self.font_path, annotate.target_metrics(self.m.out_w)["size"])
        return not all(has_glyph(main, ch) for ch in text)

    def caption_fonts(self) -> list:
        size = annotate.target_metrics(self.m.out_w)["size"]
        return [load_font(self.font_path, size)] + ([load_font(self.sym_path, size)] if self.sym_path else [])

    @staticmethod
    def _wrap_runs(text: str, fonts: list, max_w: float) -> list:
        """Перенос по словам с учётом ширины знаков из разных шрифтов."""
        lines, cur = [], ""
        for word in text.split():
            trial = f"{cur} {word}".strip()
            if cur and runs_width(glyph_runs(trial, fonts)) > max_w:
                lines.append(cur)
                cur = word
            else:
                cur = trial
        return lines + [cur]

    @staticmethod
    def _draw_lines(d, x0: float, y0: float, lines: list, fonts: list, lh: int, stroke: int, center_w: float = 0):
        """Строки красным с белой обводкой; center_w > 0 — центрировать в этой ширине от x0."""
        for ln in lines:
            runs = glyph_runs(ln, fonts)
            x = x0 + (center_w - runs_width(runs)) / 2 if center_w else x0
            for f, s in runs:
                d.text((x, y0 + lh / 2), s, font=f, fill=annotate.RED, anchor="lm",
                       stroke_width=stroke, stroke_fill=annotate.WHITE)
                x += f.getlength(s)
            y0 += lh

    def _target_mixed(self, img: Image.Image, d: ImageDraw.ImageDraw, rect, text: str, avoid: list):
        """Рамка, стрелка и подпись цели, как annotate.draw_target, но подпись со сменой шрифта.

        Нужна, когда в подписи есть «⌘»: draw_target рисует одним шрифтом и дал бы «квадратик».
        Размеры, цвета, поиск места и стрелка — те же функции annotate, чтобы вид не расходился.
        """
        m = annotate.target_metrics(img.width)
        pad = m["pad"]
        box = (rect[0] - pad, rect[1] - pad, rect[2] + pad, rect[3] + pad)
        fonts = self.caption_fonts()
        lines = self._wrap_runs(text, fonts, int(img.width * 0.42))
        stroke, lh = max(2, m["size"] // 8), round(m["size"] * 1.2) + m["size"] // 4
        tw = max(runs_width(glyph_runs(ln, fonts)) for ln in lines) + 2 * stroke
        th = lh * len(lines) + 2 * stroke
        gap = max(int(m["size"] * 1.6), 30)
        lab, _ = annotate.place_label(img, box, (tw, th), gap, [box] + avoid)
        d.rectangle(box, outline=annotate.WHITE, width=m["line"] + 2)
        d.rectangle(box, outline=annotate.RED, width=m["line"])
        centre = ((box[0] + box[2]) / 2, (box[1] + box[3]) / 2)
        p0 = annotate._edge_point(lab, centre)
        p1 = annotate._edge_point(box, p0)
        annotate._arrow(d, p0, p1, m["line"] + 3, annotate.WHITE)
        annotate._arrow(d, p0, p1, m["line"], annotate.RED)
        self._draw_lines(d, lab[0] + stroke, lab[1] + stroke, lines, fonts, lh, stroke)
        avoid.append(tuple(lab))

    def _bottom_caption(self, d: ImageDraw.ImageDraw, text: str):
        """Подпись без цели — внизу по центру, красная с белой обводкой, как у draw_target.

        Рисуется по знакам со сменой шрифта: в подписи бывает «⌘», которого нет в Arial.
        """
        fonts = self.caption_fonts()
        lines = self._wrap_runs(text, fonts, self.m.out_w * 0.9)
        size = fonts[0].size
        lh, stroke = round(size * 1.3), max(2, size // 8)
        y = self.m.out_h - self.m.safe_bottom - lh * len(lines) - stroke  # над Dock и панелью задач
        self._draw_lines(d, 0, y, lines, fonts, lh, stroke, center_w=self.m.out_w)

    def card(self, title: str) -> Image.Image:
        """Заставка главы: тёмный фон, заголовок по центру, красная черта."""
        img = Image.new("RGB", (self.m.out_w, self.m.out_h), CARD_BG)
        d = ImageDraw.Draw(img)
        font = load_font(self.font_path, max(16, round(self.m.out_w / 24)))
        text = annotate.wrap_text(title, font, int(self.m.out_w * 0.8))
        spacing = font.size // 4
        l, t, r, b = d.multiline_textbbox((0, 0), text, font=font, spacing=spacing)
        x, y = (self.m.out_w - (r - l)) / 2 - l, (self.m.out_h - (b - t)) / 2 - t
        d.multiline_text((x, y), text, font=font, fill="white", spacing=spacing, align="center")
        by = (self.m.out_h + (b - t)) / 2 + self.m.margin
        d.rectangle((self.m.out_w * 0.4, by, self.m.out_w * 0.6, by + max(3, self.m.out_h // 270)), fill=annotate.RED)
        return img
