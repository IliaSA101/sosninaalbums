// Слова подписей и имён по словарю 035: в альбоме — «разворот», на обложке — «обложка», «лист» — только
// у изделия «Виньетка — отдельный лист». Чистая логика, без DOM.
import { isCover, unitOf } from '../../base/geom/sheet'
import type { Settings, SpreadView } from '../../base/core/types'

export interface UnitWords {
  acc: string // «на разворот»: Один стиль на …
  gen: string // «Подписи разворота»
  genThis: string // «этого разворота»
  on: string // «на развороте»
  onCap: string // «На развороте» — начало фразы
}

const SPREAD: UnitWords = { acc: 'разворот', gen: 'разворота', genThis: 'этого разворота', on: 'на развороте', onCap: 'На развороте' }
const SHEET: UnitWords = { acc: 'лист', gen: 'листа', genThis: 'этого листа', on: 'на листе', onCap: 'На листе' }
const COVER: UnitWords = { acc: 'обложку', gen: 'обложки', genThis: 'этой обложки', on: 'на обложке', onCap: 'На обложке' }

export function unitWords(st: Pick<Settings, 'sheet'> | undefined, sp?: Pick<SpreadView, 'id'> | null): UnitWords {
  if (isCover(sp)) return COVER
  return unitOf(st).one === 'лист' ? SHEET : SPREAD
}
