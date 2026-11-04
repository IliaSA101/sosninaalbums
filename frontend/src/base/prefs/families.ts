// Семейства раскладок (041): порядок сегментов фильтра. Подписи — panels/gallery/variants.ts FAMILIES (основа не знает панелей);
// тест variants.test.ts следит, чтобы списки не разошлись.
import type { Family } from '../core/types'

export const FAMILY_IDS: readonly Family[] = ['dense', 'hero', 'air', 'bleed', 'strip', 'figure']

// Набор семейств из хранилища или события: только известные имена, без повторов, в порядке сегментов.
// Мусор — «Все» (`[]`); набор из всех шести тоже «Все» (065).
export function normFamilies(v: unknown): Family[] {
  if (!Array.isArray(v)) return []
  const out = FAMILY_IDS.filter((id) => v.includes(id))
  return out.length === FAMILY_IDS.length ? [] : out
}
