"""Пиксели для macshots.py: Pillow (сравнение кадров) и PNG без Pillow."""

import os
import struct
import zlib

PIX_DELTA = 24  # разница яркости 0–255 ниже — шум сжатия, часы в строке меню
CMP_WIDTH = 480  # сравнение на уменьшенной копии: быстрее, курсор и сглаживание не мешают


def _pil():
    try:
        from PIL import Image, ImageChops  # noqa: PLC0415 — Pillow может не поставиться

        return Image, ImageChops
    except ImportError:
        return None, None


def changed_fraction(a, b):
    """Доля пикселей, где яркость отличается больше PIX_DELTA (на копии шириной CMP_WIDTH)."""
    _, chops = _pil()
    w = min(CMP_WIDTH, a.size[0])
    h = max(1, round(a.size[1] * w / a.size[0]))
    a2 = a.convert("L").resize((w, h))
    b2 = b.convert("L").resize((w, h))
    d = chops.difference(a2, b2).point(lambda v: 255 if v > PIX_DELTA else 0)
    return d.histogram()[255] / (w * h)


def is_uniform(img):
    """Снимок окна без права записи — одна заливка."""
    lo, hi = img.convert("L").getextrema()
    return hi - lo <= 2


def open_image(path):
    image, _ = _pil()
    if image is None or not os.path.isfile(path):
        return None
    try:
        im = image.open(path)
        im.load()
        return im
    except OSError:
        return None


def png_solid(width, height, rgb):
    """PNG одного цвета без Pillow: метка синхронизации для обоев рабочего стола."""

    def chunk(kind, data):
        body = kind + data
        return struct.pack(">I", len(data)) + body + struct.pack(">I", zlib.crc32(body) & 0xFFFFFFFF)

    row = b"\x00" + bytes(rgb) * width
    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress(row * height, 6))
        + chunk(b"IEND", b"")
    )
