// Тексты окна «Горячие клавиши» (074): строки статуса, вопроса «Заменить?» и перекрытий. Клавиши — только из реестра
// (chordText): «Ctrl+D» руками не пишется (страж keys.guard).
import { codeOfChord, modsOfChord } from '../../base/keys/chord'
import { chordText, codeText } from '../../base/keys/chordtext'
import type { KeymapEntry, KeyScope } from '../../base/types/keys'

// Подпись области в строке списка и в переключателе команд клавиши («везде» — без подписи).
export const SCOPE_TAG: Readonly<Record<KeyScope, string>> = { app: '', canvas: 'на холсте', library: 'в ленте фото', bg: 'режим фона' }
// То же в начале фразы.
export const SCOPE_LEAD: Readonly<Record<KeyScope, string>> = {
  app: 'Везде',
  canvas: 'На холсте',
  library: 'В ленте фото',
  bg: 'В режиме фона',
}

// Имя команды в кавычках: внутренние «…» из самого имени («Флажок «отобрано»») становятся „…“ (design §8).
export const q = (label: string): string => `«${label.replaceAll('«', '„').replaceAll('»', '“')}»`

export const HIDDEN = ' (скрыта на этом уровне)'
export const hid = (visible: boolean): string => (visible ? '' : HIDDEN)

export const plural = (n: number, a: string, b: string, c: string): string =>
  n % 10 === 1 && n % 100 !== 11 ? a : n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20) ? b : c

// «Клавиша B» читается лучше, чем голое «B» в начале фразы; с модификатором и длинные имена — как есть.
export function phrase(chord: string, mac: boolean): string {
  const plain = modsOfChord(chord).length === 0 && codeText(codeOfChord(chord)).length === 1
  return plain ? 'Клавиша ' + chordText(chord, mac) : chordText(chord, mac)
}

// Вопрос «Заменить?»: сочетание уже у другой команды той же области.
export const askText = (chord: string, holder: string, visible: boolean, mac: boolean): string =>
  `${phrase(chord, mac)} уже у ${q(holder)}${hid(visible)}. Если заменить, у ${q(holder)} этой клавиши не будет.`

// Статус после назначения: «B — «Во весь экран» (у «X» больше нет)».
export function assignedText(chord: string, label: string, replaced: { label: string; visible: boolean } | null, mac: boolean): string {
  const t = `${chordText(chord, mac)} — ${q(label)}`
  return replaced ? `${t} (у ${q(replaced.label)}${hid(replaced.visible)} больше нет)` : t
}

export const removedText = (chord: string, label: string, mac: boolean): string => `${chordText(chord, mac)} убрана у ${q(label)}`
export const resetOneText = (label: string): string => `${q(label)} — стандартные клавиши`
export const resetAllText = (n: number): string => `Все клавиши — как было (своих было ${n})`

// Перекрытие в другой области: у кого область главнее, та команда и сработает.
export function shadowNote(
  e: KeymapEntry,
  other: KeymapEntry,
  chord: string,
  otherRank: number,
  rank: number,
  otherVisible: boolean,
  mac: boolean,
): string {
  const t = chordText(chord, mac)
  if (otherRank > rank) return `${SCOPE_LEAD[other.scope]} ${t} — ${q(other.label)}${hid(otherVisible)}, там сработает она`
  return `${t} есть и у ${q(other.label)} (${SCOPE_TAG[other.scope] || 'везде'})${hid(otherVisible)}; ${SCOPE_TAG[e.scope]}, где действует ${q(e.label)}, сработает она`
}
