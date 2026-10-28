"""Действия сцены над страницей стенда (102, design §4): курсор идёт путём motion.path, щелчки, перенос, колесо.

Каждое действие ждёт результата по состоянию (`wait`), а не по времени; дольше WAIT_S — SceneFail с меткой
(код 4 в demo.py). Все паузы и пути умножаются на `slow` (медленный режим, design §5.4). Паузы идут через
`page.wait_for_timeout`: Playwright sync обрабатывает события захвата только внутри своих вызовов, а `time.sleep`
остановил бы приём кадров.
"""

from __future__ import annotations

import time
from typing import Any, Callable

import motion

WAIT_S = 5.0  # дольше ждать результата действия — сцена падает (код 4)
HOLD_S = 0.06  # сколько кнопка мыши держится при щелчке


class SceneFail(Exception):
    """Сцена не дождалась результата: метка, на которой упала, и что ждали."""

    def __init__(self, mark: str, what: str, reload: bool = False):
        super().__init__(f"метка {mark}: не дождались — {what}")
        self.mark, self.what = mark, what
        self.reload = reload  # причина — перезагрузка страницы (стенд пересобран): сцену можно повторить


class Pilot:
    def __init__(self, st, slow: float = 1.0, on_mark: Callable[[str, str], None] | None = None):
        self.st, self.page, self.slow = st, st.page, max(1.0, slow)
        self.x = self.y = 0.0
        self.cur = "start"  # последняя метка: в SceneFail
        self.on_mark = on_mark
        self.loads = self._loads()  # число загрузок страницы на старте прохода

    def _loads(self) -> int:
        try:
            return int(self.page.evaluate("() => Number(sessionStorage.getItem('__demoLoads') || 0)"))
        except Exception:  # noqa: BLE001 — контекст уничтожен перезагрузкой: считаем, что загрузок больше
            return -1

    def check_alive(self) -> None:
        """Страница не перезагружалась с начала прохода: иначе окно «Продолжить», разворот 1 и пустой ролик (wails dev пересобрал код)."""
        if self.loads >= 0 and self._loads() != self.loads:
            raise SceneFail(self.cur, "страница перезагрузилась во время записи (стенд пересобран? не сливать в его ветку на время записи)", reload=True)

    # ---- время ----

    def sleep(self, s: float) -> None:
        self.page.wait_for_timeout(max(0.0, s * self.slow * 1000))

    def mark(self, name: str, note: str = "") -> None:
        self.cur = name
        self.check_alive()
        if self.on_mark:
            self.on_mark(name, note)

    def wait(self, fn: Callable[[], Any], what: str, timeout: float = WAIT_S) -> Any:
        """Ждёт истину от fn() до timeout (по часам стены, без slow); иначе SceneFail."""
        end = time.monotonic() + timeout
        while True:
            v = fn()
            if v:
                return v
            if time.monotonic() >= end:
                self.check_alive()  # причина «не дождались» — перезагрузка страницы
                raise SceneFail(self.cur, what)
            self.page.wait_for_timeout(60)

    # ---- мышь ----

    def put(self, x: float, y: float) -> None:
        """Мгновенно поставить курсор (до записи): без пути."""
        self.x, self.y = x, y
        self.page.mouse.move(x, y)
        self.page.evaluate("([x, y]) => window.__demoCursor && window.__demoCursor.set(x, y)", [x, y])

    def _play(self, steps: list[motion.Step]) -> None:
        t0 = time.perf_counter()
        for s in steps:
            wait = s.t * self.slow - (time.perf_counter() - t0)
            if wait > 0.002:
                self.page.wait_for_timeout(wait * 1000)
            self.page.mouse.move(s.x, s.y)
        self.x, self.y = steps[-1].x, steps[-1].y

    def move(self, x: float, y: float, dur: float | None = None) -> None:
        a = (self.x, self.y)
        if (x, y) == a:
            return
        d = motion.default_duration(((x - a[0]) ** 2 + (y - a[1]) ** 2) ** 0.5) if dur is None else dur
        self._play(motion.path(a, (x, y), d))

    def center(self, target: Any) -> tuple[float, float]:
        """Центр видимого элемента (локатор Playwright или CSS-селектор)."""
        loc = self.page.locator(target).first if isinstance(target, str) else target.first

        def box():
            # Элемент мог перерисоваться между проверкой и измерением (окна Svelte): не падаем, ждём дальше.
            try:
                return loc.bounding_box(timeout=1000) if loc.count() > 0 and loc.is_visible() else None
            except Exception:  # noqa: BLE001 — Playwright: элемент исчез/сменился
                return None

        b = self.wait(box, f"виден элемент {target}")
        return b["x"] + b["width"] / 2, b["y"] + b["height"] / 2

    def to(self, target: Any, dx: float = 0, dy: float = 0, dur: float | None = None) -> tuple[float, float]:
        x, y = self.center(target)
        self.move(x + dx, y + dy, dur)
        return self.x, self.y

    def click(self, target: Any = None, dx: float = 0, dy: float = 0, shift: bool = False) -> None:
        """Идёт к цели (None — щелчок на месте), покой, щелчок. shift — с зажатым Shift."""
        if target is not None:
            self.to(target, dx, dy)
        self.sleep(motion.CLICK_REST)
        if shift:
            self.page.keyboard.down("Shift")
        try:
            self.page.mouse.down()
            self.page.wait_for_timeout(HOLD_S * 1000)
            self.page.mouse.up()
        finally:
            if shift:
                self.page.keyboard.up("Shift")

    def drag(self, src: Any, dst: tuple[float, float] | Any, dst_rest: float = 0.2, dur: float | None = None) -> None:
        """Перенос: нажатие на src, 3 шага по 3 px (порог 5 px), путь к dst, покой над целью, отпускание.

        dst — точка (x, y) или цель; точка нужна там, где цель видна лишь краем (слот «Новый разворот»).
        """
        self.to(src)
        self.sleep(motion.CLICK_REST)
        self.page.mouse.down()
        for x, y in motion.drag_start_steps((self.x, self.y)):
            self.page.mouse.move(x, y)
            self.page.wait_for_timeout(16)
            self.x, self.y = x, y
        tx, ty = dst if isinstance(dst, tuple) else self.center(dst)
        self.move(tx, ty, dur)
        self.sleep(dst_rest)
        self.page.mouse.up()

    def wheel_until(self, done: Callable[[], bool], what: str, dy: float = 60, every: float = 0.04, limit_s: float = 8.0) -> None:
        """Колесо по ряду разворотов: серия `wheel(0, dy)` раз в `every` с, пока done() не станет истиной."""
        end = time.monotonic() + limit_s
        while not done():
            if time.monotonic() > end:
                raise SceneFail(self.cur, what)
            self.page.mouse.wheel(0, dy)
            self.page.wait_for_timeout(every * 1000 * self.slow)


def guarded(p: "Pilot", fn: Callable[..., Any], *args: Any) -> None:
    """fn(p, *args); перезагрузка страницы посреди вызова Playwright («Execution context was destroyed») — SceneFail, не трассировка."""
    try:
        fn(p, *args)
    except SceneFail:
        raise
    except Exception as e:  # noqa: BLE001 — разбираем текст ошибки Playwright
        t = str(e)
        # Таймаут Playwright на белой странице — тоже перезагрузка: счётчик загрузок изменился или страница не отвечает.
        if "context was destroyed" in t or "navigation" in t or (p.loads >= 0 and p._loads() != p.loads):
            raise SceneFail(p.cur, "страница перезагрузилась во время записи (стенд пересобран? не сливать в его ветку на время записи)", reload=True) from None
        raise
