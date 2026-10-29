// Откуда перенесены фото при добавлении из ленты (для тоста «Фото перенесено …»). Чистая логика: store зовёт её
// до правки (movedFrom) и после (movedText).
import { COVER_ID, unitOf } from '../geom/sheet'
import type { SpreadView, View } from './types'

// Фото, пришедшее на лист из другого листа (до правки).
export type Moved = { id: string; from: SpreadView }

// «До» правки: из каких листов (кроме into) пришли фото. Считается до вызова Go — после него источника уже нет.
// Источник ищется среди всех листов ряда, в том числе в обложке (032).
export function movedFrom(sheets: SpreadView[], ids: string[], into: string | null): Moved[] {
  const from = sheets.filter((s) => s.id !== into)
  return ids.flatMap((id) => {
    const src = from.find((s) => s.photos.includes(id))
    return src ? [{ id, from: src }] : []
  })
}

// «После» правки: текст тоста, откуда перенесены фото ('' — переносов не было). view — уже новый.
export function movedText(moved: Moved[], view: View): string {
  const unit = unitOf(view.settings)
  if (moved.length === 1) {
    const src = moved[0].from
    const where = src.id === COVER_ID ? 'с обложки' : `из ${unit.of} ${view.spreads.findIndex((s) => s.id === src.id) + 1}`
    return `Фото перенесено ${where}`
  }
  if (moved.length > 1) return `Перенесено фото из других ${unit.plural[2]}: ${moved.length}`
  return ''
}
