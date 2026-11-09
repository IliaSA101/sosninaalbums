"""Дымовой тест каркаса e2e-регресса (166 S1): стенд поднят, Go отвечает, лента показывает загруженные миниатюры.

Это не регресс конкретного бага, а проверка самого каркаса: пока он красный, остальные R-тесты не показательны.
"""

from base import RegressCase, bug

LIBRARY_GRID = "[data-guide=library] .grid"


@bug(id="S0", fix="-", src="каркас 166 S1", red="остановить стенд до теста")
class SmokeOpen(RegressCase):
    def test_strip_shows_loaded_photos(self):
        st = self.call("GetState")
        self.assertIn("view", st)
        self.assertFalse(st["photos"])
        view = self.album([2, 2])
        self.assertEqual(len(view["spreads"]), 2)
        self.assertEqual(len(self.call("GetState")["photos"]), 12)  # вся папка data/photos
        # миниатюры готовятся в фоне: ждём naturalWidth > 0 у всех видимых <img> ленты, не фиксированное время
        self.wait_js(
            "(sel) => { const im = [...document.querySelectorAll(sel + ' img')];"
            " return im.length >= 4 && im.every((i) => i.complete && i.naturalWidth > 0) }",
            LIBRARY_GRID,
            timeout=30,
        )
