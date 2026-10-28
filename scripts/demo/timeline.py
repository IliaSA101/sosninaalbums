"""Чистая часть сборки роликов (102, Д2): журнал кадров и меток, пересчёт в постоянную частоту, нарезка по меткам.

Без файлов, Pillow и GStreamer: на входе строки журналов и числа, на выходе номера кадров и длительности.
Контракт каталога прогона — docs/specs/0.23/102-readme-video/design.md §2.2.
"""

from __future__ import annotations

import bisect
import json
import statistics
from dataclasses import dataclass
from typing import Hashable, Sequence


class JournalError(ValueError):
    """Журнал кадров или меток не по контракту (Д1 → Д2)."""


@dataclass(frozen=True)
class Frame:
    i: int  # номер файла frames/<i:06d>.jpg
    t: float  # секунды эпохи (metadata.timestamp CDP)


@dataclass(frozen=True)
class Mark:
    t: float  # time.time() в момент метки
    name: str
    note: str = ""


def _lines(text: str, what: str):
    for n, raw in enumerate(text.splitlines(), 1):
        if not raw.strip():
            continue
        try:
            obj = json.loads(raw)
        except json.JSONDecodeError as e:
            raise JournalError(f"{what}, строка {n}: не JSON ({e.msg})") from None
        if not isinstance(obj, dict):
            raise JournalError(f"{what}, строка {n}: ожидался объект")
        yield n, obj


def _num(obj: dict, key: str, what: str, n: int) -> float:
    v = obj.get(key)
    if isinstance(v, bool) or not isinstance(v, (int, float)):
        raise JournalError(f"{what}, строка {n}: нет числа «{key}»")
    return float(v)


def parse_frames(text: str) -> list[Frame]:
    """frames.jsonl: {"i": 1, "t": 1729612345.123}; номера растут, время не убывает."""
    out: list[Frame] = []
    for n, o in _lines(text, "frames.jsonl"):
        i = _num(o, "i", "frames.jsonl", n)
        t = _num(o, "t", "frames.jsonl", n)
        if i != int(i) or i < 0:
            raise JournalError(f"frames.jsonl, строка {n}: «i» не номер кадра")
        if out and (int(i) <= out[-1].i or t < out[-1].t):
            raise JournalError(f"frames.jsonl, строка {n}: номер или время назад")
        out.append(Frame(int(i), t))
    if not out:
        raise JournalError("frames.jsonl пуст")
    return out


def parse_marks(text: str) -> list[Mark]:
    """marks.jsonl: первая метка start, последняя end, время строго растёт."""
    out: list[Mark] = []
    for n, o in _lines(text, "marks.jsonl"):
        t = _num(o, "t", "marks.jsonl", n)
        name = o.get("mark")
        if not isinstance(name, str) or not name:
            raise JournalError(f"marks.jsonl, строка {n}: нет имени «mark»")
        note = o.get("note", "")
        if not isinstance(note, str):
            raise JournalError(f"marks.jsonl, строка {n}: «note» не строка")
        if out and t <= out[-1].t:
            raise JournalError(f"marks.jsonl, строка {n}: метка «{name}» не позже предыдущей")
        out.append(Mark(t, name, note))
    if len(out) < 2 or out[0].name != "start" or out[-1].name != "end":
        raise JournalError("marks.jsonl: нужны метки start первой и end последней")
    return out


def mark_time(marks: Sequence[Mark], name: str, after: float | None = None) -> float:
    """Время первой метки name (позже after, если задано)."""
    for m in marks:
        if m.name == name and (after is None or m.t > after):
            return m.t
    raise JournalError(f"нет метки «{name}»")


def cut_by_marks(marks: Sequence[Mark], a: str, b: str) -> tuple[float, float]:
    """Отрезок от метки a до метки b: время начала и конца; b ищется позже a."""
    t0 = mark_time(marks, a)
    t1 = mark_time(marks, b, after=t0)
    return t0, t1


def out_count(t0: float, t1: float, fps: int, speed: float = 1.0) -> int:
    """Сколько кадров постоянной частоты вмещает отрезок (в медленном режиме время делится на speed)."""
    if t1 <= t0 or fps <= 0 or speed <= 0:
        raise JournalError("пустой отрезок записи")
    return max(1, round((t1 - t0) / speed * fps))


def resample(frames: Sequence[Frame], t0: float, t1: float, fps: int = 30, speed: float = 1.0) -> list[int]:
    """Номера исходных кадров на сетке t0 + k·speed/fps: берётся последний кадр с t ≤ момента.

    Screencast шлёт кадр только при перерисовке, в покое кадров нет — последний держится до следующего.
    Момент раньше первого кадра получает первый (страница уже показывала его).
    """
    ts = [f.t for f in frames]
    out: list[int] = []
    for k in range(out_count(t0, t1, fps, speed)):
        j = bisect.bisect_right(ts, t0 + k * speed / fps) - 1
        out.append(frames[max(j, 0)].i)
    return out


def kept_spans(t0: float, t1: float, cuts: Sequence[tuple[float, float]]) -> list[tuple[float, float]]:
    """Отрезки [t0, t1] без вырезок: cuts — пары (от, до), любой порядок; пересечения и выход за границы сводятся."""
    spans, cur = [], t0
    for a, b in sorted(c for c in cuts if c[1] > c[0]):
        a, b = max(a, t0), min(b, t1)
        if b <= a or b <= cur:
            continue
        if a > cur:
            spans.append((cur, a))
        cur = max(cur, b)
    if cur < t1:
        spans.append((cur, t1))
    return spans


def resample_cuts(frames: Sequence[Frame], t0: float, t1: float, cuts: Sequence[tuple[float, float]], fps: int = 30, speed: float = 1.0) -> list[int]:
    """resample для окна с вырезками: отрезки склеиваются встык, у каждого своя сетка (шов — жёсткая склейка кадров)."""
    out: list[int] = []
    for a, b in kept_spans(t0, t1, cuts):
        out += resample(frames, a, b, fps, speed)
    if not out:
        raise JournalError("вырезки съели всё окно записи")
    return out


def frame_at(frames: Sequence[Frame], t: float) -> int:
    """Номер кадра, видимого в момент t (последний с t ≤ момента; раньше первого — первый)."""
    j = bisect.bisect_right([f.t for f in frames], t) - 1
    return frames[max(j, 0)].i


def grid_ms(n: int, fps: int) -> list[int]:
    """Длительности n кадров сетки в мс с накопленным округлением: сумма точна (n·1000/fps)."""
    return [round((k + 1) * 1000 / fps) - round(k * 1000 / fps) for k in range(n)]


def dedupe(items: Sequence[tuple[Hashable, int]]) -> list[tuple[Hashable, int]]:
    """Подряд идущие одинаковые кадры (по ключу) сливаются, длительности складываются."""
    out: list[tuple[Hashable, int]] = []
    for key, ms in items:
        if out and out[-1][0] == key:
            out[-1] = (key, out[-1][1] + ms)
        else:
            out.append((key, ms))
    return out


def capture_fps(frames: Sequence[Frame], t0: float, t1: float) -> float:
    """Частота захвата по медиане интервалов между кадрами отрезка (0 — кадров меньше двух)."""
    ts = [f.t for f in frames if t0 <= f.t <= t1]
    gaps = [b - a for a, b in zip(ts, ts[1:]) if b > a]
    return 1 / statistics.median(gaps) if gaps else 0.0
