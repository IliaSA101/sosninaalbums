"""Ролики справочника F1 (140, Т6): браузерная часть — стенд с записью видео Playwright, подготовка и один проход статьи.

Запись — штатный recordVideo (webm): подкладывается в `Browser.new_context` через прокси, остальное — Stand из
scripts/e2e/driver.py (охрана порта, ожидание интерфейса, закрытие стартового окна и тура). Курсор и круги щелчков —
init-скрипт scripts/demo/cursor.js, только в записанной странице. Подготовка проекта (папка, развороты) идёт через
методы App до создания записываемой страницы: в ролике нет ни загрузки, ни набора фото.
Проход статьи: прогрев (без записи: кэш миниатюр и предпросмотров) → подготовка заново → запись.
"""

from __future__ import annotations

import json
import shutil
import time
from pathlib import Path
from types import ModuleType

import gvlib
from gvlib import H, W, Refuse

# Страница стенда, о которой стоит знать при разборе провала.
DUMP_JS = """
() => ({
  dialogs: [...document.querySelectorAll('[role=dialog]')].map((d) => (d.innerText || '').slice(0, 160)),
  toasts: [...document.querySelectorAll('[role=status],[role=alert]')].map((d) => (d.innerText || '').slice(0, 120)),
  head: (document.body.innerText || '').slice(0, 240),
})
"""


class _RecBrowser:
    """Прокси Browser: new_context получает параметры записи, остальное — как есть."""

    def __init__(self, browser, rec_dir: Path):
        self._b, self._dir = browser, str(rec_dir)

    def new_context(self, **kw):
        return self._b.new_context(record_video_dir=self._dir, record_video_size={"width": W, "height": H}, **kw)

    def __getattr__(self, name):
        return getattr(self._b, name)


def make_stand_class():
    """RecStand — Stand с записью; класс создаётся лениво: без Playwright импорт модуля не падает (тесты).

    Playwright sync нельзя перезапустить в том же процессе, поэтому стенд один на весь прогон: контексты меняются
    через open_context, а rec_dir выбирает, пишется ли страница (None — не пишется).
    """
    from driver import Stand

    class RecStand(Stand):
        def __init__(self, *a, **kw):
            super().__init__(*a, **kw)
            self.rec_dir: Path | None = None
            self.t_page = 0.0  # monotonic создания страницы: видео идёт с этого момента
            self._real = None

        def open_context(self, profile):
            if self._real is None:
                self._real = self._browser
            if self.rec_dir is not None:
                self.rec_dir.mkdir(parents=True, exist_ok=True)
            self._browser = _RecBrowser(self._real, self.rec_dir) if self.rec_dir is not None else self._real
            self.t_page = time.monotonic()
            super().open_context(profile)

    return RecStand


def stage_photos(work: Path, src: Path | None = None) -> Path:
    """Копии stock под нейтральными именами «Свадьба/IMG_0001.jpg…» (набор A роликов README): в кадре нет ни путей
    машины, ни имён файлов Pexels (102-9)."""
    import record as demo

    nums = demo.load_photos()
    if src is not None and (not src.is_absolute() or not src.is_dir()):
        raise Refuse(f"--photos {src}: нужна существующая папка по абсолютному пути")
    return demo.stage_photos({"A": nums["A"]}, src or gvlib.default_photos(), work)["A"]


def prepare(st, level: str, folder: Path, n_photos: int) -> list[str]:
    """Пустой альбом с папкой фото; возвращает ID фото в порядке ленты. dpi 100: «!» мало разрешения в кадр не нужен."""
    st.set_level(level)
    st.call("NewProject", "album")
    st.call("SetSettings", {**st.view()["settings"], "dpi": 100})
    for f in st.view().get("folders") or []:
        st.call("RemoveFolder", f)
    st.call("AddFolderPath", str(folder))
    if not st.wait_until(lambda: len(st.call("GetState")["photos"]) >= n_photos, 60):
        raise Refuse(f"в папке не набралось {n_photos} фото за 60 с")
    ids = [p["id"] for p in st.call("GetState")["photos"]]
    st.sleep(0.8)
    st.wait_imgs_loaded("[data-guide=library] .grid", 30)
    return ids


def dump(st, out: Path, name: str) -> None:
    """Снимок и состояние страницы при провале: не одна строка ошибки, а то, что было на экране."""
    out.mkdir(parents=True, exist_ok=True)
    try:
        st.page.screenshot(path=str(out / f"{name}.png"))
        info = {"page": st.ev(DUMP_JS), "console": st.console_problems()[:10], "failed": st.failed[-10:]}
        (out / f"{name}.json").write_text(json.dumps(info, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    except Exception:  # noqa: BLE001 — страница могла закрыться: снимок не обязателен
        pass


def run_article(st, art: ModuleType, out: Path, folder: Path, record: bool = True, shots: bool = False, warm: bool = True) -> dict:
    """Один проход статьи на общем стенде; возвращает данные спутника. Провал сцены — pilot.SceneFail, дамп в out/fail/."""
    import pilot
    import privacy

    prof = st.profile
    fail = out / "fail"
    n = len(folder_files(folder))

    # 1. Подготовка и прогрев — без записи.
    st.rec_dir = None
    st.open_context(prof)
    art.prepare(st, prepare(st, art.LEVEL, folder, n))
    if warm:
        p = pilot.Pilot(st, 1.0)
        p.put(*art.START)
        try:
            pilot.guarded(p, lambda p, *a: art.scene(p, st))
        except pilot.SceneFail as e:
            dump(st, fail, f"{art.ID}-warm-{e.mark}")
            raise
        art.prepare(st, prepare(st, art.LEVEL, folder, n))

    # 2. Запись: свежая страница на подготовленном проекте.
    rec_dir = out / "rec" / art.ID
    shutil.rmtree(rec_dir, ignore_errors=True)
    st.rec_dir = rec_dir if record else None
    st.open_context(prof)
    t_page = st.t_page
    marks: dict[str, float] = {}
    found: list[str] = []
    ident = privacy.current_identity()

    def on_mark(name: str, note: str = "") -> None:
        marks[name] = time.monotonic()
        found.extend(privacy.findings_text(name, privacy.scan(st.ev(privacy.COLLECT_JS), *ident)).splitlines())
        if shots:
            (out / "shots").mkdir(parents=True, exist_ok=True)
            st.page.screenshot(path=str(out / "shots" / f"{art.ID}-{len(marks):02d}-{name}.png"))

    p = pilot.Pilot(st, 1.0, on_mark)
    p.put(*art.START)
    st.sleep(0.3)
    try:
        pilot.guarded(p, lambda p, *a: art.scene(p, st))
    except Exception as e:
        dump(st, fail, f"{art.ID}-{getattr(e, 'mark', 'scene')}")
        raise
    video = st.page.video if record else None
    problems = st.console_problems()
    st.ctx.close()  # видео дописывается при закрытии контекста
    raw = Path(video.path()) if video is not None else None
    if "start" not in marks or "end" not in marks:
        raise Refuse(f"{art.ID}: сценарий не поставил метки start и end")
    win = gvlib.trim_window(t_page, marks["start"], marks["end"])
    return {"id": art.ID, "raw": str(raw) if raw else "", **win, "privacy": found, "console": problems, "note": art.NOTE}


def folder_files(folder: Path) -> list[Path]:
    return sorted(folder.glob("*.jpg"))
