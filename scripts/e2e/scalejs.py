"""JS-фрагменты для scale.py (089): измерения геометрии страницы. Селекторы — data-guide, роли, data-атрибуты места."""

import math

# S1: горизонтальной прокрутки документа нет.
HSCROLL_JS = """
() => {
  const d = document.documentElement, b = document.body;
  const over = Math.max(0, d.scrollWidth - d.clientWidth, b.scrollWidth - innerWidth);
  return { over, vw: innerWidth, vh: innerHeight, dpr: devicePixelRatio };
}
"""

# S2: зоны на месте — меню, кнопка «Экспорт», ряд плиток ленты, лента альбома; высота холста.
ZONES_JS = """
() => {
  const vw = innerWidth, vh = innerHeight, missing = [];
  const inside = (r) => r.width > 0 && r.height > 0 && r.left >= -1 && r.top >= -1 && r.right <= vw + 1 && r.bottom <= vh + 1;
  const rect = (s) => { const e = document.querySelector(s); return e ? e.getBoundingClientRect() : null; };
  const m = rect('[role=menubar]'); if (!m || !inside(m)) missing.push('меню');
  const ex = [...document.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Экспорт');
  if (!ex || !inside(ex.getBoundingClientRect())) missing.push('кнопка «Экспорт»');
  const tile = [...document.querySelectorAll('[data-guide=library] [data-thumb]')].find((e) => inside(e.getBoundingClientRect()));
  if (!tile) missing.push('ряд плиток ленты фото');
  const tl = rect('[data-guide=timeline]'); if (!tl || !inside(tl)) missing.push('лента альбома');
  const cv = rect('[data-guide=canvas]'); if (!cv || cv.width <= 0) missing.push('холст');
  return { missing, canvasH: cv ? Math.round(cv.height * 10) / 10 : 0 };
}
"""

# S3: верхнее окно внутри окна программы; если выше — последняя кнопка достижима прокруткой окна.
DIALOG_FIT_JS = """
() => {
  const ds = [...document.querySelectorAll('[role=dialog]')], d = ds[ds.length - 1];
  if (!d) return { fits: false, reach: null, rect: null };
  const vw = innerWidth, vh = innerHeight, r = d.getBoundingClientRect();
  const fits = r.left >= -1 && r.top >= -1 && r.right <= vw + 1 && r.bottom <= vh + 1;
  const btns = [...d.querySelectorAll('button')].filter((b) => !b.classList.contains('x') && b.getBoundingClientRect().width > 0);
  let reach = null;
  if (btns.length) {
    const last = btns[btns.length - 1];
    last.scrollIntoView({ block: 'nearest' });
    const b = last.getBoundingClientRect();
    reach = b.top >= -1 && b.bottom <= vh + 1 && b.left >= -1 && b.right <= vw + 1;
  }
  return { fits, reach, rect: [r.left, r.top, r.right, r.bottom].map((x) => Math.round(x)), vw, vh };
}
"""

# S4: меню в правом нижнем углу окна: событие contextmenu с координатами угла (обработчики берут clientX/clientY).
# S4: рамка последнего открытого меню (null — меню нет).
MENU_RECT_JS = """
() => {
  const m = [...document.querySelectorAll('[role=menu]')].pop(); if (!m) return null;
  const b = m.getBoundingClientRect();
  return { top: b.top, bottom: b.bottom, left: b.left, right: b.right, vw: innerWidth, vh: innerHeight };
}
"""

CORNER_MENU_JS = """
async (sel) => {
  const el = document.querySelector(sel); if (!el) return null;
  el.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: innerWidth - 4, clientY: innerHeight - 4 }));
  await new Promise((r) => setTimeout(r, 250));
  const ms = [...document.querySelectorAll('[role=menu]')], m = ms[ms.length - 1];
  const b = m ? m.getBoundingClientRect() : null;
  return { menu: b ? { right: b.right, bottom: b.bottom, left: b.left, top: b.top } : null, vw: innerWidth, vh: innerHeight };
}
"""

# S6: резкость миниатюр ленты: naturalWidth ≥ 0,9 × ширина × DPR (загруженные и видимые).
LIB_SHARP_JS = """
() => {
  const g = document.querySelector('[data-guide=library] .grid'); if (!g) return { total: 0, soft: 0 };
  const gb = g.getBoundingClientRect(); let total = 0, soft = 0;
  for (const im of g.querySelectorAll('img')) {
    const r = im.getBoundingClientRect();
    if (r.bottom <= gb.top || r.top >= gb.bottom || r.width === 0 || !im.complete || im.naturalWidth === 0) continue;
    total++; if (im.naturalWidth < 0.9 * r.width * devicePixelRatio) soft++;
  }
  return { total, soft };
}
"""

# S6: текст блока не выходит за рамку блока (px экрана): рамка — .hit.block, строки — svg text рядом.
TEXT_FIT_JS = """
(sid) => {
  const sp = document.querySelector(`[data-guide=canvas] [data-spread="${sid}"]`); if (!sp) return null;
  const blk = [...sp.querySelectorAll('.blk')].find((b) => b.querySelector('.hit.block')); if (!blk) return null;
  const f = blk.querySelector('.hit.block').getBoundingClientRect();
  let over = 0;
  for (const t of blk.querySelectorAll('svg text')) {
    const r = t.getBoundingClientRect();
    over = Math.max(over, f.left - r.left, r.right - f.right, f.top - r.top, r.bottom - f.bottom);
  }
  return { over };
}
"""

# S7: растр узора и стикера на холсте: px из адреса и сторона в CSS px (узор — ширина img, стикер — длинная сторона без поворота).
RASTER_JS = """
() => {
  const px = (i) => { const m = /[?&]px=([0-9]+)/.exec(i.getAttribute('src') || ''); return m ? Number(m[1]) : null };
  return {
    pattern: [...document.querySelectorAll('[data-guide=canvas] img[data-pattern]')].map((i) => ({ px: px(i), side: i.offsetWidth })),
    sticker: [...document.querySelectorAll('[data-guide=canvas] .stk img')].map((i) => ({ px: px(i), side: Math.max(i.offsetWidth, i.offsetHeight) })),
  }
}
"""

# S7: счётчик change, пришедших сами (до ручного), и ручной change по КОПИИ списка (без копии — бесконечный цикл).
MQ_NATIVE_JS = "() => window.__e2e.mqNative"
MQ_KICK_JS = """
() => {
  const copy = [...window.__e2e.mqls];
  for (const l of copy) l.dispatchEvent(new Event('change'));
  return copy.length;
}
"""


def expected_px(side: float, dpr: float) -> int:
    """Ожидаемый px растра: ceil(сторона × DPR / 256) × 256 (шаг как у patternPx во фронте)."""
    return max(1, math.ceil(side * dpr / 256 - 1e-9)) * 256


def menu_verdict(r) -> str:
    """Итог одной проверки меню: «unopened» (меню нет — проверка ничего не проверила), «overflow» или «ok».
    r — {menu: {top,bottom,left,right}|None, vw, vh} либо None (нет цели)."""
    if not r or not r.get("menu"):
        return "unopened"
    m = r["menu"]
    if m["bottom"] > r["vh"] + 1 or m["right"] > r["vw"] + 1 or m["top"] < -1 or m["left"] < -1:
        return "overflow"
    return "ok"
