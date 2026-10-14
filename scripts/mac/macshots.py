"""Помощник scripts/mac (029 design §3.2–3.5): выбор окна, сравнение снимков, shots.json.

Pillow необязателен: без него changed = null, а ok решается по списку окон.

  win     --windows F [--pid "1 2"] [--owner RE] [--pick front|largest]  -> "id x y w h" (точки)
  record  --out D --name N [--windows F --pid --owner --pick --label --rect "x y w h" --why ...]
                                                                       -> "да|нет<TAB>почему"
  get     --out D --name N --field F                                   -> поле записи shots.json
  diff    A B                                                          -> доля изменённых пикселей
  motion  DIR [--with-count]                                           -> доля разных соседних кадров
                                                                          (+ их число: «0.099 24»)
  screen  --windows F [--png P]                                        -> фактический экран: точки, масштаб, пиксели снимка
  inrect  --rect "x y w h" [--leftmost]  (stdin: строки «x y w h<TAB>текст» от ocr)
                                                                       -> первая строка с центром внутри рамки
"""

import argparse
import json
import os
import re
import sys

MIN_CHANGED = 0.02  # меньше — на снимке одни обои (нет права записи экрана, issue #8782)
PIX_DELTA = 24  # разница яркости 0–255 ниже — шум сжатия, часы в строке меню
MIN_WIN = 40  # окна уже 40 точек — полоски и тени, не окна
FRAME_MOVED = 0.005  # соседние кадры видео «разные», если изменилось больше 0,5 % пикселей
CMP_WIDTH = 480  # сравнение на уменьшенной копии: быстрее, курсор и сглаживание не мешают


# ---------- чистые функции ----------


def parse_pids(s):
    return {int(x) for x in (s or "").split() if x.isdigit()}


def parse_rect(s):
    """«x y w h» (пробелы или запятые) -> [x, y, w, h] или None."""
    parts = [p for p in re.split(r"[\s,]+", (s or "").strip()) if p]
    if len(parts) != 4:
        return None
    try:
        return [float(p) for p in parts]
    except ValueError:
        return None


def pick_window(wins, pids=(), owner="", pick="largest"):
    """Окно по pid или владельцу; front — первое в списке (CGWindowList — спереди назад)."""
    rx = re.compile(owner, re.I) if owner else None
    cand = []
    for w in wins:
        r = w.get("rect") or [0, 0, 0, 0]
        if len(r) != 4 or r[2] < MIN_WIN or r[3] < MIN_WIN:
            continue
        if w.get("pid") in pids or (rx is not None and rx.search(w.get("owner", ""))):
            cand.append(w)
    if not cand:
        return None
    if pick == "front":
        return cand[0]
    return max(cand, key=lambda w: w["rect"][2] * w["rect"][3])


def scale_rect(rect, scale):
    """Точки экрана -> пиксели снимка."""
    return [int(round(v * scale)) for v in rect]


def clamp_rect(rect, size):
    """[x, y, w, h] в пикселях, обрезанная по снимку; None — вне снимка."""
    x0, y0 = max(0, rect[0]), max(0, rect[1])
    x1, y1 = min(size[0], rect[0] + rect[2]), min(size[1], rect[1] + rect[3])
    if x1 - x0 < 1 or y1 - y0 < 1:
        return None
    return [x0, y0, x1 - x0, y1 - y0]


def snapshot_scale(png_width, screen_points_width, fallback):
    """Масштаб по факту снимка: режим экрана может не совпасть с тем, что отдал screencapture."""
    if png_width and screen_points_width:
        return png_width / screen_points_width
    return fallback or 1.0


def decide_ok(file_ok, changed, want_window, window_found):
    if not file_ok:
        return False, "снимка нет"
    if changed is not None and changed < MIN_CHANGED:
        return False, f"не отличается от стола ({changed:.3f}): обои без окон — нет права записи экрана?"
    if want_window and not window_found:
        return False, "нужного окна нет в списке окон"
    return True, ""


def join_why(*parts):
    return "; ".join(p for p in parts if p)


def upsert(shots, rec):
    """Повтор шага заменяет запись с тем же именем."""
    out = [s for s in shots if s.get("name") != rec["name"]]
    out.append(rec)
    return out


def rect_contains_centre(rect, box):
    """Центр рамки box [x, y, w, h] внутри rect [x, y, w, h] (границы включены)."""
    cx, cy = box[0] + box[2] / 2, box[1] + box[3] / 2
    return rect[0] <= cx <= rect[0] + rect[2] and rect[1] <= cy <= rect[1] + rect[3]


def screen_line(screen, scale, png_size=None):
    """Одна строка для отчёта: «1920×1080 точек, масштаб 2, снимок 3840×2160 px»."""
    if not screen or not screen[0]:
        return ""
    line = f"{int(round(screen[0]))}×{int(round(screen[1]))} точек, масштаб {scale:g}"
    if png_size:
        line += f", снимок {png_size[0]}×{png_size[1]} px"
    return line


def moved_count(fractions):
    """Сколько пар соседних кадров изменились: ролик, где мало движения, но оно есть (ожидание на шагах)."""
    return sum(1 for f in fractions if f > FRAME_MOVED)


def moved_share(fractions):
    """Доля пар соседних кадров, где картинка изменилась."""
    if not fractions:
        return 0.0
    return moved_count(fractions) / len(fractions)


# ---------- Pillow ----------


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


def _open(path):
    image, _ = _pil()
    if image is None or not os.path.isfile(path):
        return None
    try:
        im = image.open(path)
        im.load()
        return im
    except OSError:
        return None


# ---------- команды ----------


def load_windows(path):
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except (OSError, ValueError):
        return {}


def cmd_win(a):
    doc = load_windows(a.windows)
    w = pick_window(doc.get("windows", []), parse_pids(a.pid), a.owner, a.pick)
    if w is None:
        return 1
    r = [int(round(v)) for v in w["rect"]]
    print(w["id"], *r)
    return 0


def cmd_record(a):
    name = a.name
    rel = f"png/{name}.png"
    full_path = os.path.join(a.out, rel)
    win_rel = f"png/{name}-win.png"
    win_path = os.path.join(a.out, win_rel)
    doc = load_windows(a.windows) if a.windows else {}
    want = bool(a.pid.strip() or a.owner)
    w = pick_window(doc.get("windows", []), parse_pids(a.pid), a.owner, a.pick) if want else None

    full = _open(full_path)
    file_ok = os.path.isfile(full_path) and os.path.getsize(full_path) > 0
    size = list(full.size) if full is not None else None
    screen = doc.get("screen") or [0, 0]
    scale = snapshot_scale(size[0] if size else 0, screen[0], doc.get("scale"))

    changed = None
    base_path = os.path.join(a.out, a.base)
    if full is not None and os.path.abspath(base_path) != os.path.abspath(full_path):
        base = _open(base_path)
        if base is not None:
            changed = round(changed_fraction(base, full), 4)

    notes = []
    target = None
    label_rect = parse_rect(a.rect)
    if label_rect is not None:
        target = {"label": a.label, "rect": scale_rect(label_rect, scale)}
    elif w is not None:
        target = {"label": a.label or w.get("owner", ""), "rect": scale_rect(w["rect"], scale)}

    # Окно отдельно: screencapture -l без права даёт заливку — тогда вырезаем из полного снимка
    win = _open(win_path)
    if w is not None and full is not None and (win is None or is_uniform(win)):
        box = clamp_rect(scale_rect(w["rect"], scale), full.size)
        if box is not None:
            full.crop((box[0], box[1], box[0] + box[2], box[1] + box[3])).save(win_path)
            notes.append("окно вырезано из полного снимка")
    has_win = os.path.isfile(win_path)

    ok, why = decide_ok(file_ok, changed, want, w is not None)
    if ok and a.need_rect and label_rect is None:
        ok, why = False, f"«{a.label}» не найдена"
    rec = {
        "name": name,
        "ok": ok,
        "why": join_why(why, a.why, *notes),
        "file": rel if file_ok else None,
        "win": win_rel if has_win else None,
        "macos": a.macos,
        "lang": a.lang,
        "screen": size,
        "scale": round(scale, 3),
        "target": target,
        # рамка окна в пикселях снимка: по ней рамка цели переводится в координаты крупного плана (-win)
        "window": scale_rect(w["rect"], scale) if w is not None else None,
        "changed": changed,
    }
    path = os.path.join(a.out, "shots.json")
    try:
        with open(path, encoding="utf-8") as f:
            shots = json.load(f)
    except (OSError, ValueError):
        shots = []
    with open(path, "w", encoding="utf-8") as f:
        json.dump(upsert(shots, rec), f, ensure_ascii=False, indent=2)
        f.write("\n")
    print(("да" if ok else "нет") + "\t" + rec["why"])
    return 0


def cmd_get(a):
    try:
        with open(os.path.join(a.out, "shots.json"), encoding="utf-8") as f:
            shots = json.load(f)
    except (OSError, ValueError):
        return 1
    for s in shots:
        if s.get("name") == a.name:
            v = s.get(a.field)
            print("" if v is None else json.dumps(v, ensure_ascii=False) if not isinstance(v, str) else v)
            return 0
    return 1


def cmd_diff(a):
    x, y = _open(a.a), _open(a.b)
    if x is None or y is None:
        return 1
    print(f"{changed_fraction(x, y):.4f}")
    return 0


def cmd_motion(a):
    files = sorted(f for f in os.listdir(a.dir) if f.lower().endswith((".png", ".jpg")))
    prev, fr = None, []
    for f in files:
        cur = _open(os.path.join(a.dir, f))
        if cur is None:
            continue
        if prev is not None:
            fr.append(changed_fraction(prev, cur))
        prev = cur
    if not fr:
        return 1
    print(f"{moved_share(fr):.3f}" + (f" {moved_count(fr)}" if a.with_count else ""))
    return 0


def cmd_screen(a):
    doc = load_windows(a.windows)
    img = _open(a.png) if a.png else None
    line = screen_line(doc.get("screen"), doc.get("scale") or 1.0, list(img.size) if img else None)
    if not line:
        return 1
    print(line)
    return 0


def cmd_inrect(a):
    rect = parse_rect(a.rect)
    if rect is None:
        return 1
    found = []
    for line in sys.stdin.read().splitlines():
        box = parse_rect(line.split("\t", 1)[0])
        if box is not None and rect_contains_centre(rect, box):
            found.append((box[0], line))
    if not found:
        return 1
    # --leftmost: строка боковой колонки, а не заголовок окна с тем же текстом
    print(min(found, key=lambda f: f[0])[1] if a.leftmost else found[0][1])
    return 0


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)

    w = sub.add_parser("win")
    w.add_argument("--windows", required=True)
    w.add_argument("--pid", default="")
    w.add_argument("--owner", default="")
    w.add_argument("--pick", choices=["front", "largest"], default="largest")
    w.set_defaults(fn=cmd_win)

    r = sub.add_parser("record")
    r.add_argument("--out", required=True)
    r.add_argument("--name", required=True)
    r.add_argument("--base", default="png/01-desktop.png")
    r.add_argument("--windows", default="")
    r.add_argument("--pid", default="")
    r.add_argument("--owner", default="")
    r.add_argument("--pick", choices=["front", "largest"], default="largest")
    r.add_argument("--label", default="")
    r.add_argument("--rect", default="")
    r.add_argument("--why", default="")
    r.add_argument("--need-rect", action="store_true", help="без --rect вариант не удался (кнопка не найдена)")
    r.add_argument("--macos", default="")
    r.add_argument("--lang", default="")
    r.set_defaults(fn=cmd_record)

    g = sub.add_parser("get")
    g.add_argument("--out", required=True)
    g.add_argument("--name", required=True)
    g.add_argument("--field", required=True)
    g.set_defaults(fn=cmd_get)

    d = sub.add_parser("diff")
    d.add_argument("a")
    d.add_argument("b")
    d.set_defaults(fn=cmd_diff)

    m = sub.add_parser("motion")
    m.add_argument("dir")
    m.add_argument("--with-count", action="store_true", help="после доли — число изменившихся пар")
    m.set_defaults(fn=cmd_motion)

    sc = sub.add_parser("screen")
    sc.add_argument("--windows", required=True)
    sc.add_argument("--png", default="")
    sc.set_defaults(fn=cmd_screen)

    i = sub.add_parser("inrect")
    i.add_argument("--rect", required=True)
    i.add_argument("--leftmost", action="store_true", help="из подходящих — самая левая, а не первая")
    i.set_defaults(fn=cmd_inrect)

    a = p.parse_args(argv)
    # bash читает ответ как UTF-8 при любой локали раннера
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    return a.fn(a)


if __name__ == "__main__":
    sys.exit(main())
