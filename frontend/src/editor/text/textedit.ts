// Чистая часть правки текста на развороте (спека 006, п. 9–12): сессия ввода,
// перенос и ширина блока с прилипанием, кегль. Всё в мм разворота.
import { clampBox, snapBox, snapEdge, type SnapLines } from '../../base/geom/snap'
import type { Rect } from '../../base/core/types'

// Сессия ввода (design §3): ≤ 32 знаков [A-Za-z0-9-]; в Go под ней сливаются шаги undo.
export function sessionId(now: number, n: number): string {
  return `${now.toString(36)}-${n.toString(36)}`.slice(0, 32)
}

// Пустой (только пробелы) текст Go считает «удалить блок».
export const isBlank = (s: string) => s.trim() === ''

// Границы кегля — как в Go (зажим 6..200 pt).
export const MIN_SIZE = 6
export const MAX_SIZE = 200
export const clampSize = (n: number, fallback = 14) =>
  Number.isFinite(n) ? Math.min(MAX_SIZE, Math.max(MIN_SIZE, Math.round(n))) : fallback

// Что открывать в редакторе: шаблон с полями ({Фамилия}…), если он есть, а не подставленный текст (030).
export const editText = (t: { text: string; src?: string }): string => t.src ?? t.text

// Текст из редактора: переводы строк к \n, без нулевых знаков.
export const cleanInput = (s: string) => s.replace(/\r\n?/g, '\n').replaceAll('\0', '')

export interface MoveResult {
  x: number
  y: number
  guideX: number | null
  guideY: number | null
}

// Перенос блока: сдвиг start на (dx, dy), не выходя за разворот, затем прилипание.
export function moveBox(start: Rect, dx: number, dy: number, lines: SnapLines, thr: number, spreadW: number, pageH: number): MoveResult {
  const c = clampBox({ x: start.x + dx, y: start.y + dy, w: start.w, h: start.h }, spreadW, pageH)
  const s = snapBox(c, lines, thr)
  const k = clampBox({ x: s.x, y: s.y, w: start.w, h: start.h }, spreadW, pageH)
  return { x: k.x, y: k.y, guideX: s.guideX, guideY: s.guideY }
}

export const MIN_BOX_W = 10 // как в Go (W 10..ширина листа)

export interface ResizeResult {
  x: number
  w: number
  guide: number | null
}

// Ширина блока боковой ручкой: двигается один край, противоположный стоит.
export function resizeBox(start: Rect, side: 'l' | 'r', dx: number, lines: SnapLines, thr: number, spreadW: number): ResizeResult {
  if (side === 'r') {
    const edge = Math.min(spreadW, Math.max(start.x + MIN_BOX_W, start.x + start.w + dx))
    const s = snapEdge(edge, lines, thr)
    const e = Math.min(spreadW, Math.max(start.x + MIN_BOX_W, s.x))
    return { x: start.x, w: e - start.x, guide: s.guide }
  }
  const right = start.x + start.w
  const edge = Math.min(right - MIN_BOX_W, Math.max(0, start.x + dx))
  const s = snapEdge(edge, lines, thr)
  const e = Math.min(right - MIN_BOX_W, Math.max(0, s.x))
  return { x: e, w: right - e, guide: s.guide }
}
