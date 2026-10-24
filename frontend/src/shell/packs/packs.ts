// Подписи окна «Наборы библиотеки» (067 Ф2): строки списка, вопросы и тосты. Go присылает числа и сырые строки, текст — здесь.
// Название, автор, лицензия, источник и описание — недоверенный текст из манифеста: только как текст, без разметки и ссылок.
import { plural } from '../../base/words/plural'
import { sizeLabel } from '../dialogs/backups'

// Типы ответов Go (app.PackInfo и др.): интерфейс читает их как есть.
export interface PackInfo {
  id: string
  title: string
  about: string
  author: string
  license: string
  source: string
  origin: string // builtin | file | net
  version: number
  bg: number
  sticker: number
  spread: number
  frame: number // рамочки-картинки (079)
  print: number // типографии (профили печати)
  bytes: number
  removable: boolean
}
export interface PackSkip {
  name: string
  why: string
}
// ask (строка, значения Go): '' — поставлен; same — эта версия уже стоит; update — вопрос «Заменить?» (token); older — стоит версия новее.
export interface PackAdd {
  pack: PackInfo
  ask: string
  fromVersion: number
  token: string
  skipped: PackSkip[] | null
}
// Согласие на сеть (app.LibraryNet): allowed — разрешено, atStart — проверять при запуске, checked — unix мс последней проверки.
export interface LibraryNet {
  allowed: boolean
  atStart: boolean
  checked: number
  skipped: number // сетевых наборов в пропуске (удалены пользователем)
}
export interface PackList {
  packs: PackInfo[] | null
  net?: LibraryNet // у старого Go поля нет
  busy: boolean
  readOnly: boolean
}
export interface PackRemove {
  files: number
  kept: number
  bytes: number
}

// Вопросы окна, пока они открыты.
export type PackAsk =
  | { kind: 'remove'; id: string; title: string; text: string; note: string }
  | { kind: 'replace'; token: string; title: string; text: string }

export const BUSY_HINT = 'Библиотека обновляется'
export const EMPTY_TEXT = 'Других наборов пока нет. Добавьте набор файлом — кнопка «Добавить из файла…».'
export const ADD_HINT = 'Набор — файл .zip с фонами, стикерами и разворотами'
export const BUILTIN_NOTE = 'не удаляется'
export const WHY_MAX = 3 // сколько пропущенных предметов называем в тосте

const ORIGIN: Record<string, string> = { file: 'Из файла', net: 'Из сети', builtin: 'Рисует программа' }

const PART: [keyof Pick<PackInfo, 'bg' | 'sticker' | 'spread' | 'frame' | 'print'>, [string, string, string]][] = [
  ['bg', ['фон', 'фона', 'фонов']],
  ['sticker', ['стикер', 'стикера', 'стикеров']],
  ['spread', ['разворот', 'разворота', 'разворотов']],
  ['frame', ['рамочка', 'рамочки', 'рамочек']],
  ['print', ['типография', 'типографии', 'типографий']],
]

// «12 фонов, 8 разворотов» — только то, что есть в наборе.
export function contentsLabel(p: Pick<PackInfo, 'bg' | 'sticker' | 'spread' | 'frame' | 'print'>, sep = ', '): string {
  return PART.filter(([k]) => p[k] > 0)
    .map(([k, forms]) => `${p[k]} ${plural(p[k], forms)}`)
    .join(sep)
}

// Первая строка под названием: «версия 3 · 12 фонов · 30 стикеров · 8 разворотов · 41 МБ». Размер встроенного неизвестен — не пишем.
export function metaLine(p: PackInfo): string {
  const parts: string[] = []
  if (p.origin !== 'builtin' && p.version > 0) parts.push(`версия ${p.version}`)
  const c = contentsLabel(p, ' · ')
  if (c) parts.push(c)
  if (p.bytes > 0) parts.push(sizeLabel(p.bytes))
  return parts.join(' · ')
}

// Вторая строка: «Автор: Марина · Лицензия: CC0-1.0 · Источник: пример.рф · Из файла». Пустое пропускается.
export function creditLine(p: PackInfo): string {
  const parts: string[] = []
  if (p.author) parts.push(`Автор: ${p.author}`)
  if (p.license) parts.push(`Лицензия: ${p.license}`)
  if (p.source) parts.push(`Источник: ${p.source}`) // текстом: ссылкой не делаем
  const from = ORIGIN[p.origin]
  if (from) parts.push(from)
  return parts.join(' · ')
}

// Встроенные первыми (Go и так отдаёт их первыми, но порядок окна не зависит от этого).
export function sortPacks(list: readonly PackInfo[]): PackInfo[] {
  return [...list].sort((a, b) => Number(b.origin === 'builtin') - Number(a.origin === 'builtin'))
}

// Кроме встроенных наборов нет: подсказка вместо пустого места.
export const onlyBuiltin = (list: readonly PackInfo[]): boolean => list.every((p) => p.origin === 'builtin')

// Пропущенные предметы архива: «пропущено: 2 (имя — причина; …)».
export function skippedLabel(skipped: readonly PackSkip[] | null | undefined): string {
  const n = skipped?.length ?? 0
  if (!skipped || n === 0) return ''
  const why = skipped.slice(0, WHY_MAX).map((s) => `${s.name} — ${s.why}`)
  const rest = n - why.length
  return `пропущено: ${n} (${why.join('; ')}${rest > 0 ? ` и ещё ${rest}` : ''})`
}

// Тост «Набор «Айвори» добавлен: 12 фонов, 8 разворотов» / «обновлён до версии 3». Пропущенное — в конце.
export function addedText(r: PackAdd, replaced: boolean): string {
  const p = r.pack
  const head = replaced ? `Набор «${p.title}» обновлён до версии ${p.version}` : `Набор «${p.title}» добавлен`
  const c = contentsLabel(p)
  const skipped = skippedLabel(r.skipped)
  return [c ? `${head}: ${c}` : head, skipped].filter(Boolean).join('; ')
}

// Итог «Добавить из файла…», когда набор не ставится сразу. Пусто — ничего показывать не надо.
export function notAddedText(r: PackAdd): string {
  const t = `«${r.pack.title}»`
  if (r.ask === 'same') return `Набор ${t} уже есть`
  if (r.ask === 'older') return `Набор ${t}: уже стоит версия новее${r.fromVersion > 0 ? ` (${r.fromVersion})` : ''}`
  return ''
}

// Диалог закрыт без выбора: Go отдаёт пустой итог.
export const cancelled = (r: PackAdd | null | undefined): boolean => !r || (!r.pack?.id && !r.ask)

export const replaceQuestion = (r: PackAdd): string => `Заменить набор «${r.pack.title}» версии ${r.fromVersion} версией ${r.pack.version}?`

// «Удалить набор «Айвори»?»: что пропадёт, что останется и почему (числа — PlanRemovePack).
export function removeQuestion(p: Pick<PackInfo, 'title' | 'bg' | 'sticker' | 'spread' | 'frame' | 'print'>): string {
  return (
    `Удалить набор «${p.title}»? ${goneText(p)} ` +
    'В открытом альбоме всё останется; в других альбомах — станет пропавшим и вернётся при повторной загрузке набора.'
  )
}

// «Его фоны, стикеры и развороты пропадут из окон.» — по составу набора. Состав пуст — «Его содержимое пропадёт…».
function goneText(p: Pick<PackInfo, 'bg' | 'sticker' | 'spread' | 'frame' | 'print'>): string {
  const names = PART.filter(([k]) => p[k] > 0).map(
    ([k]) => ({ bg: 'фоны', sticker: 'стикеры', spread: 'развороты', frame: 'рамочки', print: 'типографии' })[k],
  )
  if (names.length === 0) return 'Его содержимое пропадёт из окон.'
  const list = names.length > 1 ? `${names.slice(0, -1).join(', ')} и ${names[names.length - 1]}` : names[0]
  return `Его ${list} пропадут из окон.`
}

// Вторая строка вопроса: сколько места освободится и почему часть файлов останется.
export function removeNote(r: PackRemove): string {
  const free =
    r.files > 0
      ? `Освободится ${sizeLabel(r.bytes)} (${r.files} ${plural(r.files, ['файл', 'файла', 'файлов'])}).`
      : 'Место не освободится: все файлы набора нужны ещё где-то.'
  if (r.kept <= 0) return free
  const kept = `${r.kept} ${plural(r.kept, ['файл', 'файла', 'файлов'])}`
  const stay = plural(r.kept, ['останется', 'останутся', 'останутся'])
  return `${free} ${kept} ${stay}: нужны открытому альбому, вашим фонам или другим наборам.`
}

export const removedText = (title: string): string => `Набор «${title}» удалён`
