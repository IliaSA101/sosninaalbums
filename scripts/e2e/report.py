"""Отчёт e2e (089, design.md §6): метрики, пороги, report.json / report.md, разбор журнала, сравнение.

Чистая логика без браузера и без сети: её проверяет test_report.py. Командная строка здесь одна —
`python scripts/e2e/report.py --compare <старый> <новый>` (метрики, ставшие хуже в 1,5 раза и больше).
Код выхода отчёта: 0 — gate зелёные; 1 — провал gate; 2 — отказ (нет стенда, набора, неверные аргументы).
"""

from __future__ import annotations

import argparse
import json
import math
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

GATE, TARGET, INFO = "gate", "target", "info"
KINDS = (GATE, TARGET, INFO)
EXIT_OK, EXIT_FAIL, EXIT_REFUSE = 0, 1, 2
WORSE_RATIO = 1.5  # во сколько раз хуже — помечается при сравнении

# ---- Метрики ---------------------------------------------------------------------------------


@dataclass
class Metric:
    id: str
    value: float | None
    kind: str = INFO
    limit: float | None = None  # действующий порог (уже с SA_TEST_SLOW для времён)
    ok: bool | None = None  # None — не измерено или без порога
    note: str = ""
    unit: str = ""

    def as_json(self) -> dict:
        return {"id": self.id, "value": self.value, "limit": self.limit, "kind": self.kind, "ok": self.ok, "note": self.note}


@dataclass
class Report:
    level: str  # go | stand | scale
    started: str
    quick: bool = False
    count: int = 0
    slow: float = 1.0
    machine: dict = field(default_factory=dict)
    metrics: list[Metric] = field(default_factory=list)

    def add(
        self,
        id: str,
        value: float | int | None,
        *,
        kind: str = INFO,
        limit: float | None = None,
        time: bool = False,
        note: str = "",
        unit: str = "",
    ) -> Metric:
        """Добавить метрику. time=True — порог в миллисекундах, умножается на SA_TEST_SLOW (как testkit.Limit)."""
        if kind not in KINDS:
            raise ValueError(f"вид метрики {kind!r}: ожидается gate, target или info")
        if kind != INFO and limit is None:
            raise ValueError(f"метрика {id}: у вида {kind} нужен порог")
        eff = None if limit is None else limit * (self.slow if time else 1)
        m = Metric(id, None if value is None else _num(value), kind, eff, None, note, unit)
        if m.value is not None and eff is not None and kind != INFO:
            m.ok = m.value <= eff
            if not m.ok and kind == TARGET:
                m.note = (note + "; " if note else "") + "цель не достигнута"
        elif m.value is None and not m.note:
            m.note = "не измерено"
        self.metrics.append(m)
        return m

    def get(self, id: str) -> Metric | None:
        return next((m for m in self.metrics if m.id == id), None)

    def failed(self) -> list[Metric]:
        """Только gate: target и info провалом не бывают."""
        return [m for m in self.metrics if m.kind == GATE and m.ok is False]

    def exit_code(self) -> int:
        return EXIT_FAIL if self.failed() else EXIT_OK

    def as_json(self) -> dict:
        return {
            "level": self.level,
            "started": self.started,
            "quick": self.quick,
            "count": self.count,
            "slow": self.slow,
            "machine": self.machine,
            "metrics": [m.as_json() for m in self.metrics],
        }


def _num(v: float | int) -> float | int:
    """Целые остаются целыми, дробные — до 0,01: отчёт читает человек."""
    if isinstance(v, bool):
        return int(v)
    if isinstance(v, int):
        return v
    if math.isnan(v) or math.isinf(v):
        return 0
    return round(v, 2)


def _fmt(v: float | int | None) -> str:
    if v is None:
        return "—"
    return str(int(v)) if float(v).is_integer() else f"{v:.2f}".rstrip("0").rstrip(".")


def to_md(r: Report) -> str:
    """report.md: таблица «метрика / значение / порог / вид / итог» и сводка."""
    rows = ["| метрика | значение | порог | вид | итог |", "| --- | --- | --- | --- | --- |"]
    for m in r.metrics:
        lim = "—" if m.limit is None else "≤ " + _fmt(m.limit)
        if m.ok is True:
            res = "OK"
        elif m.ok is False:
            res = "ПРОВАЛ" if m.kind == GATE else "цель не достигнута"
        else:
            res = "—"
        val = _fmt(m.value) + (f" {m.unit}" if m.unit and m.value is not None else "")
        note = f" ({m.note})" if m.note else ""
        rows.append(f"| {m.id} | {val} | {lim} | {m.kind} | {res}{note} |")
    bad = r.failed()
    head = [
        f"# e2e: {r.level}",
        "",
        f"Начало {r.started}, фото {r.count}, {'быстрый' if r.quick else 'полный'} прогон, SA_TEST_SLOW {_fmt(r.slow)}.",
    ]
    if r.machine:
        head.append("Машина: " + ", ".join(f"{k} {v}" for k, v in r.machine.items()) + ".")
    tail = [
        "",
        f"Итог: {'ПРОВАЛ, gate нарушено: ' + str(len(bad)) + ' (' + ', '.join(m.id for m in bad) + ')' if bad else 'gate зелёные'}.",
    ]
    return "\n".join(head + [""] + rows + tail) + "\n"


def write_report(r: Report, out: Path) -> None:
    out.mkdir(parents=True, exist_ok=True)
    (out / "report.json").write_text(json.dumps(r.as_json(), ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    (out / "report.md").write_text(to_md(r), encoding="utf-8")


def load_report(path: Path) -> dict:
    """Читает report.json по пути к файлу или к папке прогона."""
    p = path / "report.json" if path.is_dir() else path
    data = json.loads(p.read_text(encoding="utf-8"))
    if not isinstance(data, dict) or not isinstance(data.get("metrics"), list):
        raise ValueError(f"{p.name}: не отчёт e2e (нет metrics)")
    return data


# ---- Журнал log.txt (контракт G ↔ U, design.md §8 п. 2) -----------------------------------

_DUR = re.compile(r"(\d+(?:\.\d+)?)(ns|µs|μs|us|ms|s|m|h)")
_UNIT_MS = {"ns": 1e-6, "µs": 1e-3, "μs": 1e-3, "us": 1e-3, "ms": 1.0, "s": 1000.0, "m": 60000.0, "h": 3600000.0}


def parse_duration(s: str) -> float:
    """Длительность в формате Go time.Duration («1m3.5s», «850ms», «0s») → миллисекунды."""
    s = s.strip().rstrip(",)")
    pos, total = 0, 0.0
    if s == "0":
        return 0.0
    while pos < len(s):
        m = _DUR.match(s, pos)
        if not m:
            raise ValueError(f"не длительность Go: {s!r}")
        total += float(m.group(1)) * _UNIT_MS[m.group(2)]
        pos = m.end()
    if pos == 0:
        raise ValueError("пустая длительность")
    return total


LOG_PATTERNS = {
    "folders": re.compile(r"папок \d+: (\d+) фото \(обход (\S+), чтение (\S+?)[,)]"),
    # Хвост «, снято запросов M» (090 G2) — только когда клиент снимал запросы окна.
    "thumbs": re.compile(r"миниатюры: (\d+) за ([^\s,]+)(?:, снято запросов (\d+))?"),
    "memory": re.compile(r"память: (.+?) — Go (\d+) МБ, куча (\d+) МБ"),
    "layouts": re.compile(r"прогрев раскладок: (\d+) разворотов за (\S+)"),
    "autobuild": re.compile(r"автосборка: (\d+) фото → (\d+) разворотов за (\S+)"),
    "export": re.compile(r"экспорт: (\d+) разворотов за (\S+)"),
}
BAD_WORDS = ("паника", "ошибка", "не удалось")


def parse_log(text: str) -> dict[str, list[dict]]:
    """Строки журнала → события по видам. Длительности — миллисекунды (ключи *_ms)."""
    out: dict[str, list[dict]] = {k: [] for k in LOG_PATTERNS}
    for line in text.splitlines():
        for kind, rx in LOG_PATTERNS.items():
            m = rx.search(line)
            if not m:
                continue
            g = m.groups()
            if kind == "folders":
                ev = {"photos": int(g[0]), "walk_ms": parse_duration(g[1]), "read_ms": parse_duration(g[2])}
            elif kind == "thumbs":
                ev = {"photos": int(g[0]), "ms": parse_duration(g[1]), "cancelled": int(g[2] or 0)}
            elif kind == "memory":
                ev = {"stage": g[0], "go_mb": int(g[1]), "heap_mb": int(g[2])}
            elif kind == "layouts":
                ev = {"spreads": int(g[0]), "ms": parse_duration(g[1])}
            elif kind == "autobuild":
                ev = {"photos": int(g[0]), "spreads": int(g[1]), "ms": parse_duration(g[2])}
            else:
                ev = {"spreads": int(g[0]), "ms": parse_duration(g[1])}
            out[kind].append(ev)
            break
    return out


def bad_lines(text: str) -> list[str]:
    """Строки с «ПАНИКА», «ошибка», «не удалось» (без учёта регистра) — для метрики log_bad."""
    return [ln for ln in text.splitlines() if any(w in ln.casefold() for w in BAD_WORDS)]


# ---- Сравнение двух прогонов ---------------------------------------------------------------


def compare(old: dict, new: dict, ratio: float = WORSE_RATIO) -> list[dict]:
    """Метрики, у которых новое значение хуже старого в ratio раз и больше (меньше — лучше везде).

    Метрики без числа с одной из сторон и с нулём в старом прогоне пропускаются: делить не на что.
    """
    prev = {m["id"]: m for m in old["metrics"]}
    res = []
    for m in new["metrics"]:
        o = prev.get(m["id"])
        a, b = (o or {}).get("value"), m.get("value")
        if not isinstance(a, (int, float)) or not isinstance(b, (int, float)) or a <= 0:
            continue
        if b / a >= ratio:
            res.append({"id": m["id"], "old": a, "new": b, "ratio": round(b / a, 2), "kind": m.get("kind", INFO)})
    return res


def compare_text(old: dict, new: dict) -> str:
    worse = compare(old, new)
    lines = [f"Сравнение: {old.get('started', '?')} → {new.get('started', '?')} ({old.get('level', '?')} / {new.get('level', '?')})"]
    if old.get("level") != new.get("level"):
        lines.append("Уровни разные: сравнение приблизительное.")
    if not worse:
        lines.append(f"Хуже в {_fmt(WORSE_RATIO)} раза и больше — нет.")
    for w in worse:
        lines.append(f"  хуже  {w['id']:<28} {_fmt(w['old']):>10} → {_fmt(w['new']):<10} ×{_fmt(w['ratio'])} ({w['kind']})")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(prog="report.py", description="Сравнение отчётов e2e (089).")
    ap.add_argument("--compare", nargs=2, metavar=("СТАРЫЙ", "НОВЫЙ"), required=True, help="папки прогонов или report.json")
    args = ap.parse_args(argv)
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8")  # консоль Windows (cp1251) не знает «→»
        except (AttributeError, ValueError):
            pass
    try:
        old, new = (load_report(Path(p)) for p in args.compare)
    except (OSError, ValueError) as e:
        print(f"e2e: {e}", file=sys.stderr)
        return EXIT_REFUSE
    print(compare_text(old, new))
    return EXIT_OK  # помеченное «хуже» — для глаз, не провал (design.md §6)


if __name__ == "__main__":
    sys.exit(main())
