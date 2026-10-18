import type { PagePreset, Settings } from '../../base/core/types'

// Пресеты размера страницы приходят из Go (App.PagePresets, 047): здесь только помощники.
// dpi/layflat/safe/bleed заданы лишь там, где их требует типография; 0/false/нет поля — пресет их не трогает.
export type Preset = Pick<PagePreset, 'label' | 'w' | 'h' | 'dpi' | 'layflat' | 'safe' | 'bleed' | 'hint'>

// Та же формула, что px() в internal/export.
export const exportPx = (mm: number, dpi: number) => Math.round((mm / 25.4) * dpi)

type Form = Pick<Settings, 'pageW' | 'pageH' | 'dpi' | 'layflat' | 'safe' | 'bleed'>

// Пресет подсвечен, когда совпадают все заданные в нём поля.
export function presetActive(p: Preset, s: Form): boolean {
  if (+s.pageW !== p.w || +s.pageH !== p.h) return false
  if (p.dpi && +s.dpi !== p.dpi) return false
  if (p.layflat && !s.layflat) return false
  if (p.safe && +s.safe !== p.safe) return false
  if (p.bleed && (Number(s.bleed) || 0) !== p.bleed) return false
  return true
}

// Что пресет меняет в форме; поля и промежуток не трогает.
export function presetFields(p: Preset): Partial<Settings> {
  const f: Partial<Settings> = { pageW: p.w, pageH: p.h }
  if (p.dpi) f.dpi = p.dpi
  if (p.layflat) f.layflat = true
  if (p.safe) f.safe = p.safe
  if (p.bleed) f.bleed = p.bleed
  return f
}

// Подпись типографии в списке: группа "" — «Общие».
export const groupLabel = (group: string): string => group || 'Общие'

// Типография для списка: у подсвеченного пресета — его группа; нет подсвеченного — последняя выбранная
// (настройки окна), если она есть в списке; иначе «Общие» (""). Форму не меняет.
export function presetGroupOf(list: readonly PagePreset[], form: Form, saved = ''): string {
  const act = list.find((p) => presetActive(p, form))
  if (act) return act.group ?? ''
  return saved && list.some((p) => (p.group ?? '') === saved) ? saved : ''
}

// Ряды кнопок: общий (group "") первым, дальше по группам в порядке появления («Фотосфера:»).
export interface PresetRow {
  group: string
  items: PagePreset[]
}
export function presetRows(list: readonly PagePreset[]): PresetRow[] {
  const rows: PresetRow[] = []
  for (const p of list) {
    const g = p.group ?? ''
    const row = rows.find((r) => r.group === g)
    if (row) row.items.push(p)
    else rows.push({ group: g, items: [p] })
  }
  return rows.sort((a, b) => Number(b.group === '') - Number(a.group === ''))
}
