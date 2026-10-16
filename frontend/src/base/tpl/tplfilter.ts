// Фильтр видов шаблонов виньетки (049, решение №123): чистая логика под тесты. Вид размечает Go (VigChoice.family),
// подписи — здесь. Сегменты рисует общий Seg (050); свои шаблоны вида не имеют и под фильтр не попадают.

export type VigFamily = 'side' | 'big' | 'inrow' | 'group' | 'round' | 'staff'
export type VigFamilyFilter = VigFamily | 'all'

// Виды в порядке сегментов: «Все · С края · Крупно · В ряду · С групповым · Круги · Предметники».
export const VIG_FAMILIES: readonly { id: VigFamily; label: string }[] = [
  { id: 'side', label: 'С края' },
  { id: 'big', label: 'Крупно' },
  { id: 'inrow', label: 'В ряду' },
  { id: 'group', label: 'С групповым' },
  { id: 'round', label: 'Круги' },
  { id: 'staff', label: 'Предметники' },
]

export interface FamilyItem {
  id: string
  own?: boolean
  family?: string
}

// Виды, что есть среди каталожных шаблонов: пустые сегменты не показываются. Один вид — выбирать не из чего.
export function familiesIn(list: readonly FamilyItem[]): readonly { id: VigFamily; label: string }[] {
  const have = new Set(list.filter((c) => !c.own).map((c) => c.family))
  const found = VIG_FAMILIES.filter((f) => have.has(f.id))
  return found.length > 1 ? found : []
}

// Фильтр, которого уже нет среди видов (числа поменялись, шаблонов вида не осталось), — «Все».
export const activeFamily = (f: VigFamilyFilter, present: readonly { id: VigFamily }[]): VigFamilyFilter =>
  f !== 'all' && present.some((p) => p.id === f) ? f : 'all'

// Шаблоны под фильтром. Выбранный (current) виден всегда — иначе стрелки ходили бы мимо него и выбор «терялся»;
// свои не фильтруются.
export function shownTpls<T extends FamilyItem>(list: readonly T[], family: VigFamilyFilter, current?: string): T[] {
  if (family === 'all') return [...list]
  return list.filter((c) => c.own || c.family === family || c.id === current)
}

// Шаблоны под фильтром вида ▦ в порядке Go: по ним идёт Пробел, чтобы листание не выходило за то, что на экране.
// Вид, которого уже нет среди каталожных (числа поменялись), — «Все».
export const visibleTpls = <T extends FamilyItem>(list: readonly T[], family: VigFamilyFilter, current?: string): T[] =>
  shownTpls(list, activeFamily(family, familiesIn(list)), current)
