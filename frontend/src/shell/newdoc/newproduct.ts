// Новое изделие (спека 031): альбом или виньетка — отдельный лист. Чистая логика выбора размера.
import { SHEET_PRESETS } from '../../base/geom/sheet'

export type ProductKind = 'album' | 'sheet'

// Пределы стороны листа, мм: те же, что зажимает Go (model.ClampSettings), чтобы форма не обещала лишнего.
export const SIZE_MIN = 50
export const SIZE_MAX = 1000

// Выбор размера листа: номер пресета или «свой».
export type SizePick = number | 'custom'

export interface Dims {
  w: number
  h: number
}

// Размер из полей ввода (number или null у пустого поля) → мм или null, если не годится.
export function sheetDims(w: unknown, h: unknown): Dims | null {
  const ok = (v: unknown): v is number => typeof v === 'number' && Number.isFinite(v) && v >= SIZE_MIN && v <= SIZE_MAX
  return ok(w) && ok(h) ? { w, h } : null
}

// Размер выбранного пресета или своих полей.
export function pickedDims(pick: SizePick, w: unknown, h: unknown): Dims | null {
  if (pick === 'custom') return sheetDims(w, h)
  const p = SHEET_PRESETS[pick]
  return p ? { w: p.w, h: p.h } : null
}

// Подсказка под полями: что не так с размером.
export function sizeHint(pick: SizePick, w: unknown, h: unknown): string {
  if (pick !== 'custom' || sheetDims(w, h)) return ''
  return `Сторона листа — от ${SIZE_MIN} до ${SIZE_MAX} мм`
}
