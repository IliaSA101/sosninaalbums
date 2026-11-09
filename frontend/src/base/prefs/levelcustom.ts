// Уровень «Свой» (спека 011, design §4): набор строк как разница с основой, опоры, переключение.
// Чистая логика без Svelte; состояние и сохранение — prefs.svelte.ts. Таблица — levels.ts.
import {
  FEATURES,
  OBJECT_FEATURE,
  RANK,
  blocker,
  normBase,
  type Base,
  type Custom,
  type Feature,
  type FeatureId,
  type Level,
  type ObjectKind,
  type RowId,
} from './levels'

const ROW = new Map<RowId, Feature>(FEATURES.map((f) => [f.id, f]))

// Основа: без неё программа не работает — в «Своём» не снимается.
export const locked = (r: Feature | RowId): boolean => {
  const id = typeof r === 'string' ? r : r.id
  return id === 'base' || id === 'menu'
}

export const isRow = (id: string): id is RowId => ROW.has(id as RowId)

export const inBase = (r: Feature, b: Base): boolean => RANK[r.min] <= RANK[b]

// Транзитивные опоры и зависящие считаются один раз при загрузке модуля (строк 23, глубина ≤ 1).
function closure(start: RowId, next: (id: RowId) => readonly RowId[]): RowId[] {
  const seen = new Set<RowId>([start])
  const stack = [start]
  while (stack.length) {
    for (const n of next(stack.pop()!)) {
      if (!seen.has(n)) {
        seen.add(n)
        stack.push(n)
      }
    }
  }
  return FEATURES.filter((f) => f.id !== start && seen.has(f.id)).map((f) => f.id)
}
const direct = (id: RowId): readonly RowId[] => ROW.get(id)?.needs ?? []
const reverse = (id: RowId): readonly RowId[] => FEATURES.filter((g) => g.needs?.includes(id)).map((g) => g.id)
const NEEDS = new Map<RowId, RowId[]>(FEATURES.map((f) => [f.id, closure(f.id, direct)]))
const DEPS = new Map<RowId, RowId[]>(FEATURES.map((f) => [f.id, closure(f.id, reverse)]))

// Все опоры строки (без неё самой), в порядке таблицы: «Нужна: …».
export const needsOf = (id: RowId): readonly RowId[] => NEEDS.get(id) ?? []
// Все строки, которые держатся на этой (без неё самой): «От неё зависят: …».
export const dependentsOf = (id: RowId): readonly RowId[] => DEPS.get(id) ?? []

export const shortOf = (id: RowId): string => ROW.get(id)?.short ?? id

// Пустой набор на основе: «Свой» без правок равен основе.
export const fromBase = (base: Base): Custom => ({ base, on: [], off: [] })

// Смена основы начинает набор заново.
export const setBase = (b: Base): Custom => fromBase(b)

// Включённые строки «Своего». Несогласованный набор из файла (функция без опоры) гасим, а не
// включаем опору: файл мог править человек или другая версия.
export function rows(c: Custom): Set<RowId> {
  const base = normBase(c.base)
  const on = new Set(c.on)
  const off = new Set(c.off)
  const set = new Set<RowId>()
  for (const f of FEATURES) if (locked(f) || (inBase(f, base) && !off.has(f.id)) || on.has(f.id)) set.add(f.id)
  for (let changed = true; changed;) {
    changed = false
    for (const f of FEATURES) {
      if (set.has(f.id) && f.needs?.some((n) => !set.has(n))) {
        set.delete(f.id)
        changed = true
      }
    }
  }
  return set
}

const expand = (ids: Iterable<RowId>): Set<FeatureId> => {
  const out = new Set<FeatureId>()
  for (const id of ids) {
    out.add(id)
    for (const a of ROW.get(id)?.also ?? []) out.add(a)
  }
  return out
}

// Доступные функции (id строк и их also). 'custom' без набора — «Простой» (spec п. 19).
export function features(level: Level, c: Custom | null): Set<FeatureId> {
  if (level === 'custom') return expand(rows(c ?? fromBase('simple')))
  return expand(FEATURES.filter((f) => inBase(f, level)).map((f) => f.id))
}

// Минимальная разница набора с основой: on — сверх основы, off — снято (заблокированные не снимаются).
export function diff(base: Base, set: ReadonlySet<RowId>): Custom {
  const on: string[] = []
  const off: string[] = []
  for (const f of FEATURES) {
    const inB = inBase(f, base)
    if (set.has(f.id) && !inB) on.push(f.id)
    else if (!set.has(f.id) && inB && !locked(f)) off.push(f.id)
  }
  return { base, on, off }
}

export interface Toggled {
  custom: Custom
  added: RowId[] // что включилось (вместе с самой строкой, если она была выключена)
  removed: RowId[] // что выключилось (вместе с самой строкой, если она была включена)
}

// Галочка в колонке «Свой». Включение тянет опоры, снятие гасит зависящих; чужие id набора остаются.
export function toggle(c: Custom, row: RowId, on: boolean): Toggled {
  if (!isRow(row) || locked(row)) return { custom: c, added: [], removed: [] }
  const cur = rows(c)
  const group = [row, ...(on ? needsOf(row) : dependentsOf(row))]
  const changed = FEATURES.filter((f) => group.includes(f.id) && cur.has(f.id) !== on).map((f) => f.id)
  const next = new Set(cur)
  for (const id of changed) {
    if (on) next.add(id)
    else next.delete(id)
  }
  const d = diff(normBase(c.base), next)
  const foreign = (l: string[]) => l.filter((id) => !isRow(id))
  const merge = (a: string[], b: string[]) => [...new Set([...a, ...b])]
  return {
    custom: { base: d.base, on: merge(d.on, foreign(c.on)), off: merge(d.off, foreign(c.off)) },
    added: on ? changed : [],
    removed: on ? [] : changed,
  }
}

// Что мешает править объект. Готовый уровень — какой нужен; «Свой» — какую строку включить.
export function blockerOf(kind: ObjectKind, level: Level, c: Custom | null): { level: Base } | { row: RowId } | null {
  if (level !== 'custom') {
    const need = blocker(kind, level)
    return need ? { level: need } : null
  }
  const row = OBJECT_FEATURE[kind]
  return rows(c ?? fromBase('simple')).has(row) ? null : { row }
}
