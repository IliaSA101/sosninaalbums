// «Сохранить альбом как макет…» (131 U3, сц. 12): чистая часть окна — слова, зажимы, сборка параметров для SaveLook.
// Слияние одинаковых разворотов и предел 24 шага считает Go (SaveLook); окно только собирает выбор.
import type { SegItem } from '../../base/tpl/Seg.svelte'
import type { SaveLookOpts } from './looktypes'
import type { DecorCatalog } from '../../panels/decor/decor'
import { SAVE_HINT, saveThemes } from '../../panels/decor/spreadtpl'

export const SAVE_LOOK_TITLE = 'Сохранить альбом как макет'
export const SAVE_LOOK_HINT = `${SAVE_HINT} Одинаковые повторяемые развороты сольются в один шаг; шагов не больше 24. Текст заготовок войдёт как есть: перед передачей макета замените его на «[Имена]», «[Дата]».`
export const NO_REPEAT = 'Повторяемых нет: серия — только первый и последний шаги'
export const NO_SPREADS = 'В альбоме нет разворотов — макет из него не сохранить'
export const NO_COVER = 'В альбоме нет обложки'
export const savedText = (name: string): string => `Макет «${name}» сохранён в библиотеку`

export type SaveScope = 'all' | 'selected'

export const scopeItems = (selected: number): SegItem<SaveScope>[] => [
  { id: 'all', label: 'Все' },
  {
    id: 'selected',
    label: 'Выделенные',
    off: selected === 0,
    title: selected === 0 ? 'Выделите развороты в ленте (Ctrl+щелчок, на Mac — ⌘)' : undefined,
  },
]

// Темы окна: настоящие темы каталога, «Мои» — всегда (туда кладётся по умолчанию, даже пока она пуста).
export function themeItems(cat: Pick<DecorCatalog, 'themes'> | null): { id: string; name: string }[] {
  const real = saveThemes(cat)
  return real.some((t) => t.id === 'mine') ? real : [...real, { id: 'mine', name: 'Мои' }]
}

// Тема по умолчанию — «Мои»: чужие наборы окно не правит, а открытая в «Макеты альбома» тема может быть только чтением.
export const DEFAULT_THEME = 'mine'

// Сколько выбрано: все развороты или выделенные в ленте (в порядке альбома).
export const countOf = (scope: SaveScope, all: number, selected: number): number => (scope === 'selected' ? selected : all)

// Выделение из двух и больше разворотов — явное намерение «сохранить выбранные»; один выделенный — случайный щелчок.
export const defaultScope = (selected: number): SaveScope => (selected >= 2 ? 'selected' : 'all')

// Повторяемые по умолчанию: со второго по предпоследний. Меньше трёх разворотов — повторяемых нет (с > по).
export const defaultRepeat = (n: number): { from: number; to: number } => ({ from: 2, to: n - 1 })

// Позиции 1…n; мусор — нижний предел. Пара с > по означает «повторяемых нет» и уходит в Go как есть.
export function clampPos(v: number, n: number): number {
  const x = Math.trunc(v)
  return Number.isFinite(x) ? Math.min(Math.max(1, n), Math.max(1, x)) : 1
}

// «По»: 0…N (0 — повторяемых нет); мусор и пусто — 0.
export const clampTo = (v: number, n: number): number => Math.min(n, Math.max(0, Math.trunc(Number.isFinite(v) ? v : 0)))

export const hasRepeat = (from: number, to: number): boolean => from <= to

export interface SaveForm {
  theme: string
  name: string
  ids: string[]
  from: number
  to: number
  places: boolean
  drafts: boolean
  cover: boolean
}

// Параметры SaveLook (design §8): ids — развороты в порядке альбома, repFrom/repTo — позиции среди них с 1.
export function saveOpts(f: SaveForm, hasCover: boolean): SaveLookOpts {
  return {
    theme: f.theme,
    name: f.name.trim(),
    ids: f.ids,
    repFrom: f.from,
    repTo: f.to,
    places: f.places,
    drafts: f.drafts,
    cover: f.cover && hasCover,
  }
}
