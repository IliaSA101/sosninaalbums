// Пункты меню «Место ▾» (044, 076): порядок везде один — прямоугольное · скруглённое · круглое (как форма в виньетке).
import type { ShapeKey } from '../../base/geom/shape'
import type { IconName } from '../../base/ui/Icon.svelte'

export const PLACE_ADD_ITEMS: readonly { shape: ShapeKey; label: string; icon: IconName }[] = [
  { shape: 'rect', label: 'Прямоугольное', icon: 'place-add' },
  { shape: 'round', label: 'Скруглённое', icon: 'place-rounded' },
  { shape: 'oval', label: 'Круглое', icon: 'place-round' },
]
