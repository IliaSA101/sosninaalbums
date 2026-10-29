"""Запись роликов README (102, design §2.2, §3, §5): стенд → подготовка → захват CDP → журнал меток.

Каталог прогона `.gotmp/demo/<ГГГГММДД-ЧЧММСС>/` рабочей копии (контракт Д1 → Д2, design §2.2):
`A/frames/000001.jpg…`, `A/frames.jsonl`, `A/marks.jsonl`, `A/console.txt`, `A/privacy.txt`, `meta.json`; то же для `B/`.
Чистая часть (аргументы, список фото, каталог) — без браузера; Playwright подключает scripts/e2e/driver.py лениво.
Захват: `Page.startScreencast` (jpeg 90), кадр пишется на диск в обработчике и подтверждается: Playwright sync
не потокобезопасен, отдельный поток записи не годится (отступление от design §5.1; кадры 20–30 КБ пишутся за доли мс).
"""

from __future__ import annotations

import base64
import datetime as dt
import json
import os
import shutil
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "scripts" / "e2e"))

import common  # noqa: E402 — путь e2e добавлен строкой выше
from common import Refuse  # noqa: E402

import journal  # noqa: E402
import privacy  # noqa: E402

W, H = 1600, 1000  # окно ролика, CSS px при масштабе 1 (реш. 102-6)
GOTMP = ROOT / ".gotmp"
HERE = Path(__file__).resolve().parent
DPI_NOTE = 100  # dpi альбома на стенде: «!» «мало разрешения» не должен попасть в кадр (фото Pexels — 2000 px)
SCAN_TEXT = ("Ищу фото", "Читаю папку")  # строка чтения папок в панели ленты: после первого фокуса страницы


# ---- Чистая часть: аргументы, фото, каталог ----------------------------------------------------


def check_port(port: int) -> int:
    """Порт стенда 34116–34199; 34115 — владельца (common.check_port)."""
    return common.check_port(port)


def run_dir(arg: str | None, now: dt.datetime | None = None, root: Path | None = None) -> Path:
    """Каталог прогона: по умолчанию .gotmp/demo/<время>; --out — только абсолютный путь внутри .gotmp рабочей копии.

    Пустой и относительный путь — отказ, а не «текущая папка» (случай 18.10: пустая переменная = корень диска).
    """
    base = ((root or ROOT) / ".gotmp").resolve()
    if arg is None:
        return base / "demo" / (now or dt.datetime.now()).strftime("%Y%m%d-%H%M%S")
    if not arg.strip():
        raise Refuse("--out пуст: нужен абсолютный путь внутри .gotmp")
    p = Path(arg)
    if not p.is_absolute():
        raise Refuse(f"--out {arg!r}: нужен абсолютный путь внутри {base}")
    p = p.resolve()
    if p == base or base not in p.parents:
        raise Refuse(f"--out {arg!r}: вне {base}")
    return p


def parse_photos(text: str) -> dict[str, list[int]]:
    """photos.txt: строки `A 22 23 …` и `B 44 …`, `#` — комментарий. Повтор номера в сцене и не число — отказ."""
    out: dict[str, list[int]] = {}
    for n, raw in enumerate(text.splitlines(), 1):
        line = raw.split("#", 1)[0].split()
        if not line:
            continue
        scene, nums = line[0], line[1:]
        if scene not in ("A", "B"):
            raise Refuse(f"photos.txt, строка {n}: сцена {scene!r}, нужна A или B")
        try:
            vals = [int(x) for x in nums]
        except ValueError:
            raise Refuse(f"photos.txt, строка {n}: номер не число") from None
        out.setdefault(scene, []).extend(vals)
    for scene, vals in out.items():
        if len(set(vals)) != len(vals):
            raise Refuse(f"photos.txt: в сцене {scene} номера повторяются")
    return out


def load_photos(path: Path | None = None) -> dict[str, list[int]]:
    return parse_photos((path or HERE / "photos.txt").read_text(encoding="utf-8"))


def stage_photos(nums: dict[str, list[int]], src: Path, dst: Path) -> dict[str, Path]:
    """Копии stock под нейтральными именами: A → «Свадьба/IMG_0001.jpg…», B → «Свадьба-B/…» (реш. 102-9)."""
    dirs = {"A": dst / "photos" / "Свадьба", "B": dst / "photos" / "Свадьба-B"}
    for scene, vals in nums.items():
        d = dirs[scene]
        if d.exists():
            shutil.rmtree(d)  # прежний набор мог быть другим: имена IMG_NNNN идут по порядку
        d.mkdir(parents=True)
        for i, n in enumerate(vals, 1):
            f = src / f"stock-{n:02d}.jpg"
            if not f.is_file():
                raise Refuse(f"нет фото {f.name} в {src}: нужен набор .testphotos/stock (docs/qa/stock-photos.md)")
            shutil.copyfile(f, d / f"IMG_{i:04d}.jpg")
    return dirs


# ---- Захват ----------------------------------------------------------------------------------


class Capture:
    """CDP screencast в каталог кадров; `frames.jsonl` — {i, t} на кадр (t — metadata.timestamp, секунды эпохи)."""

    def __init__(self, st, scene_dir: Path):
        self.st, self.dir = st, scene_dir
        shutil.rmtree(scene_dir / "frames", ignore_errors=True)  # повтор сцены: кадры прежней попытки не смешиваем
        (scene_dir / "frames").mkdir(parents=True, exist_ok=True)
        self.n = 0
        self._log = (scene_dir / "frames.jsonl").open("w", encoding="utf-8")
        self._on = False

    def _frame(self, ev: dict) -> None:
        if self._on:
            self.n += 1
            (self.dir / "frames" / f"{self.n:06d}.jpg").write_bytes(base64.b64decode(ev["data"]))
            self._log.write(json.dumps({"i": self.n, "t": ev["metadata"]["timestamp"]}) + "\n")
        try:
            self.st.cdp.send("Page.screencastFrameAck", {"sessionId": ev["sessionId"]})
        except Exception:  # noqa: BLE001 — страницу закрыли (перезагрузка, конец сцены): подтверждать некому
            pass

    def start(self) -> None:
        self.st.cdp.on("Page.screencastFrame", self._frame)
        self._on = True
        self.st.cdp.send(
            "Page.startScreencast", {"format": "jpeg", "quality": 90, "maxWidth": W, "maxHeight": H, "everyNthFrame": 1}
        )

    def stop(self) -> None:
        self._on = False
        try:
            self.st.cdp.send("Page.stopScreencast")
        finally:
            self._log.close()

    def fps(self) -> float:
        """Медиана частоты кадров по frames.jsonl (к/с); 0 — кадров меньше двух."""
        ts = [json.loads(x)["t"] for x in (self.dir / "frames.jsonl").read_text().splitlines()]
        gaps = sorted(b - a for a, b in zip(ts, ts[1:]) if b > a)
        return round(1.0 / gaps[len(gaps) // 2], 1) if gaps else 0.0


# ---- Подготовка и прогон -----------------------------------------------------------------------


def stand_log(name: str) -> Path:
    """log.txt стенда: os.UserConfigDir (Linux — $XDG_CONFIG_HOME или ~/.config), папка SosninaAlbums-stand-<имя>."""
    base = os.environ.get("XDG_CONFIG_HOME", "").strip() or str(Path.home() / ".config")
    return Path(base) / f"SosninaAlbums-stand-{common.check_stand(name)}" / "log.txt"


def prepare(st, folder: Path, scene: str, photos_n: int) -> list[str]:
    """Пустой альбом с папкой фото; для B — два готовых разворота из последних 8 фото (свободные — первыми в ленте).

    Возвращает ID фото в порядке ленты. «Стандартный» уровень, dpi альбома DPI_NOTE.
    """
    st.set_level("standard")
    st.call("NewProject", "album")
    st.call("SetSettings", {**st.view()["settings"], "dpi": DPI_NOTE})
    st.call("AddFolderPath", str(folder))
    if not st.wait_until(lambda: len(st.call("GetState")["photos"]) >= photos_n, 60):
        raise Refuse(f"в папке не набралось {photos_n} фото за 60 с")
    ids = [p["id"] for p in st.call("GetState")["photos"]]
    if scene == "B":
        st.call("NewSpread", -1, ids[photos_n - 8 : photos_n - 4])
        st.call("NewSpread", -1, ids[photos_n - 4 : photos_n])
    st.sleep(1.0)
    st.wait_imgs_loaded("[data-guide=library] .grid", 30)
    st.wait_until(lambda: not any(s in st.ev("() => document.querySelector('[data-bar=library]')?.innerText || ''") for s in SCAN_TEXT), 20, 0.2)
    return ids


def _shot(st, path: Path) -> None:
    try:
        st.page.screenshot(path=str(path))
    except Exception:  # noqa: BLE001 — страница могла уйти на перезагрузку: снимок не обязателен
        pass


RELOAD_TRIES = 3  # сколько раз повторить сцену, если страницу перезагрузили (wails dev пересобрал код стенда)


def run_scene(st, scene: str, folder: Path, photos_n: int, out: Path, slow: float = 1.0, capture: bool = True) -> dict:
    """Сцена с повтором при перезагрузке страницы (до RELOAD_TRIES раз); прочий сбой — наружу."""
    import pilot

    for attempt in range(1, RELOAD_TRIES + 1):
        try:
            if attempt > 1:
                st.open_context(st.profile)  # свежая страница: прежняя могла застрять на окне запуска
            return _run_scene(st, scene, folder, photos_n, out, slow, capture)
        except pilot.SceneFail as e:
            if not e.reload or attempt == RELOAD_TRIES:
                raise
            print(f"demo: сцена {scene}: страница перезагрузилась, повтор {attempt + 1}/{RELOAD_TRIES}", file=sys.stderr)
    raise AssertionError("недостижимо")


def _run_scene(st, scene: str, folder: Path, photos_n: int, out: Path, slow: float = 1.0, capture: bool = True) -> dict:
    """Одна попытка сцены: подготовка, прогрев-проход (без захвата), заново подготовка, запись второго прохода.

    Возвращает {frames, fps, privacy, console} для meta.json. Находка приватности — в privacy.txt (код 3 решает demo.py).
    """
    import pilot
    import scenes

    out.mkdir(parents=True, exist_ok=True)
    fn = {"A": scenes.scene_a, "B": scenes.scene_b}[scene]
    # Прогрев: раскладки, предпросмотр автосборки и миниатюры оседают в кэше стенда (на wails dev Go без оптимизаций).
    ids = prepare(st, folder, scene, photos_n)
    try:
        pilot.guarded(pilot.Pilot(st, slow), lambda p, *a: fn(p, st, ids))
    except pilot.SceneFail as e:
        _shot(st, out / f"fail-warmup-{e.mark}.png")
        raise
    ids = prepare(st, folder, scene, photos_n)
    j = journal.Journal(out / "marks.jsonl")
    cap = Capture(st, out)
    found: list[str] = []
    ident = privacy.current_identity()

    def on_mark(name: str, note: str = "") -> None:
        j.mark(name, note)
        found.extend(privacy.findings_text(name, privacy.scan(st.ev(privacy.COLLECT_JS), *ident)).splitlines())

    p = pilot.Pilot(st, slow, on_mark)
    p.put(*scenes.START[scene])
    st.sleep(0.3)
    if capture:
        cap.start()
    try:
        pilot.guarded(p, lambda p, *a: fn(p, st, ids))
    except pilot.SceneFail as e:
        _shot(st, out / f"fail-{e.mark}.png")  # код 4 называет метку; снимок — что было на экране
        raise
    finally:
        if capture:
            cap.stop()
        j.close()
        (out / "privacy.txt").write_text("\n".join(found) + ("\n" if found else ""), encoding="utf-8")
    return {"frames": cap.n, "fps": cap.fps() if capture and cap.n > 1 else 0.0, "privacy": len(found)}


def record_all(port: int, out: Path, photos_src: Path, slow: float = 1.0, scenes_to_run: tuple[str, ...] = ("A", "B")) -> dict:
    """Весь прогон на стенде: фото, сцены A и B, console.txt, meta.json. Возвращает meta (код выхода — у demo.py).

    Сцена, не дождавшаяся результата, поднимает pilot.SceneFail: журнал и кадры до падения остаются в каталоге.
    """
    from driver import Stand

    check_port(port)
    out = run_dir(str(out)) if out is not None else run_dir(None)  # вне .gotmp рабочей копии — отказ
    nums = load_photos()
    dirs = stage_photos(nums, photos_src, GOTMP / "demo")
    out.mkdir(parents=True, exist_ok=True)
    cursor = (HERE / "cursor.js").read_text(encoding="utf-8")
    meta: dict = {"w": W, "h": H, "slow": slow, "scenes": {}}
    with Stand(port, out / "browser", common.Profile("demo", W, H, 1.0, "ролик README"), init_scripts=(cursor,)) as st:
        if slow > 1:
            st.cdp.send("Animation.enable")
            st.cdp.send("Animation.setPlaybackRate", {"playbackRate": 1.0 / slow})
        meta["app"] = st.ev("() => document.body.innerText.match(/v(\\d+\\.\\d+\\.\\d+)/)?.[1] || ''")
        for scene in scenes_to_run:
            st.mark(f"scene {scene}")
            meta["scenes"][scene] = run_scene(st, scene, dirs[scene], len(nums[scene]), out / scene, slow)
        st.write_console()
        text = (out / "browser" / "console.txt").read_text(encoding="utf-8")
        for scene in meta["scenes"]:
            (out / scene / "console.txt").write_text(text, encoding="utf-8")  # одна консоль страницы на оба прохода
    meta["fps_capture"] = min((v["fps"] for v in meta["scenes"].values() if v["fps"]), default=0.0)
    (out / "meta.json").write_text(json.dumps(meta, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    return meta


def default_photos() -> Path:
    """.testphotos/stock рабочей копии; у worktree агента — основного дерева (оно выше по пути, .claude/worktrees/…)."""
    for base in (ROOT, *ROOT.parents):
        p = base / ".testphotos" / "stock"
        if p.is_dir():
            return p
    raise Refuse("нет набора .testphotos/stock (docs/qa/stock-photos.md); задайте --photos")


def console_problems(out: Path) -> list[str]:
    """Сообщения консоли прогона, кроме меток этапов и «не наше» (driver.NOT_OURS)."""
    from driver import NOT_OURS

    f = out / "console.txt"
    if not f.is_file():
        return []
    return [x for x in f.read_text(encoding="utf-8").splitlines() if x.startswith(("error", "warning", "pageerror")) and not any(s in x for s in NOT_OURS)]


def record(port: int, out: Path, photos: Path | None = None, slow: float = 1.0) -> int:
    """Вызов demo.py: запись сцен A и B в готовый каталог `out`; код выхода (design §7).

    0 — готово; 2 — отказ (порт, фото, браузер, стенд); 3 — личное в тексте страницы; 4 — сцена не дождалась результата;
    6 — ошибки консоли страницы.
    """
    import pilot

    try:
        meta = record_all(port, Path(out), photos or default_photos(), float(slow))
    except Refuse as e:
        print(f"demo: {e}", file=sys.stderr)
        return 2
    except pilot.SceneFail as e:
        print(f"demo: {e}", file=sys.stderr)
        return 4
    if any(v["privacy"] for v in meta["scenes"].values()):
        print("demo: личное в тексте страницы — см. privacy.txt сцен", file=sys.stderr)
        return 3
    bad = console_problems(Path(out))
    if bad:
        print("demo: ошибки консоли страницы:\n  " + "\n  ".join(bad[:10]), file=sys.stderr)
        return 6
    return 0
