"""Путь курсора для роликов README (102, design §4.2): чистая часть, без браузера.

`path(a, b, dur)` даёт точки с отметками времени: сглаживание ease-in-out (кубическое), лёгкая дуга поперёк пути,
первая и последняя точки — ровно a и b. Шаг между соседними точками не больше MAX_STEP CSS px (кадр 60 Гц): длинный
путь получает больше точек (то есть идёт дольше при том же числе кадров в секунду, а не прыгает).
Исполняет путь pilot.py: `page.mouse.move` на каждую точку и ожидание до её отметки (`steps=` Playwright не годится:
шаги идут без пауз, рывком).
"""

from __future__ import annotations

import math
from dataclasses import dataclass

FPS = 60  # частота точек пути
MAX_STEP = 40.0  # CSS px за кадр 60 Гц: больше — курсор «телепортируется»
BASE_DUR = 0.45  # с на путь любой длины
PER_1000PX = 0.35  # с на каждую тысячу px
ARC = 0.06  # смещение середины поперёк пути, доля длины (в design 4–8 %)
CLICK_REST = 0.12  # с покоя перед щелчком


@dataclass(frozen=True)
class Step:
    t: float  # секунды от начала пути
    x: float
    y: float


def ease(p: float) -> float:
    """Кубическое ease-in-out; ease(0) == 0 и ease(1) == 1 точно (концы не зависят от округления)."""
    if p <= 0.0:
        return 0.0
    if p >= 1.0:
        return 1.0
    if p < 0.5:
        return 4.0 * p * p * p
    q = -2.0 * p + 2.0
    return 1.0 - q * q * q / 2.0


def default_duration(dist: float) -> float:
    """0,45 с + 0,35 с на 1000 px (design §4.2)."""
    return BASE_DUR + PER_1000PX * max(0.0, dist) / 1000.0


def _points(a: tuple[float, float], b: tuple[float, float], n: int, arc: float) -> list[tuple[float, float]]:
    dx, dy = b[0] - a[0], b[1] - a[1]
    length = math.hypot(dx, dy)
    nx, ny = (-dy / length, dx / length) if length > 0 else (0.0, 0.0)  # нормаль к пути
    pts = [a]
    for i in range(1, n):
        p = i / n
        e = ease(p)
        off = arc * length * math.sin(math.pi * p)  # дуга: максимум в середине, на концах 0
        pts.append((a[0] + dx * e + nx * off, a[1] + dy * e + ny * off))
    pts.append(b)
    return pts


def path(
    a: tuple[float, float],
    b: tuple[float, float],
    dur: float | None = None,
    fps: int = FPS,
    arc: float = ARC,
    max_step: float = MAX_STEP,
) -> list[Step]:
    """Точки пути от a к b. dur None — по длине (default_duration).

    Число отрезков n = ceil(dur·fps), а если шаг всё равно длиннее max_step — больше: путь растягивается по времени.
    a == b — одна точка. Отрицательная длительность и fps ≤ 0 — ValueError.
    """
    if fps <= 0:
        raise ValueError("fps должен быть положительным")
    length = math.hypot(b[0] - a[0], b[1] - a[1])
    if length == 0:
        return [Step(0.0, float(a[0]), float(a[1]))]
    d = default_duration(length) if dur is None else dur
    if d < 0:
        raise ValueError("длительность не может быть отрицательной")
    n = max(2, math.ceil(d * fps))
    while True:
        pts = _points(a, b, n, arc)
        if max(math.hypot(q[0] - p[0], q[1] - p[1]) for p, q in zip(pts, pts[1:])) <= max_step:
            break
        n += 1 + n // 20  # растёт быстро, но без большого перелёта
    return [Step(i / fps, x, y) for i, (x, y) in enumerate(pts)]  # растянутый путь просто длиннее по времени


def drag_start_steps(a: tuple[float, float], px: float = 3.0, count: int = 3) -> list[tuple[float, float]]:
    """Три шага по 3 px после нажатия: порог переноса — 5 px (drag.md), курсор «подхватывает» фото."""
    return [(a[0] + px * (i + 1), a[1] + px * (i + 1) * 0.4) for i in range(count)]
