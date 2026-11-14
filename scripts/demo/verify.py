"""Проверка готового каталога прогона и summary.md (102, Д2; критерии 102-10, 102-11, design §7): без записи и без стенда.

Коды: 3 — личное в тексте страницы (privacy.txt), 6 — ошибки консоли (console.txt), 5 — бюджет размера,
2 — нет файла или свойства не те. Несколько нарушений — код первого по этому порядку.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path

from PIL import Image

import compose as cp
import timeline as tl
from record import console_problems  # один фильтр с записью: метки сцен и NOT_OURS ошибкой не считаются

ROLL_S = (25.0, 35.0)  # длительность роликов A и B, секунды
HERO_S = (12.0, 16.0)  # заставка
MIN_CAPTURE_FPS = 24.0
ORDER = (3, 6, 5, 2)


@dataclass
class Report:
    problems: list[tuple[int, str]] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)
    facts: dict = field(default_factory=dict)

    def bad(self, code: int, text: str) -> None:
        self.problems.append((code, text))

    @property
    def code(self) -> int:
        codes = {c for c, _ in self.problems}
        return next((c for c in ORDER if c in codes), 0)


def _nonempty(p: Path) -> list[str]:
    return [s for s in p.read_text(encoding="utf-8").splitlines() if s.strip()] if p.is_file() else []


def _webp_facts(p: Path) -> dict:
    with Image.open(p) as im:
        n = getattr(im, "n_frames", 1)
        ms = 0
        for k in range(n):
            im.seek(k)
            im.load()  # duration кадра появляется в info только после load
            ms += int(im.info.get("duration", 0))
        return {"size": im.size, "frames": n, "dur": ms / 1000, "bytes": p.stat().st_size}


def check_run(root: Path, roll=ROLL_S, hero=HERO_S) -> Report:
    """Свойства и бюджеты готовых файлов, приватность, консоль, согласованность часов кадров и меток."""
    rep = Report()
    try:
        run = cp.load_run(root)
    except cp.Refuse as e:
        rep.bad(2, str(e))
        return rep
    w, h = int(run.meta.get("w", 1600)), int(run.meta.get("h", 1000))
    f = rep.facts
    for name, sc in run.scenes.items():
        t0, t1 = sc.window()
        want = sc.kept(run.slow)  # с вырезками ожидания (compose.cut_rules)
        fps = tl.capture_fps(sc.frames, t0, t1)
        lag = t0 - sc.frames[0].t
        f[name] = {"window": want, "capture_fps": fps, "frames": len(sc.frames), "marks": len(sc.marks), "lag": lag}
        if not -0.05 <= lag <= 2.0:
            rep.bad(2, f"сцена {name}: метка start на {lag:.2f} с от первого кадра — часы кадров и меток расходятся")
        if run.slow == 1 and 0 < fps < MIN_CAPTURE_FPS:
            rep.warnings.append(f"сцена {name}: захват {fps:.0f} к/с < {MIN_CAPTURE_FPS:.0f} — снимать в медленном режиме (design §5.4)")
        if (found := _nonempty(sc.dir / "privacy.txt")):
            rep.bad(3, f"сцена {name}: privacy.txt — {len(found)} находок, например: {found[0][:120]}")
        if (cons := console_problems(sc.dir)):
            rep.bad(6, f"сцена {name}: console.txt — {len(cons)} строк, например: {cons[0][:120]}")
        mp = root / f"{name}.mp4"
        if not mp.is_file():
            rep.bad(2, f"нет {mp.name}")
            continue
        try:
            pr = cp.probe_mp4(mp)
        except cp.Refuse as e:
            rep.bad(2, str(e))
            continue
        f[name].update(pr, bytes=mp.stat().st_size)
        if "264" not in pr["codec"] or "High" not in pr["profile"]:
            rep.bad(2, f"{mp.name}: кодек {pr['codec']} {pr['profile']}, нужен H.264 High")
        if (pr["w"], pr["h"]) != (w, h) or abs(pr["fps"] - cp.FPS) > 0.01:
            rep.bad(2, f"{mp.name}: {pr['w']}×{pr['h']} {pr['fps']:g} к/с, нужно {w}×{h} {cp.FPS} к/с")
        if not roll[0] <= pr["dur"] <= roll[1]:
            rep.bad(2, f"{mp.name}: {pr['dur']:.1f} с вне {roll[0]:g}–{roll[1]:g} с")
        if abs(pr["dur"] - want) > 0.1:
            rep.bad(2, f"{mp.name}: {pr['dur']:.2f} с, а в записи {want:.2f} с")
        if f[name]["bytes"] > cp.MP4_MAX:
            rep.bad(5, f"{mp.name}: {f[name]['bytes'] / cp.MIB:.1f} МБ > {cp.MP4_MAX / cp.MIB:.1f} МБ")
    wp = root / "demo.webp"
    if wp.is_file():
        f["webp"] = wf = _webp_facts(wp)
        if wf["frames"] < 2:
            rep.bad(2, "demo.webp не анимирован")
        if wf["size"] not in {(960, 600), (800, 500)}:
            rep.bad(2, f"demo.webp: {wf['size'][0]}×{wf['size'][1]}, нужно 960×600 (800×500 на запасной ступени)")
        if not hero[0] <= wf["dur"] <= hero[1]:
            rep.bad(2, f"demo.webp: {wf['dur']:.1f} с вне {hero[0]:g}–{hero[1]:g} с")
        if wf["bytes"] > cp.WEBP_MAX:
            rep.bad(5, f"demo.webp: {wf['bytes'] / cp.MIB:.2f} МБ > {cp.WEBP_MAX / cp.MIB:.1f} МБ")
    else:
        rep.bad(2, "нет demo.webp")
    sp = root / "screenshot.jpg"
    if sp.is_file():
        with Image.open(sp) as im:
            f["still"] = {"size": im.size, "bytes": sp.stat().st_size}
        if im.size != (w, h):
            rep.bad(2, f"screenshot.jpg: {im.size[0]}×{im.size[1]}, нужно {w}×{h}")
        if f["still"]["bytes"] > cp.SHOT_MAX:
            rep.bad(5, f"screenshot.jpg: {f['still']['bytes'] // 1024} КБ > {cp.SHOT_MAX // 1024} КБ")
    else:
        rep.bad(2, "нет screenshot.jpg")
    if run.slow > 1:
        rep.warnings.append(f"запись в медленном режиме ×{run.slow:g}: паузы и пути длиннее, при сборке время делится")
    return rep


def render_summary(root: Path, rep: Report) -> str:
    """summary.md: длительности, размеры, захват, режим, консоль, приватность, предупреждения."""
    build: dict = {}
    if (root / "build.json").is_file():
        build = json.loads((root / "build.json").read_text(encoding="utf-8"))
    L = [f"# Прогон {root.name}", "", "| Файл | Длительность | Размер | Заметка |", "| --- | --- | --- | --- |"]
    for n in ("A", "B"):
        d = rep.facts.get(n, {})
        q = build.get(f"mp4-{n}", {})
        if "dur" in d:
            L.append(f"| {n}.mp4 | {d['dur']:.1f} с | {d['bytes'] / cp.MIB:.2f} МБ | {d['w']}×{d['h']}, {d['fps']:g} к/с, CRF {q.get('q', '?')} ({q.get('backend', '?')}) |")
        else:
            L.append(f"| {n}.mp4 | — | — | нет файла |")
    wf, st = rep.facts.get("webp"), rep.facts.get("still")
    wb = build.get("webp", {})
    if wf:
        L.append(f"| demo.webp | {wf['dur']:.1f} с | {wf['bytes'] / cp.MIB:.2f} МБ | {wf['size'][0]}×{wf['size'][1]}, {wf['frames']} кадров, {wb.get('fps', '?')} к/с, quality {wb.get('quality', '?')} |")
    if st:
        L.append(f"| screenshot.jpg | — | {st['bytes'] // 1024} КБ | {st['size'][0]}×{st['size'][1]}, quality {build.get('still', {}).get('quality', '?')} |")
    L += ["", "## Захват", ""]
    for n in ("A", "B"):
        d = rep.facts.get(n)
        if d:
            L.append(f"- {n}: {d['frames']} кадров, медиана интервалов → {d['capture_fps']:.0f} к/с, меток {d['marks']}, окно {d['window']:.1f} с")
    L += ["", "## Проверки", ""]
    for n in ("A", "B"):
        d = (root / n)
        L.append(f"- {n}: консоль — {len(console_problems(d))} строк, приватность — {len(_nonempty(d / 'privacy.txt'))} находок")
    L += ["", "## Предупреждения и нарушения", ""]
    items = [f"- {w}" for w in rep.warnings] + [f"- код {c}: {t}" for c, t in rep.problems]
    L += items or ["- нет"]
    return "\n".join(L) + "\n"
