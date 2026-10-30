# -*- coding: utf-8 -*-
"""
Журнал длиннее записи и снимки шагов (spec 036): чистая логика над планом, без ffmpeg и Pillow.

Раннер Windows пишет кадры по таймеру, и запись бывает короче журнала (секунды, а то и десятки
секунд хвоста). Хвост до FREEZE_MAX держится последним кадром; длиннее — живое за концом
отбрасывается, а стоп-кадры берутся из PNG-снимков шага (shots.json артефакта), если снимок
нашёлся. Шаг, от которого за концом записи ничего не осталось, — отказ: ролик потерял бы шаг.

Те же снимки подсказывают рамку цели стоп-кадра, у которого раннер её не записал (rect null):
метка кнопки на снимке названа в подписи в кавычках («Все равно открыть»). Подтверждает рамку
Pillow в overlay.py (кусок кадра совпадает с куском снимка), здесь — только выбор кандидата.
"""
import re
from pathlib import Path

from overlay_plan import _Builder, step_text

FREEZE_MAX = 1.0  # хвост журнала за концом записи, который прощается заморозкой последнего кадра, с


class RecordShort(Exception):
    """За концом записи теряется шаг, для которого нет снимка."""


def parse_shots(data, base: Path) -> list:
    """shots.json -> [{name, path, label, rect}] только годных снимков с существующим файлом."""
    out = []
    if not isinstance(data, list):
        return out
    for s in data:
        if not isinstance(s, dict) or not s.get("ok") or not isinstance(s.get("file"), str):
            continue
        path = Path(base) / s["file"]
        if not path.is_file():
            continue
        t = s.get("target") or {}
        rect = t.get("rect")
        ok_rect = isinstance(rect, list) and len(rect) == 4 and all(isinstance(v, (int, float)) for v in rect)
        out.append({"name": s.get("name", ""), "path": str(path), "label": t.get("label") or "",
                    "rect": rect if ok_rect else None})
    return out


def norm(text: str) -> str:
    """Для сравнения метки с цитатой: регистр, «ё», пунктуация и многоточия не мешают."""
    return " ".join(re.sub(r"[^\w\s]", " ", text.casefold().replace("ё", "е")).split())


def shot_for_target(caption: str, shots: list, used: set):
    """Снимок с рамкой цели, чья метка — цитата в «ёлочках» из подписи; занятые пропускаются.

    Цитата целиком, а не подстрока: «готово» в «Программа открылась — готово» не метка.
    """
    quoted = {norm(q) for q in re.findall(r"«([^«»]+)»", caption)}
    for s in shots:
        if s["rect"] and s["name"] not in used and norm(s["label"]) in quoted:
            return s
    return None


def shot_for_hold(e: dict, shots: list, used: set):
    """Снимок стоп-кадра: по имени в журнале (hold.shot), иначе с той же рамкой цели."""
    name = e.get("shot")
    if name:
        return next((s for s in shots if s["name"] == name), None)
    rect = e.get("rect")
    if rect:
        return next((s for s in shots if s["rect"] == list(rect) and s["name"] not in used), None)
    return None


# ── Подгонка плана под длину записи ──────────────────────────────────────
def _items(plan) -> list:
    return [{"seg": s, "kind": s.kind, "src0": s.src0, "src1": s.src1, "dur": s.dur} for s in plan.segs]


def _rebuild(plan, items: list):
    """Пересчитать кадры по оставшимся отрезкам (накопительное округление, как в build_plan)."""
    b = _Builder(plan.fps)
    for it in items:
        s = it["seg"]
        b.add(it["kind"], it["src0"], it["src1"], it["dur"], plates=s.plates, target=s.target, title=s.title,
              line=s.line, hold_ev=s.hold_ev, shot=it.get("shot"))
    plan.segs = b.segs
    return plan


def fit_to_record(plan, j, off: float, rec_end: float, shots: list, used: set) -> tuple:
    """План по длине записи rec_end (с): (plan, предупреждения). RecordShort — потерян шаг.

    j — журнал ролика (у главы — срез с нумерацией внутри главы), off — смещение записи.
    """
    plan.rec_end = rec_end
    last = max((s.src1 for s in plan.segs if s.kind in ("live", "fast")), default=0.0)
    if last <= rec_end + 1e-6:
        return plan, []
    frame = 1 / plan.fps
    if last <= rec_end + FREEZE_MAX:  # хвост короткий: последний кадр записи держится до конца плана
        for s in plan.segs:
            if s.kind in ("live", "fast") and s.src1 > rec_end:
                s.src0 = max(0.0, min(s.src0, rec_end - 2 * frame))
                s.src1 = rec_end
        return plan, [f"запись короче журнала на {last - rec_end:.2f} с: последний кадр удержан до конца"]
    items, dropped_raw, cut_raw, dropped_holds, kept_shots = [], 0, 0, 0, 0
    for it in _items(plan):
        s = it["seg"]
        if s.kind in ("live", "fast"):
            if it["src0"] >= rec_end - frame:
                dropped_raw += 1
                continue
            if it["src1"] > rec_end:
                cut_raw += 1
                it["dur"] *= (rec_end - it["src0"]) / (it["src1"] - it["src0"])
                it["src1"] = rec_end
        elif s.kind == "hold" and s.src0 > rec_end:
            shot = shot_for_hold(s.hold_ev, shots, used)
            if shot is None:
                dropped_holds += 1
                continue
            used.add(shot["name"])
            it["shot"] = shot["path"]
            kept_shots += 1
        items.append(it)
    while items and items[-1]["kind"] == "card":  # заставка без содержимого за ней не нужна
        items.pop()
    for k, it in enumerate(items):
        if it["kind"] == "card" and k + 1 < len(items) and items[k + 1]["kind"] == "card":
            it["drop"] = True
    items = [it for it in items if not it.get("drop")]
    left = {p.text for it in items if it["kind"] != "card" for p in it["seg"].plates if p.kind == "step"}
    lost = [step_text(e) for e in j.events if e["ev"] == "step" and e["t"] + off >= rec_end - frame
            and step_text(e) not in left]
    if lost:
        raise RecordShort(f"запись ({rec_end:.2f} с) короче журнала на {last - rec_end:.1f} с, "
                          f"потерян шаг без снимка: {'; '.join(lost)}")
    plan = _rebuild(plan, items)
    msg = (f"запись короче журнала на {last - rec_end:.1f} с: за концом живых отрезков отброшено "
           f"{dropped_raw}, обрезано {cut_raw}, стоп-кадров без снимка {dropped_holds}, "
           f"стоп-кадров по снимкам {kept_shots}")
    return plan, [msg]
