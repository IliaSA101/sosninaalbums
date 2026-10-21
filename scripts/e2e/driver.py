"""Браузерная часть e2e (089): Playwright + Chromium против стенда 3411x.

Без Playwright импорт модуля не падает (тесты чистых частей идут без браузера): ошибка — при первом запуске.
Что делает: запускает headless Chromium с заданным окном и масштабом, подписывается на события Go в странице
(`window.runtime.EventsOn`, время — `performance.now()`), копит длинные задачи (`PerformanceObserver`),
вызывает методы `window.go.app.App`, меряет кадры прокрутки, память страницы (CDP) и снимает экран.
Нативных диалогов не вызывает; на порт владельца не ходит (охрана — common.check_port).
"""

from __future__ import annotations

import math
import os
import threading
import time
from pathlib import Path
from typing import Any, Callable

from common import Profile, PROFILES, Refuse

# Консольные сообщения «не наше» (docs/QA.md): оверлей wails dev в браузере и проверка Vite через blob-воркер.
NOT_OURS = ("reading 'nodes'", "violates the following Content Security Policy", "Creating a worker from 'blob:")

# Подписки страницы ставятся до скриптов приложения: длинные задачи с первой секунды.
INIT_JS = """
(() => {
  const E = (window.__e2e = { lt: [], ev: [], hooked: false, mqls: [], mqNative: 0 });
  try {
    new PerformanceObserver((l) => { for (const e of l.getEntries()) E.lt.push({ s: e.startTime, d: e.duration }) })
      .observe({ type: 'longtask', buffered: true });
  } catch (e) {}
  // S7: CDP-эмуляция DPR не шлёт change уже созданным спискам — запоминаем списки с resolution (089-D17).
  const mm = window.matchMedia.bind(window);
  window.matchMedia = (q) => {
    const l = mm(q);
    if (String(q).includes('resolution')) { E.mqls.push(l); l.addEventListener('change', () => { E.mqNative++ }) }
    return l;
  };
})()
"""

# События Go → журнал страницы. Тяжёлые полезные нагрузки не копируем: только числа.
HOOK_JS = """
() => {
  const E = window.__e2e; if (E.hooked) return true;
  const R = window.runtime; if (!R || !R.EventsOn) return false;
  const t = () => performance.now();
  R.EventsOn('scan', (s) => E.ev.push({ n: 'scan', t: t(), active: !!(s && s.active), found: s ? s.found : 0, read: s ? s.read : 0 }));
  R.EventsOn('thumbs', (d, total) => E.ev.push({ n: 'thumbs', t: t(), done: d, total }));
  R.EventsOn('library', (ph) => E.ev.push({ n: 'library', t: t(), count: ph ? ph.length : 0 }));
  R.EventsOn('view', (v) => E.ev.push({ n: 'view', t: t(), spreads: v && v.spreads ? v.spreads.length : (v && v.order ? v.order.length : 0) }));
  R.EventsOn('open:progress', (d, total) => E.ev.push({ n: 'open', t: t(), done: d, total }));
  E.hooked = true; return true;
}
"""

SCROLL_JS = """
async ({ sel, axis, dur }) => new Promise((res) => {
  const el = document.querySelector(sel);
  if (!el) return res(null);
  const max = axis === 'x' ? el.scrollWidth - el.clientWidth : el.scrollHeight - el.clientHeight;
  const t0 = performance.now(); let last = t0; const dts = [];
  const step = (t) => {
    const p = Math.min(1, (t - t0) / dur);
    if (axis === 'x') el.scrollLeft = max * p; else el.scrollTop = max * p;
    dts.push(t - last); last = t;
    if (p < 1) requestAnimationFrame(step); else res({ dts: dts.slice(1), t0, t1: t, max });
  };
  requestAnimationFrame(step);
})
"""

# Видимые <img> внутри прокручиваемого контейнера: сколько всего и сколько загружено (naturalWidth > 0).
VISIBLE_IMGS_JS = """
(sel) => {
  const box = document.querySelector(sel); if (!box) return null;
  const b = box.getBoundingClientRect(); let total = 0, loaded = 0;
  for (const im of box.querySelectorAll('img')) {
    const r = im.getBoundingClientRect();
    if (r.bottom <= b.top || r.top >= b.bottom || r.right <= b.left || r.left >= b.right || r.width === 0) continue;
    total++; if (im.complete && im.naturalWidth > 0) loaded++;
  }
  return { total, loaded };
}
"""

CALL_JS = "([name, args]) => window.go.app.App[name](...args)"


def percentile(values: list[float], p: float) -> float:
    """Перцентиль p (0…100) методом ближайшего ранга; пусто — 0."""
    if not values:
        return 0.0
    s = sorted(values)
    k = max(0, min(len(s) - 1, math.ceil(p * len(s) / 100) - 1))
    return s[k]


MARK = "=="  # тип записи в Stand.console: метка этапа, а не сообщение страницы


class Stand:
    """Страница стенда в Chromium. Использовать как контекстный менеджер."""

    def __init__(self, port: int, out: Path, profile: Profile | None = None, headed: bool = False, wait_s: float = 60.0):
        self.port, self.out, self.headed, self.wait_s = port, out, headed, wait_s
        self.profile = profile or PROFILES["A"]
        self.console: list[tuple[str, str]] = []
        self.failed: list[tuple[int, str]] = []  # ответы 4xx/5xx: код и путь
        self._pw = self._browser = self.ctx = self.page = self.cdp = None
        self._busy_since: float | None = None  # начало текущего вызова в страницу (для сторожа)
        self.hang_s = 180.0

    # ---- жизненный цикл ----

    def __enter__(self) -> "Stand":
        try:
            from playwright.sync_api import sync_playwright
        except ImportError:
            raise Refuse("нет Python Playwright (pip install playwright; playwright install chromium)") from None
        self.out.mkdir(parents=True, exist_ok=True)
        (self.out / "shots").mkdir(exist_ok=True)
        self._pw = sync_playwright().start()
        try:
            self._browser = self._pw.chromium.launch(headless=not self.headed)
        except Exception as e:  # noqa: BLE001 — любая причина отказа запуска браузера → код 2
            self.close()
            raise Refuse(f"Chromium не запустился: {str(e).splitlines()[0]}") from None
        self.open_context(self.profile)
        threading.Thread(target=self._watch, daemon=True).start()
        return self

    def _watch(self) -> None:
        """Сторож: вызов в страницу висит дольше hang_s — страница зависла; выходим с кодом 2 (Playwright сам без таймаута)."""
        while self._pw is not None:
            time.sleep(5)
            b = self._busy_since
            if b is not None and time.monotonic() - b > self.hang_s:
                print(f"e2e: страница не отвечает {self.hang_s:g} с — прогон прерван", flush=True)
                os._exit(2)

    def __exit__(self, *_exc) -> None:
        self.close()

    def open_context(self, profile: Profile) -> None:
        """Новая страница на профиле масштаба; прежний контекст закрывается (проект стенда остаётся тем же)."""
        if self.ctx is not None:
            self.ctx.close()
        self.profile = profile
        self.ctx = self._browser.new_context(
            viewport={"width": profile.w, "height": profile.h}, device_scale_factor=profile.dpr, ignore_https_errors=True
        )
        self.ctx.set_default_timeout(30000)
        self.ctx.add_init_script(INIT_JS)
        self.page = self.ctx.new_page()
        self.page.on("console", lambda m: self.console.append((m.type, m.text)) if m.type in ("error", "warning") else None)
        self.page.on("pageerror", lambda e: self.console.append(("pageerror", str(e))))
        self.page.on("dialog", lambda d: d.dismiss())
        # Консоль пишет «Failed to load resource» без адреса: адрес (путь без строки запроса) — в отдельную строку.
        self.page.on("response", lambda r: self.failed.append((r.status, r.url.split("?")[0].split("localhost:")[-1])) if r.status >= 400 else None)
        self.cdp = self.ctx.new_cdp_session(self.page)
        try:
            self.page.goto(f"http://localhost:{self.port}/", wait_until="load", timeout=int(self.wait_s * 1000))
            self.page.wait_for_function(
                "() => window.go && window.go.app && window.go.app.App && window.runtime && document.querySelector('[role=menubar]')",
                timeout=int(self.wait_s * 1000),
            )
        except Exception as e:  # noqa: BLE001
            raise Refuse(f"стенд на порту {self.port} не отдал интерфейс: {str(e).splitlines()[0]}") from None
        self.page.evaluate(HOOK_JS)
        self.dismiss_start()

    def mark(self, stage: str) -> None:
        """Метка этапа в console.txt: и среди сообщений консоли, и среди ответов 4xx — видно, на каком шаге пошли ошибки."""
        self.console.append((MARK, stage))
        self.failed.append((0, f"== {stage}"))

    def dismiss_start(self) -> None:
        """Стартовое окно («Продолжить», «Последние») само открывается при загрузке и закрывает всё остальное: Esc.

        Настройку «Показывать при запуске» не трогаем — она в config.json стенда.
        """
        self.wait_until(lambda: self.page.locator("[role=dialog]").count() > 0, 3, 0.2)
        for _ in range(3):
            if self.page.locator("[role=dialog]").count() == 0:
                break
            self.page.keyboard.press("Escape")
            self.sleep(0.3)

    def close(self) -> None:
        for obj in (self.ctx, self._browser):
            try:
                if obj is not None:
                    obj.close()
            except Exception:  # noqa: BLE001, S110 — закрытие после сбоя: молча
                pass
        try:
            if self._pw is not None:
                self._pw.stop()
        except Exception:  # noqa: BLE001, S110
            pass
        self.ctx = self._browser = self._pw = None

    # ---- примитивы ----

    def ev(self, js: str, arg: Any = None) -> Any:
        self._busy_since = time.monotonic()
        try:
            return self.page.evaluate(js, arg) if arg is not None else self.page.evaluate(js)
        finally:
            self._busy_since = None

    def call(self, name: str, *args: Any) -> Any:
        """Метод App (window.go.app.App); ошибка Go приходит исключением Playwright."""
        self._busy_since = time.monotonic()
        try:
            return self.page.evaluate(CALL_JS, [name, list(args)])
        finally:
            self._busy_since = None

    def now(self) -> float:
        return self.ev("performance.now()")

    def sleep(self, s: float) -> None:
        self.page.wait_for_timeout(int(s * 1000))

    def wait_until(self, fn: Callable[[], Any], timeout: float, poll: float = 0.25) -> Any:
        """Ждёт, пока fn() вернёт истину; по таймауту — None."""
        end = time.monotonic() + timeout
        while True:
            v = fn()
            if v:
                return v
            if time.monotonic() >= end:
                return None
            self.sleep(poll)

    # ---- события и длинные задачи ----

    def events(self, name: str, since: float = 0.0) -> list[dict]:
        return self.ev("([n, s]) => window.__e2e.ev.filter((e) => e.n === n && e.t >= s)", [name, since])

    def long_tasks(self, t0: float, t1: float | None = None) -> list[float]:
        """Длительности длинных задач, начавшихся в [t0, t1], мс."""
        lt = self.ev("() => window.__e2e.lt")
        return [x["d"] for x in lt if x["s"] >= t0 - 5 and (t1 is None or x["s"] <= t1)]

    def wait_quiet(self, since: float, quiet_ms: int = 1500, max_s: float = 90.0) -> bool:
        """Тихо: после since длинных задач не было последние quiet_ms. False — не дождались."""

        def quiet() -> bool:
            r = self.ev("() => { const l = window.__e2e.lt; return { now: performance.now(), last: l.length ? Math.max(...l.map((x) => x.s + x.d)) : 0 } }")
            return r["now"] - max(r["last"], since) > quiet_ms

        return bool(self.wait_until(quiet, max_s, 0.3))

    # ---- измерения ----

    def scroll_frames(self, sel: str, axis: str = "y", dur_ms: int = 3000) -> dict | None:
        """Прокрутка контейнера за dur_ms, шаг на каждый rAF: интервалы кадров и длинные задачи окна."""
        r = self.ev(SCROLL_JS, {"sel": sel, "axis": axis, "dur": dur_ms})
        if r is None:
            return None
        lt = self.long_tasks(r["t0"], r["t1"])
        return {"dts": r["dts"], "p95": percentile(r["dts"], 95), "max": max(r["dts"], default=0.0), "longest": max(lt, default=0.0), "span": r["max"]}

    def visible_imgs(self, sel: str) -> dict | None:
        return self.ev(VISIBLE_IMGS_JS, sel)

    def wait_imgs_loaded(self, sel: str, timeout: float) -> float | None:
        """Секунд до загрузки всех видимых <img> контейнера (None — не дождались или нет контейнера)."""
        t0 = time.monotonic()

        def done() -> bool:
            r = self.visible_imgs(sel)
            return bool(r) and r["total"] > 0 and r["loaded"] == r["total"]

        return time.monotonic() - t0 if self.wait_until(done, timeout, 0.15) else None

    def dom_counts(self) -> dict:
        return self.ev("() => ({ nodes: document.getElementsByTagName('*').length, imgs: document.images.length })")

    def heap_mb(self, gc: bool = True) -> float:
        """JSHeapUsedSize после сборки мусора (CDP), МБ."""
        if gc:
            self.cdp.send("HeapProfiler.collectGarbage")
        self.cdp.send("Performance.enable")
        for m in self.cdp.send("Performance.getMetrics")["metrics"]:
            if m["name"] == "JSHeapUsedSize":
                return round(m["value"] / 1048576, 1)
        return 0.0

    def view(self) -> dict:
        return self.call("GetView")

    def shot(self, name: str) -> str:
        """Снимок окна в shots/<name>.png; возвращает имя файла (без пути — в отчёт)."""
        fn = "".join(c if c.isalnum() or c in "-_." else "_" for c in name) + ".png"
        self.page.screenshot(path=str(self.out / "shots" / fn))
        return fn

    # ---- меню и окна (по data-item / ролям, не по координатам) ----

    def menu_item(self, top: str, item: str) -> None:
        """Файл/Правка/Вид/Справка ▸ пункт по data-item."""
        self.page.locator("[role=menubar] button", has_text=top).first.click()
        self.page.locator(f'[role=menu] [data-item="{item}"]').first.click(timeout=5000)

    def set_level(self, level: str) -> str | None:
        """Вид ▸ Уровень ▸ <level> через меню (интерфейс узнаёт о смене); возвращает прежний уровень.
        Часть пунктов («Все развороты», галереи) на уровне «Простой» из меню спрятана — стенд e2e стартует с него."""
        self.page.locator("[role=menubar] button", has_text="Вид").first.click()
        # Подменю открывает наведение; щелчок поверх него закрыл бы его обратно (MenuBar.pick).
        self.page.locator('[role=menu] [data-item="level"]').first.hover(timeout=5000)
        self.page.locator('[role=menu] [data-item^="level-"]').first.wait_for(timeout=5000)
        prev = self.ev(
            "() => { const c = document.querySelector('[role=menu] [data-item^=\"level-\"][aria-checked=true]');"
            " return c ? c.dataset.item.slice(6) : null }"
        )
        if prev == level:
            self.page.keyboard.press("Escape")
        else:
            self.page.locator(f'[role=menu] [data-item="level-{level}"]').first.click(timeout=5000)
        return prev

    def close_dialog(self) -> bool:
        """Esc закрывает верхнее окно; True — окон не осталось."""
        self.page.keyboard.press("Escape")
        return bool(self.wait_until(lambda: self.page.locator("[role=dialog]").count() == 0, 3, 0.1))

    # ---- консоль ----

    def console_problems(self) -> list[str]:
        return [f"{t}: {x}" for t, x in self.console if t != MARK and not any(s in x for s in NOT_OURS)]

    def write_console(self) -> None:
        lines = [f"== {x}" if t == MARK else f"{t}: {x}" for t, x in self.console]
        lines += [f"http {c}: {u}" if c else u for c, u in self.failed]
        (self.out / "console.txt").write_text("\n".join(lines) + ("\n" if lines else ""), encoding="utf-8")
