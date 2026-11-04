// Слова окна цвета и пипетки по словарю листа (035): в альбоме листов — «лист», в обычном — «разворот», у обложки —
// «обложка». Склонения здесь, потому что Unit их не хранит. Чистая логика, без DOM.
import { isCover, unitOf } from '../geom/sheet'
import type { Settings } from '../core/types'

export interface ColorWords {
  by: string // «по развороту»: Щёлкните …
  onCap: string // «На развороте» — начало фразы
}

export function colorWords(st: Pick<Settings, 'sheet'> | undefined, sheetId: string | undefined): ColorWords {
  if (isCover(sheetId ? { id: sheetId } : null)) return { by: 'по обложке', onCap: 'На обложке' }
  return unitOf(st).one === 'лист' ? { by: 'по листу', onCap: 'На листе' } : { by: 'по развороту', onCap: 'На развороте' }
}

// Подсказка режима пипетки: кнопка окна и полоса над холстом говорят одно и то же.
export const eyeHint = (w: ColorWords): string => `Щёлкните ${w.by} — возьмём цвет, каким он выйдет в печати`
