"""Лента разворотов: зоны разреза и склейки под Ctrl (R2, R3; 158). Признак — elementFromPoint по строке группы и результат в Go."""

from base import RegressCase, bug

CHIPS = ".timeline .chip[data-photo]"
GROUPS = ".timeline .group"
TOL = 1  # px: проход идёт по целым пикселям
B = 2  # белая рамка миниатюры (timeline.ts)
INSIDE = 3  # px внутрь от края изображения: зона разреза заходит в фото на 4

# Проход по середине первой группы ленты: серии подряд идущих элементов (первый класс или тег) и их ширина в px.
SCAN_JS = """
() => {
  const g = document.querySelector('.timeline .group'), r = g.getBoundingClientRect(), y = r.top + 26, run = [];
  for (let x = r.left; x < r.right + 30; x++) {
    const e = document.elementFromPoint(x + 0.5, y), k = e?.classList[0] ?? e?.tagName;
    if (run.at(-1)?.[0] === k) run.at(-1)[1]++; else run.push([k, 1]);
  }
  return run;
}
"""

CHIP_BOXES_JS = """
() => [...document.querySelectorAll('.timeline .chip[data-photo]')].map((e) => {
  const r = e.getBoundingClientRect(); return { left: r.left, right: r.right, y: r.top + r.height / 2 };
})
"""


class StripCase(RegressCase):
    """Общее для лент: альбом из двух групп по три фото и ожидание, что ряд миниатюр нарисован."""

    def strip(self) -> list[dict]:
        self.album([3, 3])
        self.wait_js("(sel) => document.querySelectorAll(sel).length === 6", CHIPS)
        self.stable_rect(CHIPS)
        return self.st.ev(CHIP_BOXES_JS)

    def timeline_has(self, cls: str, on: bool = True) -> None:
        self.wait_js("([c, on]) => document.querySelector('.timeline').classList.contains(c) === on", [cls, on])

    def widths(self, kind: str) -> list[int]:
        return [w for k, w in self.st.ev(SCAN_JS) if k == kind]

    def ctrl_click(self, x: float, y: float) -> None:
        self.page.mouse.move(x, y)
        self.page.keyboard.down("Control")
        self.timeline_has("cutting")
        self.page.mouse.click(x, y)
        self.page.keyboard.up("Control")
        self.timeline_has("cutting", False)


@bug(id="R2", fix="b882eed3", src="158-7: QA руками (QA 08.10, К3)", red="revert b882eed3 (зона разреза 6 px, черта 14 px под Ctrl)")
class StripZones(StripCase):
    def test_ctrl_zones_widths(self):
        boxes = self.strip()
        # Курсор над лентой: pointermove нужен cutKey, класс .cutting ставит настоящий keydown.
        self.page.mouse.move(boxes[1]["left"] + 30, boxes[1]["y"])

        # Без Ctrl зон разреза нет, черта между группами — 14.
        self.timeline_has("cutting", False)
        self.assertEqual(self.widths("cut"), [])
        joins = self.widths("join")
        self.assertEqual(len(joins), 1)
        self.assertAlmostEqual(joins[0], 14, delta=TOL)

        # С Ctrl: две зоны разреза по 14 (6 в зазоре + по 4 внутрь соседних фото), черта 24.
        self.page.keyboard.down("Control")
        self.timeline_has("cutting")
        cuts, joins = self.widths("cut"), self.widths("join")
        self.assertEqual(len(cuts), 2, f"зон разреза в группе из трёх фото {cuts}")
        for w in cuts:
            self.assertAlmostEqual(w, 14, delta=TOL, msg=f"зона разреза {w} px")
        self.assertEqual(len(joins), 1)
        self.assertAlmostEqual(joins[0], 24, delta=TOL, msg=f"черта склейки {joins[0]} px")

        # Ctrl отпущен: как без него.
        self.page.keyboard.up("Control")
        self.timeline_has("cutting", False)
        self.assertEqual(self.widths("cut"), [])
        self.assertAlmostEqual(self.widths("join")[0], 14, delta=TOL)

    def test_ctrl_zones_during_drag(self):
        boxes = self.strip()
        # Перенос фото, затем Ctrl: идёт перенос — зон разреза нет, черта прежние 14 (бросок в край группы не отнимаем).
        rev0 = self.rev()
        self.page.mouse.move(boxes[0]["left"] + 30, boxes[0]["y"])
        self.page.mouse.down()
        self.page.mouse.move(boxes[0]["left"] + 60, boxes[0]["y"] + 100, steps=6)
        self.timeline_has("dragging")
        self.page.keyboard.down("Control")
        self.page.mouse.move(boxes[0]["left"] + 62, boxes[0]["y"] + 101)
        self.timeline_has("cutting")
        self.assertEqual(self.widths("cut"), [], "во время переноса зоны разреза ловят мышь")
        joins = self.widths("join")
        self.assertEqual(len(joins), 1)
        self.assertAlmostEqual(joins[0], 14, delta=TOL, msg=f"черта при переносе {joins[0]} px")
        # Esc отменяет перенос: проект не менялся.
        self.page.keyboard.press("Escape")
        self.page.keyboard.up("Control")
        self.page.mouse.up()
        self.timeline_has("dragging", False)
        self.assertEqual(self.rev(), rev0)


@bug(id="R3", fix="b882eed3", src="158 (а): QA руками, Ctrl+щелчок у края фото", red="revert b882eed3 (зона разреза не заходит в фото)")
class StripCut(StripCase):
    def test_ctrl_click_near_edge_cuts(self):
        # Край изображения — внутренний край рамки миниатюры; щелчок в 3 px от него по обе стороны разреза режет.
        for side in ("справа от разреза", "слева от разреза"):
            with self.subTest(side=side):
                self.reopen("A")
                boxes = self.strip()
                x = boxes[1]["left"] + B + INSIDE if side.startswith("справа") else boxes[0]["right"] - B - INSIDE
                rev0 = self.rev()
                self.ctrl_click(x, boxes[1]["y"])
                # Исход один из двух: правка дошла из Go или группа попала в выделение; ждём любой, чтобы провал был утверждением, а не таймаутом.
                self.wait_js("([r, sel]) => window.__reg.rev > r || document.querySelector(sel)", [rev0, GROUPS + ".picked"])
                view = self.call("GetView")
                self.assertEqual(sorted(len(s["photos"]) for s in view["spreads"]), [1, 2, 3], "Ctrl+щелчок у края не разрезал группу")
                self.expect(self.q(".toast .ghost")).to_be_visible(timeout=self.lim(10) * 1000)  # «Вернуть»
                self.wait_js("(sel) => document.querySelectorAll(sel).length === 3", GROUPS)
                self.assertEqual(self.q(GROUPS + ".picked").count(), 0, "группа попала в выделение вместо разреза")

    def test_ctrl_click_middle_picks_group(self):
        # Середина фото — прежнее «группа в выделение»: разворотов столько же, тоста нет.
        boxes = self.strip()
        mid = (boxes[1]["left"] + boxes[1]["right"]) / 2
        rev0 = self.rev()
        self.ctrl_click(mid, boxes[1]["y"])
        self.expect(self.q(GROUPS + ".picked")).to_have_count(1, timeout=self.lim(10) * 1000)
        self.assertEqual(self.q(GROUPS).count(), 2)
        self.assertEqual(self.q(".toast").count(), 0)
        self.assertEqual(self.rev(), rev0)
