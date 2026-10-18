// Чистая логика окна «Собрать проект в папку» (спека 069): строки сводки, хода и выбор «что собирать».
// Числа и объёмы считает Go (CollectInfo, collect:progress) — здесь только слова.
import { plural } from '../../base/words/plural'
import type { CollectProgress } from '../../base/core/jobs.svelte'

// Сводка Go (collect.Info): photos — фото альбома, extra — ещё фото из папок альбома (режим «Все»).
export interface CollectInfo {
  photos: number
  extra: number
  own: number
  guides: number
  missing: number
  missingGuides: number
  bytes: number
  scanning: boolean
  folderName: string // имя папки альбома, как её создаст Go (fsfera.FolderName)
}

const MB = 1 << 20
const GB = 1 << 30

const comma = (v: number, prec: number) => v.toFixed(prec).replace('.', ',')

// Объём как у Go (collect.SizeText): «6,3 ГБ», «4,5 МБ», «120 МБ», «< 0,1 МБ».
export function sizeText(n: number): string {
  if (n >= GB) return `${comma(n / GB, 1)} ГБ`
  if (n < MB / 10) return '< 0,1 МБ'
  if (n < 10 * MB) return `${comma(n / MB, 1)} МБ`
  return `${comma(n / MB, 0)} МБ`
}

// «2,4 из 6,2 ГБ»: единица одна, по общему объёму.
export function sizePair(done: number, total: number): string {
  if (total >= GB) return `${comma(done / GB, 1)} из ${comma(total / GB, 1)} ГБ`
  if (total >= 10 * MB) return `${comma(done / MB, 0)} из ${comma(total / MB, 0)} МБ`
  return `${comma(done / MB, 1)} из ${comma(total / MB, 1)} МБ`
}

// «1 240»: разряды неразрывным пробелом.
export const num = (n: number): string => String(n).replace(/\B(?=(\d{3})+(?!\d))/g, ' ')

// «(312 фото, 6,2 ГБ)» — подсказка у варианта; сводки ещё нет — пусто.
export function optionHint(photos: number | null, bytes: number): string {
  return photos === null ? '' : `(${num(photos)} фото, ${sizeText(bytes)})`
}

// «Ещё: свои фоны и стикеры — 3, разметка типографии — 2.»; нечего сказать — пусто.
export function extraLine(i: Pick<CollectInfo, 'own' | 'guides'>): string {
  const parts = [i.own > 0 ? `свои фоны и стикеры — ${i.own}` : '', i.guides > 0 ? `разметка типографии — ${i.guides}` : ''].filter(Boolean)
  return parts.length ? `Ещё: ${parts.join(', ')}.` : ''
}

// «4 фото не найдены — в папку не попадут.»; слова те же, что в вопросе Go (missingQuestion).
export function missingLine(i: Pick<CollectInfo, 'missing' | 'missingGuides'>): string {
  const g = i.missingGuides
  const what = [
    i.missing > 0 ? `${i.missing} фото` : '',
    g > 0 ? `${g} ${plural(g, ['разметка', 'разметки', 'разметок'])} типографии` : '',
  ].filter(Boolean)
  if (what.length === 0) return ''
  if (i.missing === 1 && g === 0) return '1 фото не найдено — в папку не попадёт.'
  if (i.missing === 0 && g === 1) return '1 разметка типографии не найдена — в папку не попадёт.'
  return `${what.join(' и ')} не найдены — в папку не попадут.`
}

// Заглушка имени папки до ответа Go: настоящее имя (запретные знаки, обрезка) строит Go и отдаёт в CollectInfo.folderName.
export const folderStub = (saved: boolean, projectName: string): string => (saved ? projectName : 'Альбом')

export function folderHint(name: string): string {
  return `В выбранной папке появится папка «${name}»: альбом, «Фото», «Свои фоны и стикеры», «Разметка». Открытый альбом не меняется.`
}

// «Скопируется 6,2 ГБ.»; в альбоме нет файлов — только сам альбом.
export function totalLine(i: Pick<CollectInfo, 'bytes'> | null): string {
  if (!i) return ''
  return i.bytes > 0 ? `Скопируется ${sizeText(i.bytes)}.` : 'В папку запишется только файл альбома.'
}

// Заглушки до ответа Go: места под числа и итог заняты сразу, окно не прыгает, когда ответ приходит.
export const PENDING_HINT = '(… фото)'
export const PENDING_TOTAL = 'Скопируется …'

// Строки сводки для окна: готовые числа или заглушки той же высоты; pending — бледный вид.
export interface Summary {
  albumHint: string
  fullHint: string
  total: string
  pending: boolean // итог ещё не пришёл
}

export function summaryOf(album: CollectInfo | null, full: CollectInfo | null, cur: CollectInfo | null): Summary {
  return {
    albumHint: album ? optionHint(album.photos, album.bytes) : PENDING_HINT,
    fullHint: full ? optionHint(full.photos + full.extra, full.bytes) : PENDING_HINT,
    total: cur ? totalLine(cur) + (cur.scanning ? ' ' + SCANNING_LINE : '') : PENDING_TOTAL,
    pending: !cur,
  }
}

export const SCANNING_LINE = 'Папки альбома ещё читаются — число фото может вырасти.'

// «Скопировано 120 из 312 · 2,4 из 6,2 ГБ»; пока Go не прислал первый ход — «Готовим список файлов…».
export function progressLine(p: CollectProgress): string {
  if (p.totalFiles <= 0) return 'Готовим список файлов…'
  return `Скопировано ${num(p.doneFiles)} из ${num(p.totalFiles)} · ${sizePair(p.doneBytes, p.totalBytes)}`
}

// Доля хода 0…100 по байтам (файлы разного размера); общий объём неизвестен — по файлам; ничего — null (полоса «бежит»).
export function progressPercent(p: CollectProgress): number | null {
  if (p.totalBytes > 0) return Math.min(100, (p.doneBytes / p.totalBytes) * 100)
  if (p.totalFiles > 0) return Math.min(100, (p.doneFiles / p.totalFiles) * 100)
  return null
}

// Что на экране: пока идёт копирование — полоса и «Остановить», иначе настройки.
export type CollectPhase = 'closed' | 'setup' | 'running'
export const phaseOf = (open: boolean, collecting: boolean): CollectPhase => (!open ? 'closed' : collecting ? 'running' : 'setup')

// Ответ Go на сводку устарел: за время запроса окно переоткрыли или альбом изменился.
export const infoStale = (mine: number, last: number): boolean => mine !== last

// Выбор «Только фото альбома / Все фото из папок» запоминается (как настройки экспорта).
const KEY = 'collectAll'

export function loadAll(): boolean {
  try {
    return localStorage.getItem(KEY) === '1'
  } catch {
    return false // без хранилища — по умолчанию «Только фото альбома»
  }
}

export function saveAll(all: boolean) {
  try {
    localStorage.setItem(KEY, all ? '1' : '0')
  } catch {
    // Не сохранилось — не страшно.
  }
}
