"""e2e масштаба экрана на стенде (089 L2-scale, design.md §5): профили A/B/C (D — по ключу), сценарии S1–S7.

    python scripts/e2e/scale.py --port 34118 --stand e2e [--profiles A,B,C] [--quick]

Эмуляция: viewport = CSS-размер окна, device_scale_factor = масштаб Windows (так WebView2 и видит 125/150 %).
Подготовка — через window.go.app.App без диалогов: папка фото, 6 разворотов, узор, стикер, рамка, текст.
Все метрики gate — «нарушений ≤ 0»: число нарушений на профиле, детали — в примечании; скриншоты — shots/.
Выход: 0 — gate зелёные; 1 — провал; 2 — отказ (нет стенда, набора, браузера, проект не чистый).
"""

from __future__ import annotations

import contextlib
import datetime as dt
import sys
import time

import common
from big import reachable
from common import Refuse
from driver import Stand
from report import GATE, INFO, Report, write_report
from scalejs import (
    CORNER_MENU_JS,
    DIALOG_FIT_JS,
    HSCROLL_JS,
    LIB_SHARP_JS,
    MENU_RECT_JS,
    MQ_KICK_JS,
    MQ_NATIVE_JS,
    RASTER_JS,
    TEXT_FIT_JS,
    ZONES_JS,
    expected_px,
    menu_verdict,
)

# Окна S3: (подпись, как открыть). «bar» — кнопка ленты по data-guide, иначе (меню, пункт по data-item).
WINDOWS = [
    ("autobuild", ("bar", "autobuild")),
    ("export", ("Файл", "export")),
    ("guide", ("Справка", "guide")),
    ("help", ("Справка", "help")),
    ("bg-gallery", ("Правка", "bgGallery")),
    ("borders", ("Правка", "borderGallery")),
    ("start", ("Справка", "startWindow")),
    ("levels", ("Вид", "compareLevels")),
    ("bins", ("bar", "bins")),
]
MENUS = [("Файл", "file"), ("Правка", "edit"), ("Вид", "view"), ("Справка", "help")]
LONG_TEXT = "Очень длинная строка текста для проверки рамки блока при масштабе экрана 125 и 150 процентов"
S7_FACTOR = 1.5


def parser():
    ap = common.base_parser("scale.py", "e2e масштаба экрана на стенде (089, L2-scale).")
    ap.add_argument("--profiles", default=common.DEFAULT_PROFILES, help="профили через запятую (A,B,C; D — 2560×1440 при 150 %%)")
    return ap


def s7_gate() -> bool:
    """S7 — gate после U3 (есть base/ui/dpr.svelte.ts), до неё info (design.md §3.4)."""
    return (common.ROOT / "frontend" / "src" / "base" / "ui" / "dpr.svelte.ts").is_file()


class Scale:
    def __init__(self, args, dirs, out, rep: Report, st: Stand):
        self.a, self.dirs, self.out, self.rep, self.st = args, dirs, out, rep, st
        self.ids: list[str] = []  # id разворотов в порядке ленты
        self.photos: list[str] = []
        self.broken = 0
        self.touched = False  # проект стенда уже менялся нами: по окончании вернуть пустой альбом
        self.first = ""  # ключ первого профиля: на нём S7
        self.a_profiles: list[str] = []

    # ---- подготовка (проект стенда; профиль A) ----

    def prep(self) -> None:
        st, rep = self.st, self.rep
        v = st.view()
        if v.get("folders") or v.get("canUndo") or v.get("spreads") or v.get("dirty"):
            raise Refuse("проект стенда не чистый (есть папки или правки): очистите стенд (Файл ▸ Новый альбом) и повторите")
        self.touched = True
        want = common.count_photos(self.dirs[:1])
        t0 = st.now()
        st.call("AddFolderPath", str(self.dirs[0]))
        if not st.wait_until(lambda: (st.events("library", t0) or [{"count": 0}])[-1]["count"] >= want, 240, 0.3):
            raise Refuse("список фото не собрался за 240 с")
        st.wait_quiet(t0, 1000, 60)
        self.photos = [p["id"] for p in st.call("GetState")["photos"]]
        if len(self.photos) < 22:
            raise Refuse(f"в наборе {len(self.photos)} фото, нужно не меньше 22")
        pos = len(self.photos) - 22  # из конца списка: первые ряды ленты остаются свободными и видны
        for n in (1, 2, 3, 4, 5, 7):
            st.call("NewSpread", -1, self.photos[pos : pos + n])
            pos += n
        sp = st.view()["spreads"]
        self.ids = [s["id"] for s in sp]
        notes = []
        sid = self.ids[1]
        notes.append("узор: " + self._pattern(sid))
        notes.append("стикер: " + self._first("sticker", lambda i: st.call("AddSticker", sid, 0, i)))
        notes.append("рамка: " + self._first("frame", lambda i: st.call("ApplyBorder", sid, sp[1]["photos"], i)))
        notes.append("текст: " + self._text(sid))
        rep.add("prep", len(self.ids), note="разворотов; " + "; ".join(notes))
        st.wait_quiet(st.now() - 100, 800, 30)

    def _pattern(self, sid: str) -> str:
        st = self.st
        try:
            for it in st.call("DecorCatalog", "bg")["items"][:15]:
                st.call("ApplyBackground", sid, "spread", it["id"])
                if next(s for s in st.view()["spreads"] if s["id"] == sid).get("patterns"):
                    return "есть"
        except Exception as e:  # noqa: BLE001
            return f"нет ({str(e).splitlines()[0][:60]})"
        return "нет (в каталоге не нашлось узора)"

    def _first(self, kind: str, put) -> str:
        try:
            items = self.st.call("DecorCatalog", kind)["items"]
            if not items:
                return "нет (каталог пуст)"
            put(items[0]["id"])
            return "есть"
        except Exception as e:  # noqa: BLE001
            return f"нет ({str(e).splitlines()[0][:60]})"

    def _text(self, sid: str) -> str:
        st = self.st
        try:
            v = st.call("AddText", sid, 0, "e2e")
            sp = next(s for s in v["spreads"] if s["id"] == sid)
            st.call("SetText", sp["texts"][-1]["id"], LONG_TEXT, "e2e")
            return "есть"
        except Exception as e:  # noqa: BLE001
            return f"нет ({str(e).splitlines()[0][:60]})"

    # ---- вспомогательное ----

    def add(self, key: str, prof: str, value, *, note: str = "", kind: str = GATE, limit: float = 0) -> None:
        """Метрика профиля: id вида `s1_hscroll_px.A`; gate — «нарушений ≤ limit»."""
        self.rep.add(f"{key}.{prof}", value, kind=kind, limit=limit if kind == GATE else None, note=note, unit="")

    def go_to(self, i: int) -> None:
        """Сделать разворот i текущим: щелчок по группе ленты альбома."""
        self.st.page.locator(f'[data-guide=timeline] .group[data-index="{i}"]').first.click(position={"x": 12, "y": 12}, timeout=5000)
        self.st.sleep(0.4)

    def clear(self) -> None:
        """Закрыть открытые окна и меню."""
        st = self.st
        for _ in range(4):
            if st.page.locator("[role=dialog], [role=menu]").count() == 0:
                break
            st.page.keyboard.press("Escape")
            st.sleep(0.25)

    @contextlib.contextmanager
    def full_level(self):
        """Уровень «Полный» на время шага: галереи и часть пунктов на «Простом» спрятаны (prefs.can), стенд e2e стартует с него.
        Без этого окно «не открывается» и шаг ничего не проверяет. Прежний уровень возвращаем (как B6 в big.py)."""
        self.clear()
        prev = self.st.set_level("full")
        try:
            yield
        finally:
            self.clear()
            if prev and prev != "full":
                self.st.set_level(prev)

    def guarded(self, name: str, prof: str, fn) -> None:
        self.st.mark(f"{prof} {name}")
        t0 = time.monotonic()
        try:
            fn()
            print(f"    {prof} {name}: {time.monotonic() - t0:.1f} с", flush=True)
        except Refuse:
            raise
        except Exception as e:  # noqa: BLE001 — сбой шага не прячет остальные
            self.broken += 1
            self.rep.add(f"{name}_error.{prof}", None, note=f"сбой: {str(e).splitlines()[0][:160]}")
            print(f"  {prof} {name}: сбой — {str(e).splitlines()[0][:160]}")
            self.clear()

    # ---- профиль ----

    def profile(self, prof: common.Profile) -> None:
        st, k = self.st, prof.key
        st.open_context(prof)
        st.wait_quiet(st.now() - 100, 1000, 60)
        self.go_to(1)
        st.shot(f"{k}-main")
        for name, fn in (
            ("s1", self.s1),
            ("s2", self.s2),
            ("s3", self.s3),
            ("s4", self.s4),
            ("s5", self.s5),
            ("s6", self.s6),
        ):
            self.guarded(name, k, lambda fn=fn: fn(k))
        if k == self.first:
            self.guarded("s7", k, self._s7)  # масштаб на лету — один раз, на первом профиле

    def s1(self, k: str) -> None:
        r = self.st.ev(HSCROLL_JS)
        self.add("s1_hscroll_px", k, r["over"], note=f"окно {r['vw']}×{r['vh']}, DPR {r['dpr']:g}")

    def s2(self, k: str) -> None:
        r = self.st.ev(ZONES_JS)
        self.add("s2_zones_missing", k, len(r["missing"]), note=", ".join(r["missing"]))
        self.add("s2_canvas_deficit_px", k, max(0, 240 - r["canvasH"]), note=f"холст {r['canvasH']:g} px, нужно ≥ 240")
        self.rep.add(f"s2_canvas_h_px.{k}", r["canvasH"], unit="px")

    def s3(self, k: str) -> None:
        with self.full_level():
            bad, unopened, notes = self._s3_windows(k)
        self.add("s3_dialog_overflow", k, len(bad), note="; ".join(notes)[:400])
        # Окно не открылось — проверка ничего не проверила: отдельный gate, не тихий 0 (QA 20.10).
        self.add("s3_dialog_unopened", k, len(unopened), note=", ".join(unopened))

    def _s3_windows(self, k: str) -> tuple[list[str], list[str], list[str]]:
        st = self.st
        bad, unopened, notes = [], [], []
        for name, how in WINDOWS:
            self.clear()
            try:
                if how[0] == "bar":
                    st.page.locator(f"[data-guide={how[1]}]").first.click(timeout=5000)
                else:
                    st.menu_item(*how)
                shown = st.wait_until(lambda: st.page.locator("[role=dialog]").count() > 0, 3, 0.15)
                st.sleep(0.3)
                if not shown:
                    unopened.append(name)
                    notes.append(f"{name}: не окно")
                    continue
                r = st.ev(DIALOG_FIT_JS)
                st.shot(f"{k}-{name}")
                if not r["fits"] or r["reach"] is False:
                    bad.append(name)
                    notes.append(f"{name}: вылезает {r['rect']}, кнопки {'недоступны' if r['reach'] is False else 'ok'}")
            except Exception as e:  # noqa: BLE001
                unopened.append(name)
                notes.append(f"{name}: не открылось ({str(e).splitlines()[0][:50]})")
            finally:
                self.clear()
        return bad, unopened, notes

    def s4(self, k: str) -> None:
        with self.full_level():
            bad, unopened, notes = self._s4_menus(k)
        self.add("s4_menu_overflow", k, len(bad), note="; ".join(notes)[:400])
        # Меню не открылось — проверка ничего не проверила: отдельный gate, не тихий 0 (QA 20.10).
        self.add("s4_menu_unopened", k, len(unopened), note=", ".join(unopened))

    def _s4_menus(self, k: str) -> tuple[list[str], list[str], list[str]]:
        st = self.st
        bad, unopened, notes = [], [], []

        def judge(name: str, r) -> None:
            v = menu_verdict(r)
            if v == "unopened":
                unopened.append(name)
                notes.append(f"меню {name}: не открылось")
            elif v == "overflow":
                bad.append(name)
                notes.append(f"меню {name}: вылезает (низ {r['menu']['bottom']:.0f} из {r['vh']})")

        for top, mid in MENUS:
            self.clear()
            st.page.locator("[role=menubar] button", has_text=top).first.click()
            st.sleep(0.25)
            m = st.ev(MENU_RECT_JS)
            judge(top, {"menu": m, "vw": m["vw"], "vh": m["vh"]} if m else None)
            if k == (self.a_profiles[0]):
                st.shot(f"{k}-menu-{mid}")
            self.clear()
        # Меню у правого нижнего края: событие contextmenu в углу окна (обработчики берут clientX/clientY).
        for name, sel in (("разворот", "[data-guide=timeline] .group"), ("фото", "[data-guide=canvas] [data-photo][data-drop]")):
            self.clear()
            r = st.ev(CORNER_MENU_JS, sel)
            if r is None:
                unopened.append(name)
                notes.append(f"меню {name}: нет цели {sel}")
                continue
            st.sleep(0.3)
            judge(name, r)
            self.clear()
        return bad, unopened, notes

    def s5(self, k: str) -> None:
        """Перенос фото из ленты в место 2 разворота и обмен 1↔3 — по GetView, а не по виду."""
        st = self.st
        sid2, sid4 = self.ids[1], self.ids[3]
        bad, notes = 0, []
        before = next(s for s in st.view()["spreads"] if s["id"] == sid2)["photos"]
        self.go_to(1)
        # Свободная (не в альбоме) плитка из видимого ряда ленты: id берём из DOM.
        free = st.ev("() => { const t = document.querySelector('[data-guide=library] [data-thumb]:not(.used)'); return t ? t.getAttribute('data-thumb') : null }")
        tile = st.page.locator(f'[data-guide=library] [data-thumb="{free}"]') if free else None
        if tile is None or tile.count() == 0:
            notes.append("перенос: нет свободной плитки в видимой ленте")
            bad += 1
        else:
            cell = st.page.locator(f'[data-guide=canvas] [data-spread="{sid2}"][data-photo][data-drop]').nth(1)
            self.drag(tile, cell, 0.9)  # правая четверть ячейки = «после»
            st.sleep(0.6)
            after = next(s for s in st.view()["spreads"] if s["id"] == sid2)["photos"]
            if free not in after[1:4]:
                bad += 1
                notes.append(f"перенос: фото не там, куда целились (было {len(before)}, стало {len(after)})")
            else:
                st.call("Undo")  # проект для следующего профиля — прежний
        self.go_to(3)
        cells = st.page.locator(f'[data-guide=canvas] [data-spread="{sid4}"][data-photo][data-drop]')
        ph0 = next(s for s in st.view()["spreads"] if s["id"] == sid4)["photos"]
        if cells.count() < 3:
            bad += 1
            notes.append(f"обмен: на развороте 4 только {cells.count()} ячеек")
        else:
            self.drag(cells.nth(0), cells.nth(2), 0.5)
            st.sleep(0.6)
            ph1 = next(s for s in st.view()["spreads"] if s["id"] == sid4)["photos"]
            if not (ph1[0] == ph0[2] and ph1[2] == ph0[0]):
                bad += 1
                notes.append("обмен 1↔3: фото не поменялись местами")
            else:
                st.call("Undo")
        self.add("s5_drag_miss", k, bad, note="; ".join(notes))

    def drag(self, src, dst, fx: float) -> None:
        """Перенос мышью: 10 шагов, как рука; fx — доля ширины цели."""
        st = self.st
        a, b = src.bounding_box(), dst.bounding_box()
        if not a or not b:
            raise RuntimeError("цель переноса вне экрана")
        st.page.mouse.move(a["x"] + a["width"] / 2, a["y"] + a["height"] / 2)
        st.page.mouse.down()
        st.page.mouse.move(b["x"] + b["width"] * fx, b["y"] + b["height"] / 2, steps=10)
        st.sleep(0.15)
        st.page.mouse.up()

    def s6(self, k: str) -> None:
        st = self.st
        self.go_to(1)
        r = st.ev(TEXT_FIT_JS, self.ids[1])
        if r is None:
            self.rep.add(f"s6_text_overflow_px.{k}", None, note="нет текстового блока на развороте 2")
        else:
            self.add("s6_text_overflow_px", k, max(0, round(r["over"] - 2, 1)), note=f"выход текста за рамку {r['over']:.1f} px (допуск 2)")
        bad, notes = 0, []
        for label, size in (("стандарт", None), ("максимум", "max")):
            if size:
                st.ev("() => { const r = document.querySelector('[data-guide=thumb-size]'); r.value = r.max; r.dispatchEvent(new Event('input', { bubbles: true })) }")
                st.sleep(1.0)
            st.wait_imgs_loaded("[data-guide=library] .grid", 60)
            s = st.ev(LIB_SHARP_JS)
            if s["soft"]:
                bad += s["soft"]
                notes.append(f"{label}: мягких {s['soft']} из {s['total']}")
            st.shot(f"{k}-library-{label}")
        st.ev("() => { const r = document.querySelector('[data-guide=thumb-size]'); if (r) { r.value = r.defaultValue; r.dispatchEvent(new Event('input', { bubbles: true })) } }")
        self.add("s6_thumbs_soft", k, bad, note="; ".join(notes))

    # ---- S7: масштаб на лету ----

    def _s7(self) -> None:
        st, p = self.st, self.st.profile
        self.go_to(1)
        before = st.ev(RASTER_JS)
        if not before["pattern"] and not before["sticker"]:
            self.rep.add("s7_raster_stale", None, note="нет узора и стикера на развороте 2: проверять нечего")
            return
        dpr = p.dpr * S7_FACTOR
        native0 = st.ev(MQ_NATIVE_JS)
        st.cdp.send("Emulation.setDeviceMetricsOverride", {"width": p.w, "height": p.h, "deviceScaleFactor": dpr, "mobile": False})
        st.sleep(0.5)
        native = st.ev(MQ_NATIVE_JS) - native0
        # CDP-эмуляция не шлёт change уже созданным спискам (089-D17): шлём сами, по копии списка.
        kicked = st.ev(MQ_KICK_JS)
        st.sleep(1.5)
        after = st.ev(RASTER_JS)
        st.shot(f"S7-dpr-{dpr:g}")
        st.cdp.send("Emulation.clearDeviceMetricsOverride")
        wrong = []
        for n in ("pattern", "sticker"):
            for i, im in enumerate(after[n]):
                want = expected_px(im["side"], dpr)
                if im["px"] != want:
                    wrong.append(f"{n}[{i}] px {im['px']} вместо {want} (сторона {im['side']})")
        kind = GATE if s7_gate() else INFO
        self.rep.add(
            "s7_raster_stale",
            len(wrong),
            kind=kind,
            limit=0 if kind == GATE else None,
            note=f"DPR {p.dpr:g} → {dpr:g}: " + ("; ".join(wrong[:3]) or "px совпал с ожидаемым") + ("" if kind == GATE else " (info: нет dpr.svelte.ts)"),
        )
        self.rep.add("s7_mq_native", native, kind=INFO, note=f"change пришёл сам: {native}; ручных рассылок: {kicked}")

    def finish(self) -> None:
        st, rep = self.st, self.rep
        if not self.touched:  # отказ до первой правки (проект не чистый): ничего не пишем, чужое не трогаем
            return
        st.write_console()
        bad = st.console_problems()
        rep.add("console_errors", len(bad), kind=GATE, limit=0, note="; ".join(bad[:3])[:300])
        if self.broken:
            rep.add("steps_failed", self.broken, kind=GATE, limit=0, note="сценарий оборвался: см. *_error выше")
        if self.touched:  # проект стенда — как был: пустой альбом (чужой не чистый проект не трогаем)
            st.call("NewProject", "album")


def run(args) -> int:
    common.check_port(args.port)
    common.check_stand(args.stand)
    slow = common.slow_factor()
    out = common.out_dir(args.out, "scale")
    profiles = common.parse_profiles(args.profiles)
    dirs = common.photo_dirs(args.photos, True)  # масштаб — на малом наборе a/ (120 фото)
    reachable(args.port)
    rep = Report("scale", dt.datetime.now().isoformat(timespec="seconds"), args.quick, common.count_photos(dirs), slow, common.machine_info())
    print(f"e2e scale: порт {args.port}, стенд {args.stand}, профили {','.join(p.key for p in profiles)}; отчёт: {out}")
    try:
        with Stand(args.port, out, profiles[0], headed=args.headed) as st:
            r = Scale(args, dirs, out, rep, st)
            r.a_profiles = [p.key for p in profiles]
            r.first = r.a_profiles[0]
            try:
                r.prep()
                for p in profiles:
                    print(f"  профиль {p.key}: {p.w}×{p.h} ×{p.dpr:g}…")
                    r.profile(p)
            finally:
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
