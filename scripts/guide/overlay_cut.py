# -*- coding: utf-8 -*-
"""
Ускоряемые зоны ролика-инструкции (spec 036): неявный простой, длинные move и type.

Чистая логика над временем записи, без Journal: overlay_plan.py вызывает её из flush.
Раннер не всегда пишет idle там, где ждёт (установка, прокрутка с OCR), и живое видео
получается в разы длиннее цели 45-95 с — эти зоны режутся здесь.
"""

SETTLE_KEEP = 1.2  # после действия простой показывается живьём столько секунд, остальное ускоряется
MOVE_CAP = 2.5  # длинный move (курсор, прокрутка с OCR) ужимается до этой длины
TYPE_CAP = 3.0  # то же для набора текста


def _merge(iv: list) -> list:
    out = []
    for a, c in sorted(iv):
        if out and a <= out[-1][1]:
            out[-1][1] = max(out[-1][1], c)
        else:
            out.append([a, c])
    return out


def fast_zones(events: list, off: float, lo: float, hi: float, windows: list, idle_cap: float) -> list:
    """Зоны [lo, hi] записи, которые идут ускоренно: (с, по, длина в ролике).

    Длинные move/type ужимаются до MOVE_CAP/TYPE_CAP (зона, разрезанная стоп-кадром, получает
    долю длины). Промежуток без move/keys/type (с окнами плашек) и без стоп-кадров длиннее
    SETTLE_KEEP + idle_cap: первые SETTLE_KEEP живьём, остальное — idle_cap. Метки note
    промежуток не разрывают: они не занимают время.
    """
    zones, busy = [], []
    for e in events:
        if e["ev"] not in ("move", "type", "keys"):
            continue
        s0, s1 = e["t0"] + off, e["t1"] + off
        a, c = max(s0, lo), min(s1, hi)
        if c <= a:
            continue
        busy.append((a, c))
        cap = {"move": MOVE_CAP, "type": TYPE_CAP}.get(e["ev"])
        if cap and s1 - s0 > cap:
            zones.append((a, c, cap * (c - a) / (s1 - s0)))
    zones.sort()
    kept = []
    for z in zones:
        if not kept or z[0] >= kept[-1][1]:  # пересекающиеся (не бывает в журнале) — берём первую
            kept.append(z)
    zones = kept
    busy += [(max(w0, lo), min(w1, hi)) for w0, w1 in windows if min(w1, hi) > max(w0, lo)]
    pos = lo
    for a, c in _merge(busy) + [[hi, hi]]:
        if a - pos > SETTLE_KEEP + idle_cap + 1e-6:  # допуск: 1,8 с — ровно предел, а не больше
            zones.append((pos + SETTLE_KEEP, a, idle_cap))
        pos = max(pos, c)
    return sorted(zones)


def merge_fast(items: list) -> list:
    """Подряд идущие ускоренные отрезки (src1 == src0 следующего, те же плашки) — в один.

    items — словари {kind, src0, src1, dur, kw}; длина слитого — большая из длин: ожидание
    подряд не растёт (неявный простой + idle раннера дают один отрезок, а не два).
    """
    out = []
    for it in items:
        p = out[-1] if out else None
        if (p and it["kind"] == "fast" and p["kind"] == "fast" and abs(p["src1"] - it["src0"]) < 1e-6
                and p["kw"].get("plates") == it["kw"].get("plates")):
            p["src1"] = it["src1"]
            p["dur"] = max(p["dur"], it["dur"])
        else:
            out.append(dict(it))
    return out
