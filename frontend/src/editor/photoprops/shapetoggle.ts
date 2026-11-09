// Строка «Форма» в карточке места (076): три значка ▭ ▢ ◯ — чистая часть под тест. Подписи — единственное число;
// глифы те же, что у фильтра формы в виньетке (`FORMS` в panels/vignette/vigform.ts), но editor/* на panels/* не смотрит.
import type { ShapeKey } from '../../base/geom/shape'

export const SHAPE_ROW = 'Форма'

export const SHAPE_BTNS: readonly { key: ShapeKey; glyph: string; title: string }[] = [
  { key: 'rect', glyph: '▭', title: 'Прямоугольник' },
  { key: 'round', glyph: '▢', title: 'Скруглённые углы' },
  { key: 'oval', glyph: '◯', title: 'Круг; при разных Ш и В — овал' },
]

// Подсвечен значок, только если так у всех выделенных (флаги — из placesel).
export const pressedOf = (k: ShapeKey, f: { allRect: boolean; allRounded: boolean; allOval: boolean }): boolean =>
  k === 'rect' ? f.allRect : k === 'round' ? f.allRounded : f.allOval
