"""Общее для big.py и scale.py (089): аргументы, охрана §7, профили масштаба, пути стенда, процесс стенда.

Чистые части (охрана порта и выхода, профили, коэффициент SA_TEST_SLOW, пути, разбор вывода PowerShell)
проверяет test_report.py без браузера. Браузерная часть — driver.py.
Корень проекта берётся от пути скрипта, не из переменной окружения: пустая переменная не должна превратить
путь в корень диска (случай 18.10).
"""

from __future__ import annotations

import argparse
import datetime as dt
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
E2E_OUT = ROOT / ".gotmp" / "e2e"
PHOTOS = ROOT / ".testphotos-e2e"
OWNER_PORT = 34115  # стенд владельца: сюда скрипты не ходят
PORT_MIN, PORT_MAX = 34116, 34199
STAND_RX = re.compile(r"^[a-z0-9]{1,16}$")  # как osx.ParseStand (после приведения к нижнему регистру)


class Refuse(Exception):
    """Отказ запуска: печатается строкой, код выхода 2, ничего не записано."""


# ---- Охрана --------------------------------------------------------------------------------


def check_port(port: int) -> int:
    if port == OWNER_PORT:
        raise Refuse(f"порт {OWNER_PORT} — стенд владельца, скрипты на него не ходят; возьмите {PORT_MIN}–{PORT_MAX}")
    if not PORT_MIN <= port <= PORT_MAX:
        raise Refuse(f"порт {port} вне {PORT_MIN}–{PORT_MAX}")
    return port


def check_stand(name: str) -> str:
    n = (name or "").strip().lower()
    if not STAND_RX.match(n):
        raise Refuse(f"имя стенда {name!r}: латиница и цифры, до 16 знаков")
    return n


def out_dir(arg: str | None, level: str, now: dt.datetime | None = None, base: Path | None = None) -> Path:
    """Папка отчёта: по умолчанию .gotmp/e2e/<время>-<уровень>; --out — только абсолютный путь внутри .gotmp/e2e."""
    base = (base or E2E_OUT).resolve()
    if arg is None:
        return base / f"{(now or dt.datetime.now()).strftime('%Y%m%d-%H%M%S')}-{level}"
    if not arg.strip():
        raise Refuse("--out пуст: нужен абсолютный путь внутри .gotmp/e2e")
    p = Path(arg)
    if not p.is_absolute():
        raise Refuse(f"--out {arg!r}: нужен абсолютный путь внутри {base}")
    p = p.resolve()
    if p != base and base not in p.parents:
        raise Refuse(f"--out {arg!r}: вне {base}")
    return p


def slow_factor(env: dict | None = None) -> float:
    """SA_TEST_SLOW (как testkit.Limit): не задана или пуста — 1; число меньше 1 или не число — отказ."""
    raw = (os.environ if env is None else env).get("SA_TEST_SLOW", "").strip()
    if raw == "":
        return 1.0
    try:
        v = float(raw)
    except ValueError:
        raise Refuse(f"SA_TEST_SLOW={raw!r}: не число") from None
    if not v >= 1:
        raise Refuse(f"SA_TEST_SLOW={raw!r}: нужно число не меньше 1")
    return v


# ---- Профили масштаба (design.md §5) -----------------------------------------------------


@dataclass(frozen=True)
class Profile:
    key: str
    w: int  # CSS px
    h: int
    dpr: float
    note: str


PROFILES: dict[str, Profile] = {
    "A": Profile("A", 1280, 760, 1.0, "обычное окно, как QA"),
    "B": Profile("B", 1536, 785, 1.25, "1920×1080 при 125 %, развёрнуто"),
    "C": Profile("C", 1280, 641, 1.5, "1920×1080 при 150 %, развёрнуто"),
    "D": Profile("D", 1707, 881, 1.5, "2560×1440 при 150 %"),
}
DEFAULT_PROFILES = "A,B,C"


def parse_profiles(spec: str) -> list[Profile]:
    keys = [k.strip().upper() for k in (spec or "").split(",") if k.strip()]
    if not keys:
        raise Refuse("--profiles пуст: например A,B,C")
    bad = [k for k in keys if k not in PROFILES]
    if bad:
        raise Refuse(f"профиль {', '.join(bad)}: известны {', '.join(PROFILES)}")
    return [PROFILES[k] for k in dict.fromkeys(keys)]


# ---- Аргументы ------------------------------------------------------------------------------


def base_parser(prog: str, desc: str) -> argparse.ArgumentParser:
    ap = argparse.ArgumentParser(prog=prog, description=desc)
    ap.add_argument("--port", type=int, required=True, help=f"порт стенда {PORT_MIN}–{PORT_MAX}")
    ap.add_argument("--stand", required=True, help="имя стенда (SA_STAND): папка данных SosninaAlbums-stand-<имя>")
    ap.add_argument("--out", default=None, help="папка отчёта (абсолютный путь внутри .gotmp/e2e)")
    ap.add_argument("--quick", action="store_true", help="папка a/ (120 фото), без ожидания полного прогрева")
    ap.add_argument("--photos", action="append", default=None, metavar="ПАПКА", help="папка с фото (повторяемо); по умолчанию набор .testphotos-e2e")
    ap.add_argument("--headed", action="store_true", help="показать окно браузера (только владельцу)")
    return ap


def photo_dirs(args_photos: list[str] | None, quick: bool, root: Path | None = None) -> list[Path]:
    """Папки с фото: --photos или набор 089 (a/ — быстрый, a/ и b/ — полный). Нет папки — отказ."""
    base = (root or PHOTOS)
    if args_photos is not None:
        dirs = [Path(p) for p in args_photos]
        if any(not str(p).strip() for p in dirs):
            raise Refuse("--photos пуст")
    else:
        dirs = [base / "a"] if quick else [base / "a", base / "b"]
    for d in dirs:
        if not d.is_absolute():
            raise Refuse(f"папка фото {d}: нужен абсолютный путь")
        if not d.is_dir():
            raise Refuse(f"нет набора фото: {d.name} (go run scripts/e2e-photos.go → .testphotos-e2e/)")
    return [d.resolve() for d in dirs]


def count_photos(dirs: list[Path]) -> int:
    """Число JPEG в папках с подпапками (как обход библиотеки): ожидаемое число плиток."""
    n = 0
    for d in dirs:
        for _r, _s, files in os.walk(d):
            n += sum(1 for f in files if f.lower().endswith((".jpg", ".jpeg")))
    return n


# ---- Папки данных стенда ------------------------------------------------------------------


def config_root() -> Path:
    """os.UserConfigDir: %AppData% в Windows. Без переменной — отказ, а не «текущая папка»."""
    ad = os.environ.get("APPDATA", "").strip()
    if not ad:
        raise Refuse("нет переменной APPDATA: папку данных стенда не найти")
    return Path(ad)


def stand_dir(name: str) -> Path:
    return config_root() / f"SosninaAlbums-stand-{check_stand(name)}"


def tail_from(path: Path, offset: int) -> tuple[str, int]:
    """Строки log.txt, дописанные после offset; возвращает (текст, новый размер)."""
    try:
        size = path.stat().st_size
    except OSError:
        return "", offset
    if size < offset:
        offset = 0  # журнал обрезали
    with path.open("rb") as f:
        f.seek(offset)
        return f.read().decode("utf-8", "replace"), size


# ---- Процесс стенда и машина (только чтение) ---------------------------------------------


def ps(cmd: str, timeout: float = 20) -> str:
    """PowerShell-запрос только на чтение; нет PowerShell или сбой — пустая строка."""
    try:
        r = subprocess.run(
            ["powershell", "-NoProfile", "-NonInteractive", "-Command", cmd],
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=timeout,
        )
        return r.stdout.strip()
    except (OSError, subprocess.SubprocessError):
        return ""


def parse_int(s: str) -> int | None:
    m = re.search(r"\d+", s or "")
    return int(m.group()) if m else None


def stand_pid(port: int) -> int | None:
    """PID процесса, слушающего порт стенда (design.md F7)."""
    return parse_int(ps(f"(Get-NetTCPConnection -LocalPort {int(port)} -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1).OwningProcess"))


def peak_working_set_mb(pid: int | None) -> float | None:
    if not pid:
        return None
    v = parse_int(ps(f"(Get-Process -Id {int(pid)} -ErrorAction SilentlyContinue).PeakWorkingSet64"))
    return None if v is None else round(v / 1048576, 1)


def cpu_load() -> int | None:
    return parse_int(ps("(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average"))


def machine_info() -> dict:
    name = ps("(Get-CimInstance Win32_Processor | Select-Object -First 1).Name")
    ram = parse_int(ps("[int]((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1GB)"))
    return {"cpu": name or "?", "threads": os.cpu_count() or 0, "ramGB": ram or 0}


def drop_empty(out: Path) -> None:
    """Отказ до первой записи: пустые папки отчёта (shots/, сама папка) не оставляем — «ничего не записано»."""
    for d in (out / "shots", out):
        try:
            d.rmdir()  # только пустую
        except OSError:
            pass


def run_main(fn) -> int:
    """Единая обёртка: Refuse → строка и код 2."""
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8")  # консоль Windows и канал: кириллица без кракозябр
        except (AttributeError, ValueError):
            pass
    try:
        return fn()
    except Refuse as e:
        print(f"e2e: {e}", file=sys.stderr)
        return 2
