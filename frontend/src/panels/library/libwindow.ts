// Окно рядов ленты фото (073, U4): в DOM — только видимые ряды ± запас, остальное заменяют распорки.
// Числа повторяют CSS .grid в Library.svelte: padding 12px 16px, gap 16px, ячейка size × size.
export const GAP = 16
export const PAD_X = 16
export const PAD_Y = 12
// Запас рядов сверху и снизу от видимой части.
export const EXTRA = 2

// Высота ленты фото по умолчанию; сплиттер между лентой и лентой альбома (D15, 089).
export const LIB_DEF = 240
export const SPLITTER = 6
// Всё в ленте, кроме сетки: рамка сверху 1 + полоса LibraryBar (отступы 2·8 + кнопка 30 + линия 1).
export const LIB_CHROME = 1 + 2 * 8 + 30 + 1

// Нижний предел высоты ленты (D15): один полный ряд плиток — полоса, поля сетки сверху и снизу, плитка.
// Ниже ряд обрезается; при сжатии окна лента уступает холсту только до этой высоты.
export const rowFloor = (size: number): number => LIB_CHROME + 2 * PAD_Y + size

// Фактический нижний предел: не выше заданной высоты eff — если её выбрали меньше ряда (крупные плитки
// или ручной выбор), лента ниже не опустится, но и выше своей высоты не растёт.
export const libFloor = (size: number, eff: number): number => Math.min(rowFloor(size), eff)

export interface WinIn {
  n: number // фото в ленте
  width: number // clientWidth сетки (с полями)
  height: number // clientHeight сетки (с полями)
  scrollTop: number
  size: number // сторона ячейки (--size)
}

export interface Win {
  cols: number
  rowH: number
  rows: number
  r0: number // первый нарисованный ряд
  r1: number // за последним нарисованным рядом
  padTop: number // высота верхней распорки, 0 — распорки нет
  padBottom: number
}

export const colsOf = (width: number, size: number): number => Math.max(1, Math.floor((width - 2 * PAD_X + GAP) / (size + GAP)))

export function libWindow({ n, width, height, scrollTop, size }: WinIn): Win {
  const cols = colsOf(width, size)
  const rowH = size + GAP
  const rows = Math.ceil(Math.max(0, n) / cols)
  if (rows === 0) return { cols, rowH, rows, r0: 0, r1: 0, padTop: 0, padBottom: 0 }
  // Браузер не даст прокрутить дальше конца; устаревший scrollTop (список только что сократился) подрезаем.
  const total = 2 * PAD_Y + rows * rowH - GAP
  const st = Math.max(0, Math.min(scrollTop, total - height))
  const r0 = Math.min(rows - 1, Math.max(0, Math.floor((st - PAD_Y) / rowH) - EXTRA))
  const r1 = Math.min(rows, Math.max(r0 + 1, Math.ceil((st + height - PAD_Y) / rowH) + EXTRA))
  // Распорка + gap = высота пропущенных рядов, поэтому из неё вычитаем один GAP.
  return {
    cols,
    rowH,
    rows,
    r0,
    r1,
    padTop: r0 > 0 ? r0 * rowH - GAP : 0,
    padBottom: r1 < rows ? (rows - r1) * rowH - GAP : 0,
  }
}

// Куда прокрутить сетку, чтобы фото с номером index (точка с долей fy его высоты) осталось под курсором:
// cy — положение курсора от верха сетки. Якорь — индекс, а не элемент: при зуме элемент мог уйти из окна.
export function anchorTop(index: number, cols: number, size: number, cy: number, fy: number): number {
  const top = PAD_Y + Math.floor(index / Math.max(1, cols)) * (size + GAP)
  return Math.max(0, top - (cy - fy * size))
}

// Куда прокрутить ленту, чтобы ряд фото с номером index был виден целиком (128): у ближнего края, с полем PAD_Y.
// null — ряд уже виден, прокручивать не надо.
export function revealTop(index: number, scrollTop: number, height: number, width: number, size: number): number | null {
  const top = PAD_Y + Math.floor(index / colsOf(width, size)) * (size + GAP)
  const bottom = top + size
  if (top >= scrollTop && bottom <= scrollTop + height) return null
  return top < scrollTop ? Math.max(0, top - PAD_Y) : Math.max(0, bottom + PAD_Y - height)
}

// Новый список ленты (088): куда встало верхнее видимое фото прежнего. Пропало — ближайшее следующее
// из уцелевших; таких нет — прежний номер, зажатый в список; список пуст — 0.
export function anchorAfter(prev: string[], next: string[], topIndex: number): number {
  if (next.length === 0) return 0
  const pos = new Map(next.map((id, i) => [id, i]))
  for (let i = Math.max(0, topIndex); i < prev.length; i++) {
    const j = pos.get(prev[i])
    if (j !== undefined) return j
  }
  return Math.min(Math.max(0, topIndex), next.length - 1)
}

// Прокрутка после смены списка: верхний видимый ряд остаётся на месте экрана (образец — Ctrl+колесо, 053).
// null — прокручивать не надо (лента у самого верха или список прежний).
export function keepScroll(prev: string[], next: string[], scrollTop: number, width: number, size: number): number | null {
  if (scrollTop <= 0 || width <= 0 || prev.length === 0) return null
  if (prev.length === next.length && prev.every((id, i) => id === next[i])) return null
  const at = photoAt(prev.length, width, size, PAD_X, scrollTop)
  if (!at) return null
  return anchorTop(anchorAfter(prev, next, at.index), colsOf(width, size), size, 0, at.fy)
}

// Фото под точкой (x, y — от левого верха сетки с учётом прокрутки: y = clientY − верх + scrollTop):
// индекс и доля высоты ячейки. В щели и за краем — ближайшая ячейка; пусто — null.
export function photoAt(n: number, width: number, size: number, x: number, y: number): { index: number; fy: number } | null {
  if (n <= 0) return null
  const cols = colsOf(width, size)
  const rows = Math.ceil(n / cols)
  const rowH = size + GAP
  const row = Math.max(0, Math.min(rows - 1, Math.floor((y - PAD_Y) / rowH)))
  const col = Math.max(0, Math.min(cols - 1, Math.floor((x - PAD_X) / rowH)))
  const index = Math.min(n - 1, row * cols + col)
  const fy = Math.max(0, Math.min(1, (y - PAD_Y - row * rowH) / size))
  return { index, fy }
}
