// Разметка типографии (спека 032 для обложки, 066 для разворотов): чистая логика подложки.
// Что в файле и ok/warn решает Go (GuideView); здесь только перевод в проценты листа и адрес картинки.
import { coverGuideUrl, spreadGuideUrl } from '../../base/core/media'
import type { GuideView } from '../../base/core/types'

// cover — обложка; spreads — разворот и лист виньетки (один файл на весь альбом).
export type MarkupKind = 'cover' | 'spreads'

export const OPACITY_DEF = 0.5
export const OPACITY_MIN = 0.05
// Go отдаёт не больше 200 линий; лишнее не рисуем, что бы ни пришло.
export const MAX_LINES = 200

export const markupUrl = (kind: MarkupKind, rev: number): string => (kind === 'cover' ? coverGuideUrl(rev) : spreadGuideUrl(rev))

// Картинку запрашиваем только когда Go её отдаст: ok и есть сведённое изображение (PSD без него — одни линии).
export const hasImage = (g: GuideView | null | undefined): boolean => !!g && g.ok && g.image

// «Убрать» нужна, пока разметка выбрана — и когда файл пропал с диска (ok=false): убрать запись всё равно можно.
export const canClear = (g: GuideView | null | undefined): boolean => !!g

export function clampOpacity(v: number): number {
  return Number.isFinite(v) ? Math.min(1, Math.max(OPACITY_MIN, v)) : OPACITY_DEF
}

// Адрес картинки растёт строго: даже при том же номере правки (тот же файл выбрали заново) кэш сбивается.
export const nextRev = (prev: number, viewRev: number): number => Math.max(Math.trunc(viewRev) || 0, prev + 1)

export interface LinePos {
  v: boolean // вертикальная: left, иначе top
  pct: number // доля листа, %
}

// Линии PSD, мм листа → проценты; w, h — размер листа в мм (разворот, лист, обложка). Файла нет — линий нет.
export function linePositions(g: GuideView | null | undefined, w: number, h: number): LinePos[] {
  if (!g || !g.ok || !g.lines?.length || !(w > 0) || !(h > 0)) return []
  const out: LinePos[] = []
  for (const l of g.lines.slice(0, MAX_LINES)) {
    const size = l.v ? w : h
    if (!Number.isFinite(l.at) || l.at < 0 || l.at > size) continue
    out.push({ v: l.v, pct: (l.at / size) * 100 })
  }
  return out
}
