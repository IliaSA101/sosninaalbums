// Карточка обложки (спека 032, п. 9–13): чистые решения — группы типографий, строки «Итог» и «Корешок»,
// сборка CoverSpec из поля. Числа считает Go (View.cover); здесь только подписи и разбор ввода.
import { plural } from '../../base/words/plural'
import type { CoverPreset, CoverSpec, CoverView } from '../../base/core/types'
import { mmText } from '../../editor/cover/cover'

export const CUSTOM_ID = 'custom'

export interface PresetGroup {
  group: string
  items: CoverPreset[]
}

// Список «Типография»: группы в порядке появления (Фотосфера, Нетпринт), пустые не нужны.
// Варианты без группы (group '') — в конце: в списке они идут обычными <option>, без пустой подписи.
export function presetGroups(list: readonly CoverPreset[]): PresetGroup[] {
  const out: PresetGroup[] = []
  for (const p of list) {
    const g = out.find((x) => x.group === p.group)
    if (g) g.items.push(p)
    else out.push({ group: p.group, items: [p] })
  }
  return [...out.filter((x) => x.group !== ''), ...out.filter((x) => x.group === '')]
}

// Размеры сторонки (мм) — по зоне «Задняя» из Go; её нет — null.
export function sideSize(cv: Pick<CoverView, 'zones'>): { w: number; h: number } | null {
  const z = cv.zones.find((x) => x.kind === 'back' || x.kind === 'front')
  return z ? { w: z.rect.w, h: z.rect.h } : null
}

// «Итог: 623 × 331 мм — 7358 × 3909 px при 300 dpi».
export function totalLine(cv: Pick<CoverView, 'sheet' | 'pxW' | 'pxH'>, dpi: number): string {
  return `Итог: ${mmText(cv.sheet.w)} × ${mmText(cv.sheet.h)} мм — ${cv.pxW} × ${cv.pxH} px при ${dpi} dpi`
}

// «Корешок 14 мм при 20 разворотах».
export function spineLine(cv: Pick<CoverView, 'spine' | 'n'>): string {
  return `Корешок ${mmText(cv.spine)} мм при ${cv.n} ${plural(cv.n, ['развороте', 'разворотах', 'разворотах'])}`
}

// Таблица пресета кончилась раньше альбома: берётся последняя строка (сценарий 8).
export function overLine(cv: Pick<CoverView, 'over' | 'spec'>): string {
  const last = cv.spec.steps?.at(-1)
  if (!cv.over || !last) return ''
  return `Больше ${last.upto} ${plural(last.upto, ['разворота', 'разворотов', 'разворотов'])} — вне таблицы типографии`
}

// Корешок по таблице типографии (поля a и b не нужны) или по формуле a + b·n.
export const hasTable = (spec: Pick<CoverSpec, 'steps'>): boolean => (spec.steps?.length ?? 0) > 0

// Число из поля ввода: запятая допустима, пусто и мусор — null, отрицательное — null.
export function parseNum(text: string): number | null {
  const t = text.trim().replace(',', '.')
  if (t === '') return null
  const v = Number(t)
  return Number.isFinite(v) && v >= 0 ? v : null
}

export type SpecField = 'w' | 'h' | 'wrap' | 'hinge' | 'a' | 'b'

// Новая спецификация с одним изменённым полем. Правка a или b переводит корешок на формулу (steps
// сбрасываются), остальное Go переключит на «Свои размеры» сам. Равные числа — null (правки нет).
export function specWith(spec: CoverSpec, field: SpecField, value: number): CoverSpec | null {
  if ((spec[field] ?? 0) === value) return null
  const next: CoverSpec = { ...spec, [field]: value }
  if (field === 'a' || field === 'b') next.steps = []
  return next
}

// «Свои размеры» в списке «Типография»: числа остаются теми, что на экране, меняется пометка. Корешок
// из таблицы становится формулой: a = корешок, который Go уже посчитал для текущего числа разворотов
// (View.cover.spine), b = 0 — толщина не скачет, а поля a и b появляются. Формульный корешок не трогаем.
// Пресет custom у Go — сброс к его числам, поэтому выбор идёт через SetCoverSpec, как правка поля.
export function customSpec(cv: Pick<CoverView, 'spec' | 'spine'>): CoverSpec {
  const next: CoverSpec = { ...cv.spec, preset: CUSTOM_ID }
  if (hasTable(cv.spec)) {
    next.steps = []
    next.a = cv.spine
    next.b = 0
  }
  return next
}

// Значение поля для показа: у w и h «0 — по странице альбома» заменено размерами из Go.
export function fieldValue(cv: Pick<CoverView, 'spec' | 'zones'>, field: SpecField): number {
  if (field === 'w' || field === 'h') {
    const side = sideSize(cv)
    if (side) return side[field]
  }
  return cv.spec[field] ?? 0
}

// Кнопка строки «Корешок» (058 В): надписи нет — «Надпись» добавляет; есть (Go прислал её ID) — выделяет.
export function spineAction(cv: Pick<CoverView, 'spineText'> | null | undefined): { label: string; kind: 'add' | 'select' } {
  return cv?.spineText ? { label: 'Выделить надпись', kind: 'select' } : { label: 'Надпись', kind: 'add' }
}
