// Развороты со своим цветом или фоном против цвета и фона альбома (115): кого считать своим и слова вопроса.
// Что значит «свой», решает Go (bgSet, ownBg по album у слоёв); здесь только отбор номеров для вопроса и тексты.
import type { SpreadView } from '../../base/core/types'
import { ownBg, type Unit } from '../../base/geom/sheet'
import { plural } from '../../base/words/plural'
import { formatRanges } from '../layouts/ranges'

export type OwnKind = 'color' | 'bg'

// Сколько номеров показывать в вопросе; остальные — «и ещё N».
export const OWN_NUMS_MAX = 12

export const OWN_KEEP = 'Оставить свой'
export const OWN_ALL = 'Как в альбоме'

type Spreads = { spreads: readonly SpreadView[] }

// Номера (с 1, по view.spreads; обложки там нет) разворотов, у которых свой цвет хотя бы на одной странице.
export function ownColorNums(v: Spreads): number[] {
  const out: number[] = []
  v.spreads.forEach((s, i) => {
    if ((s.bgSet?.[0] ?? '') !== '' || (s.bgSet?.[1] ?? '') !== '') out.push(i + 1)
  })
  return out
}

export interface OwnBg {
  nums: number[] // развороты со своим узором или фото фона на любой области
  photos: number // сколько своих фото фона на них уйдёт
}

export function ownBgOf(v: Spreads): OwnBg {
  const nums: number[] = []
  let photos = 0
  v.spreads.forEach((s, i) => {
    const b = ownBg(s.backdrops).length
    if (b + ownBg(s.patterns).length > 0) nums.push(i + 1)
    photos += b
  })
  return { nums, photos }
}

// «3, 7, 18»; больше OWN_NUMS_MAX номеров — первые и «и ещё N».
export function ownNumsText(nums: number[]): string {
  const head = nums.slice(0, OWN_NUMS_MAX)
  const rest = nums.length - head.length
  return rest > 0 ? `${formatRanges(head)}, и ещё ${rest}` : formatRanges(head)
}

export interface OwnAsk {
  title: string
  text: string
  note: string // серая строка под вопросом; пусто — нет
  keep: string
  all: string
}

// «У 3 разворотов / У 21 разворота» (родительный): формы для 1, 2, 5.
const GEN: Record<string, [string, string, string]> = {
  разворот: ['разворота', 'разворотов', 'разворотов'],
  лист: ['листа', 'листов', 'листов'],
}
// «на 1 развороте / на 3 разворотах» (предложный).
const LOC: Record<string, [string, string, string]> = {
  разворот: ['развороте', 'разворотах', 'разворотах'],
  лист: ['листе', 'листах', 'листах'],
}

const cap = (w: string) => w[0].toUpperCase() + w.slice(1)

export function ownAskText(kind: OwnKind, own: OwnBg, unit: Unit): OwnAsk {
  const n = own.nums.length
  const who = `У ${n} ${plural(n, GEN[unit.one])}`
  const them = n === 1 ? 'его' : 'их'
  const list = ownNumsText(own.nums)
  if (kind === 'color') {
    return {
      title: `${cap(unit.many)} со своим цветом`,
      text: `${who} свой цвет: ${list}. Новый цвет альбома ${them} не коснётся. Сделать ${them} как в альбоме?`,
      note: '',
      keep: OWN_KEEP,
      all: OWN_ALL,
    }
  }
  return {
    title: `${cap(unit.many)} со своим фоном`,
    text: `${who} свой фон: ${list}. Фон альбома на ${n === 1 ? 'нём' : 'них'} не виден. Сделать ${them} как в альбоме?`,
    note: own.photos > 0 ? `Фото фона с ${n === 1 ? 'него' : 'них'} уйдут: ${own.photos}` : '',
    keep: OWN_KEEP,
    all: OWN_ALL,
  }
}

// Тост после «Как в альбоме»: «Цвет альбома — теперь и на 3 разворотах».
export function ownDoneText(kind: OwnKind, n: number, unit: Unit): string {
  return `${kind === 'color' ? 'Цвет' : 'Фон'} альбома — теперь и на ${n} ${plural(n, LOC[unit.one])}`
}
