"""Регресс переноса (166): R7 событие view посреди переноса фото на холсте; R9 — Ctrl-замена фото на «Авто» (168).

Лежит отдельно от reg_canvas.py (R4): два исполнителя правят разные файлы.
Бросок берёт цель по нынешнему DOM (rehit в drag.svelte.ts). Тест держит кнопку, меняет вид через Go и отпускает
без единого движения мыши: тогда цель, найденная на последнем движении, устарела, и только rehit берёт верную.
"""

from base import RegressCase, bug

CELL = ".cell[data-drop=cell]"

# Фото под точкой: верхняя ячейка холста в стопке элементов (призрак и подсветки мышь не ловят).
UNDER_JS = """
([x, y]) => {
  const c = document.elementsFromPoint(x, y).find((e) => e.dataset && e.dataset.drop === 'cell');
  return c ? c.dataset.photo : null;
}
"""

DRAGGING = "() => document.body.classList.contains('dragging')"
IDLE = "() => !document.body.classList.contains('dragging') && !document.querySelector('.ghost-stack')"


@bug(id="R7", fix="82579107", src="149 Т5, правка агента посреди переноса", red="revert 82579107 (rehit в drag.svelte.ts)")
class DragMidView(RegressCase):
    def _center(self, photo: str) -> tuple[float, float]:
        b = self.q(f".cell[data-photo='{photo}']").first.bounding_box()
        return b["x"] + b["width"] / 2, b["y"] + b["height"] / 2

    def _grab(self, a: str, x: float, y: float) -> None:
        """Нажать на фото a и довести мышь до (x, y); кнопка остаётся зажатой, перенос идёт."""
        ax, ay = self._center(a)
        m = self.page.mouse
        m.move(ax, ay)
        m.down()
        m.move(ax + 20, ay + 10, steps=3)  # дальше порога 5 px: перенос начался
        m.move(x, y, steps=8)
        self.wait_js(DRAGGING)

    def _photos_after_drop(self, before: int, timeout: float = 6) -> list[str]:
        """Фото разворота после броска; не дождались новой правки — провал с понятным текстом, а не таймаут."""
        from playwright.sync_api import TimeoutError as PwTimeout

        try:
            return self.wait_rev(before, timeout=timeout)["spreads"][0]["photos"]
        except PwTimeout:
            self.fail("после броска вид не изменился: Go отказал или бросок ушёл мимо (ошибка в тосте?)")

    def _await_view(self, after: int) -> None:
        self.wait_js("(r) => window.__reg.rev > r", after)

    def test_drop_after_view_event(self):
        """Агент поменял соседние фото местами, мышь стоит: бросок обменивается с фото, что под курсором сейчас."""
        sid_photos = self.album([3])["spreads"][0]["photos"]
        a, b, c = sid_photos
        self.wait_js("(n) => document.querySelectorAll('.cell[data-drop=cell]').length >= n", 3)
        bx, by = self._center(b)
        self._grab(a, bx, by)
        before = self.rev()

        self.call("SwapPhotos", b, c)  # теперь на месте b стоит c; событие view идёт и в эту страницу
        self._await_view(before)
        self.wait_js("([i, p]) => document.querySelector(`.cell[data-index='${i}']`)?.dataset.photo === p", [1, c])
        self.stable_rect(".cell[data-index='1']")  # ячейки отъехали и встали
        self.assertEqual(self.st.ev(UNDER_JS, [bx, by]), c, "под курсором должно быть c")
        self.assertTrue(self.st.ev(DRAGGING), "перенос оборван событием view")

        before = self.rev()
        self.page.mouse.up()  # без движения: цель — только та, что пересчитал rehit
        got = self._photos_after_drop(before)
        self.assertEqual(got, [c, a, b], f"обмен ушёл не с тем фото, что под курсором: {got}")
        self.wait_js(IDLE)

    def test_drop_target_photo_removed(self):
        """Агент убрал фото-цель, мышь стоит: бросок идёт на фото, что встало под курсор, а не на пропавшее."""
        a, b, c = self.album([3])["spreads"][0]["photos"]
        self.wait_js("(n) => document.querySelectorAll('.cell[data-drop=cell]').length >= n", 3)
        bx, by = self._center(b)
        self._grab(a, bx, by)
        before = self.rev()

        self.call("RemovePhotos", [b])  # раскладка из двух фото: на месте b теперь c
        self._await_view(before)
        self.wait_js("(n) => document.querySelectorAll('.cell[data-drop=cell]').length === n", 2)
        self.stable_rect(f".cell[data-photo='{c}']")
        self.assertEqual(self.st.ev(UNDER_JS, [bx, by]), c, "под курсором должно быть c")
        self.assertTrue(self.st.ev(DRAGGING), "перенос оборван событием view")

        before = self.rev()
        self.page.mouse.up()
        got = self._photos_after_drop(before)
        self.assertEqual(got, [c, a], f"бросок ушёл не на фото под курсором: {got}")
        self.wait_js(IDLE)
        self.settle()
        self.expect(self.q(".toast.error")).to_have_count(0)


HINT_JS = "() => document.querySelector('.ghost-stack .hint')?.textContent ?? ''"
FREE_THUMB_JS = "() => document.querySelector('.thumb:not(.used)')?.dataset.thumb ?? null"


@bug(id="R9", fix="168 U1", src="168: замена фото на «Авто» с Ctrl, раскладка не меняется", red="cellSide без keep в hit.ts (центр из ленты снова «после»)")
class ReplaceAuto(RegressCase):
    def _center(self, sel: str) -> tuple[float, float]:
        b = self.q(sel).first.bounding_box()
        return b["x"] + b["width"] / 2, b["y"] + b["height"] / 2

    def _carry(self, thumb: str, cell: str, ctrl: bool) -> None:
        """Фото из ленты довести до центра ячейки; Ctrl нажимаем уже посреди переноса, мышь после этого стоит."""
        tx, ty = self._center(f".thumb[data-thumb='{thumb}']")
        m = self.page.mouse
        m.move(tx, ty)
        m.down()
        m.move(tx + 20, ty + 10, steps=3)
        cx, cy = self._center(f".cell[data-photo='{cell}']")
        m.move(cx, cy, steps=8)
        self.wait_js(DRAGGING)
        if ctrl:
            self.page.keyboard.down("Control")

    def _drop(self, ctrl: bool) -> None:
        self.page.mouse.up()
        if ctrl:
            self.page.keyboard.up("Control")
        self.wait_js(IDLE)

    def test_ctrl_replace_auto(self):
        """Ctrl: фото встаёт на место второго, прямоугольники те же, прежнее в ленте; Ctrl+Z возвращает; без Ctrl — пятое фото.

        Фото стенда одного размера (1600×1067, scripts/qa-albums.go), поэтому геометрию со своим кадром
        (другие пропорции, подрезка) покрывает Go — TestReplacePhotoKeepsRects; здесь — жест и cellSide."""
        v0 = self.album([4])
        photos = v0["spreads"][0]["photos"]
        rects = v0["spreads"][0]["rects"]
        self.wait_js("(n) => document.querySelectorAll('.cell[data-drop=cell]').length === n", 4)
        self.stable_rect(f".cell[data-photo='{photos[1]}']")
        new = self.st.ev(FREE_THUMB_JS)
        self.assertTrue(new, "в ленте нет свободного фото")

        # Без Ctrl: подсказка клавиши, фото встаёт рядом (5 фото, как раньше).
        self._carry(new, photos[1], ctrl=False)
        self.wait_js("() => document.querySelector('.ghost-stack .hint')?.textContent === 'Ctrl — заменить фото'")
        rev0 = self.rev()
        self._drop(ctrl=False)
        self.assertEqual(len(self.wait_rev(rev0)["spreads"][0]["photos"]), 5, "без Ctrl фото должно встать рядом")
        rev0 = self.rev()
        self.page.keyboard.press("Control+z")
        v = self.wait_rev(rev0)
        self.assertEqual(v["spreads"][0]["photos"], photos)

        # С Ctrl: подпись «Заменить фото» сразу после нажатия, замена без перестройки.
        self.wait_js("(n) => document.querySelectorAll('.cell[data-drop=cell]').length === n", 4)
        self.stable_rect(f".cell[data-photo='{photos[1]}']")
        self._carry(new, photos[1], ctrl=True)
        self.wait_js("() => document.querySelector('.ghost-stack .hint')?.textContent === 'Заменить фото'")
        rev0 = self.rev()
        self._drop(ctrl=True)
        sp = self.wait_rev(rev0)["spreads"][0]
        self.assertEqual(sp["photos"], [photos[0], new, photos[2], photos[3]], "новое фото должно встать на место второго")
        self.assertEqual(sp["rects"], rects, "раскладка сменилась")
        self.wait_js("(id) => !document.querySelector(`.thumb[data-thumb='${id}']`)?.classList.contains('used')", photos[1])
        self.expect(self.q(".thumb[data-thumb='%s'] .check" % photos[1])).to_have_count(0)
        self.expect(self.q(".toast")).to_contain_text("Прежнее фото вернулось в ленту")

        # Ctrl+Z — один шаг назад.
        rev0 = self.rev()
        self.page.keyboard.press("Control+z")
        v = self.wait_rev(rev0)
        self.assertEqual(v["spreads"][0]["photos"], photos)
        self.assertEqual(v["spreads"][0]["rects"], rects)
