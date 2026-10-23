// Выравнивание и распределение (спека 010, design §5): состав кнопок, опоры и минимум объектов.
// Чистая логика; вызов Go и «Относительно» — align.svelte.ts, полоса — AlignBar, меню — menu.ts.
// Сдвигает Go (AlignItems): здесь только «сколько нужно выделить», чтобы не звать холостую правку.
import { plural } from '../../base/words/plural'

export type AlignOp = 'left' | 'hcenter' | 'right' | 'top' | 'vcenter' | 'bottom' | 'distH' | 'distV'
export type AlignTo = 'sel' | 'page' | 'spread'

export interface AlignAction {
  op: AlignOp
  label: string // пункт меню и подсказка кнопки
  icon: 'al-left' | 'al-hcenter' | 'al-right' | 'al-top' | 'al-vcenter' | 'al-bottom' | 'dist-h' | 'dist-v'
}

export const ALIGN_OPS: readonly AlignAction[] = [
  { op: 'left', label: 'По левому краю', icon: 'al-left' },
  { op: 'hcenter', label: 'По центру по горизонтали', icon: 'al-hcenter' },
  { op: 'right', label: 'По правому краю', icon: 'al-right' },
  { op: 'top', label: 'По верхнему краю', icon: 'al-top' },
  { op: 'vcenter', label: 'По центру по вертикали', icon: 'al-vcenter' },
  { op: 'bottom', label: 'По нижнему краю', icon: 'al-bottom' },
  { op: 'distH', label: 'Распределить по горизонтали', icon: 'dist-h' },
  { op: 'distV', label: 'Распределить по вертикали', icon: 'dist-v' },
]

// «Относительно: …» — подписи в родительном падеже («относительно Выделения»).
export const ALIGN_TO: readonly { to: AlignTo; label: string; hint: string }[] = [
  { to: 'sel', label: 'Выделения', hint: 'Опора — рамка вокруг всех выделенных' },
  { to: 'page', label: 'Страницы', hint: 'Опора — поля страницы, на которой объект' },
  { to: 'spread', label: 'Разворота', hint: 'Опора — поля всего разворота' },
]

export const isDist = (op: AlignOp): boolean => op === 'distH' || op === 'distV'

const OPS = new Set<string>(ALIGN_OPS.map((a) => a.op))
export const isAlignOp = (s: unknown): s is AlignOp => typeof s === 'string' && OPS.has(s)

// Сохранённое значение (localStorage) приводится к опоре; чужое — «Выделения».
export function normAlignTo(s: unknown): AlignTo {
  return s === 'page' || s === 'spread' ? s : 'sel'
}

// Опор две (спека 100 п. 4): отдельная для одного объекта и для двух и больше.
export type AlignSlot = 'one' | 'many'
export const alignSlot = (count: number): AlignSlot => (count >= 2 ? 'many' : 'one')

// Для одного «Выделения» не бывает: чужое и 'sel' из хранилища — «Страницы».
export function normAlignToOne(s: unknown): AlignTo {
  return s === 'spread' ? 'spread' : 'page'
}

// Почему кнопка опоры неактивна (подсказка говорит, что делать); null — можно.
export function alignToBlock(to: AlignTo, count: number): string | null {
  return to === 'sel' && count < 2 ? 'Относительно выделения — от 2 объектов: Shift+щелчок добавит ещё' : null
}

// Минимум объектов — как в Go: к выделению выравнивать от 2, распределять от 3;
// к странице и развороту — от 1 и от 2. Меньше — Go вернёт холостую правку.
export function alignNeed(op: AlignOp, to: AlignTo): number {
  const base = to === 'sel' ? 2 : 1
  return isDist(op) ? base + 1 : base
}

// Почему действие неактивно; null — можно.
export function alignBlock(op: AlignOp, to: AlignTo, count: number): string | null {
  const need = alignNeed(op, to)
  return count >= need ? null : `Выделите ${need} ${plural(need, ['объект', 'объекта', 'объектов'])}`
}
