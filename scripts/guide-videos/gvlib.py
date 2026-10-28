"""Ролики справочника F1 (спека 140, Т6): чистая часть — пути, охрана, окно обрезки, запись-спутник, хеш статьи.

Без браузера и стенда (тесты — test_gvlib.py). Браузерная часть — gvrec.py, сценарии — <id>.py, запуск — gv.py.
Корень проекта берётся от пути скрипта: пустая переменная окружения не должна превратить путь в корень диска (18.10).
Курсор, круги щелчков, путь мыши и приватность кадра — готовые части роликов README (scripts/demo: cursor.js, pilot,
motion, privacy); записываются они не CDP-кадрами, а штатным recordVideo Playwright (на входе encode.sh — его webm).
"""

from __future__ import annotations

import hashlib
import json
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
DEMO = ROOT / "scripts" / "demo"
for _p in (DEMO, ROOT / "scripts" / "e2e"):  # плоские импорты pilot, motion, privacy, common, driver
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from common import Refuse, check_port  # noqa: E402  (scripts/e2e/common.py: порт 34116–34199, не 34115)

W, H = 1280, 760  # окно ролика, CSS px (design §6)
IDS = ("autobuild", "manual", "variants", "vignette", "cover")  # первая волна (0.30-140-27)
ID_RX = re.compile(r"^[a-z0-9][a-z0-9-]{2,39}$")  # как fetch.ValidID
MIN_S, MAX_S = 20.0, 40.0  # длина ролика первой волны, с (решение владельца 0.30-140-27)
LEAD = 0.3  # с запаса до метки start: кадры страницы при записи идут чуть позже создания страницы
TAIL = 0.4  # с после метки end


def main_root(root: Path = ROOT) -> Path:
    """Корень основного дерева: у worktree агента `.git` — файл `gitdir: <основное>/.git/worktrees/<имя>`.

    docs/private лежит только в основном дереве и вне git, поэтому ролики пишутся туда, а не в копию агента.
    """
    g = root / ".git"
    if g.is_dir():
        return root
    try:
        text = g.read_text(encoding="utf-8")
    except OSError:
        return root
    m = re.match(r"gitdir:\s*(.+?)[/\\]\.git[/\\]worktrees[/\\]", text.strip())
    return Path(m.group(1)) if m else root


def default_out(root: Path = ROOT) -> Path:
    return main_root(root) / "docs" / "private" / "mockups" / "0.30" / "videos"


def check_out(arg: str | None, root: Path = ROOT) -> Path:
    """Папка роликов: по умолчанию docs/private/mockups/0.30/videos основного дерева; --out — только внутри неё
    или внутри .gotmp рабочей копии. Пустой и относительный путь — отказ, а не «текущая папка»."""
    if arg is None:
        return default_out(root)
    if not arg.strip():
        raise Refuse("--out пуст: нужен абсолютный путь внутри docs/private/mockups или .gotmp")
    p = Path(arg)
    if not p.is_absolute():
        raise Refuse(f"--out {arg!r}: нужен абсолютный путь")
    p = p.resolve()
    bases = (default_out(root).parent.resolve(), (root / ".gotmp").resolve())
    if not any(p == b or b in p.parents for b in bases):
        raise Refuse(f"--out {arg!r}: вне {bases[0]} и {bases[1]}")
    return p


def check_ids(spec: str) -> list[str]:
    """`all` или список через запятую из первой волны; повторы убираются, неизвестный id — отказ."""
    if (spec or "").strip().lower() == "all":
        return list(IDS)
    ids = [s.strip() for s in (spec or "").split(",") if s.strip()]
    if not ids:
        raise Refuse("статьи не названы: all или список через запятую")
    bad = [s for s in ids if s not in IDS]
    if bad:
        raise Refuse(f"нет сценария для {', '.join(bad)}; есть {', '.join(IDS)}")
    return list(dict.fromkeys(ids))


def default_photos(root: Path = ROOT) -> Path:
    """.testphotos/stock рабочей копии; у worktree — основного дерева (в git набора нет)."""
    for base in (root, main_root(root)):
        p = base / ".testphotos" / "stock"
        if p.is_dir():
            return p
    raise Refuse("нет набора .testphotos/stock (docs/qa/stock-photos.md); задайте --photos")


def trim_window(t_page: float, t_start: float, t_end: float) -> dict:
    """Окно обрезки сырого webm: видео идёт с создания страницы (t_page), сценарий — от метки start до end (часы
    monotonic). start не меньше 0; длина — end−start плюс запас с обеих сторон."""
    if not (t_page <= t_start < t_end):
        raise ValueError("нужно t_page ≤ t_start < t_end")
    start = max(0.0, t_start - t_page - LEAD)
    return {"start": round(start, 2), "seconds": round(t_end - t_start + (t_start - t_page - start) + TAIL, 2)}


def length_note(seconds: float) -> str:
    """Пустая строка — длина в пределах 20–40 с; иначе предупреждение для README."""
    if seconds < MIN_S:
        return f"короче {MIN_S:g} с"
    if seconds > MAX_S:
        return f"длиннее {MAX_S:g} с"
    return ""


def sidecar_path(webm: Path) -> Path:
    return webm.with_suffix(".json")


def write_sidecar(webm: Path, data: dict) -> None:
    sidecar_path(webm).write_text(json.dumps(data, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")


def read_sidecar(webm: Path) -> dict:
    """Спутник сырого webm: {id, start, seconds, …}. Нет файла — {} (encode.sh берёт весь ролик)."""
    p = sidecar_path(webm)
    if not p.is_file():
        return {}
    try:
        d = json.loads(p.read_text(encoding="utf-8"))
    except (OSError, ValueError) as e:
        raise Refuse(f"{p.name}: не читается — {e}") from None
    if not isinstance(d, dict):
        raise Refuse(f"{p.name}: ожидался объект")
    return d


def fnv1a32(s: str) -> int:
    """FNV-1a 32 по байтам UTF-8 (как articleHash; здесь — образец для проверки articlehash.mjs)."""
    h = 0x811C9DC5
    for b in s.encode("utf-8"):
        h = ((h ^ b) * 0x01000193) & 0xFFFFFFFF
    return h


def article_hash(title: str, what: str = "", where: str = "", how: str = "") -> str:
    """Хеш текста статьи: FNV-1a 32 по сырым `title\\nwhat\\nwhere\\nhow` (пустое поле — пустая строка), 8 hex."""
    return f"{fnv1a32(chr(10).join((title, what or '', where or '', how or ''))):08x}"


def inside(box: dict, view: dict, pad: float = 0.0) -> bool:
    """Прямоугольник box (x, y, width, height) целиком в view, с отступом pad от краёв view."""
    return (
        box["x"] >= view["x"] + pad
        and box["y"] >= view["y"] + pad
        and box["x"] + box["width"] <= view["x"] + view["width"] - pad
        and box["y"] + box["height"] <= view["y"] + view["height"] - pad
    )


def merge_run(old: object, ok: dict, failed: dict) -> dict:
    """Слияние run.json по статьям: снятые в этом прогоне заменяют запись, остальные остаются как были.

    Упавшая статья прежнюю удачную запись не стирает (старый ролик на диске цел), но попадает в failed;
    снятая заново — из failed уходит. Повреждённый или чужой old (не объект) — пустой: прогон не должен падать.
    """
    prev = old if isinstance(old, dict) else {}
    prev_ok = prev.get("ok") if isinstance(prev.get("ok"), dict) else {}
    prev_failed = prev.get("failed") if isinstance(prev.get("failed"), dict) else {}
    merged_ok = {**prev_ok, **ok}
    merged_failed = {k: v for k, v in prev_failed.items() if k not in ok}
    merged_failed.update(failed)
    return {"ok": merged_ok, "failed": merged_failed}


def read_run(path: Path) -> dict:
    """Прежний run.json; нет файла или он битый — {} (слияние начнёт с пустого)."""
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {}


def file_info(path: Path, url: str = "") -> dict:
    """Размер и sha256 готового файла ролика (как в encode.sh); файл читается кусками."""
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    d = {"size": path.stat().st_size, "sha256": h.hexdigest()}
    if url:
        d["url"] = url
    return d
