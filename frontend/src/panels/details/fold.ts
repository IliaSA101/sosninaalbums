import type { SegItem } from '../../base/tpl/Seg.svelte'

// Фото через корешок (064): три положения из пары полей настроек. Правило подбора считает Go;
// здесь только перевод пары в положение и тексты (макет docs/specs/0.15/064-mock-fold.html).
export type FoldMode = 'none' | 'wide' | 'any'
export type FoldFields = { layflat: boolean; foldAny: boolean }

// foldAny без layflat смысла не имеет (Go его зажимает), поэтому «нет» главнее.
export function foldOf(s: { layflat?: boolean; foldAny?: boolean }): FoldMode {
  if (!s.layflat) return 'none'
  return s.foldAny ? 'any' : 'wide'
}

export function foldFields(m: FoldMode): FoldFields {
  return { layflat: m !== 'none', foldAny: m === 'any' }
}

export const FOLD_ITEMS: readonly SegItem<FoldMode>[] = [
  { id: 'none', label: 'Нет', title: 'Фото не заходят на корешок' },
  { id: 'wide', label: 'Только широкие', title: 'Шире высоты в 1,2 раза, не меньше 20 % на каждой странице' },
  { id: 'any', label: 'Любые фото', title: 'Через корешок — любые фото, и вертикальные тоже' },
]

const NOTES: Record<FoldMode, string> = {
  none: 'Каждая страница сама по себе: фото не заходят на корешок.',
  wide: 'Через корешок — только кадры шире высоты в 1,2 раза, не меньше 20 % на каждой странице: лицо на сгиб не попадёт.',
  any: 'Через корешок — любые фото: лесенка на весь разворот, столбец по сгибу. Лицо может оказаться на сгибе.',
}

export const foldNote = (m: FoldMode): string => NOTES[m]

// Вторая строка: пока выбрано не то, что в альбоме. Только «Нет» перекладывает свёрстанное.
export function foldChangeNote(applied: FoldMode, picked: FoldMode): string {
  if (applied === picked) return ''
  if (picked === 'none') return 'После «Применить» развороты с фото через корешок разложатся заново.'
  return 'Свёрстанные развороты останутся как есть; правило — для «Авто», «Вариантов» и автосборки.'
}
