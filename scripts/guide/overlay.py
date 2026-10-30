# -*- coding: utf-8 -*-
"""
Монтаж ролика-инструкции: сырая запись раннера + журнал шагов -> mp4 со стоп-кадрами, стрелками,
плашками «Шаг N из M» и клавиш, ускоренными ожиданиями (spec 036, design §2-3).

Запуск (локально у оркестратора; в CI не вызывается):
    python -I scripts/guide/overlay.py <папка артефакта или steps.jsonl> --out docs/install/video/mac-15.mp4
        [--raw raw.mp4] [--width 1920] [--check] [--frames <папка>] [--mask x,y,w,h ...]
        [--no-layers] [--offset <с>] [--font <ttf>] [--font-sym <ttf>] [--split | --chapter N]

Папка артефакта: журнал лежит в <папка>/video/steps.jsonl или <папка>/steps.jsonl, запись — рядом
(raw.mp4). --check — только проверки design §3.4 и отчёт без сборки. --frames — по PNG на каждый
стоп-кадр (их смотрят глазами). --mask — серая заливка прямоугольника (пиксели кадра) на всём
ролике: личные пути в записи владельца. --no-layers — стоп-кадры без плашек и стрелок.
--offset — задать смещение записи вручную (журнал, написанный по записи владельца, без метки sync).
--split — по ролику на главу, имя по названию главы: <out>-unzip.mp4 («Распакуйте архив»),
<out>-start.mp4 («Первый запуск…»), <out>-damaged.mp4 («…повреждено»), <out>-fromzip.mp4 («Если
запустили прямо из архива»), <out>-unblock.mp4 («Если кнопки «Выполнить в любом случае» нет»),
остальные — <out>-chN.mp4 (N — номер главы); --chapter N — только глава N прямо в --out. Длина
проверяется у каждого ролика; у каждой главы своя заставка и «Шаг N из M» внутри главы;
всё до первой главы отбрасывается.

Время журнала и записи связывает красная вспышка `sync`: off = время кадра вспышки - sync.t.
Не нашли вспышку — отказ, подгонять вслепую нельзя: стоп-кадр взял бы кадр после нажатия.

Коды выхода: 0 — готово (предупреждения не ошибка), 1 — нет ffmpeg/ffprobe, 2 — нет метки
синхронизации или журнал не разбирается, 3 — ролик длиннее 120 с, 4 — журнал не сходится с
записью, 5 — файл больше 8 МБ даже при crf 30, 6 — ffmpeg упал.
"""
import argparse
import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


sys.path.insert(0, str(Path(__file__).resolve().parent))
import overlay_draw  # noqa: E402
import overlay_fit  # noqa: E402
import overlay_plan as op  # noqa: E402
from PIL import Image, ImageChops  # noqa: E402

MAX_MB = 8
MIN_DUR, MAX_DUR, HARD_DUR = 30.0, 95.0, 120.0
SYNC_WINDOW = 10.0  # метка должна быть в первых секундах записи
SYNC_FPS = 30  # шаг поиска вспышки: точнее исходных 15 кадр/с
CRFS = (26, 30)
CHANGED = 0.003  # доля изменившихся пикселей: «после нажатия что-то произошло»
DIFFERENT = 0.005  # соседние стоп-кадры должны различаться
TARGET_SAME = 0.12  # рамка цели из снимка: доля заметно отличающихся пикселей кадра и снимка не больше


class Fail(Exception):
    def __init__(self, code: int, msg: str):
        super().__init__(msg)
        self.code = code


# ── ffmpeg ───────────────────────────────────────────────────────────────
def need_tools():
    for tool in ("ffmpeg", "ffprobe"):
        if not shutil.which(tool):
            raise Fail(1, f"Нет {tool} в PATH: установите ffmpeg")


def ff(args: list, capture: bool = False) -> bytes:
    cmd = ["ffmpeg", "-nostdin", "-y", "-loglevel", "error", *args]
    r = subprocess.run(cmd, capture_output=True)
    if r.returncode != 0:
        tail = r.stderr.decode("utf-8", "replace")[-600:]
        raise Fail(6, f"ffmpeg завершился с кодом {r.returncode}: {tail}")
    return r.stdout if capture else b""


def probe(path: Path) -> dict:
    r = subprocess.run(
        ["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
         "stream=codec_name,pix_fmt,width,height:format=duration,size", "-of", "json", str(path)],
        capture_output=True, text=True, encoding="utf-8",
    )
    if r.returncode != 0 or not r.stdout.strip():
        raise Fail(6, f"ffprobe не прочитал {path}: {r.stderr.strip()[-300:]}")
    data = json.loads(r.stdout)
    if not data.get("streams"):
        raise Fail(6, f"в {path} нет видео")
    s, f = data["streams"][0], data["format"]
    return {"w": s["width"], "h": s["height"], "codec": s["codec_name"], "pix_fmt": s["pix_fmt"],
            "duration": float(f["duration"]), "size": int(f["size"])}


def grab(raw: Path, t: float, w: int, h: int, out: Path, tmax: float = None):
    """Один кадр записи в момент t (точный поиск: -ss до -i), приведённый к размеру ролика.

    tmax — длина записи: кадр за концом (хвост журнала) заменяется последним.
    """
    if tmax is not None:
        t = max(0.0, min(t, tmax - 0.1))
    ff(["-ss", f"{t:.3f}", "-i", str(raw), "-frames:v", "1", "-vf", f"scale={w}:{h}:flags=bicubic", str(out)])
    if not out.is_file():
        raise Fail(4, f"в записи нет кадра в {t:.2f} с")


def detect_offset(raw: Path, sync: dict) -> float:
    """off = время кадра красной вспышки в записи - sync.t. Вспышка в первых 10 с, иначе отказ."""
    vf = f"fps={SYNC_FPS},crop='min(64,iw)':ih:0:0,scale=1:1:flags=area"
    data = ff(["-t", str(SYNC_WINDOW), "-i", str(raw), "-vf", vf, "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
              capture=True)
    samples = [tuple(data[i:i + 3]) for i in range(0, len(data) - 2, 3)]
    idx = op.find_flash_frame(samples, sync["color"])
    if idx is None:
        raise Fail(2, f"Метка синхронизации {sync['color']} не найдена в первых {SYNC_WINDOW:.0f} с записи: "
                      "монтаж отказывает (сырой ролик остаётся)")
    return idx / SYNC_FPS - sync["t"]


# ── Проверки годности (design §3.4) ──────────────────────────────────────
def diff_ratio(a: Image.Image, b: Image.Image) -> float:
    """Доля заметно изменившихся пикселей между двумя кадрами (в уменьшенном сером виде)."""
    size = (192, 108)
    d = ImageChops.difference(a.convert("L").resize(size), b.convert("L").resize(size))
    hist = d.histogram()
    return sum(hist[25:]) / (size[0] * size[1])


def is_uniform(img: Image.Image) -> bool:
    """Область одной заливкой — под стрелкой пусто (та же логика, что macshots.is_uniform)."""
    lo, hi = img.convert("L").getextrema()
    return hi - lo <= 2


def check_duration(dur: float) -> list:
    """Длина ролика: вне 30-95 с — предупреждение, больше 120 с — отказ (код 3)."""
    if dur > HARD_DUR:
        raise Fail(3, f"Ролик {dur:.0f} с — длиннее {HARD_DUR:.0f} с")
    if not MIN_DUR <= dur <= MAX_DUR:
        return [f"длина {dur:.1f} с вне {MIN_DUR:.0f}-{MAX_DUR:.0f} с"]
    return []


def target_matches(frame: Image.Image, shot: Image.Image, rect: tuple) -> bool:
    """Кусок кадра под рамкой совпадает с куском снимка и не однороден: цель в записи там же, где на снимке."""
    box = tuple(round(v) for v in rect)
    if box[0] < 0 or box[1] < 0 or box[2] > frame.width or box[3] > frame.height or box[2] <= box[0] or box[3] <= box[1]:
        return False
    a, b = frame.convert("L").crop(box), shot.convert("L").resize(frame.size).crop(box)
    if is_uniform(a):  # пустой фон совпадает с чем угодно
        return False
    hist = ImageChops.difference(a, b).histogram()
    return sum(hist[40:]) / (a.width * a.height) <= TARGET_SAME


def find_target(base: Image.Image, s: op.Seg, shots: list, used: set):
    """Рамка цели, которую журнал не записал: из снимка шага, чья метка названа в подписи; None — нет.

    Снимок, не прошедший проверку пикселями, остаётся свободным для другого стоп-кадра.
    """
    cand = overlay_fit.shot_for_target(s.target.caption, shots, used)
    if cand is None:
        return None
    shot = Image.open(cand["path"]).convert("RGB")
    k = base.width / shot.width
    x, y, w, h = (v * k for v in cand["rect"])
    rect = (x, y, x + w, y + h)
    if not target_matches(base, shot, rect):
        return None
    used.add(cand["name"])
    return rect


def check_holds(plan: op.Plan, j: op.Journal, raw_frames: dict, raw: Path, work: Path) -> tuple:
    """Предупреждения по стоп-кадрам и пометка «со стрелкой» на каждом. raw_frames: seg -> Image."""
    warns, arrows = [], {}
    prev = None
    for n, s in enumerate(plan.holds(), 1):
        img = raw_frames[id(s)]
        e = s.hold_ev
        arrows[id(s)] = s.target.rect is not None
        mv = next((m for m in j.events if m["ev"] == "move" and m["t0"] >= e["t"]), None)
        if mv is not None and not s.shot:
            after = work / f"after-{n:02d}.png"
            grab(raw, mv["t1"] + 0.5 + (s.src0 - e["t"]), plan.out_w, plan.out_h, after, plan.rec_end)
            if diff_ratio(img, Image.open(after)) <= CHANGED:
                warns.append(f"стоп-кадр {n} (строка {s.line}): ничего не произошло после действия")
        if prev is not None and diff_ratio(prev, img) < DIFFERENT:
            warns.append(f"стоп-кадры {n - 1} и {n}: запись застыла (кадры почти одинаковы)")
        if s.target.rect:
            x0, y0, x1, y1 = (round(v) for v in s.target.rect)
            if x1 > x0 and y1 > y0 and is_uniform(img.crop((x0, y0, x1, y1))):
                warns.append(f"стоп-кадр {n} (строка {s.line}): под стрелкой пусто (область однородна)")
        if s.target.rect is None and not e.get("keys"):
            warns.append(f"стоп-кадр {n} (строка {s.line}): стоп-кадр без цели — подпись без стрелки")
        prev = img
    return warns, arrows


# ── Сборка ───────────────────────────────────────────────────────────────
def build_graph(plan: op.Plan, assets: list, masks: list) -> tuple:
    """Входы ffmpeg (кроме записи) и filter_complex. assets[i] — PNG отрезка i или None."""
    w, h, fps = plan.out_w, plan.out_h, plan.fps
    scale = f"scale={w}:{h}:flags=bicubic:out_color_matrix=bt709"
    boxes = "".join(f",drawbox=x={x0}:y={y0}:w={x1 - x0}:h={y1 - y0}:color=0x808080:t=fill" for x0, y0, x1, y1 in masks)
    # запись с переменной частотой (экран без изменений кадров не пишет): без выравнивания короткий
    # живой отрезок на статичном экране оказывается пустым и выпадает из ролика; по той же причине
    # окно trim не уже одного кадра (отрезок в 0,05 с иначе не ловит ни одного кадра)
    nraw = sum(1 for s in plan.segs if s.kind in ("live", "fast"))
    outs = "".join(f"[c{i}]" for i in range(nraw))
    inputs, parts, labels = [], ([f"[0:v]fps={fps},split={nraw}{outs}"] if nraw else []), []
    k = 0  # номер входа; 0 — запись
    r = 0  # номер ветки записи
    for i, s in enumerate(plan.segs):
        png = assets[i]
        if png is not None:
            k += 1
            inputs += ["-loop", "1", "-framerate", str(fps), "-t", f"{s.frames / fps + 0.5:.3f}", "-i", str(png)]
        end = f"fps={fps},trim=end_frame={s.frames},setsar=1,format=yuv420p"
        if s.kind in ("hold", "card"):
            parts.append(f"[{k}:v]{scale},{end}[v{i}]")
        else:
            speed = f",setpts=PTS*{s.dur / (s.src1 - s.src0):.6f}" if s.kind == "fast" else ""
            pad = "tpad=stop_mode=clone:stop=-1"
            main = (f"[c{r}]trim=start={s.src0:.3f}:end={max(s.src1, s.src0 + 1.01 / fps):.3f},setpts=PTS-STARTPTS{speed},"
                    f"{scale}{boxes},fps={fps},{pad},trim=end_frame={s.frames},setsar=1,format=yuv420p")
            r += 1
            if png is None:
                parts.append(f"{main}[v{i}]")
            else:  # плашки: прозрачный PNG поверх живого отрезка
                parts.append(f"{main}[m{i}];[m{i}][{k}:v]overlay=0:0:format=auto:shortest=1,"
                             f"format=yuv420p,trim=end_frame={s.frames}[v{i}]")
        labels.append(f"[v{i}]")
    graph = ";".join(parts) + ";" + "".join(labels) + f"concat=n={len(labels)}:v=1:a=0,setpts=N/({fps}*TB)[v]"
    return inputs, graph  # setpts=N: у однокадровых отрезков concat путает метки, кадры иначе теряются


def make_assets(plan: op.Plan, painter, raw: Path, masks: list, layers: bool, work: Path, frames_dir,
                prefix: str = "", shots: list = (), used: set = None) -> tuple:
    """PNG для отрезков + сырые кадры стоп-кадров (для проверок)."""
    used = set() if used is None else used
    assets, raw_frames, cache = [], {}, {}
    n = 0
    for s in plan.segs:
        if s.kind == "card":
            p = work / f"card-{len(assets):02d}.png"
            painter.card(s.title).save(p)
            assets.append(p)
        elif s.kind == "hold":
            n += 1
            if s.shot:  # запись кончилась раньше: кадр — из снимка шага
                base = Image.open(s.shot).convert("RGB").resize((plan.out_w, plan.out_h), Image.BICUBIC)
            else:
                rp = work / f"raw-{n:02d}.png"
                grab(raw, s.src0, plan.out_w, plan.out_h, rp, plan.rec_end)
                base = Image.open(rp).convert("RGB")
            raw_frames[id(s)] = base
            if s.target.rect is None and shots:
                rect = find_target(base, s, shots, used)
                if rect:
                    s.target = op.Target(rect, s.target.caption)
            p = work / f"hold-{n:02d}.png"
            painter.hold_frame(base, s, masks, layers).save(p)
            if frames_dir:
                shutil.copyfile(p, frames_dir / f"{prefix}hold-{n:02d}.png")
            assets.append(p)
        elif s.plates and layers:
            if s.plates not in cache:
                p = work / f"plates-{len(cache):02d}.png"
                painter.plates_layer(s.plates)[0].save(p)
                cache[s.plates] = p
            assets.append(cache[s.plates])
        else:
            assets.append(None)
    return assets, raw_frames


def encode(raw: Path, inputs: list, graph: str, crf: int, work: Path, out: Path):
    script = work / "graph.txt"
    script.write_text(graph, encoding="utf-8")
    ff(["-i", str(raw), *inputs, "-filter_complex_script", str(script), "-map", "[v]", "-an",
        "-c:v", "libx264", "-preset", "medium", "-crf", str(crf), "-pix_fmt", "yuv420p", "-r", str(op.FPS),
        "-colorspace", "bt709", "-color_primaries", "bt709", "-color_trc", "bt709",
        "-movflags", "+faststart", str(out)])


def is_faststart(path: Path) -> bool:
    data = path.read_bytes()
    moov, mdat = data.find(b"moov"), data.find(b"mdat")
    return 0 <= moov < mdat


def locate(path: Path, raw_arg: str) -> tuple:
    if path.is_dir():
        jp = next((p for p in (path / "video" / "steps.jsonl", path / "steps.jsonl") if p.is_file()), None)
        if jp is None:
            raise Fail(2, f"В {path} нет video/steps.jsonl")
    elif path.is_file():
        jp = path
    else:
        raise Fail(2, f"Не найдено: {path}")
    raw = Path(raw_arg) if raw_arg else jp.parent / "raw.mp4"
    if not raw.is_file():
        raise Fail(2, f"Нет записи {raw}: укажите --raw")
    return jp, raw


def report(plan: op.Plan, arrows: dict, warns: list, off: float):
    print(f"Смещение записи off = {off:.3f} с; ролик {plan.out_w}x{plan.out_h}, "
          f"{plan.duration:.1f} с ({plan.total_frames} кадров)")
    print(f"{'#':>3}  {'тип':<5} {'с':>7} {'длит., с':>8}  описание")
    t = 0.0
    for i, s in enumerate(plan.segs, 1):
        if s.kind == "hold":
            what = f"{s.target.caption} [{'со стрелкой' if arrows.get(id(s)) else 'без стрелки'}]"
        elif s.kind == "card":
            what = s.title
        else:
            what = ", ".join(p.text for p in s.plates)
        print(f"{i:>3}  {s.kind:<5} {t:>7.2f} {s.frames / plan.fps:>8.2f}  {what}")
        t += s.frames / plan.fps
    holds = plan.holds()
    with_arrow = sum(1 for s in holds if s.target.rect)
    print(f"Стоп-кадров {len(holds)}, со стрелкой {with_arrow}, без {len(holds) - with_arrow}")
    for w in warns:
        print(f"ПРЕДУПРЕЖДЕНИЕ: {w}")


# Имя ролика по названию главы (подстрока без регистра, первое совпадение); номер главы для имени
# не годится: в 0.11 первой идёт «Распакуйте архив», в 0.10 — «Первый запуск».
CHAPTER_SUFFIXES = (
    ("распакуйте архив", "unzip"),
    ("прямо из архива", "fromzip"),
    ("повреждено", "damaged"),
    ("выполнить в любом случае", "unblock"),
    ("первый запуск", "start"),
)


def chapter_suffix(title: str, n: int) -> str:
    low = " ".join((title or "").casefold().split())
    for key, suffix in CHAPTER_SUFFIXES:
        if key in low:
            return suffix
    return f"ch{n}"


def chapter_out(out: Path, n: int, title: str = "") -> Path:
    return out.with_name(f"{out.stem}-{chapter_suffix(title, n)}{out.suffix}")


def jobs_for(j: op.Journal, args, out: str) -> list:
    """Что собирать: [(журнал, выходной файл или None, префикс кадров)]."""
    if not args.split and not args.chapter:
        return [(j, out, "")]
    parts = op.split_chapters(j)
    if not parts:
        raise Fail(2, "В журнале нет глав (chapter): делить нечего")
    if args.chapter:
        if not 1 <= args.chapter <= len(parts):
            raise Fail(2, f"--chapter {args.chapter}: в журнале глав {len(parts)}")
        return [(parts[args.chapter - 1], out, "")]
    base = Path(out) if out else None
    res, seen = [], set()
    for n, p in enumerate(parts, 1):
        target = chapter_out(base, n, p.events[0].get("title", "")) if base else None
        if target in seen:  # две главы с одним названием не затирают друг друга
            target = chapter_out(base, n)
        seen.add(target)
        res.append((p, str(target) if target else None, f"{base.stem if base else 'ch'}-{n}-"))
    return res


def render(args, j, plan, raw, info, out, frames_dir, prefix, shots, used) -> tuple:
    """Один ролик: стоп-кадры, проверки, ffmpeg. Возвращает (предупреждения, пометки стрелок)."""
    masks = [overlay_draw.parse_mask(m) for m in args.mask]
    painter = overlay_draw.Painter(plan.out_w, plan.out_h, args.font, args.font_sym)
    warns = check_duration(plan.duration)
    with tempfile.TemporaryDirectory(prefix="overlay-") as tmp:
        work = Path(tmp)
        assets, raw_frames = make_assets(plan, painter, raw, masks, not args.no_layers, work, frames_dir, prefix,
                                         shots, used)
        w2, arrows = check_holds(plan, j, raw_frames, raw, work)
        warns += w2
        if not args.check:
            if not out:
                raise Fail(2, "Нужен --out (или --check)")
            out = Path(out)
            out.parent.mkdir(parents=True, exist_ok=True)
            inputs, graph = build_graph(plan, assets, masks)
            for crf in CRFS:
                encode(raw, inputs, graph, crf, work, out)
                if out.stat().st_size <= MAX_MB * 1024 * 1024:
                    break
            else:
                raise Fail(5, f"{out.name} больше {MAX_MB} МБ даже при crf {CRFS[-1]}")
            warns += check_output(out, plan)
    return warns, arrows


def load_shots(path: Path) -> list:
    """Снимки шагов артефакта (shots.json): кадры за концом записи и рамки целей, которых нет в журнале."""
    f = path / "shots.json" if path.is_dir() else None
    if f is None or not f.is_file():
        return []
    try:
        return overlay_fit.parse_shots(json.loads(f.read_text(encoding="utf-8")), path)
    except (ValueError, OSError):
        return []


def run(args) -> int:
    need_tools()
    jp, raw = locate(Path(args.path), args.raw)
    try:
        j = op.parse_journal(jp.read_text(encoding="utf-8"), require_sync=args.offset is None)
    except ValueError as exc:
        raise Fail(2, f"{jp.name}: {exc}") from None
    info = probe(raw)
    if args.offset is not None:
        off = args.offset
    else:
        off = detect_offset(raw, j.sync())
    out_w = args.width
    out_h = round(info["h"] * out_w / info["w"] / 2) * 2
    frames_dir = Path(args.frames) if args.frames else None
    if frames_dir:
        frames_dir.mkdir(parents=True, exist_ok=True)
    shots = load_shots(Path(args.path))
    used = set()
    # сначала планы и длины всех роликов: отказ по длине — до долгого кодирования
    jobs = []
    for jj, out, prefix in jobs_for(j, args, args.out):
        plan = op.build_plan(jj, off, out_w, out_h)
        title = next((e["title"] for e in jj.events if e["ev"] == "chapter"), "")
        try:
            plan, fit_warns = overlay_fit.fit_to_record(plan, jj, off, info["duration"], shots, used)
        except overlay_fit.RecordShort as exc:
            raise Fail(4, f"журнал длиннее записи: {title or 'ролик'}: {exc}") from None
        try:
            check_duration(plan.duration)
        except Fail as exc:
            raise Fail(exc.code, f"{title or 'ролик'}: {exc}") from None
        jobs.append((jj, plan, out, prefix, title, fit_warns))
    for jj, plan, out, prefix, title, fit_warns in jobs:
        warns, arrows = render(args, jj, plan, raw, info, out, frames_dir, prefix, shots, used)
        if len(jobs) > 1 or args.chapter:
            print(f"== {title}: {out or '(без файла)'}")
        report(plan, arrows, fit_warns + warns, off)
    return 0


def check_output(out: Path, plan: op.Plan) -> list:
    """Формат итога: H.264, yuv420p, faststart, длина по плану."""
    warns = []
    p = probe(out)
    if p["codec"] != "h264" or p["pix_fmt"] != "yuv420p":
        warns.append(f"формат {p['codec']}/{p['pix_fmt']}, ожидался h264/yuv420p")
    if not is_faststart(out):
        warns.append("moov не в начале файла (нет faststart)")
    if abs(p["duration"] - plan.duration) > 2 / plan.fps:
        warns.append(f"длина файла {p['duration']:.2f} с, по плану {plan.duration:.2f} с")
    return warns


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description="Монтаж ролика-инструкции по записи и журналу шагов")
    ap.add_argument("path", help="папка артефакта или steps.jsonl")
    ap.add_argument("--raw", default="", help="сырая запись (по умолчанию raw.mp4 рядом с журналом)")
    ap.add_argument("--out", default="", help="итоговый mp4")
    ap.add_argument("--width", type=int, default=1920, help="ширина ролика, px")
    ap.add_argument("--check", action="store_true", help="только проверки и отчёт, без сборки")
    ap.add_argument("--frames", default="", help="папка для PNG стоп-кадров")
    ap.add_argument("--mask", action="append", default=[], help="x,y,w,h в пикселях кадра: серая заливка")
    ap.add_argument("--no-layers", action="store_true", help="без плашек и стрелок")
    ap.add_argument("--offset", type=float, default=None, help="смещение записи, с (вместо поиска вспышки)")
    ap.add_argument("--split", action="store_true", help="по ролику на главу, имя по названию: <out>-unzip.mp4, -start, -damaged, -fromzip, -unblock, -chN")
    ap.add_argument("--chapter", type=int, default=0, help="собрать только главу N (с 1)")
    ap.add_argument("--font", default="", help="свой .ttf с кириллицей")
    ap.add_argument("--font-sym", default="", help="свой .ttf с символами клавиш (⌘)")
    args = ap.parse_args(argv)
    if args.split and args.chapter:
        ap.error("--split и --chapter вместе не нужны")
    if args.width < 160 or args.width % 2:
        ap.error("--width: чётное число не меньше 160")
    try:
        return run(args)
    except Fail as exc:
        print(f"Ошибка: {exc}", file=sys.stderr)
        return exc.code


if __name__ == "__main__":
    for _s in (sys.stdout, sys.stderr):
        _s.reconfigure(encoding="utf-8")
    sys.exit(main())
