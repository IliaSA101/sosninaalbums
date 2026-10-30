// Список и ячейки окна сравнения (138): чистые функции без Svelte. Список замораживается при открытии
// (как у окна 128): фильтр и отметки во время сравнения его не меняют, иначе кадр ушёл бы из-под стрелок.
import { plural } from '../../base/words/plural'

export const MAX_CELLS = 4

export interface CompareState {
  list: string[] // замороженный список ленты
  cells: string[] // id кадров в ячейках, порядок ленты
  active: number // номер активной ячейки
}
// Почему окно не открылось: тосты Т2 (ничего не выделено), Т3 (больше 4), Т4 (в ленте одно фото).
export type OpenWhy = 'none' | 'many' | 'alone'

export const activeId = (s: CompareState): string => s.cells[s.active]

// Выделенные кадры, которые сейчас видны в ленте, в порядке ленты (скрытые фильтром в сравнение не идут).
export function pickSelected(visible: readonly string[], selected: readonly string[]): string[] {
  const sel = new Set(selected)
  return visible.filter((id) => sel.has(id))
}

// Список заморожен; активной становится ячейка с кадром focus (нет такой — первая).
function make(list: readonly string[], cells: string[], focus: string): CompareState {
  return { list: [...list], cells, active: Math.max(0, cells.indexOf(focus)) }
}

// Открытие по выделению в ленте: 2–4 — эти кадры; один — он и следующий видимый (у последнего — предыдущий).
export function openBySelection(visible: readonly string[], selected: readonly string[]): CompareState | OpenWhy {
  if (visible.length === 1) return 'alone'
  const ids = pickSelected(visible, selected)
  if (ids.length === 0) return 'none'
  if (ids.length > MAX_CELLS) return 'many'
  if (ids.length === 1) return openPair(visible, ids[0])
  return make(visible, ids, ids[0])
}

// Из окна 128: показанный кадр и соседний по замороженному списку (следующий, у последнего — предыдущий).
export function openPair(list: readonly string[], shown: string): CompareState | OpenWhy {
  const at = list.indexOf(shown)
  if (at < 0 || list.length < 2) return 'alone'
  const cells = at + 1 < list.length ? [shown, list[at + 1]] : [list[at - 1], shown]
  return make(list, cells, shown)
}

// Ответ SimilarTo: кадры в порядке ленты (чужие и повторные отброшены, не больше 4); активный — исходный origin.
export function openIds(list: readonly string[], ids: readonly string[], origin: string): CompareState | OpenWhy {
  const want = new Set(ids)
  const cells = list.filter((id) => want.has(id)).slice(0, MAX_CELLS)
  if (cells.length < 2) return 'alone'
  return make(list, cells, origin)
}

// ← →: кадр в активной ячейке меняется на соседний по списку. Кадры других ячеек и ушедшие пропускаются,
// место ячейки не меняется. Край или некуда идти — то же состояние.
export function stepCell(s: CompareState, d: 1 | -1, alive: (id: string) => boolean): CompareState {
  const here = s.list.indexOf(activeId(s))
  if (here < 0) return s
  for (let i = here + d; i >= 0 && i < s.list.length; i += d) {
    const id = s.list[i]
    if (s.cells.includes(id) || !alive(id)) continue
    const cells = [...s.cells]
    cells[s.active] = id
    return { ...s, cells }
  }
  return s
}

// ✕ или Delete: ячейка уходит, активной становится соседняя (следующая, у последней — предыдущая).
export function removeCell(s: CompareState, at: number): CompareState {
  if (at < 0 || at >= s.cells.length) return s
  const cells = s.cells.filter((_, i) => i !== at)
  const active = at < s.active ? s.active - 1 : Math.min(s.active, cells.length - 1)
  return { ...s, cells, active: Math.max(0, active) }
}

// Кадры, ушедшие из альбома, пропадают вместе с ячейкой; активный ушёл — на его место встаёт соседняя.
export function dropGone(s: CompareState, alive: (id: string) => boolean): CompareState {
  const cur = activeId(s)
  const cells = s.cells.filter(alive)
  if (cells.length === s.cells.length) return s
  const at = cells.indexOf(cur)
  return { ...s, cells, active: at >= 0 ? at : Math.max(0, Math.min(s.active, cells.length - 1)) }
}

// Меньше двух кадров — это уже не сравнение: окно закрывается, оставшийся выделяется (0.29-138-11).
export const isLastLeft = (s: CompareState): boolean => s.cells.length < 2

// Выделение ленты на выходе: оставшиеся видимые кадры, якорь — активный (нет среди видимых — первый из них).
export function closeSelection(s: CompareState, visible: ReadonlySet<string>): { ids: string[]; anchor: string | null } {
  const ids = s.cells.filter((id) => visible.has(id))
  const cur = activeId(s)
  return { ids, anchor: ids.includes(cur) ? cur : (ids[0] ?? null) }
}

// Подпись ячейки: «имя · N из M» по замороженному списку.
export function cellCaption(s: CompareState, at: number, name: string): string {
  return `${name} · ${s.list.indexOf(s.cells[at]) + 1} из ${s.list.length}`
}

// Подпись окна: «Сравнение · 3 кадра».
export const title = (n: number): string => `Сравнение · ${n} ${plural(n, ['кадр', 'кадра', 'кадров'])}`

// Строка клавиш под подписью окна (Т8): метки клавиш заполняет keymap.text. Без функции «Отметки» нет «лучший» и «отметки».
// mod — «Ctrl» или «⌘» для колеса.
export function compareHint(mod: string, marks: boolean): string {
  const tail = marks ? ' · Enter — лучший · отметки — клавишами ленты' : ''
  return `щелчок, Tab — выбрать кадр · ← → — заменить соседним · щелчок{key:photoPreview| или %} — 1:1 у всех · ${mod}+колесо — зум · Shift+перетаскивание — сдвинуть один{key:fitZoom| · % — вписать}${tail} · Esc — закрыть`
}

// Свои клавиши окна без каталога (0.29-138-13): Esc, ← →, Tab, Enter («лучший»). Delete и Backspace окно берёт из реестра
// (команда binRemove, страж keys.guard не пускает «командные» клавиши в обработчики). onButton — фокус на кнопке окна:
// Enter там — её щелчок. Остальные клавиши (команды каталога) окно ищет в реестре само.
export type OwnKey = 'close' | 'prev' | 'next' | 'tab' | 'shiftTab' | 'best'
export interface KeyLike {
  key: string
  ctrlKey: boolean
  metaKey: boolean
  altKey: boolean
  shiftKey: boolean
}
export function ownKey(e: KeyLike, onButton: boolean): OwnKey | null {
  if (e.key === 'Escape') return 'close'
  if (e.ctrlKey || e.metaKey || e.altKey) return null
  if (e.key === 'Tab') return e.shiftKey ? 'shiftTab' : 'tab'
  if (e.key === 'Enter') return onButton ? null : 'best'
  if (!e.shiftKey && e.key === 'ArrowLeft') return 'prev'
  if (!e.shiftKey && e.key === 'ArrowRight') return 'next'
  return null
}
