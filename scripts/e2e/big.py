"""e2e большого альбома на стенде (089 L2, design.md §4): сценарии B0–B10.

    python scripts/e2e/big.py --port 34118 --stand e2e [--quick] [--cold] [--warm-wait 900]

Лента фото 600, прокрутка лент (кадры, длинные задачи), DOM и <img>, правка, открытие e2e120.salbum, память
страницы и процесса стенда, консоль и log.txt. Время Go на стенде — только info (wails dev собирает без
оптимизаций). Выход: 0 — gate зелёные; 1 — провал gate; 2 — отказ (нет стенда, набора, браузера, проект не чистый).
"""

from __future__ import annotations

import datetime as dt
import json
import os
import re
import statistics
import sys
import time
import urllib.request
from pathlib import Path

import common
from common import Refuse
from driver import Stand
from report import GATE, Report, bad_lines, parse_log, write_report

GRID = "[data-guide=library] .grid"
TIMELINE = "[data-guide=timeline] .timeline"
DETAILS = "aside.details .scroll"

EDIT_JS = """
async ([id, name]) => {
  const E = window.__e2e; const t = performance.now();
  await window.go.app.App.SetSpreadName(id, name);
  const seen = () => E.ev.some((e) => e.n === 'view' && e.t >= t);
  for (let i = 0; i < 120 && !seen(); i++) await new Promise((r) => requestAnimationFrame(r));
  await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
  return performance.now() - t;
}
"""

CALL_TIMED_JS = """
async ([name, args]) => { const t = performance.now(); await window.go.app.App[name](...args); return performance.now() - t; }
"""


def parser():
    ap = common.base_parser("big.py", "e2e большого альбома на стенде (089, L2).")
    ap.add_argument("--cold", action="store_true", help="сменить mtime файлов набора: холодный кэш миниатюр")
    ap.add_argument("--warm-wait", type=float, default=900.0, help="сколько ждать конца прогрева миниатюр, с (полный режим)")
    ap.add_argument("--album", default=None, help="альбом для B8 (по умолчанию .testphotos-e2e/e2e120.salbum)")
    return ap


def reachable(port: int) -> None:
    try:
        urllib.request.urlopen(f"http://localhost:{port}/", timeout=4).close()
    except Exception:  # noqa: BLE001 — любая причина = стенд не отвечает
        raise Refuse(f"стенд на порту {port} не отвечает: поднимите его (scripts/stand.ps1) и повторите") from None


E2E_NAME = re.compile(r"(?:^|-)e2e-\d+\.jpg$", re.IGNORECASE)  # e2e-001.jpg и ссылки s001-01-e2e-336.jpg


def check_cold(dirs):
    """--cold меняет mtime файлов: только в наборе e2e (рядом manifest.txt генератора), не в чужой папке."""
    for d in dirs:
        if not (d.parent / "manifest.txt").is_file():
            raise Refuse(f"--cold только для набора e2e: рядом с {d} нет manifest.txt (go run scripts/e2e-photos.go); чужие фото не трогаем")


def cold(dirs):
    """mtime файлов e2e-*.jpg набора → новые ключи кэша миниатюр (прочее, симлинки и папки данных не трогаем)."""
    check_cold(dirs)
    now = time.time()
    n = 0
    for d in dirs:
        for root, _subs, files in os.walk(d, followlinks=False):
            for f in files:
                p = Path(root) / f
                if E2E_NAME.search(f) and not p.is_symlink() and p.is_file():
                    os.utime(p, (now, now))
                    n += 1
    return n


STAND_HEAD = re.compile(r"^стенд (\S+?), http://localhost:(\d+)")


def check_stand_log(name, port, gotmp=None):
    """Имя стенда ↔ порт по первой строке .gotmp/wails-dev-<имя>.log (stand.ps1): B8 пишет config.json стенда.

    Нет журнала — стенд поднят не stand.ps1: сверить нечем, возвращаем False (печатает вызывающий).
    """
    base = gotmp or Path(os.environ.get("GOTMPDIR") or common.ROOT / ".gotmp")
    log = base / f"wails-dev-{name}.log"
    try:
        with log.open(encoding="utf-8-sig") as f:
            head = f.readline()
    except OSError:
        return False
    m = STAND_HEAD.match(head)
    if not m or m.group(1) != name or int(m.group(2)) != port:
        raise Refuse(f"стенд {name!r} ≠ порт {port}: {log.name} говорит «{head.strip()[:80]}»; B8 пишет config.json стенда — остановлено")
    return True


def write_atomic(path, text):
    """Через временный файл и os.replace: оборванный прогон не оставит полфайла."""
    tmp = path.with_name(path.name + ".e2e-tmp")
    tmp.write_text(text, encoding="utf-8")
    os.replace(tmp, path)


class Run:
    """Состояние прогона: отчёт, браузер, журнал программы."""

    def __init__(self, args, dirs, out, rep: Report, st: Stand):
        self.a, self.dirs, self.out, self.rep, self.st = args, dirs, out, rep, st
        self.log = common.stand_dir(args.stand) / "log.txt"
        self.log_off = self.log.stat().st_size if self.log.exists() else 0
        self.expected = rep.count
        self.t_list = 0.0  # начало B1 (performance.now)
        self.heap0 = 0.0
        self.broken = 0
        self.dom: list[tuple[str, dict, bool]] = []  # этап, счётчики, входит ли в gate (список 600 и открытие e2e120)
        self._count, self._last_t = 0, 0.0

    def step(self, name, fn):
        self.st.mark(name)
        try:
            fn()
        except Refuse:
            raise
        except Exception as e:  # noqa: BLE001 — сбой шага не должен прятать результаты остальных
            self.broken += 1
            self.rep.add(f"{name}_error", None, note=f"сбой: {str(e).splitlines()[0][:160]}")
            print(f"  {name}: сбой — {str(e).splitlines()[0][:160]}")

    # ---- B0 ----
    def b0(self):
        st = self.st
        v = st.view()
        if v.get("folders") or v.get("canUndo") or v.get("spreads") or v.get("dirty"):
            raise Refuse("проект стенда не чистый (есть папки или правки): остановите прогон и очистите стенд (Файл ▸ Новый альбом)")
        self.heap0 = st.heap_mb()
        self.rep.add("cpu_load_start", common.cpu_load(), unit="%")
        self.rep.add("js_heap_start_mb", self.heap0)
        self.rep.machine["profile"] = f"A {st.profile.w}×{st.profile.h} ×{st.profile.dpr:g}"

    # ---- B1–B4 ----
    def b1(self):
        st, rep = self.st, self.rep
        if self.a.cold:
            rep.add("cold_files_touched", cold(self.dirs))
        self.t_list = t0 = st.now()
        target = 0
        for d in self.dirs:
            target += common.count_photos([d])
            st.call("AddFolderPath", str(d))
            done = st.wait_until(lambda: self._list_done(t0, target), 240, 0.2)
            if not done:
                raise RuntimeError(f"список не дождались: ждали {target} фото")
        t_done = self._last_t
        rep.add("list_ms", t_done - t0, unit="мс", note=f"фото {self._count}")
        if self._count != self.expected:
            rep.add("list_count_mismatch", self._count, note=f"ожидали {self.expected}")
        st.wait_quiet(t0)
        lt = st.long_tasks(t0)
        rep.add("list_longest_ms", max(lt, default=0), kind=GATE, limit=500, time=True, unit="мс")
        rep.add("list_total_ms", sum(lt), kind=GATE, limit=1000, time=True, unit="мс")
        dom = st.dom_counts()
        self.dom.append(("после списка", dom, True))
        st.shot("big-list")

    def _list_done(self, t0, target) -> bool:
        st = self.st
        lib = st.events("library", t0)
        scan = st.events("scan", t0)
        self._count = lib[-1]["count"] if lib else 0
        times = [e["t"] for e in (lib[-1:] + scan[-1:])]
        self._last_t = max(times, default=t0)
        started = any(e["active"] for e in scan)
        idle = bool(scan) and not scan[-1]["active"]
        return self._count >= target and (idle or not started)

    def b2(self):
        s = self.st.wait_imgs_loaded(GRID, 120)
        self.rep.add("first_screen_thumbs_ms", None if s is None else s * 1000, unit="мс", note="" if s is not None else "не загрузились за 120 с")

    def b3(self):
        st, rep = self.st, self.rep
        fr = st.scroll_frames(GRID, "y", 3000)
        if fr is None:
            raise RuntimeError(f"нет контейнера {GRID}")
        rep.add("strip_frame_p95_ms", fr["p95"], kind=GATE, limit=34, time=True, unit="мс", note=f"кадров {len(fr['dts'])}")
        rep.add("strip_frame_max_ms", fr["max"], kind=GATE, limit=120, time=True, unit="мс")
        rep.add("strip_longtask_ms", fr["longest"], kind=GATE, limit=100, time=True, unit="мс")
        st.ev("(s) => { const g = document.querySelector(s); g.scrollTop = g.scrollHeight }", GRID)
        ok = st.wait_imgs_loaded(GRID, 180)
        rep.add("jump_end_thumbs_ms", None if ok is None else ok * 1000, unit="мс", note="" if ok is not None else "не загрузились за 180 с")
        st.ev("(s) => { document.querySelector(s).scrollTop = 0 }", GRID)

    def b4(self):
        st, rep = self.st, self.rep
        if self.a.quick:
            rep.add("warm_all_ms", None, note="пропущено: --quick")
            return
        t0 = self.t_list

        def warmed():
            th = st.events("thumbs", t0)
            return next((e for e in th if e["total"] > 0 and e["done"] >= e["total"]), None)

        e = st.wait_until(warmed, self.a.warm_wait, 2.0)
        rep.add("warm_all_ms", None if not e else e["t"] - t0, unit="мс", note="" if e else f"прогрев не кончился за {self.a.warm_wait:g} с")

    # ---- B5–B7 ----
    def b5(self):
        st, rep, page = self.st, self.rep, self.st.page
        btn = page.locator("[data-guide=autobuild]")
        btn.click()
        dlg = page.locator("[role=dialog]")
        dlg.first.wait_for(timeout=10000)
        go = dlg.locator("button.primary").last
        t_open = time.monotonic()
        if not st.wait_until(lambda: go.is_enabled(), 180, 0.3):
            raise RuntimeError("кнопка «Собрать» не стала доступной за 180 с")
        rep.add("autobuild_dialog_ready_ms", (time.monotonic() - t_open) * 1000, unit="мс")
        st.shot("big-autobuild-dialog")
        t0 = st.now()
        go.click()
        ok = st.wait_until(lambda: any(e["spreads"] > 0 for e in st.events("view", t0)) and page.locator("[role=dialog]").count() == 0, 300, 0.2)
        if not ok:
            raise RuntimeError("развороты не появились за 300 с")
        t_view = min(e["t"] for e in st.events("view", t0) if e["spreads"] > 0)
        rep.add("autobuild_ui_ms", t_view - t0, unit="мс")
        st.wait_quiet(t0)
        rep.add("autobuild_apply_longest_ms", max(st.long_tasks(t0), default=0), unit="мс")
        self.dom.append(("после автосборки", st.dom_counts(), False))

    def b6(self):
        st, rep = self.st, self.rep
        fr = st.scroll_frames(TIMELINE, "x", 3000)
        if fr is None:
            raise RuntimeError(f"нет ленты альбома {TIMELINE}")
        rep.add("timeline_frame_p95_ms", fr["p95"], kind=GATE, limit=34, time=True, unit="мс", note=f"кадров {len(fr['dts'])}")
        rep.add("timeline_longtask_ms", fr["longest"], kind=GATE, limit=100, time=True, unit="мс")
        st.ev("(s) => { document.querySelector(s).scrollLeft = 0 }", TIMELINE)
        # «Все развороты» — функция уровня «Стандартный»+, а стенд e2e стартует с «Простого»: пункта в меню нет.
        prev = st.set_level("full")
        try:
            st.menu_item("Вид", "allSpreads")
            st.sleep(0.5)
            g = st.scroll_frames(DETAILS, "y", 3000)
            if g:
                rep.add("allspreads_frame_p95_ms", g["p95"], unit="мс")
                rep.add("allspreads_longtask_ms", g["longest"], unit="мс")
            self.dom.append(("«Все развороты»", st.dom_counts(), False))
            st.shot("big-allspreads")
        finally:
            if prev and prev != "full":
                st.set_level(prev)  # уровень стенда возвращаем: прогон не должен менять настройки

    def b7(self):
        st, rep = self.st, self.rep
        sp = st.view()["spreads"]
        if not sp:
            raise RuntimeError("нет разворотов для правки")
        times = [st.ev(EDIT_JS, [sp[0]["id"], f"e2e-{i}"]) for i in range(5)]
        rep.add("edit_frame_ms", statistics.median(times), kind=GATE, limit=60, time=True, unit="мс", note="медиана из 5")
        rep.add("edit_first_ms", times[0], unit="мс")

    # ---- B8 ----
    def b8(self):
        st, rep = self.st, self.rep
        album = Path(self.a.album) if self.a.album else common.PHOTOS / "e2e120.salbum"
        if not album.is_file():
            rep.add("open_ms", None, note=f"пропущено: нет {album.name} (go run scripts/qa-bigalbum.go, docs/qa/e2e.md)")
            return
        cfg = common.stand_dir(self.a.stand) / "config.json"
        if not cfg.is_file():
            raise RuntimeError("нет config.json стенда: путь альбома в «Последние» не дописать")
        path = str(album.resolve())
        orig = cfg.read_text(encoding="utf-8")
        try:
            data = json.loads(orig)
            data["recent"] = [path] + [p for p in data.get("recent", []) if p != path]
            write_atomic(cfg, json.dumps(data, ensure_ascii=False, indent=2))
            t0 = st.now()
            ms = st.ev(CALL_TIMED_JS, ["OpenRecent", [path]])
            rep.add("open_ms", ms, unit="мс")
            st.wait_quiet(t0)
            rep.add("open_longest_ms", max(st.long_tasks(t0), default=0), kind=GATE, limit=500, time=True, unit="мс")
            self.dom.append(("после открытия e2e120", st.dom_counts(), True))
            st.shot("big-open")
        finally:
            # Возврат записи: остальное в config.json могло измениться за время прогона — правим только «recent».
            try:
                now = json.loads(cfg.read_text(encoding="utf-8"))
                now["recent"] = [p for p in now.get("recent", []) if p != path]
                write_atomic(cfg, json.dumps(now, ensure_ascii=False, indent=2))
            except (OSError, ValueError):
                write_atomic(cfg, orig)

    def b9(self):
        st, rep = self.st, self.rep
        rep.add("js_heap_mb", st.heap_mb(), kind=GATE, limit=250, unit="МБ", note="пик: 600 фото + открытый альбом")
        rep.add("go_ws_peak_mb", common.peak_working_set_mb(common.stand_pid(self.a.port)), kind=GATE, limit=3072, unit="МБ")

    # ---- B10 ----
    def b10(self):
        st, rep = self.st, self.rep
        t0 = st.now()
        st.call("NewProject", "album")
        st.wait_quiet(t0, 1500, 60)
        leak = st.heap_mb() - self.heap0
        rep.add("js_heap_leak_mb", leak, kind=GATE, limit=50, unit="МБ", note="после «Нового» минус начало")
        # DOM и <img>: gate — максимум после списка и после открытия e2e120; остальные этапы — для сведения.
        gated = [d for _, d, g in self.dom if g]
        note = "; ".join(f"{n}: {d['nodes']}/{d['imgs']}" for n, d, _ in self.dom)
        rep.add("dom_nodes", max((d["nodes"] for d in gated), default=None), kind=GATE, limit=4000, note=note)
        rep.add("img_count", max((d["imgs"] for d in gated), default=None), kind=GATE, limit=300, note="узлы/<img> по этапам — у dom_nodes")
        for n, d, g in self.dom:
            if not g:
                rep.add(f"dom_nodes ({n})", d["nodes"], note=f"<img> {d['imgs']}")
        rep.add("cpu_load_end", common.cpu_load(), unit="%")

    def finish(self):
        """Консоль и хвост log.txt с начала прогона."""
        st, rep = self.st, self.rep
        st.write_console()
        bad = st.console_problems()
        rep.add("console_errors", len(bad), kind=GATE, limit=0, note="; ".join(bad[:3])[:300])
        text, _ = common.tail_from(self.log, self.log_off)
        (self.out / "log-tail.txt").write_text(text, encoding="utf-8")
        badl = bad_lines(text)
        rep.add("log_bad", len(badl), kind=GATE, limit=0, note="; ".join(badl[:2])[:300])
        ev = parse_log(text)
        if ev["folders"]:
            f = ev["folders"][0]
            rep.add("log_scan_walk_ms", f["walk_ms"], unit="мс")
            rep.add("log_scan_read_ms", f["read_ms"], unit="мс")
        if ev["thumbs"]:
            rep.add("log_thumbs_ms", ev["thumbs"][-1]["ms"], unit="мс", note=f"фото {ev['thumbs'][-1]['photos']}")
        if ev["autobuild"]:
            rep.add("log_autobuild_ms", ev["autobuild"][-1]["ms"], unit="мс")
        for m in ev["memory"]:
            rep.add(f"log_mem_{m['stage']}", m["go_mb"], unit="МБ", note=f"куча {m['heap_mb']} МБ")
        if self.broken:
            rep.add("steps_failed", self.broken, kind=GATE, limit=0, note="сценарий оборвался: см. *_error выше")


def run(args) -> int:
    common.check_port(args.port)
    common.check_stand(args.stand)
    slow = common.slow_factor()
    out = common.out_dir(args.out, "stand")
    dirs = common.photo_dirs(args.photos, args.quick)
    expected = common.count_photos(dirs)
    if expected == 0:
        raise Refuse("в папках набора нет JPEG")
    reachable(args.port)
    if args.cold:
        check_cold(dirs)
    if not check_stand_log(args.stand, args.port):
        print(f"  внимание: нет wails-dev-{args.stand}.log — стенд поднят не stand.ps1, имя и порт не сверены")
    rep = Report("stand", dt.datetime.now().isoformat(timespec="seconds"), args.quick, expected, slow, common.machine_info())
    print(f"e2e big: порт {args.port}, стенд {args.stand}, фото {expected}, {'quick' if args.quick else 'полный'}; отчёт: {out}")
    try:
        with Stand(args.port, out, headed=args.headed) as st:
            r = Run(args, dirs, out, rep, st)
            st.mark("b0")
            r.b0()
            for name in ("b1", "b2", "b3", "b4", "b5", "b6", "b7", "b8", "b9", "b10"):
                print(f"  {name}…")
                r.step(name, getattr(r, name))
            r.finish()
    except Refuse:
        common.drop_empty(out)
        raise
    write_report(rep, out)
    bad = rep.failed()
    print("\n" + (f"ПРОВАЛ gate: {', '.join(m.id for m in bad)}" if bad else "gate зелёные") + f"\nОтчёт: {out / 'report.md'}")
    return rep.exit_code()


def main() -> int:
    return common.run_main(lambda: run(parser().parse_args()))


if __name__ == "__main__":
    sys.exit(main())
