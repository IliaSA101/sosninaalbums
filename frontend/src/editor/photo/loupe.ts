// Лупа окна крупного просмотра (128): чистые функции без Svelte. Размеры — px превью (nat) и px окна (box);
// геометрию кадра рисует окно по place(), значения только из этих функций (холст считает в мм, здесь свой масштаб).

export interface Size {
  w: number
  h: number
}
export interface Pt {
  x: number
  y: number
}
// s — px экрана на px картинки, 0 — вписано; cx, cy — точка кадра в центре окна, доли (0..1).
export interface Loupe {
  s: number
  cx: number
  cy: number
}

// Предел зума ступени 1: 1:1 по превью (выше — оригинал, ступень 2). Вписано тоже не больше 1:1: мелкая картинка не растягивается.
export const MAX_SCALE = 1

export const FIT: Loupe = { s: 0, cx: 0.5, cy: 0.5 }
// Сдвиг меньше этого — щелчок, не перетаскивание (как THRESHOLD переноса в drag.svelte.ts).
export const CLICK_PX = 5
// Пропорции «те же» при расхождении до 1 %: серия кадров одной камеры.
const SAME_RATIO = 0.01
const EPS = 1e-6

export const isClick = (dx: number, dy: number): boolean => Math.hypot(dx, dy) < CLICK_PX

const known = (z: Size): boolean => z.w > 0 && z.h > 0

// Масштаб «вписано»: вся картинка видна, но не крупнее 1:1; размеры неизвестны — 0.
export function fitScale(nat: Size, box: Size): number {
  if (!known(nat) || !known(box)) return 0
  return Math.min(box.w / nat.w, box.h / nat.h, MAX_SCALE)
}

const clamp = (v: number, lo: number, hi: number): number => Math.min(hi, Math.max(lo, v))

// Позиция одной оси: кадр меньше окна стоит по центру, больше — центр зажат, край кадра не отходит от края окна.
function axis(len: number, room: number, c: number): number {
  if (len <= room) return (room - len) / 2
  return clamp(room / 2 - c * len, room - len, 0)
}

// Где стоит кадр в окне и каких он размеров, px.
export function place(l: Loupe, nat: Size, box: Size): { left: number; top: number; w: number; h: number } {
  const fit = fitScale(nat, box)
  if (fit <= 0) return { left: 0, top: 0, w: 0, h: 0 }
  const s = l.s > fit + EPS ? l.s : fit
  const w = nat.w * s
  const h = nat.h * s
  return { left: axis(w, box.w, l.cx), top: axis(h, box.h, l.cy), w, h }
}

// Состояние с зажатым центром: хранится то, что видно, поэтому следующий жест начинается от настоящего места.
function settle(l: Loupe, nat: Size, box: Size): Loupe {
  const fit = fitScale(nat, box)
  if (fit <= 0 || l.s <= fit + EPS) return FIT
  const p = place(l, nat, box)
  return { s: l.s, cx: (box.w / 2 - p.left) / p.w, cy: (box.h / 2 - p.top) / p.h }
}

// Масштаб s с точкой под курсором на месте; s ограничен [вписано, max], у вписанного — FIT.
function scaleTo(l: Loupe, s: number, nat: Size, box: Size, at: Pt | null, max: number): Loupe {
  const fit = fitScale(nat, box)
  if (fit <= 0) return l
  const cur = place(l, nat, box)
  const next = clamp(s, fit, Math.max(fit, max))
  if (next <= fit + EPS) return FIT
  const pt = at ?? { x: box.w / 2, y: box.h / 2 }
  // Точка кадра под курсором (доли), затем центр, при котором она остаётся под ним.
  const fx = (pt.x - cur.left) / cur.w
  const fy = (pt.y - cur.top) / cur.h
  return settle({ s: next, cx: fx + (box.w / 2 - pt.x) / (nat.w * next), cy: fy + (box.h / 2 - pt.y) / (nat.h * next) }, nat, box)
}

// Вписано ↔ max в точке. Кадр, который и так не меньше max (фото меньше окна), не меняется.
export function toggle(l: Loupe, nat: Size, box: Size, at: Pt | null, max: number): Loupe {
  const fit = fitScale(nat, box)
  if (fit <= 0) return l
  if (l.s > fit + EPS) return FIT
  if (fit >= max) return l
  return scaleTo(l, max, nat, box, at, max)
}

// Зум в k раз у точки (колесо, щипок, Mod+= / Mod+−); предел — [вписано, max].
export function zoomAt(l: Loupe, k: number, nat: Size, box: Size, at: Pt | null, max: number): Loupe {
  if (!Number.isFinite(k) || k <= 0) return l
  const cur = place(l, nat, box)
  if (cur.w <= 0) return l
  return scaleTo(l, (cur.w / nat.w) * k, nat, box, at, max)
}

// Перетаскивание увеличенного кадра: кадр едет за курсором, значит центр уходит в обратную сторону.
export function pan(start: Loupe, dx: number, dy: number, nat: Size, box: Size): Loupe {
  if (start.s <= 0 || !known(nat)) return start
  return settle({ s: start.s, cx: start.cx - dx / (nat.w * start.s), cy: start.cy - dy / (nat.h * start.s) }, nat, box)
}

// Листание: пропорции те же (±1 %) — тот же масштаб и точка («мигание» серии); другие или неизвестные — вписано.
export function carry(l: Loupe, prev: Size, next: Size): Loupe {
  if (l.s <= 0) return l
  if (!known(prev) || !known(next)) return FIT
  return Math.abs(prev.w / prev.h / (next.w / next.h) - 1) <= SAME_RATIO ? l : FIT
}

// Значок в подписи окна: «Вписано · 48 %», «1:1», «130 %». Размеры неизвестны (картинка не загружена) — пусто.
export function zoomLabel(l: Loupe, nat: Size, box: Size): string {
  const fit = fitScale(nat, box)
  if (fit <= 0) return ''
  const s = place(l, nat, box).w / nat.w
  if (s <= fit + EPS) return fit >= MAX_SCALE - EPS ? '1:1' : `Вписано · ${Math.round(fit * 100)} %`
  return Math.abs(s - 1) < EPS ? '1:1' : `${Math.round(s * 100)} %`
}

// Строка клавиш под подписью окна (Т5): метки клавиш заполняет keymap.text, чтобы показывать назначенные клавиши.
// С холста листания и отметок нет; уровень без отметок (marks) — тоже без них. mod — «Ctrl» или «⌘» для колеса.
export function hintLine(fromLibrary: boolean, mod: string, marks: boolean): string {
  const lead = fromLibrary ? '← → — листать · ' : ''
  const tail = fromLibrary && marks ? ' · отметки — клавишами ленты' : ''
  return `${lead}щелчок{key:photoPreview| или %} — 1:1 · ${mod}+колесо — зум{key:fitZoom| · % — вписать}${tail} · Esc — закрыть`
}
