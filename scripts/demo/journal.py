"""Журнал меток записи (102, design §2.2): `<сцена>/marks.jsonl` — контракт записи (Д1) и сборки (Д2).

Строка — JSON-объект `{"t": <time.time()>, "mark": "drop", "note": "5 фото"}`; первая метка `start`, последняя `end`,
`t` строго растёт. Время — те же часы эпохи, что `metadata.timestamp` кадров CDP (стенд и браузер на одной машине).
Чистая часть: разбор и проверка строк (без браузера), запись — `Journal`.
"""

from __future__ import annotations

import json
import time
from dataclasses import dataclass
from pathlib import Path
from typing import TextIO


class JournalError(ValueError):
    """Строка или журнал не соответствуют контракту."""


@dataclass(frozen=True)
class Mark:
    t: float
    mark: str
    note: str = ""


def parse_line(line: str) -> Mark:
    """Одна строка журнала; любое отклонение от контракта — JournalError."""
    try:
        obj = json.loads(line)
    except json.JSONDecodeError as e:
        raise JournalError(f"не JSON: {e.msg}") from None
    if not isinstance(obj, dict):
        raise JournalError("строка — не объект")
    t, name, note = obj.get("t"), obj.get("mark"), obj.get("note", "")
    if isinstance(t, bool) or not isinstance(t, (int, float)) or t != t or t in (float("inf"), float("-inf")) or t <= 0:
        raise JournalError("t — положительное конечное число")
    if not isinstance(name, str) or not name.strip():
        raise JournalError("mark — непустая строка")
    if not isinstance(note, str):
        raise JournalError("note — строка")
    return Mark(float(t), name, note)


def parse_marks(text: str) -> list[Mark]:
    """Весь журнал: пустые строки пропускаются, номер строки — в тексте ошибки. Правила §2.2 проверяет `validate`."""
    marks: list[Mark] = []
    for n, line in enumerate(text.splitlines(), 1):
        if not line.strip():
            continue
        try:
            marks.append(parse_line(line))
        except JournalError as e:
            raise JournalError(f"строка {n}: {e}") from None
    validate(marks)
    return marks


def validate(marks: list[Mark]) -> None:
    """Не пусто, первая — start, последняя — end, время строго растёт."""
    if not marks:
        raise JournalError("журнал пуст")
    if marks[0].mark != "start":
        raise JournalError(f"первая метка {marks[0].mark!r}, нужна start")
    if marks[-1].mark != "end":
        raise JournalError(f"последняя метка {marks[-1].mark!r}, нужна end")
    for a, b in zip(marks, marks[1:]):
        if not b.t > a.t:
            raise JournalError(f"метка {b.mark!r}: время не растёт ({b.t} ≤ {a.t})")


def load(path: Path) -> list[Mark]:
    return parse_marks(path.read_text(encoding="utf-8"))


class Journal:
    """Запись меток построчно; каждая строка сразу на диск (сбой сцены не теряет журнал)."""

    def __init__(self, path: Path, clock=time.time):
        self.path, self._clock = path, clock
        self.marks: list[Mark] = []
        self._f: TextIO | None = path.open("w", encoding="utf-8")

    def mark(self, name: str, note: str = "") -> Mark:
        if self._f is None:
            raise JournalError("журнал закрыт")
        t = self._clock()
        if self.marks and not t > self.marks[-1].t:
            t = self.marks[-1].t + 1e-6  # часы не шагнули: строгий рост важнее точности в микросекунду
        m = Mark(t, name, note)
        self.marks.append(m)
        self._f.write(json.dumps({"t": m.t, "mark": m.mark, "note": m.note}, ensure_ascii=False) + "\n")
        self._f.flush()
        return m

    def close(self) -> None:
        if self._f is not None:
            self._f.close()
            self._f = None
