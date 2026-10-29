# -*- coding: utf-8 -*-
"""
Ролик из полноэкранных снимков со стрелками (запасной путь, если запись экрана раннера не вышла).

Запуск:
    python -I scripts/guide/slideshow.py docs/install/assets/mac-steps.mp4 a.png b.png ...
Каждый снимок показывается SEC секунд (по умолчанию 3), кадр 1920×1080 с полями цвета фона, без звука,
H.264 yuv420p (играет в браузере GitHub). Нужен ffmpeg в PATH; код 1 — ffmpeg нет или он упал.
"""
import argparse
import shutil
import subprocess
import sys
from pathlib import Path

W, H, FPS = 1920, 1080, 15
BG = "0x222222"


def slideshow_cmd(images, out, sec=3.0):
    """Команда ffmpeg: по картинке на кадр-паузу, склейка concat, масштаб с полями до W×H."""
    cmd = ["ffmpeg", "-y", "-loglevel", "error"]
    for img in images:
        cmd += ["-loop", "1", "-t", f"{sec:g}", "-i", str(img)]
    parts = [
        f"[{i}:v]scale={W}:{H}:force_original_aspect_ratio=decrease,"
        f"pad={W}:{H}:(ow-iw)/2:(oh-ih)/2:color={BG},setsar=1,fps={FPS}[v{i}]"
        for i in range(len(images))
    ]
    chain = "".join(f"[v{i}]" for i in range(len(images)))
    graph = ";".join(parts) + f";{chain}concat=n={len(images)}:v=1:a=0,format=yuv420p[v]"
    cmd += ["-filter_complex", graph, "-map", "[v]", "-c:v", "libx264", "-crf", "26",
            "-preset", "slow", "-tune", "stillimage", "-movflags", "+faststart", str(out)]
    return cmd


def make(images, out, sec=3.0) -> bool:
    """True, если ролик собран. Без ffmpeg или картинок — False, без исключений."""
    images = [Path(i) for i in images if Path(i).is_file()]
    if not images or not shutil.which("ffmpeg"):
        return False
    Path(out).parent.mkdir(parents=True, exist_ok=True)
    res = subprocess.run(slideshow_cmd(images, out, sec), capture_output=True, text=True, check=False)
    if res.returncode != 0:
        print(res.stderr, file=sys.stderr)
    return res.returncode == 0 and Path(out).is_file()


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("out", type=Path)
    ap.add_argument("images", nargs="+", type=Path)
    ap.add_argument("--sec", type=float, default=3.0)
    args = ap.parse_args(argv)
    return 0 if make(args.images, args.out, args.sec) else 1


if __name__ == "__main__":
    sys.exit(main())
