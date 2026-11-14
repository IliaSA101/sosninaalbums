// Зум холста (спека 008, design §6.1). Состояние хранится в мм разворота: ресайз окна и
// смена вида его не трогают, только пересчитывают пиксели. Чистые функции, без Svelte.
export const MIN_ZOOM = 1
export const MAX_ZOOM = 8
export const HOLD_MS = 400 // Пробел дольше этого — не «короткое нажатие»

export interface ZoomState {
  z: number // масштаб относительно «вписать» (1 = 100 %)
  cx: number // мм разворота в центре окна
  cy: number
}

// Окно: px области Stage под полосой раскладок; k0 — px на мм при z = 1; sw, ph — разворот в мм.
// pad — px у левого и правого края под боковыми панелями (inspector/panels.ts), по умолчанию 0.
// padB — px снизу окна, которые «вписанный» лист оставляет свободными сверх FIT_TOP (063; с 095 у Stage — 0, механизм оставлен).
export interface Viewport {
  w: number
  h: number
  k0: number
  sw: number
  ph: number
  pad?: number
  padB?: number
}

// «Вписать» (владелец 2026-10-21, owner-ux §1.9): 16 px сверху и снизу — полосы под листом нет, мини-панель
// у фото у самого низа рисуется поверх ленты разворотов (MiniPanel). Было (2026-10-17): снизу MINI_GAP + MINI_H + 2.
export const FIT_TOP = 16
export const FIT_BOTTOM = FIT_TOP
// Высота под лист при «вписать» в окне высотой h.
export const fitH = (h: number): number => h - FIT_TOP - FIT_BOTTOM
// Высота окна для центровки: лист «по центру» стоит в части окна над padB.
export const effH = (v: Viewport): number => v.h - Math.min(v.padB ?? 0, v.h / 2) // в низком окне (галерея) — не больше половины

const fin = (n: number, d: number) => (Number.isFinite(n) ? n : d)
const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v))

// Вписанный разворот: масштаб 1, центр — центр разворота.
export const fit = (sw: number, ph: number): ZoomState => ({ z: MIN_ZOOM, cx: sw / 2, cy: ph / 2 })

// Центр вдоль одной оси (мм). Свободная зона — окно без pad у краёв. Разворот уже неё — лежит
// внутри, шире — закрывает её целиком (край выводится из-под панели). Уход от центра не больше
// (z − 1)/z половины разворота: при z = 1 разворот по центру, а от 100 % точка под курсором
// стоит и по горизонтали. На границе «уже/шире» диапазон стягивается в центр — без скачка.
function clampAxis(c: number, size: number, win: number, k: number, pad: number, z: number): number {
  const mid = size / 2
  const p = clamp(fin(pad, 0), 0, win / 2) / k
  const a = win / (2 * k) - p // левая граница центра у широкого, правая — у узкого
  const b = size - a
  const off = ((z - 1) / z) * mid
  return clamp(c, Math.max(Math.min(a, b), mid - off), Math.min(Math.max(a, b), mid + off))
}

// Масштаб в пределах; разворот не уходит из окна, у широкого пустых полей у краёв нет.
export function clampZoom(s: ZoomState, v: Viewport): ZoomState {
  const z = clamp(fin(s.z, MIN_ZOOM), MIN_ZOOM, MAX_ZOOM)
  if (!(v.k0 > 0) || !(v.w > 0) || !(v.h > 0)) return { z, cx: fin(s.cx, v.sw / 2), cy: fin(s.cy, v.ph / 2) }
  const k = v.k0 * z
  return {
    z,
    cx: clampAxis(fin(s.cx, v.sw / 2), v.sw, v.w, k, v.pad ?? 0, z),
    cy: clampAxis(fin(s.cy, v.ph / 2), v.ph, effH(v), k, 0, z),
  }
}

// Шаг клавиш Ctrl+= / Ctrl+− (меню «Вид»): к центру окна.
export const ZOOM_STEP = 1.25

// Масштаб к курсору: мм под (x, y) остаётся под ним. x, y — px от угла окна.
export function zoomAt(s: ZoomState, v: Viewport, factor: number, x: number, y: number): ZoomState {
  const cur = clampZoom(s, v)
  if (!(v.k0 > 0) || !Number.isFinite(factor) || !(factor > 0) || !Number.isFinite(x) || !Number.isFinite(y)) return cur
  const k = v.k0 * cur.z
  const dx = x - v.w / 2
  const dy = y - effH(v) / 2
  const mx = cur.cx + dx / k
  const my = cur.cy + dy / k
  const z = clamp(cur.z * factor, MIN_ZOOM, MAX_ZOOM)
  const k2 = v.k0 * z
  return clampZoom({ z, cx: mx - dx / k2, cy: my - dy / k2 }, v)
}

// Панорама на (dx, dy) px: содержимое едет за курсором, значит центр уходит в обратную сторону.
export function panBy(s: ZoomState, v: Viewport, dx: number, dy: number): ZoomState {
  const cur = clampZoom(s, v)
  if (!(v.k0 > 0) || !Number.isFinite(dx) || !Number.isFinite(dy)) return cur
  const k = v.k0 * cur.z
  return clampZoom({ z: cur.z, cx: cur.cx - dx / k, cy: cur.cy - dy / k }, v)
}

// Ctrl+колесо мыши и щипок тачпада (Chromium шлёт его как wheel с ctrlKey): один множитель на оба.
// Строки (deltaMode 1) — по 40 px, страницы (2) — по 800; шаг ≤ 100. Щелчок мыши (100) ≈ ×1,5, щипок (1–10) — плавно.
export function wheelFactor(deltaY: number, deltaMode: number): number {
  const unit = deltaMode === 1 ? 40 : deltaMode === 2 ? 800 : 1
  const d = clamp(fin(deltaY, 0) * unit, -100, 100)
  return Math.exp(-d * 0.004)
}

// Где стоит разворот в окне, целые px (чтобы фото не «дрожали» на дробных границах).
export function placement(s: ZoomState, v: Viewport): { left: number; top: number; width: number; height: number } {
  const k = v.k0 * s.z
  return {
    left: Math.round(v.w / 2 - s.cx * k),
    top: Math.round(effH(v) / 2 - s.cy * k),
    width: Math.round(v.sw * k),
    height: Math.round(v.ph * k),
  }
}

// Короткий Пробел без панорамы — листание раскладки; удержание или панорама — нет.
export const spaceTap = (heldMs: number, panned: boolean): boolean => !panned && heldMs >= 0 && heldMs < HOLD_MS

export const percent = (z: number): number => Math.round(z * 100)

// Бегунок масштаба (спека 016): позиция 0..1 на логарифмической шкале 100–800 %, равные доли — равные кратности.
// Формула верна, пока MIN_ZOOM = 1 (log2 1 = 0); при меньшем минимуме — (log z − log min) / (log max − log min).
const SPAN = Math.log(MAX_ZOOM / MIN_ZOOM)
export const zoomPos = (z: number): number => clamp(Math.log(clamp(fin(z, MIN_ZOOM), MIN_ZOOM, MAX_ZOOM) / MIN_ZOOM) / SPAN, 0, 1)
// Края отдаём точно: exp(log 8) не обязан быть ровно 8.
export function posZoom(t: number): number {
  const u = clamp(fin(t, 0), 0, 1)
  return u === 0 ? MIN_ZOOM : u === 1 ? MAX_ZOOM : MIN_ZOOM * Math.exp(u * SPAN)
}

// Колесо над холстом (решение владельца 2026-10-08, как в SmartAlbums): с Ctrl/Cmd (и щипок тачпада) — зум к курсору;
// без них при 100 % — ряд разворотов (spreadrow), в зуме — сдвиг листа (wheelPan). Эвристики мышь/тачпад здесь нет.
export interface WheelLike {
  deltaX: number
  deltaY: number
  deltaMode: number
  ctrlKey: boolean
  metaKey?: boolean
}

export const isZoomWheel = (e: WheelLike): boolean => e.ctrlKey || !!e.metaKey

// Колесо без Ctrl при 100 % листает ряд, если нет жеста правки. В режиме пипетки не листает (109-qa-2): смена разворота
// сняла бы режим; зум с Ctrl и сдвиг увеличенного листа от этого не зависят.
export const rowWheelAllowed = (s: { dragging: boolean; busy: boolean; pressed: boolean; eyedrop: boolean }): boolean =>
  !s.eyedrop && (s.dragging || !(s.busy || s.pressed))

// Сдвиг листа в зуме, px: колесо — по Y, Shift+колесо (Chromium шлёт deltaX) — по X, тачпад — по обеим; строки — по 40 px; содержимое едет против прокрутки.
export function wheelPan(e: WheelLike): { dx: number; dy: number } {
  const unit = e.deltaMode === 1 ? 40 : e.deltaMode === 2 ? 800 : 1
  return { dx: (0 - fin(e.deltaX, 0)) * unit, dy: (0 - fin(e.deltaY, 0)) * unit } // 0 - x: без «−0»
}

// Зум колесом и щипком (134): один предикат запретов. Нет разворота, жест правки, слот «новый» (под курсором листа нет)
// и уровень без функции «Масштаб» — зум не начинаем.
export const zoomGate = (s: { spread: boolean; busy: boolean; atNew: boolean; can: boolean }): boolean =>
  s.spread && !s.busy && !s.atNew && s.can

// Подсказка масштаба в бейдже; fit — подпись клавиши «вписать» из реестра (пусто — клавиша снята).
export const zoomTip = (mac: boolean, fit: string): string =>
  `${mac ? '⌘' : 'Ctrl'}+колесо или щипок — масштаб${fit ? `, ${fit} — вписать` : ''}`
