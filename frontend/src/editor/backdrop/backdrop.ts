// Фото на фоне (спека 009, design-backdrop): чистые решения для карточки, меню и склейки.
import { backdropLost } from '../../base/core/undo'
import { photoCss } from '../../base/geom/frame'
import type { BackdropScope, BackdropView, SpreadView } from '../../base/core/types'

// Решение живёт в основе (store.merge); здесь — для остальных.
export { backdropLost }

export const SCOPE_LABEL: Record<BackdropScope, string> = {
  spread: 'Весь разворот',
  left: 'Левая страница',
  right: 'Правая страница',
}

// Области фона, доступные в режиме: у листа виньетки страниц нет — только весь лист.
export const scopesFor = (sheet: boolean): readonly BackdropScope[] => (sheet ? ['spread'] : ['spread', 'left', 'right'])
export const scopeLabel = (scope: BackdropScope, sheet: boolean): string => (sheet && scope === 'spread' ? 'Весь лист' : SCOPE_LABEL[scope])

export const PAGE_SCOPES: readonly BackdropScope[] = ['left', 'right']

export const backdropOf = (s: SpreadView | undefined, scope: BackdropScope): BackdropView | undefined =>
  s?.backdrops?.find((b) => b.scope === scope)

// Какие зоны броска показать в карточке: если фон уже стоит по страницам или цвета разведены по
// страницам (split) — две зоны, иначе одна на весь разворот. Go всё равно снимает фоны другой формы.
export function zoneScopes(backdrops: readonly BackdropView[] | null | undefined, split: boolean): readonly BackdropScope[] {
  const pages = !!backdrops?.some((b) => b.scope !== 'spread')
  return pages || split ? PAGE_SCOPES : ['spread']
}

// transform картинки фона (045): сначала зеркало, потом наклон frame.r — как в экспорте. Нет ни того ни другого — undefined.
export const backdropCss = (b: BackdropView): string | undefined => photoCss({ r: b.frame.r, flipH: b.flipH, flipV: b.flipV })
