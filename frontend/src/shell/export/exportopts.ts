// Настройки окна экспорта (спека 009 U8, design §6.4): сборка ExportOptions, оценка размера файла,
// последний выбор в localStorage (свойство программы, не проекта — как view.svelte.ts).
import type { ExportOpts, Settings } from '../../base/core/types'

// Тип лежит в основе (store.runExport), здесь — для окна и тестов.
export type { ExportOpts }

export type Quality = 100 | 95 | 80
export const QUALITIES: { v: Quality; label: string }[] = [
  { v: 100, label: 'максимум' },
  { v: 95, label: 'печать' },
  { v: 80, label: 'просмотр' },
]
export const DPI_PRESETS: { v: number; label: string }[] = [
  { v: 300, label: 'печать' },
  { v: 150, label: 'просмотр' },
  { v: 72, label: 'экран' },
]

// Выбор в окне. dpi 0 — «из настроек альбома».
export interface ExportPrefs {
  quality: Quality
  dpi: number
  pages: boolean
}
export type ExportScope = 'all' | 'current' | 'range'

export const DEFAULT_PREFS: ExportPrefs = { quality: 95, dpi: 0, pages: false }

// Байт на пиксель по качеству. ГИПОТЕЗА по типичным JPEG фотокамер: сверить на настоящих фото при QA.
const BYTES_PER_PX: Record<Quality, number> = { 100: 0.9, 95: 0.45, 80: 0.2 }

const px = (mm: number, dpi: number) => Math.round((mm / 25.4) * dpi)

// Примерный размер одного файла: разворот (или страница при pages) по числу пикселей.
export function estimateBytes(s: Pick<Settings, 'pageW' | 'pageH' | 'dpi'>, dpi: number, quality: Quality, pages: boolean): number {
  const d = dpi > 0 ? dpi : s.dpi
  const w = pages ? px(s.pageW, d) : 2 * px(s.pageW, d)
  return w * px(s.pageH, d) * (BYTES_PER_PX[quality] ?? BYTES_PER_PX[95])
}

// «≈ 25 МБ»: до 10 МБ — один знак.
export function formatSize(bytes: number): string {
  const mb = bytes / (1024 * 1024)
  if (mb < 0.1) return '< 0,1 МБ'
  return `≈ ${mb < 10 ? mb.toFixed(1).replace('.', ',') : Math.round(mb)} МБ`
}

// Сборка опций. current — номер текущего разворота с 1. «Все» — пустой диапазон.
export function buildOptions(scope: ExportScope, current: number, rangeText: string, p: ExportPrefs): ExportOpts {
  const range = scope === 'all' ? '' : scope === 'current' ? String(current) : rangeText.trim()
  return { range, quality: p.quality, dpi: p.dpi, pages: p.pages }
}

// Образец имени файла для окна: номер разворота с нулями до ширины Go (internal/export: max(2, цифр в total)).
export function sampleName(n: number, total: number, pages: boolean): string {
  const w = Math.max(2, String(Math.max(total, n, 1)).length)
  const num = String(Math.max(1, n)).padStart(w, '0')
  return pages ? `(${num}-1.jpg, ${num}-2.jpg)` : `(${num}.jpg)`
}

const KEY = 'exportOpts'

export function loadPrefs(): ExportPrefs {
  const o = { ...DEFAULT_PREFS }
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? 'null')
    if (v && typeof v === 'object') {
      if (v.quality === 100 || v.quality === 95 || v.quality === 80) o.quality = v.quality
      if (Number.isInteger(v.dpi) && v.dpi >= 72 && v.dpi <= 1200) o.dpi = v.dpi
      if (typeof v.pages === 'boolean') o.pages = v.pages
    }
  } catch {
    // Без хранилища или с порченым значением — по умолчанию.
  }
  return o
}

export function savePrefs(p: ExportPrefs) {
  try {
    localStorage.setItem(KEY, JSON.stringify(p))
  } catch {
    // Не сохранилось — не страшно.
  }
}
