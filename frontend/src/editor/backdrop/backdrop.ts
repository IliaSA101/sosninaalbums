// Фото на фоне (спека 009, design-backdrop): чистые решения для карточки, меню и склейки.
import { backdropLost } from '../../base/core/undo'
import { photoCss } from '../../base/geom/frame'
import { center, rotate, rotateAbout, type TextBox } from '../../base/geom/textbox'
import { ownBg } from '../../base/geom/sheet'
import type { BackdropScope, BackdropView, SpreadView } from '../../base/core/types'

// Решение живёт в основе (store.merge); здесь — для остальных.
export { backdropLost }

export const SCOPE_LABEL: Record<BackdropScope, string> = {
  spread: 'Весь разворот',
  left: 'Левая страница',
  right: 'Правая страница',
  spine: 'Корешок', // только обложка; подписи обложки — из CoverView.scopes
}

// Области фона, доступные в режиме: у листа виньетки страниц нет — только весь лист.
export const scopesFor = (sheet: boolean): readonly BackdropScope[] => (sheet ? ['spread'] : ['spread', 'left', 'right'])
export const scopeLabel = (scope: BackdropScope, sheet: boolean): string => (sheet && scope === 'spread' ? 'Весь лист' : SCOPE_LABEL[scope])

export const PAGE_SCOPES: readonly BackdropScope[] = ['left', 'right']

export const backdropOf = (s: SpreadView | undefined, scope: BackdropScope): BackdropView | undefined =>
  ownBg(s?.backdrops).find((b) => b.scope === scope)

// Какие зоны броска показать в карточке: если фон уже стоит по страницам или цвета разведены по
// страницам (split) — две зоны, иначе одна на весь разворот. Go всё равно снимает фоны другой формы.
export function zoneScopes(backdrops: readonly BackdropView[] | null | undefined, split: boolean): readonly BackdropScope[] {
  const pages = ownBg(backdrops).some((b) => b.scope !== 'spread')
  return pages || split ? PAGE_SCOPES : ['spread']
}

// transform картинки фона (045): сначала зеркало, потом наклон frame.r — как в экспорте. Нет ни того ни другого — undefined.
export const backdropCss = (b: BackdropView): string | undefined => photoCss({ r: b.frame.r, flipH: b.flipH, flipV: b.flipV })

// Рамка лежит вдоль вертикали (около ±90°): для корешка это «вдоль корешка».
const upright = (deg: number): boolean => Math.abs((((deg % 180) + 180) % 180) - 90) < 45

// Живой показ поворота рамки фона (протяжка ⟳). Одно правило для всех областей (070 Г): при смене ориентации
// (около 0/180° ↔ ±90°) w и h меняются местами вокруг центра, иначе — поворот на месте. Коммит считает Go
// (Api.TurnBackdrop, geom.TurnBox; паритет testdata/parity/bdturn.json) — здесь только показ.
export function turnBackdropBox(box: TextBox, deg: number): TextBox {
  if (upright(box.rot) === upright(deg)) return rotateAbout(box, deg)
  const c = center(box)
  const w = box.h
  const h = box.w
  const r = rotate(deg, w / 2, h / 2)
  return { ...box, w, h, x: c.x - r.x, y: c.y - r.y, rot: deg }
}
