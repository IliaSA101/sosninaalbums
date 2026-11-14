// Строка под «призраком» переносимых фото (168 §8): что сделает бросок или чем его можно заменить. Чистая часть;
// «после первой замены не показывать» — флаг запуска replaceHint, как «уже показывали» у подсказки колеса (095).
import type { Payload, Target } from '../core/types'
import { HINT_TEXT } from './wheelhint.svelte'

export const REPLACE_NOTE = 'Заменить фото'
export const replaceKeyNote = (mac: boolean): string => `${mac ? '⌘' : 'Ctrl'} — заменить фото`

// Запуск программы: замена на «Авто» уже была — «Ctrl — заменить фото» больше не показываем (в config не пишется).
export const replaceHint = { used: false }

export interface NoteIn {
  target: Target | null
  payload: Payload | null
  keep: boolean // зажат Ctrl (⌘)
  mac: boolean
  used: boolean // replaceHint.used
  wheel: boolean // подсказка колеса (095) пора показать
}

// Порядок: «Заменить фото» → «Ctrl — заменить фото» → подсказка колеса. Пусто — строки нет.
export function ghostNote(i: NoteIn): string {
  const p = i.payload
  const t = i.target
  if (p?.kind === 'photos' && !p.fromSpread && t) {
    // Замена: фото «Авто» с Ctrl (одно фото) и занятое место ручного листа — то же слово для той же правки.
    if (t.kind === 'cell' && t.side === 'center' && !t.many) return REPLACE_NOTE
    if (t.kind === 'place' && t.photo) return REPLACE_NOTE
    // Подсказка о клавише: одно фото из ленты над фото «Авто» без клавиши, пока замены не было.
    if (t.kind === 'cell' && !i.keep && !i.used && p.ids.length === 1) return replaceKeyNote(i.mac)
  }
  return i.wheel ? HINT_TEXT : ''
}
