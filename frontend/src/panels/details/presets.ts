import type { PagePreset, Settings } from '../../base/core/types'

// Пресеты размера страницы приходят из Go (App.PagePresets, 047): здесь только помощники.
// dpi/layflat/safe/bleed заданы лишь там, где их требует типография; 0/false/нет поля — пресет их не трогает.
export type Preset = Pick<PagePreset, 'label' | 'w' | 'h' | 'dpi' | 'layflat' | 'safe' | 'bleed' | 'hint'>

// Та же формула, что px() в internal/export.
export const exportPx = (mm: number, dpi: number) => Math.round((mm / 25.4) * dpi)

// Файл для типографии всегда 300 dpi (design §1.3): px вводятся и показываются при нём, а не при dpi альбома.
export const FILE_DPI = 300

// Целые px из поля: пусто, мусор, дробь, ноль и минус — null (поле вернёт прежнее значение, как в карточке обложки).
export function parsePx(text: string): number | null {
  const t = text.trim()
  if (!/^\d+$/.test(t)) return null
  const v = Number(t)
  return Number.isSafeInteger(v) && v > 0 ? v : null
}

type Form = Pick<Settings, 'pageW' | 'pageH' | 'dpi' | 'layflat' | 'safe' | 'bleed'>

// Допуск формата, мм (123, design §1.2): страница на ≤ 0,25 мм от кнопки — этот формат (303,95 × 202,95 из px — Фабрика 30×20).
// Равен model.PageTol в Go; паритет — testdata/parity/pagetol.json. Ближайшие чужие форматы отстоят на 1 мм.
export const PAGE_TOL = 0.25
export const sameMm = (a: number, b: number): boolean => Math.abs(a - b) <= PAGE_TOL

// Пресет подсвечен, когда совпадают все заданные в нём поля (размер — с допуском PAGE_TOL).
export function presetActive(p: Preset, s: Form): boolean {
  if (!sameMm(+s.pageW, p.w) || !sameMm(+s.pageH, p.h)) return false
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

// Типография для списка: у подсвеченного пресета — его группа; подсвечены несколько (20×20 у Нетпринта и Фабрики —
// одни числа, 081 №12) — последняя выбранная, если она среди них; нет подсвеченного — последняя выбранная
// (настройки окна), если она есть в списке; иначе «Общие» (""). Форму не меняет.
export function presetGroupOf(list: readonly PagePreset[], form: Form, saved = ''): string {
  const act = list.filter((p) => presetActive(p, form))
  if (act.length) return (act.find((p) => (p.group ?? '') === saved) ?? act[0]).group ?? ''
  return saved && list.some((p) => (p.group ?? '') === saved) ? saved : ''
}

// Подсказка подсвеченного пресета выбранной типографии: числа у Нетпринта и Фабрики совпадают (081 №12),
// подсказка чужой группы вводила бы в заблуждение.
export function presetHint(list: readonly PagePreset[], form: Form, group: string): string | undefined {
  return list.find((p) => p.hint && (p.group ?? '') === group && presetActive(p, form))?.hint
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
