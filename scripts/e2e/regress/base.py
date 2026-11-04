"""Основа e2e-регресса (166, design §4): RegressCase, @bug, ожидания по структуре, следы при провале.

Тест не зовёт sleep / wait_for_timeout и не сравнивает снимки: время — только предел ожидания (умножается на
SA_TEST_SLOW), признак — структура (DOM, View из Go, прямоугольники). Снимок, DOM и консоль пишутся при провале, для разбора.
Playwright импортируется лениво (через driver): модуль грузится и без него — для юнит-тестов чистых частей.
"""

from __future__ import annotations

import json
import shutil
import unittest
from pathlib import Path
from typing import Any

import common
import driver
from common import PROFILES, Refuse

DOM_LIMIT = 50_000  # байт DOM корня проверки в следе провала

# Страница: журнал view-событий (rev) и шпионы над методами App. Ставится на каждый тест после загрузки.
HOOK_REG_JS = """
async () => {
  const W = window; if (W.__reg) return true;
  const R = W.runtime; if (!R || !R.EventsOn) return false;
  const v0 = await W.go.app.App.GetView();
  W.__reg = { rev: v0.rev, views: [], spies: {} };
  R.EventsOn('view', (v) => { if (v && typeof v.rev === 'number') { W.__reg.rev = v.rev; W.__reg.views.push(v.rev) } });
  return true;
}
"""

SPY_JS = """
(name) => {
  const A = window.go.app.App, S = window.__reg.spies;
  if (S[name]) return;
  const orig = A[name].bind(A); S[name] = { calls: [] };
  A[name] = (...args) => { S[name].calls.push(args); return orig(...args) };
}
"""

REC_JS = """
({ sel, id }) => {
  const W = window; W.__reg.rec = W.__reg.rec || {};
  const el = document.querySelector(sel); if (!el) return false;
  const log = (W.__reg.rec[id] = [el.textContent]);
  new MutationObserver(() => log.push(el.textContent)).observe(el, { subtree: true, childList: true, characterData: true });
  return true;
}
"""

STABLE_JS = """
({ sel, limit }) => new Promise((res) => {
  const t0 = performance.now(); let prev = null;
  const step = () => {
    const el = document.querySelector(sel);
    if (el) {
      const r = el.getBoundingClientRect(); const cur = [r.x, r.y, r.width, r.height];
      if (prev && cur.every((v, i) => v === prev[i])) return res({ x: cur[0], y: cur[1], width: cur[2], height: cur[3] });
      prev = cur;
    } else prev = null;
    if (performance.now() - t0 > limit) return res(null);
    requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
})
"""

HIT_JS = """
([x, y]) => {
  const e = document.elementFromPoint(x, y); if (!e) return null;
  const data = {}; for (const [k, v] of Object.entries(e.dataset)) data[k] = v;
  return { tag: e.tagName.toLowerCase(), cls: e.getAttribute('class') || '', role: e.getAttribute('role') || '', data };
}
"""


def bug(id: str, fix: str, src: str, red: str):
    """Паспорт теста: какой баг ловит (id по спеке), чем починен, откуда известен, как показать красный (design §7)."""

    def deco(cls):
        cls.BUG = {"id": id, "fix": fix, "src": src, "red": red}
        return cls

    return deco


class RegressStand(driver.Stand):
    """Stand без закрытия стартового окна и тура: их заменяет засев настроек (design §5), а Esc с паузами стоил бы секунды на тест."""

    def dismiss_start(self) -> None:
        return None


class RunEnv:
    """Общее состояние прогона: задаёт regress.py до первого теста."""

    st: driver.Stand | None = None
    run_dir: Path | None = None
    data: Path | None = None  # run_dir/data: photos/, Alpha.salbum, Beta.salbum
    seed: dict | None = None  # GetState().prefs после засева
    slow: float = 1.0


class Spy:
    def __init__(self, case: "RegressCase", name: str):
        self.case, self.name = case, name

    def calls(self) -> list:
        return self.case.st.ev("(n) => window.__reg.spies[n].calls", self.name)

    def count(self) -> int:
        return len(self.calls())


class Recorder:
    def __init__(self, case: "RegressCase", rid: str):
        self.case, self.rid = case, rid

    def values(self) -> list[str]:
        return self.case.st.ev("(id) => window.__reg.rec[id]", self.rid)


class RegressCase(unittest.TestCase):
    profile = "A"
    BUG: dict | None = None
    _spy_seq = 0

    # ---- жизненный цикл ----

    @property
    def st(self) -> driver.Stand:
        return RunEnv.st

    @property
    def page(self):
        return RunEnv.st.page

    def setUp(self) -> None:
        if RunEnv.st is None:
            raise Refuse("стенд прогона не поднят: запускайте через scripts/e2e/regress.py")
        self.st.console.clear()
        self.st.failed.clear()
        self._log_offset = self._log_size()
        self.reopen(self.profile)

    def reopen(self, profile: str) -> None:
        """Новый контекст браузера на профиле масштаба; проект — новый пустой, настройки засеяны."""
        self.st.open_context(PROFILES[profile])
        self.st.call("NewProject", "album")
        self.st.ev(HOOK_REG_JS)

    def tearDown(self) -> None:
        failed = self._failed()
        msgs: list[str] = []
        if not failed:
            msgs += self.st.console_problems()
        log = self._new_log()
        if "ПАНИКА" in log:
            msgs.append("в log.txt стенда ПАНИКА")
        msgs += self._restore_prefs()
        if failed or msgs:
            self._dump_trace()
        if msgs and not failed:
            self.fail("; ".join(msgs))

    def _failed(self) -> bool:
        res = getattr(getattr(self, "_outcome", None), "result", None)
        if res is None:
            return False
        bad = [getattr(t, "test_case", t) for t, _ in list(res.failures) + list(res.errors)]
        return any(t is self for t in bad)

    def _restore_prefs(self) -> list[str]:
        """След в настройках: сравнение со снимком засева; при расхождении — провал и возврат засевом через API."""
        seed = RunEnv.seed
        if seed is None:
            return []
        now = self.st.call("GetState")["prefs"]
        if now == seed:
            return []
        diff = sorted(k for k in set(now) | set(seed) if now.get(k) != seed.get(k))
        apply_seed(self.st)
        return [f"след в настройках: {', '.join(diff)}"]

    # ---- следы ----

    def _name(self) -> str:
        return ".".join(self.id().split(".")[-2:])

    def _log_files(self) -> list[Path]:
        return sorted((RunEnv.run_dir / "home").rglob("log.txt"))

    def _log_size(self) -> dict:
        return {str(p): p.stat().st_size for p in self._log_files()}

    def _new_log(self) -> str:
        out = ""
        for p in self._log_files():
            txt, _ = common.tail_from(p, self._log_offset.get(str(p), 0))
            out += txt
        return out

    def _dump_trace(self) -> None:
        """Снимок, DOM корня, консоль и хвост stand.log — в папку прогона."""
        run, n = RunEnv.run_dir, self._name()
        try:
            self.st.shot(n)
            (run / "dom").mkdir(exist_ok=True)
            html = self.st.ev("() => (document.querySelector('#app') || document.body).outerHTML") or ""
            (run / "dom" / f"{n}.html").write_text(html[:DOM_LIMIT], encoding="utf-8")
        except Exception:  # noqa: BLE001 — след не должен подменять причину провала
            pass
        self.st.write_console()
        try:
            lines = (run / "stand.log").read_text(encoding="utf-8", errors="replace").splitlines()
            (run / "stand-tail.txt").write_text("\n".join(lines[-200:]) + "\n", encoding="utf-8")
        except OSError:
            pass

    # ---- время ----

    def lim(self, seconds: float) -> float:
        """Предел ожидания в секундах с поправкой SA_TEST_SLOW."""
        return seconds * RunEnv.slow

    # ---- данные ----

    def album(self, groups: list[int]) -> dict:
        """Фото из data/photos в ленте и по развороту на каждую группу (groups[i] фото); → View."""
        need = sum(groups)
        self.st.call("AddFolderPath", str(RunEnv.data / "photos"))
        self.wait_js("(n) => window.go.app.App.GetState().then((s) => s.photos.length >= n)", need)
        ids = [p["id"] for p in self.st.call("GetState")["photos"]][:need]
        pos = 0
        for g in groups:
            self.st.call("NewSpread", -1, ids[pos : pos + g])
            pos += g
        v = self.st.call("GetView")
        self.wait_js("(r) => window.__reg.rev >= r", v["rev"])
        return v

    def fixture(self, name: str = "Alpha") -> dict:
        """Копия data/<name>.salbum в data/case-<тест>.salbum и OpenRecent. Открыть можно только путь из «Последних»."""
        src = RunEnv.data / f"{name}.salbum"
        dst = RunEnv.data / f"case-{self._name()}.salbum"
        shutil.copyfile(src, dst)
        return self.st.call("OpenRecent", str(dst))["view"]

    def reload(self) -> None:
        self.page.reload(wait_until="load")
        self.wait_js("() => window.go && window.go.app && window.go.app.App && document.querySelector('[role=menubar]')")
        self.st.ev(driver.HOOK_JS)
        self.st.ev(HOOK_REG_JS)

    # ---- Go ----

    def call(self, method: str, *args: Any) -> Any:
        return self.st.call(method, *args)

    def rev(self) -> int:
        return self.st.ev("() => window.__reg.rev")

    def wait_rev(self, after: int, timeout: float = 10) -> dict:
        self.wait_js("(r) => window.__reg.rev > r", after, timeout)
        return self.st.call("GetView")

    # ---- DOM ----

    def q(self, sel: str):
        return self.page.locator(sel)

    def expect(self, target):
        """playwright.sync_api.expect с пределом ожидания теста."""
        from playwright.sync_api import expect

        return expect(target)

    def wait_js(self, js: str, arg: Any = None, timeout: float = 10) -> Any:
        return self.page.wait_for_function(js, arg=arg, timeout=int(self.lim(timeout) * 1000)).json_value()

    def stable_rect(self, sel: str, timeout: float = 5) -> dict:
        r = self.st.ev(STABLE_JS, {"sel": sel, "limit": self.lim(timeout) * 1000})
        if r is None:
            self.fail(f"прямоугольник {sel!r} не стал устойчивым за {self.lim(timeout):g} с")
        return r

    def fonts_ready(self) -> None:
        self.st.ev("() => document.fonts.ready.then(() => true)")

    def record_text(self, sel: str) -> Recorder:
        RegressCase._spy_seq += 1
        rid = f"r{RegressCase._spy_seq}"
        if not self.st.ev(REC_JS, {"sel": sel, "id": rid}):
            self.fail(f"нет элемента {sel!r} для записи текста")
        return Recorder(self, rid)

    def hit(self, x: float, y: float) -> dict | None:
        return self.st.ev(HIT_JS, [x, y])

    def spy(self, method: str) -> Spy:
        self.st.ev(SPY_JS, method)
        return Spy(self, method)


def seed_prefs_plan() -> list[tuple[str, tuple]]:
    """Засев настроек прогона (design §5): окно «Продолжить» и оба тура («Первые шаги», продолжение на развороте) не мешают, уровень — полный."""
    return [("SetShowStart", (False,)), ("SetLevel", ("full",)), ("SetTour", ("first", "done")), ("SetTour", ("spread", "done"))]


def apply_seed(st: driver.Stand) -> dict:
    """Засев через API; возвращает снимок GetState().prefs — эталон для сверки после каждого теста."""
    for name, args in seed_prefs_plan():
        st.call(name, *args)
    return st.call("GetState")["prefs"]


def prefs_digest(p: dict) -> str:
    return json.dumps(p, sort_keys=True, ensure_ascii=False)
