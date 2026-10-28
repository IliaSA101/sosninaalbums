// Окно «Автосборка» в режиме «Весь альбом заново» (107): строки источника, вопрос, подписи «Собрать:». Чистые функции.
import type { PickResult } from '../../base/core/hooks'
import type { AutoPlan } from '../../base/core/types'
import { plural } from '../../base/words/plural'
import { unitOf, type Unit } from '../../base/geom/sheet'

export type AlbumNums = NonNullable<AutoPlan['album']>

export const ALBUM_MODE = 'Весь альбом заново'
export const ALBUM_ASK_TITLE = 'Пересобрать альбом?'
export const ALBUM_BUILD = 'Пересобрать…' // главная кнопка окна: дальше вопрос
export const NOTHING_TO_REBUILD =
  'Пересобирать нечего: все развороты ручные или оформлены. Разворот вернётся в пересборку после «Вернуть авто» и снятия оформления'

const SPREAD = unitOf(undefined)

// «на 22 разворотах» (предложный падеж)
const ON: Record<string, [string, string, string]> = {
  разворот: ['развороте', 'разворотах', 'разворотах'],
  лист: ['листе', 'листах', 'листах'],
}

// Что пересборка не трогает: «обложку, 3 ручных, 1 виньетку, 2 оформленных (надписи, стикеры, свой фон, название)».
// Пусто — трогать нечего (в альбоме одни голые развороты и нет обложки).
export function untouchedText(a: Pick<AlbumNums, 'manual' | 'vignettes' | 'decor' | 'blank'>, cover: boolean): string {
  const parts: string[] = []
  if (cover) parts.push('обложку')
  if (a.manual > 0) parts.push(`${a.manual} ${plural(a.manual, ['ручной', 'ручных', 'ручных'])}`)
  if (a.vignettes > 0) parts.push(`${a.vignettes} ${plural(a.vignettes, ['виньетку', 'виньетки', 'виньеток'])}`)
  if (a.decor > 0)
    parts.push(`${a.decor} ${plural(a.decor, ['оформленный', 'оформленных', 'оформленных'])} (надписи, стикеры, свой фон, название)`)
  if (a.blank > 0) parts.push(`${a.blank} ${plural(a.blank, ['пустой', 'пустых', 'пустых'])}`)
  return parts.join(', ')
}

// Развороты, которые остаются на месте (якоря): в «В альбоме станет…» они стоят рядом с новыми
export const albumKept = (a: Pick<AlbumNums, 'manual' | 'vignettes' | 'decor' | 'blank'>): number =>
  a.manual + a.vignettes + a.decor + a.blank

// Строка источника режима альбома. Фото — голые развороты плюс свободные, взятые в пул (галочка или прежняя сборка,
// 107-qa-1): число равно тому, что соберётся заново. В пересборке план идёт от снимка до первой сборки, и без
// свободных строка показала бы 224 при 226 фото в альбоме. Плана ещё нет — название режима; голых нет — почему.
export function albumLine(a: AlbumNums | undefined | null, cover: boolean, unit: Unit = SPREAD): string {
  if (!a) return ALBUM_MODE
  if (a.spreads <= 0) return NOTHING_TO_REBUILD
  const on = plural(a.spreads, ON[unit.one] ?? ON['разворот'])
  const keep = untouchedText(a, cover)
  return `Весь альбом: ${a.photos + a.free} фото на ${a.spreads} ${on}${keep ? `; не трогаю: ${keep}` : ''}`
}

// Первый пункт строки «Собрать:» — выбор, к которому возвращает щелчок (число фото в нём)
export function baseLabel(b: Pick<PickResult, 'from' | 'ids' | 'bin'>): string {
  if (b.from === 'selected') return `Выделенные: ${b.ids.length}`
  return b.bin ? `Подборка «${b.bin}»: ${b.ids.length}` : `Свободные фото: ${b.ids.length}`
}

export const freeLabel = (n: number): string => `Добавить свободные фото ленты: ${n}`

// Вопрос «Пересобрать альбом?» (107-5): числа из плана. n — сколько разворотов выйдет; undoKey — клавиша отмены текстом.
export function rebuildAskLines(a: AlbumNums, n: number, similar: number, cover: boolean, undoKey: string, unit: Unit = SPREAD): string[] {
  const lines = [
    `${a.spreads} ${plural(a.spreads, unit.plural)} ${plural(a.spreads, ['соберётся', 'соберутся', 'соберутся'])} заново: раскладки и фото на них сменятся`,
  ]
  if (n !== a.spreads) lines.push(`Станет: ${n} ${plural(n, unit.plural)}`)
  const keep = untouchedText(a, cover)
  if (keep) lines.push(`Не трогаю: ${keep}`)
  if (similar > 0) lines.push(`Похожих кадров уйдёт в ленту: ${similar}`)
  lines.push(undoKey ? `Отменить — ${undoKey}` : 'Отменить — «Правка ▸ Отменить»')
  return lines
}
