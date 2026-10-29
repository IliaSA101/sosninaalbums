# -*- coding: utf-8 -*-
"""
План монтажа ролика-инструкции по журналу шагов (spec 036, design §2-3.2).

Чистая логика: без ffmpeg и Pillow, поэтому проверяется быстрыми тестами. Рисование и
вызовы ffmpeg — в overlay.py / overlay_draw.py. Журнал (steps.jsonl) — контракт между
раннерами (Mac, Windows) и монтажом, формат — design §2; здесь он только разбирается.

Время журнала `t` — секунды стенных часов от начала журнала, время записи `v = t + off`,
где off находит overlay.py по красной вспышке (`sync`). План считает в времени записи.
"""
import json
import math
from dataclasses import dataclass, field

from overlay_cut import MOVE_CAP, SETTLE_KEEP, TYPE_CAP, fast_zones, merge_fast  # noqa: F401

FPS = 15
CARD_DUR = 2.0  # заставка главы
IDLE_CAP = 0.6  # ожидание раннера ускоряется до этой длины
KEYS_PRE = 0.2  # плашка клавиш: за столько до нажатия...
KEYS_POST = 0.8  # ...и столько после
MAX_CAPTION = 80
SAFE_BOTTOM_1080 = 120  # px снизу при высоте кадра 1080: нижняя подпись и плашка клавиш стоят выше

POINT_EV = ("sync", "chapter", "step", "hold", "note", "end")
SPAN_EV = ("move", "idle", "keys", "type")


# ── Журнал ───────────────────────────────────────────────────────────────
@dataclass
class Journal:
    meta: dict
    events: list = field(default_factory=list)  # без meta; у каждого события "line" — номер строки
    start: float = None  # время журнала, с которого монтировать (срез главы); None — после sync

    def sync(self):
        return next((e for e in self.events if e["ev"] == "sync"), None)


def ev_time(e: dict) -> float:
    """Момент события; у отрезков — начало."""
    return e["t"] if "t" in e else e["t0"]


def _fail(line: int, msg: str):
    raise ValueError(f"строка {line}: {msg}")


def _num(e: dict, key: str, line: int) -> float:
    v = e.get(key)
    if isinstance(v, bool) or not isinstance(v, (int, float)) or not math.isfinite(v):
        _fail(line, f"поле {key!r} должно быть числом")
    return float(v)


def _check_rect(rect, meta: dict, line: int):
    if (
        not isinstance(rect, list)
        or len(rect) != 4
        or any(isinstance(v, bool) or not isinstance(v, (int, float)) for v in rect)
    ):
        _fail(line, "rect — четыре числа [x, y, w, h] или null")
    x, y, w, h = rect
    sw, sh = meta["screen"]
    if w <= 0 or h <= 0 or x < 0 or y < 0 or x + w > sw + 0.5 or y + h > sh + 0.5:
        _fail(line, f"рамка {rect} вне экрана {sw}x{sh}")


def _check_event(e: dict, meta: dict, line: int):
    ev = e["ev"]
    if ev in POINT_EV:
        if _num(e, "t", line) < 0:
            _fail(line, "t отрицательно")
    else:
        t0, t1 = _num(e, "t0", line), _num(e, "t1", line)
        if t0 < 0 or t1 < t0:
            _fail(line, "нужно 0 <= t0 <= t1")
    if ev == "sync":
        c = e.get("color")
        hexd = "0123456789abcdefABCDEF"
        if not (isinstance(c, str) and len(c) == 7 and c[0] == "#" and all(ch in hexd for ch in c[1:])):
            _fail(line, "sync.color — цвет вида #rrggbb")
        if _num(e, "dur", line) <= 0:
            _fail(line, "sync.dur должно быть больше 0")
    elif ev in ("chapter", "step"):
        if not (isinstance(e.get("title"), str) and e["title"].strip()):
            _fail(line, f"{ev}.title не должно быть пустым")
        if ev == "step":
            n, of = e.get("n"), e.get("of")
            if not (isinstance(n, int) and isinstance(of, int) and 1 <= n <= of):
                _fail(line, "step.n и step.of — целые, 1 <= n <= of")
    elif ev == "hold":
        cap = e.get("caption")
        if not (isinstance(cap, str) and cap.strip()):
            _fail(line, "hold.caption не должна быть пустой")
        if len(cap) > MAX_CAPTION:
            _fail(line, f"hold.caption длиннее {MAX_CAPTION} знаков")
        if e.get("rect") is not None:
            _check_rect(e["rect"], meta, line)
        keys = e.get("keys")
        if keys is not None and not (isinstance(keys, list) and keys and all(isinstance(k, str) and k for k in keys)):
            _fail(line, "hold.keys — непустой список строк или null")
        if e.get("shot") is not None and not (isinstance(e["shot"], str) and e["shot"]):
            _fail(line, "hold.shot — имя снимка (строка) или null")
    elif ev == "keys":
        keys = e.get("keys")
        if not (isinstance(keys, list) and keys and all(isinstance(k, str) and k for k in keys)):
            _fail(line, "keys.keys — непустой список строк")
    elif ev == "type":
        if not (isinstance(e.get("text"), str) and e["text"]):
            _fail(line, "type.text не должен быть пустым")


def _check_meta(meta: dict, line: int):
    if meta.get("space") not in ("points", "pixels"):
        _fail(line, "meta.space — points или pixels")
    scr = meta.get("screen")
    if not (isinstance(scr, list) and len(scr) == 2 and all(isinstance(v, (int, float)) and v > 0 for v in scr)):
        _fail(line, "meta.screen — [ширина, высота] больше 0")
    sc = meta.setdefault("scale", 1)
    if isinstance(sc, bool) or not isinstance(sc, (int, float)) or sc <= 0:
        _fail(line, "meta.scale должно быть больше 0")


def parse_journal(text: str, require_sync: bool = True) -> Journal:
    """Разбор steps.jsonl с проверками design §2; ошибка — ValueError с номером строки."""
    meta = None
    events = []
    prev = -1.0
    for n, raw in enumerate(text.splitlines(), 1):
        if not raw.strip():
            continue
        try:
            e = json.loads(raw)
        except json.JSONDecodeError as exc:
            _fail(n, f"не JSON ({exc.msg})")
        if not isinstance(e, dict) or not isinstance(e.get("ev"), str):
            _fail(n, "нужен объект с полем ev")
        ev = e["ev"]
        if meta is None:
            if ev != "meta":
                _fail(n, "первая строка журнала — meta")
            _check_meta(e, n)
            meta = e
            continue
        if ev == "meta":
            _fail(n, "meta допустима только первой строкой")
        if ev not in POINT_EV and ev not in SPAN_EV:
            _fail(n, f"неизвестное событие {ev!r}")
        if events and events[-1]["ev"] == "end":
            _fail(n, "после end событий быть не должно")
        _check_event(e, meta, n)
        t = ev_time(e)
        if t < prev:
            _fail(n, f"время убывает ({t} после {prev})")
        prev = t
        e["line"] = n
        events.append(e)
    if meta is None:
        raise ValueError("строка 1: журнал пуст")
    last = len(text.splitlines())
    if not events or events[-1]["ev"] != "end":
        _fail(last, "нет завершающего end")
    if require_sync and not any(e["ev"] == "sync" for e in events):
        _fail(last, "нет sync: без метки синхронизации монтировать нельзя")
    _check_overlaps(events)
    return Journal(meta, events)


def _check_overlaps(events: list):
    """idle не пересекается с move/keys/type; step/chapter/hold не лежат внутри idle."""
    idles = [e for e in events if e["ev"] == "idle"]
    for i in idles:
        for e in events:
            if e is i:
                continue
            if e["ev"] in ("move", "keys", "type"):
                if e["t0"] < i["t1"] and i["t0"] < e["t1"]:
                    _fail(e["line"], f"{e['ev']} пересекается с idle (строка {i['line']})")
            elif e["ev"] in ("step", "chapter", "hold") and i["t0"] < e["t"] < i["t1"]:
                _fail(e["line"], f"{e['ev']} внутри idle (строка {i['line']})")


# ── Масштаб журнала в кадр ───────────────────────────────────────────────
def px_scale(meta: dict, video_w: int) -> float:
    """k из design §2: пиксель экрана -> пиксель ролика. Mac (points): экран = screen × scale."""
    scale = meta["scale"] if meta["space"] == "points" else 1
    return video_w / (meta["screen"][0] * scale)


def rect_to_frame(rect, meta: dict, video_w: int) -> tuple:
    """[x, y, w, h] журнала -> (x0, y0, x1, y1) в пикселях ролика.

    Точки переводятся в пиксели экрана (× scale), затем × k; итог для обоих пространств —
    video_w / screen_w.
    """
    unit = meta["scale"] if meta["space"] == "points" else 1
    f = unit * px_scale(meta, video_w)
    x, y, w, h = rect
    return (x * f, y * f, (x + w) * f, (y + h) * f)


# ── Синхронизация по вспышке (чистая часть) ──────────────────────────────
def hex_rgb(c: str) -> tuple:
    return (int(c[1:3], 16), int(c[3:5], 16), int(c[5:7], 16))


def _dist(a, b) -> float:
    return math.dist(a, b)


def find_flash_frame(samples: list, color: str, min_gap: float = 60.0):
    """Индекс первого кадра, где средний цвет полосы ближе к color, чем к исходному (кадр 0).

    None — вспышки нет или она неотличима от исходного фона (разница цветов < min_gap).
    """
    if not samples:
        return None
    target = hex_rgb(color)
    base = samples[0]
    if _dist(base, target) < min_gap:
        return None
    for i, s in enumerate(samples):
        if _dist(s, target) < _dist(s, base):
            return i
    return None


# ── Плашки ────────────────────────────────────────────────────────────────
@dataclass(frozen=True)
class Metrics:
    """Размеры плашек от ширины кадра; текст измеряется Pillow'ом, здесь — оценка."""

    out_w: int
    out_h: int

    @property
    def size(self) -> int:
        return max(12, round(self.out_w / 56))

    @property
    def margin(self) -> int:
        return round(self.size * 0.8)

    @property
    def safe_bottom(self) -> int:
        """Нижняя полоса, куда подписи не лезут: Dock macOS (1010-1075 при 1080) и панель задач."""
        return round(self.out_h * SAFE_BOTTOM_1080 / 1080)

    def font_size(self, kind: str) -> int:
        return self.size if kind == "step" else round(self.size * 1.3)

    def pad(self, kind: str) -> int:
        return round(self.font_size(kind) * 0.55)

    def plate_h(self, kind: str) -> int:
        return round(self.font_size(kind) * 1.2) + 2 * self.pad(kind)

    def est_w(self, kind: str, text: str) -> int:
        """Оценка ширины с запасом (реальную даёт Pillow)."""
        return round(len(text) * self.font_size(kind) * 0.62) + 2 * self.pad(kind)

    def plate_rect(self, anchor: str, kind: str, w: int, h: int) -> tuple:
        """Прямоугольник плашки по якорю: tl/tr — угол, bc/tc — по центру снизу/сверху."""
        mg = self.margin
        x = {"tl": mg, "tr": self.out_w - mg - w}.get(anchor, (self.out_w - w) / 2)
        if anchor in ("tl", "tr"):
            y = mg
        elif anchor == "tc":  # ниже ряда плашки шага, чтобы не наехать на неё
            y = mg + self.plate_h("step") + mg // 2
        else:
            y = self.out_h - self.safe_bottom - h - 1  # прямоугольник Pillow включает нижнюю строку
        return (x, y, x + w, y + h)


@dataclass(frozen=True)
class Plate:
    kind: str  # step | keys
    text: str
    anchor: str  # tl | tr | bc | tc


@dataclass(frozen=True)
class Target:
    rect: tuple  # (x0, y0, x1, y1) в пикселях ролика или None
    caption: str
    keep: tuple = ()  # рамки прежних целей этого шага: подпись на них не ложится («... в «Программы» слева» не закрывает значок)


@dataclass
class Seg:
    kind: str  # live | hold | fast | card
    src0: float  # время записи; у hold src0 == src1 == кадр стоп-кадра
    src1: float
    dur: float  # расчётная длина в ролике, с
    frames: int = 0  # целое число кадров (накопительное округление, без дрейфа)
    plates: tuple = ()
    target: Target = None
    title: str = ""  # заставка
    line: int = 0  # строка журнала hold (для отчёта)
    hold_ev: dict = None
    shot: str = None  # PNG-снимок вместо кадра записи (запись короче журнала, overlay_fit)


@dataclass
class Plan:
    segs: list
    fps: int
    out_w: int
    out_h: int
    rec_end: float = None  # длина записи, с: за ней кадров нет (overlay_fit)

    @property
    def total_frames(self) -> int:
        return sum(s.frames for s in self.segs)

    @property
    def duration(self) -> float:
        return self.total_frames / self.fps

    def holds(self) -> list:
        return [s for s in self.segs if s.kind == "hold"]


def hold_duration(caption: str) -> float:
    return min(3.5, max(2.0, 1.5 + len(caption) / 20))


def keys_text(e: dict) -> str:
    """Подпись плашки клавиш: «⌘ + Пробел» или «Ввод: Терминал»."""
    return "Ввод: " + e["text"] if e["ev"] == "type" else " + ".join(e["keys"])


def _overlap(a, b, pad: float = 0.0) -> float:
    w = min(a[2], b[2] + pad) - max(a[0], b[0] - pad)
    h = min(a[3], b[3] + pad) - max(a[1], b[1] - pad)
    return w * h if w > 0 and h > 0 else 0.0


def _pick_anchor(cands: tuple, box_of, rects: list, pad: float) -> str:
    """Первый якорь без пересечений с целями, иначе с наименьшим."""
    best = None
    for a in cands:
        area = sum(_overlap(box_of(a), r, pad) for r in rects)
        if area == 0:
            return a
        if best is None or area < best[0]:
            best = (area, a)
    return best[1]


def _scan_groups(events: list, meta: dict, m: Metrics) -> list:
    """Группы «шаг/глава»: для каждой — цели и самая длинная подпись клавиш, из них якоря плашек.

    Возвращает список групп; группа 0 — до первого step/chapter. Якорь один на группу,
    чтобы плашка не прыгала между стоп-кадрами одного шага.
    """
    groups = [{"step": None, "rects": [], "keys": "", "norect": False}]
    for e in events:
        ev = e["ev"]
        if ev in ("step", "chapter"):
            groups.append({"step": e if ev == "step" else None, "rects": [], "keys": "", "norect": False})
        elif ev == "hold":
            g = groups[-1]
            if e.get("rect"):
                g["rects"].append(rect_to_frame(e["rect"], meta, m.out_w))
            else:
                g["norect"] = True
            if e.get("keys"):
                g["keys"] = max(g["keys"], keys_text({"ev": "keys", "keys": e["keys"]}), key=len)
        elif ev in ("keys", "type"):
            g = groups[-1]
            g["keys"] = max(g["keys"], keys_text(e), key=len)
    for g in groups:
        g["step_anchor"] = "tl"
        if g["step"]:
            txt = step_text(g["step"])
            w = m.est_w("step", txt)
            g["step_anchor"] = _pick_anchor(
                ("tl", "tr"), lambda a: m.plate_rect(a, "step", w, m.plate_h("step")), g["rects"], m.margin / 2
            )
        g["keys_anchor"] = "bc"
        # подпись без цели стоит внизу по центру — плашка клавиш уходит вверх
        rects = g["rects"] + ([(m.out_w * 0.15, m.out_h * 0.78, m.out_w * 0.85, m.out_h)] if g["norect"] else [])
        if g["keys"]:
            w = m.est_w("keys", g["keys"])
            g["keys_anchor"] = _pick_anchor(
                ("bc", "tc"), lambda a: m.plate_rect(a, "keys", w, m.plate_h("keys")), rects, m.margin / 2
            )
    return groups


def step_text(e: dict) -> str:
    return f"Шаг {e['n']} из {e['of']}. {e['title']}"


class _Builder:
    """Копит отрезки; число кадров считается по накопленному времени — ролик не «уплывает»."""

    def __init__(self, fps: int):
        self.fps = fps
        self.segs = []
        self.clock = 0.0
        self.emitted = 0

    def add(self, kind: str, src0: float, src1: float, dur: float, **kw):
        end = self.clock + dur
        n = math.floor(end * self.fps + 0.5) - self.emitted
        self.clock = end
        if n <= 0:
            return None  # короче кадра: пропадает, остаток учтён в clock
        self.emitted += n
        s = Seg(kind, src0, src1, dur, frames=n, **kw)
        self.segs.append(s)
        return s


def build_plan(j: Journal, off: float, out_w: int, out_h: int, fps: int = FPS) -> Plan:
    """План монтажа design §3.2: живые отрезки, стоп-кадры, ускоренные простои, заставки."""
    meta = j.meta
    m = Metrics(out_w, out_h)
    groups = _scan_groups(j.events, meta, m)
    b = _Builder(fps)
    items = []  # отрезки до слияния соседних fast

    def emit(kind, src0, src1, dur, **kw):
        items.append({"kind": kind, "src0": src0, "src1": src1, "dur": dur, "kw": kw})

    def v(t):
        return t + off

    # окна плашек клавиш в времени записи: (с, по, текст)
    windows = [
        (v(e["t0"]) - KEYS_PRE, v(e["t1"]) + KEYS_POST, keys_text(e)) for e in j.events if e["ev"] in ("keys", "type")
    ]
    sync = j.sync()
    if j.start is not None:
        cursor = max(0.0, v(j.start))
    else:
        cursor = max(0.0, v(sync["t"]) + sync["dur"]) if sync else 0.0
    gid = 0
    gi = {"i": 0}  # номер группы, к которой относится текущее время

    def step_plate():
        g = groups[gi["i"]]
        if not g["step"]:
            return []
        return [Plate("step", step_text(g["step"]), g["step_anchor"])]

    def flush(end: float):
        """Живой отрезок [cursor, end]: границы плашек клавиш и ускоряемые зоны режут его на куски."""
        nonlocal cursor
        if end <= cursor:
            return
        zones = fast_zones(j.events, off, cursor, end, [(w0, w1) for w0, w1, _ in windows], IDLE_CAP)
        cuts = {cursor, end}
        for z in zones:
            cuts.update((z[0], z[1]))
        for w0, w1, _ in windows:  # внутри зоны плашка не режет: зона идёт одним отрезком
            cuts.update(c for c in (w0, w1) if cursor < c < end and not any(z[0] < c < z[1] for z in zones))
        pts = sorted(cuts)
        for a, c in zip(pts, pts[1:]):
            mid = (a + c) / 2
            pl = step_plate()
            z = next((z for z in zones if z[0] <= mid < z[1]), None)
            kt = next((txt for w0, w1, txt in windows if w0 <= mid < w1), None)
            if kt:
                pl.append(Plate("keys", kt, groups[gi["i"]]["keys_anchor"]))
            if z:
                emit("fast", a, c, z[2] * (c - a) / (z[1] - z[0]), plates=tuple(pl))
            else:
                emit("live", a, c, c - a, plates=tuple(pl))
        cursor = end

    prev_hold = {"gid": -1, "rects": ()}  # цели стоп-кадров текущего шага, уже показанные
    for e in j.events:
        ev = e["ev"]
        if ev == "step":
            flush(v(e["t"]))
            gid += 1
            gi["i"] = gid
        elif ev == "chapter":
            flush(v(e["t"]))
            gid += 1
            gi["i"] = gid
            emit("card", 0.0, 0.0, CARD_DUR, title=e["title"], line=e["line"])
        elif ev == "hold":
            t = v(e["t"])
            flush(t)
            g = groups[gi["i"]]
            pl = step_plate()
            if e.get("keys"):
                pl.append(Plate("keys", keys_text({"ev": "keys", "keys": e["keys"]}), g["keys_anchor"]))
            rect = rect_to_frame(e["rect"], meta, out_w) if e.get("rect") else None
            keep = prev_hold["rects"] if prev_hold["gid"] == gid else ()
            emit(
                "hold", t, t, hold_duration(e["caption"]),
                plates=tuple(pl), target=Target(rect, e["caption"], keep), line=e["line"], hold_ev=e,
            )
            prev_hold = {"gid": gid, "rects": keep + ((rect,) if rect else ())}
            cursor = max(cursor, t)
        elif ev == "idle":
            t0, t1 = v(e["t0"]), v(e["t1"])
            if t1 - t0 > IDLE_CAP:
                flush(t0)
                emit("fast", t0, t1, IDLE_CAP, plates=tuple(step_plate()))
                cursor = t1
        elif ev == "end":
            flush(v(e["t"]))
    for it in merge_fast(items):
        b.add(it["kind"], it["src0"], it["src1"], it["dur"], **it["kw"])
    return Plan(b.segs, fps, out_w, out_h)


def split_chapters(j: Journal) -> list:
    """Журнал по главам: список Journal, у каждого своя заставка и «Шаг N из M» внутри главы.

    Всё до первой главы (подготовка раннера) отбрасывается; метка sync остаётся только в целом
    журнале — срез монтируется от своего chapter. Конец главы — начало следующей (или end).
    """
    chaps = [i for i, e in enumerate(j.events) if e["ev"] == "chapter"]
    out = []
    for k, i in enumerate(chaps):
        stop = chaps[k + 1] if k + 1 < len(chaps) else len(j.events)
        evs = [dict(e) for e in j.events[i:stop] if e["ev"] != "sync"]
        if k + 1 < len(chaps):  # границу следующей главы закрывает синтетический end
            nxt = j.events[stop]
            evs.append({"ev": "end", "t": nxt["t"], "line": nxt["line"]})
        steps = [e for e in evs if e["ev"] == "step"]
        for n, e in enumerate(steps, 1):
            e["n"], e["of"] = n, len(steps)
        out.append(Journal(j.meta, evs, start=evs[0]["t"]))
    return out
