import type { Settings } from '../../base/core/types'

// Пресеты размера страницы, мм. dpi/layflat заданы только там, где их требует типография.
export interface Preset {
  label: string
  w: number
  h: number
  dpi?: number
  layflat?: boolean
  hint?: string
}

export const FSFERA_HINT = 'Как требует fsfera.ru: 6638×3354 px, sRGB, файлы 01.jpg…; важное — не ближе 8 мм к краю'

export const presets: Preset[] = [
  { label: 'Фотосфера 30×30', w: 281, h: 284, dpi: 300, layflat: true, hint: FSFERA_HINT },
  { label: '30×30 (квадрат)', w: 300, h: 300 },
  { label: '25×25', w: 250, h: 250 },
  { label: '20×20', w: 200, h: 200 },
  { label: '30×20 (альбомный)', w: 300, h: 200 },
  { label: '20×30 (книжный)', w: 200, h: 300 },
  { label: '40×30', w: 400, h: 300 },
]

// Та же формула, что px() в internal/export.
export const exportPx = (mm: number, dpi: number) => Math.round((mm / 25.4) * dpi)

type Form = Pick<Settings, 'pageW' | 'pageH' | 'dpi' | 'layflat'>

// Пресет подсвечен, когда совпадают все заданные в нём поля.
export function presetActive(p: Preset, s: Form): boolean {
  if (+s.pageW !== p.w || +s.pageH !== p.h) return false
  if (p.dpi !== undefined && +s.dpi !== p.dpi) return false
  if (p.layflat !== undefined && !!s.layflat !== p.layflat) return false
  return true
}

// Что пресет меняет в форме; поля и промежуток не трогает.
export function presetFields(p: Preset): Partial<Settings> {
  const f: Partial<Settings> = { pageW: p.w, pageH: p.h }
  if (p.dpi !== undefined) f.dpi = p.dpi
  if (p.layflat !== undefined) f.layflat = p.layflat
  return f
}
