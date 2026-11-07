"""Синтетический каталог прогона по контракту Д1 → Д2 (design §2.2): для тестов и проверки сборки без стенда.

Кадры — «похожие на интерфейс»: тёмный фон, белый лист с цветными «фото», лента внизу, курсор. Кадр пишется только
пока идёт движение, как у screencast; в хвосте каждого интервала между метками страница «стоит» (кадров нет).
Запуск вручную: `python3 scripts/demo/synthrun.py <каталог-в-.gotmp> [секунд]`.
"""

from __future__ import annotations

import json
import random
import sys
from pathlib import Path

from PIL import Image, ImageDraw

T_BASE = 1_729_612_345.0  # произвольное «время эпохи» первого кадра

MARKS = {
    "A": ["start", "open-auto", "slider", "build", "built", "flip-1", "flip-2", "flip-3", "flip-4", "flip-5", "flip-6", "again", "rebuilt", "flip-7", "end"],
    "B": ["start", "select", "drag", "drop", "variant-1", "variant-2", "variant-3", "variant-4", "gallery", "pick", "add-one", "swap", "still", "end"],
}


def _backdrop(w: int, h: int, seed: int) -> Image.Image:
    """Фон с «фото»: гладкие цветные пятна (шум, растянутый бикубикой) — сжимаются как настоящие снимки."""
    rnd = random.Random(seed)
    im = Image.new("RGB", (w, h), (34, 34, 36))
    d = ImageDraw.Draw(im)
    d.rectangle((0, 0, w, 60), fill=(28, 28, 30))
    d.rectangle((w // 8, h // 8, w - w // 8, h * 7 // 10), fill=(250, 250, 250))
    for k in range(4):
        tile = Image.new("RGB", (12, 8))
        tile.putdata([tuple(rnd.randrange(40, 230) for _ in range(3)) for _ in range(96)])
        tile = tile.resize((w // 5, h // 2), Image.BICUBIC)
        im.paste(tile, (w // 8 + 20 + k * (w // 5 + 12), h // 8 + 20))
    for j in range(12):
        tile = Image.new("RGB", (6, 4))
        tile.putdata([tuple(rnd.randrange(40, 230) for _ in range(3)) for _ in range(24)])
        im.paste(tile.resize((w // 12 - 10, h // 9), Image.BICUBIC), (10 + j * (w // 12), h - h // 9 - 30))
    return im


def make_scene(root: Path, scene: str, seconds: float, fps: int = 20, w: int = 1600, h: int = 1000, t0: float = T_BASE) -> list[float]:
    """Пишет <root>/<scene>/{frames/, frames.jsonl, marks.jsonl, console.txt, privacy.txt}; возвращает времена меток."""
    base = root / scene
    (base / "frames").mkdir(parents=True, exist_ok=True)
    names = MARKS[scene]
    step = seconds / (len(names) - 1)
    mark_t = [t0 + 0.05 + k * step for k in range(len(names))]
    bg = _backdrop(w, h, seed=ord(scene))
    frames = []
    n = 0
    t = t0
    while t < mark_t[-1]:
        phase = ((t - mark_t[0]) % step) / step
        if phase < 0.7 or n == 0:  # хвост интервала — покой: кадров нет
            n += 1
            im = bg.copy()
            d = ImageDraw.Draw(im)
            x = int(w * 0.1 + (t - t0) * w * 0.05) % (w - 200)
            d.rectangle((x, h // 2, x + 140, h // 2 + 90), fill=(230, 80, 60))
            cx, cy = int(w * (0.1 + 0.8 * phase)), int(h * (0.9 - 0.5 * phase))
            d.polygon([(cx, cy), (cx, cy + 24), (cx + 7, cy + 18), (cx + 17, cy + 18)], fill=(255, 255, 255), outline=(0, 0, 0))
            im.save(base / "frames" / f"{n:06d}.jpg", quality=90)
            frames.append({"i": n, "t": round(t, 6)})
        t += 1 / fps
    (base / "frames.jsonl").write_text("".join(json.dumps(f) + "\n" for f in frames), encoding="utf-8")
    marks = [{"t": round(mt, 6), "mark": nm, "note": ""} for mt, nm in zip(mark_t, names)]
    (base / "marks.jsonl").write_text("".join(json.dumps(m, ensure_ascii=False) + "\n" for m in marks), encoding="utf-8")
    (base / "console.txt").write_text("", encoding="utf-8")
    (base / "privacy.txt").write_text("", encoding="utf-8")
    return mark_t


def make_run(root: Path, seconds: float = 6.0, fps: int = 20, w: int = 1600, h: int = 1000, slow: int = 1) -> Path:
    """Прогон с обеими сценами и meta.json."""
    root.mkdir(parents=True, exist_ok=True)
    for sc in ("A", "B"):
        make_scene(root, sc, seconds, fps, w, h)
    meta = {"w": w, "h": h, "fps_capture": fps, "slow": slow, "app": "0.23.0"}
    (root / "meta.json").write_text(json.dumps(meta), encoding="utf-8")
    return root


if __name__ == "__main__":
    if len(sys.argv) < 2 or not sys.argv[1]:
        sys.exit("нужен каталог прогона: synthrun.py <каталог> [секунд]")
    print(make_run(Path(sys.argv[1]), float(sys.argv[2]) if len(sys.argv) > 2 else 30.0))
