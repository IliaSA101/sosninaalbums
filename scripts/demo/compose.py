"""Сборка роликов README из каталога прогона (102, Д2): mp4 (GStreamer, запас — ffmpeg), WebP-заставка, снимок, лист кадров.

Вход — каталог прогона по контракту Д1 → Д2 (design §2.2: A/ и B/ с кадрами, frames.jsonl, marks.jsonl, meta.json).
Сборка — только файлами: ни стенда, ни браузера. Проверки готового каталога и summary.md — verify.py.
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
import subprocess
from dataclasses import dataclass, field
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

import timeline as tl

ROOT = Path(__file__).resolve().parents[2]  # от пути скрипта, не из окружения: пустая переменная не даёт корень диска
GOTMP = ROOT / ".gotmp"
FPS = 30
MIB = 1024 * 1024

MP4_MAX = int(9.5 * MIB)  # вложение GitHub — до 10 МБ на бесплатном плане
MP4_Q = (22, 25, 28, 31)  # CRF: при превышении бюджета q+3 до 31, дальше отказ
WEBP_MAX = int(2.5 * MIB)
WEBP_STEPS = ((15, 960, 72), (12, 960, 72), (12, 800, 72), (12, 800, 60))  # (к/с, ширина, quality): лестница бюджета
SHOT_MAX = 400 * 1024
SHOT_Q = (88, 82, 76, 70)

# Заставка: отрезки (сцена, метка-от, метка-до). Источник истины — scenes.HERO (Д1); запас — этот список.
DEFAULT_HERO = (("B", "select", "variant-2"), ("A", "flip-1", "flip-4"))


class Refuse(Exception):
    """Нет входных данных или инструмента (код выхода 2)."""

    code = 2


class BudgetError(Exception):
    """Файл не лёг в бюджет размера даже на последней ступени (код выхода 5)."""

    code = 5


@dataclass
class Scene:
    name: str
    dir: Path
    frames: list[tl.Frame]
    marks: list[tl.Mark]

    def frame_path(self, i: int) -> Path:
        return self.dir / "frames" / f"{i:06d}.jpg"

    def window(self, a: str = "start", b: str = "end") -> tuple[float, float]:
        return tl.cut_by_marks(self.marks, a, b)

    def _size(self, i: int) -> int | None:
        try:
            return self.frame_path(i).stat().st_size
        except OSError:
            return None

    def _shown(self, t_from: float, t_ref: float, tail: float = 6.0) -> float:
        """Время первого кадра после t_from, чей размер файла заметно (≥ 25 %) отличается от кадра в t_ref.

        Метка результата ставится по DOM, а страница на wails dev дорисовывает развороты ещё секунды: кадры в это время —
        прежнее окно. Новая картинка (развороты с фото вместо окна) весит иначе. Нет файлов или нет скачка за `tail` с — t_from.
        """
        ref = self._size(tl.frame_at(self.frames, t_ref))
        if not ref:
            return t_from
        for f in self.frames:
            if f.t < t_from:
                continue
            if f.t > t_from + tail:
                break
            sz = self._size(f.i)
            if sz and abs(sz - ref) >= 0.25 * ref:
                return f.t
        return t_from

    def cuts(self) -> list[tuple[float, float]]:
        """Вырезки ожидания по меткам (нет метки в журнале — вырезки нет): от «щелчок + оставить» до первого кадра нового вида.

        Конец вырезки — метка результата, сдвинутая до первого кадра с новой картинкой (_shown): без этого после метки
        висели бы кадры прежнего окна.
        """
        out = []
        for a, b, keep in cut_rules().get(self.name, ()):
            try:
                ta = tl.mark_time(self.marks, a)
                tb = tl.mark_time(self.marks, b, after=ta)
            except tl.JournalError:
                continue
            tb = self._shown(tb, ta + keep)
            if tb - ta > keep:
                out.append((ta + keep, tb))
        return out

    def kept(self, slow: float = 1.0) -> float:
        """Длительность ролика с вырезками, секунды (в медленном режиме время делится на slow)."""
        t0, t1 = self.window()
        return sum(b - a for a, b in tl.kept_spans(t0, t1, self.cuts())) / slow


@dataclass
class Run:
    root: Path
    scenes: dict[str, Scene]
    meta: dict = field(default_factory=dict)

    @property
    def slow(self) -> float:
        v = self.meta.get("slow", 1)
        return float(v) if isinstance(v, (int, float)) and v >= 1 else 1.0


# Вырезки ожидания: сцена -> [(метка щелчка, метка результата, сколько секунд оставить после щелчка)].
# Источник истины — scenes.CUTS (Д1); запас — этот словарь. Приложение считает и рисует развороты ~4 с, окно всё это время на экране.
DEFAULT_CUTS = {"A": (("build-click", "built", 0.5), ("rebuild-click", "rebuilt", 0.5))}


def cut_rules() -> dict:
    try:
        import scenes  # type: ignore[import-not-found]

        return {k: tuple(tuple(r) for r in v) for k, v in scenes.CUTS.items()}  # type: ignore[attr-defined]
    except (ImportError, AttributeError):
        return DEFAULT_CUTS


def hero_segments() -> tuple[tuple[str, str, str], ...]:
    """Отрезки заставки из scenes.HERO; нет модуля Д1 (или он не импортируется) — запасной список."""
    try:
        import scenes  # type: ignore[import-not-found]

        return tuple(tuple(s) for s in scenes.HERO)  # type: ignore[attr-defined]
    except (ImportError, AttributeError):
        return DEFAULT_HERO


def inside_gotmp(p: Path) -> bool:
    """Путь (после разрешения ссылок) строго внутри .gotmp этой рабочей копии."""
    try:
        return p.resolve().is_relative_to(GOTMP.resolve()) and p.resolve() != GOTMP.resolve()
    except OSError:
        return False


def load_run(root: Path) -> Run:
    """Читает журналы сцен A и B; любое нарушение контракта — Refuse с понятной строкой."""
    if not root.is_dir():
        raise Refuse(f"нет каталога прогона: {root}")
    scenes: dict[str, Scene] = {}
    for name in ("A", "B"):
        d = root / name
        try:
            frames = tl.parse_frames((d / "frames.jsonl").read_text(encoding="utf-8"))
            marks = tl.parse_marks((d / "marks.jsonl").read_text(encoding="utf-8"))
        except (OSError, tl.JournalError) as e:
            raise Refuse(f"сцена {name}: {e}") from None
        scenes[name] = Scene(name, d, frames, marks)
    meta: dict = {}
    mp = root / "meta.json"
    if mp.is_file():
        try:
            meta = json.loads(mp.read_text(encoding="utf-8"))
        except json.JSONDecodeError as e:
            raise Refuse(f"meta.json: {e.msg}") from None
    return Run(root, scenes, meta)


# ---- Кадры постоянной частоты ----------------------------------------------------------------


def stage_frames(sc: Scene, idx: list[int], dest: Path) -> Path:
    """Последовательность dest/000001.jpg… по выбранным кадрам: жёсткие ссылки, нет — копии."""
    if dest.exists():
        shutil.rmtree(dest)
    dest.mkdir(parents=True)
    for k, i in enumerate(idx, 1):
        src = sc.frame_path(i)
        if not src.is_file():
            raise Refuse(f"сцена {sc.name}: нет файла кадра {src.name}")
        try:
            os.link(src, dest / f"{k:06d}.jpg")
        except OSError:
            shutil.copy2(src, dest / f"{k:06d}.jpg")
    return dest


# ---- mp4 -------------------------------------------------------------------------------------


def mp4_backend() -> str:
    """GStreamer (решение 102-7), нет его — ffmpeg."""
    if shutil.which("gst-launch-1.0"):
        return "gst"
    if shutil.which("ffmpeg"):
        return "ffmpeg"
    raise Refuse("нет ни gst-launch-1.0 (GStreamer), ни ffmpeg: mp4 собрать нечем")


def _run(cmd: list[str]) -> None:
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=900, check=False)
    if r.returncode != 0:
        raise Refuse(f"{cmd[0]} завершился с кодом {r.returncode}: {(r.stderr or r.stdout).strip()[-300:]}")


def encode_mp4(seq: Path, out: Path, q: int, backend: str) -> None:
    """Последовательность seq/%06d.jpg (30 к/с) → H.264 High, faststart; q — CRF."""
    if backend == "gst":
        cmd = [
            "gst-launch-1.0", "-q", "multifilesrc", f'location="{seq}/%06d.jpg"', "index=1",
            f"caps=image/jpeg,framerate={FPS}/1", "!", "jpegdec", "!", "videoconvert", "!", "video/x-raw,format=I420", "!",
            "x264enc", "speed-preset=slow", "pass=qual", f"quantizer={q}", "key-int-max=60", "!",
            "video/x-h264,profile=high", "!", "mp4mux", "faststart=true", "!", "filesink", f'location="{out}"',
        ]
    else:
        cmd = [
            "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-framerate", str(FPS), "-i", f"{seq}/%06d.jpg",
            "-c:v", "libx264", "-preset", "slow", "-crf", str(q), "-pix_fmt", "yuv420p", "-profile:v", "high",
            "-g", "60", "-movflags", "+faststart", str(out),
        ]
    out.unlink(missing_ok=True)
    _run(cmd)


def build_mp4(sc: Scene, out: Path, slow: float = 1.0, backend: str | None = None, limit: int = MP4_MAX) -> dict:
    """Ролик сцены: окно start…end → сетка 30 к/с → кодирование; бюджет — лестница CRF. Возвращает {q, bytes, frames}."""
    backend = backend or mp4_backend()
    t0, t1 = sc.window()
    idx = tl.resample_cuts(sc.frames, t0, t1, sc.cuts(), FPS, slow)
    seq = stage_frames(sc, idx, out.parent / f".seq-{sc.name}")
    try:
        for q in MP4_Q:
            encode_mp4(seq, out, q, backend)
            size = out.stat().st_size
            if size <= limit:
                return {"q": q, "bytes": size, "frames": len(idx), "backend": backend}
        raise BudgetError(f"{out.name}: {size / MIB:.1f} МБ при CRF {MP4_Q[-1]} — больше {limit / MIB:.1f} МБ")
    finally:
        shutil.rmtree(seq, ignore_errors=True)


def probe_mp4(path: Path) -> dict:
    """Свойства файла: codec, profile, w, h, fps, dur (секунды). gst-discoverer-1.0, нет — ffprobe."""
    if shutil.which("gst-discoverer-1.0"):
        r = subprocess.run(["gst-discoverer-1.0", str(path)], capture_output=True, text=True, timeout=120, check=False)
        t = r.stdout
        dur = re.search(r"Duration: (\d+):(\d+):([\d.]+)", t)
        w, h = re.search(r"Width: (\d+)", t), re.search(r"Height: (\d+)", t)
        fr = re.search(r"Frame rate: (\d+)/(\d+)", t)
        cod = re.search(r"video #\d+: ([^\n(]+?)\s*(?:\(([^)]*)\))?\s*\n", t)
        if not (dur and w and h and fr and cod):
            raise Refuse(f"gst-discoverer-1.0 не опознал {path.name}")
        return {
            "codec": cod.group(1).strip(), "profile": (cod.group(2) or "").replace(" Profile", ""),
            "w": int(w.group(1)), "h": int(h.group(1)), "fps": int(fr.group(1)) / int(fr.group(2)),
            "dur": int(dur.group(1)) * 3600 + int(dur.group(2)) * 60 + float(dur.group(3)),
        }
    if shutil.which("ffprobe"):
        r = subprocess.run(
            ["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
             "stream=codec_name,profile,width,height,r_frame_rate:format=duration", "-of", "json", str(path)],
            capture_output=True, text=True, timeout=120, check=False,
        )
        try:
            j = json.loads(r.stdout)
            s = j["streams"][0]
            n, d = s["r_frame_rate"].split("/")
            return {
                "codec": "H.264" if s["codec_name"] == "h264" else s["codec_name"], "profile": s.get("profile", ""),
                "w": int(s["width"]), "h": int(s["height"]), "fps": int(n) / int(d), "dur": float(j["format"]["duration"]),
            }
        except (ValueError, KeyError, IndexError):
            raise Refuse(f"ffprobe не опознал {path.name}") from None
    raise Refuse("нет gst-discoverer-1.0 и ffprobe: свойства mp4 проверить нечем")


# ---- WebP-заставка, снимок ---------------------------------------------------------------------


def _digest(p: Path, cache: dict[Path, str]) -> str:
    if p not in cache:
        cache[p] = hashlib.md5(p.read_bytes()).hexdigest()
    return cache[p]


def hero_items(run: Run, segments, fps: int) -> tuple[list[tuple[str, int]], dict[str, Path]]:
    """Кадры заставки на сетке fps: ([(ключ = хэш файла, мс)], ключ → файл); подряд одинаковые слиты — покой ничего не стоит."""
    cache: dict[Path, str] = {}
    paths: dict[str, Path] = {}
    items: list[tuple[str, int]] = []
    for scene, a, b in segments:
        sc = run.scenes.get(scene)
        if sc is None:
            raise Refuse(f"заставка: нет сцены {scene}")
        try:
            t0, t1 = sc.window(a, b)
        except tl.JournalError as e:
            raise Refuse(f"заставка, сцена {scene}: {e}") from None
        idx = tl.resample(sc.frames, t0, t1, fps, run.slow)
        for i, ms in zip(idx, tl.grid_ms(len(idx), fps)):
            p = sc.frame_path(i)
            if not p.is_file():
                raise Refuse(f"сцена {scene}: нет файла кадра {p.name}")
            key = _digest(p, cache)
            paths.setdefault(key, p)
            items.append((key, ms))
    return tl.dedupe(items), paths  # type: ignore[return-value]


def build_webp(run: Run, out: Path, segments=None, limit: int = WEBP_MAX, steps=WEBP_STEPS) -> dict:
    """Анимированный WebP из отрезков заставки; бюджет — лестница (к/с, ширина, quality)."""
    segments = segments or hero_segments()
    size = 0
    for fps, width, quality in steps:
        items, paths = hero_items(run, segments, fps)
        decoded: dict[str, Image.Image] = {}
        frames = []
        for key, _ in items:
            if key not in decoded:
                with Image.open(paths[key]) as im:
                    im = im.convert("RGB")
                    decoded[key] = im.resize((width, round(width * im.height / im.width)), Image.LANCZOS)
            frames.append(decoded[key])
        durs = [ms for _, ms in items]
        out.unlink(missing_ok=True)
        frames[0].save(out, save_all=True, append_images=frames[1:], duration=durs, loop=0, quality=quality, method=6)
        size = out.stat().st_size
        if size <= limit:
            return {"fps": fps, "width": width, "quality": quality, "bytes": size, "frames": len(frames), "ms": sum(durs)}
    raise BudgetError(f"{out.name}: {size / MIB:.1f} МБ на последней ступени — больше {limit / MIB:.1f} МБ")


def build_still(run: Run, out: Path, mark: str = "still", limit: int = SHOT_MAX) -> dict:
    """Снимок README — кадр метки `still` записи B, JPEG; не лёг в бюджет — ниже quality."""
    sc = run.scenes["B"]
    try:
        i = tl.frame_at(sc.frames, tl.mark_time(sc.marks, mark))
    except tl.JournalError as e:
        raise Refuse(f"снимок: {e}") from None
    with Image.open(sc.frame_path(i)) as im:
        im = im.convert("RGB")
    size = 0
    for q in SHOT_Q:
        im.save(out, "JPEG", quality=q, optimize=True)
        size = out.stat().st_size
        if size <= limit:
            return {"quality": q, "bytes": size, "size": im.size}
    raise BudgetError(f"{out.name}: {size // 1024} КБ при quality {SHOT_Q[-1]} — больше {limit // 1024} КБ")


# ---- Лист кадров ------------------------------------------------------------------------------


def _font(size: int):
    for p in ("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf"):
        if Path(p).is_file():
            return ImageFont.truetype(p, size)
    return ImageFont.load_default(size)


def sheet_moments(sc: Scene) -> list[tuple[float, str]]:
    """Моменты листа: кадр раз в секунду от start и кадр у каждой метки (подпись «м:сс · метка»)."""
    t0, t1 = sc.window()
    moments = [(t0 + s, "") for s in range(int(t1 - t0) + 1) if t0 + s <= t1]
    moments += [(m.t, m.name) for m in sc.marks]
    return sorted(moments, key=lambda x: (x[0], x[1] == ""))


def build_sheet(sc: Scene, out: Path, cols: int = 6, tile: tuple[int, int] = (320, 200)) -> int:
    """sheet-<сцена>.jpg: плитки с подписями; возвращает число плиток."""
    t0, _ = sc.window()
    moments = sheet_moments(sc)
    label_h = 22
    rows = -(-len(moments) // cols)
    sheet = Image.new("RGB", (cols * tile[0], rows * (tile[1] + label_h)), (20, 20, 22))
    d = ImageDraw.Draw(sheet)
    font = _font(14)
    for n, (t, name) in enumerate(moments):
        x, y = (n % cols) * tile[0], (n // cols) * (tile[1] + label_h)
        with Image.open(sc.frame_path(tl.frame_at(sc.frames, t))) as im:
            im.draft("RGB", (tile[0] * 2, tile[1] * 2))
            sheet.paste(im.convert("RGB").resize(tile, Image.LANCZOS), (x, y + label_h))
        sec = max(t - t0, 0.0)
        d.text((x + 4, y + 3), f"{int(sec // 60)}:{sec % 60:04.1f} · {name}" if name else f"{int(sec // 60)}:{sec % 60:04.1f}", fill=(255, 220, 90) if name else (200, 200, 200), font=font)
    sheet.save(out, "JPEG", quality=82)
    return len(moments)


# ---- Всё вместе --------------------------------------------------------------------------------


def compose_all(run: Run, backend: str | None = None, say=print) -> dict:
    """A.mp4, B.mp4, demo.webp, screenshot.jpg, sheet-A/B.jpg в каталог прогона; возвращает сведения для summary.md."""
    info: dict = {}
    backend = backend or mp4_backend()
    for name in ("A", "B"):
        say(f"{name}.mp4 …")
        info[f"mp4-{name}"] = build_mp4(run.scenes[name], run.root / f"{name}.mp4", run.slow, backend)
    say("demo.webp …")
    info["webp"] = build_webp(run, run.root / "demo.webp")
    say("screenshot.jpg …")
    info["still"] = build_still(run, run.root / "screenshot.jpg")
    for name in ("A", "B"):
        say(f"sheet-{name}.jpg …")
        info[f"sheet-{name}"] = build_sheet(run.scenes[name], run.root / f"sheet-{name}.jpg")
    (run.root / "build.json").write_text(json.dumps(info, ensure_ascii=False, indent=1), encoding="utf-8")
    return info


def publish(run_root: Path, docs_media: Path | None = None) -> list[Path]:
    """demo.webp и screenshot.jpg прогона → docs/media/ (в git)."""
    dest = docs_media or ROOT / "docs" / "media"
    dest.mkdir(parents=True, exist_ok=True)
    out = []
    for name in ("demo.webp", "screenshot.jpg"):
        src = run_root / name
        if not src.is_file():
            raise Refuse(f"нечего публиковать: нет {src}")
        out.append(Path(shutil.copy2(src, dest / name)))
    return out
