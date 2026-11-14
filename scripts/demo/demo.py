"""Ролики README (102): запись сценариев через стенд и сборка mp4/WebP/снимка.

    python3 scripts/demo/demo.py all --port 34119 [--publish]   запись A и B + сборка + проверка
    python3 scripts/demo/demo.py record --port 34119            только запись (модуль record.py, Д1)
    python3 scripts/demo/demo.py compose [--run КАТАЛОГ]        только сборка из последнего (или данного) прогона
    python3 scripts/demo/demo.py check [--run КАТАЛОГ]          проверка готового каталога, без записи

Коды выхода (design §7): 0 готово; 2 нет стенда, инструмента, фото или неверные аргументы; 3 личное в тексте страницы;
4 сцена не дождалась результата; 5 бюджет размера; 6 ошибки консоли или log.txt.
Каталоги — только внутри .gotmp рабочей копии: пустой или чужой путь — отказ, не «текущая папка».
"""

from __future__ import annotations

import argparse
import datetime as dt
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))  # плоские импорты соседей и под `python3 -I demo.py`

try:
    import compose as cp  # noqa: E402
    import verify as vf  # noqa: E402
except ImportError as _e:  # нет Pillow: сборка невозможна — код 2, как у прочих «нет инструмента»
    print(f"ОТКАЗ: {_e}; Pillow есть в venv e2e (scripts/guide/requirements.txt)", file=sys.stderr)
    sys.exit(2)

OWNER_PORT = 34115  # стенд владельца
PORT_MIN, PORT_MAX = 34116, 34199
RUN_RX = re.compile(r"^\d{8}-\d{6}$")


def check_port(port: int) -> int:
    if port == OWNER_PORT:
        raise cp.Refuse(f"порт {OWNER_PORT} — стенд владельца, скрипты на него не ходят; возьмите {PORT_MIN}–{PORT_MAX}")
    if not PORT_MIN <= port <= PORT_MAX:
        raise cp.Refuse(f"порт {port} вне {PORT_MIN}–{PORT_MAX}")
    return port


def resolve_dir(arg: str | None, default: Path) -> Path:
    """Каталог вывода или прогона: пустая строка — отказ; вне .gotmp рабочей копии — отказ."""
    if arg is None:
        return default
    if not arg.strip():
        raise cp.Refuse("пустой путь каталога: укажите каталог внутри .gotmp или опустите параметр")
    p = Path(arg).expanduser()
    if not p.is_absolute():
        p = cp.ROOT / p
    if not cp.inside_gotmp(p):
        raise cp.Refuse(f"{p}: каталог вне {cp.GOTMP}")
    return p


def latest_run(base: Path) -> Path:
    runs = sorted(d for d in base.iterdir() if d.is_dir() and RUN_RX.match(d.name)) if base.is_dir() else []
    if not runs:
        raise cp.Refuse(f"в {base} нет прогонов (каталоги ГГГГММДД-ЧЧММСС) — сначала `record`")
    return runs[-1]


def parser() -> argparse.ArgumentParser:
    ap = argparse.ArgumentParser(prog="demo.py", description="Ролики README: запись и сборка")
    sub = ap.add_subparsers(dest="cmd", required=True)
    for name, helptext in (("record", "записать сцены A и B"), ("all", "записать и собрать"), ("compose", "собрать из прогона"), ("check", "проверить готовый прогон")):
        p = sub.add_parser(name, help=helptext)
        if name in ("record", "all"):
            p.add_argument("--port", type=int, default=34119, help="порт стенда demo (по умолчанию 34119)")
            p.add_argument("--photos", default=None, help="каталог с stock-фото (по умолчанию .testphotos/stock основного дерева)")
            p.add_argument("--slow", type=int, default=1, choices=(1, 2), help="медленный режим (design §5.4)")
            p.add_argument("--out", default=None, help="каталог прогонов внутри .gotmp (по умолчанию .gotmp/demo)")
        else:
            p.add_argument("--run", default=None, help="каталог прогона внутри .gotmp (по умолчанию последний в .gotmp/demo)")
        if name in ("all", "compose"):
            p.add_argument("--publish", action="store_true", help="demo.webp и screenshot.jpg → docs/media/")
    return ap


def do_record(args, base: Path) -> Path:
    """Запись через модуль Д1 `record`: record.record(port, out, photos, slow) → код выхода. Нет модуля — отказ."""
    check_port(args.port)
    try:
        import record  # type: ignore[import-not-found]
    except ImportError as e:
        raise cp.Refuse(f"запись недоступна: модуль record.py (Д1) не импортируется ({e}); сборка — `compose`") from None
    run = base / dt.datetime.now().strftime("%Y%m%d-%H%M%S")
    run.mkdir(parents=True, exist_ok=False)
    photos = Path(args.photos).expanduser() if args.photos else None
    code = record.record(port=args.port, out=run, photos=photos, slow=args.slow)
    if code:
        raise _Exit(int(code))
    return run


class _Exit(Exception):
    def __init__(self, code: int):
        super().__init__(code)
        self.code = code


def do_compose(run_dir: Path, publish: bool) -> int:
    run = cp.load_run(run_dir)
    cp.compose_all(run)
    rep = vf.check_run(run_dir)
    (run_dir / "summary.md").write_text(vf.render_summary(run_dir, rep), encoding="utf-8")
    print(f"summary: {run_dir / 'summary.md'}")
    for c, t in rep.problems:
        print(f"код {c}: {t}")
    if publish and rep.code == 0:
        for p in cp.publish(run_dir):
            print(f"опубликовано: {p}")
    return rep.code


def do_check(run_dir: Path) -> int:
    rep = vf.check_run(run_dir)
    for w in rep.warnings:
        print(f"WARN: {w}")
    for c, t in rep.problems:
        print(f"код {c}: {t}")
    print("check: OK" if rep.code == 0 else f"check: FAIL ({rep.code})")
    return rep.code


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        default = cp.GOTMP / "demo"
        if args.cmd in ("record", "all"):
            base = resolve_dir(args.out, default)
            run_dir = do_record(args, base)
            return do_compose(run_dir, args.publish) if args.cmd == "all" else 0
        run_dir = latest_run(default) if args.run is None else resolve_dir(args.run, default)
        if args.cmd == "compose":
            return do_compose(run_dir, args.publish)
        return do_check(run_dir)
    except _Exit as e:
        return e.code
    except (cp.Refuse, cp.BudgetError) as e:
        print(f"ОТКАЗ: {e}", file=sys.stderr)
        return e.code


if __name__ == "__main__":
    sys.exit(main())
