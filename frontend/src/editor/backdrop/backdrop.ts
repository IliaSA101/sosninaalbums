// Фото на фоне (спека 009, design-backdrop): чистые решения для карточки, меню и склейки.
import { backdropLost } from '../../base/core/undo'
import type { BackdropScope, BackdropView, SpreadView } from '../../base/core/types'

// Решение живёт в основе (store.merge); здесь — для остальных.
export { backdropLost }

export const SCOPE_LABEL: Record<BackdropScope, string> = {
  spread: 'Весь разворот',
  left: 'Левая страница',
  right: 'Правая страница',
}

export const PAGE_SCOPES: readonly BackdropScope[] = ['left', 'right']

export const backdropOf = (s: SpreadView | undefined, scope: BackdropScope): BackdropView | undefined =>
  s?.backdrops?.find((b) => b.scope === scope)

// Какие зоны броска показать в карточке: если фон уже стоит по страницам или цвета разведены по
// страницам (split) — две зоны, иначе одна на весь разворот. Go всё равно снимает фоны другой формы.
export function zoneScopes(backdrops: readonly BackdropView[] | null | undefined, split: boolean): readonly BackdropScope[] {
  const pages = !!backdrops?.some((b) => b.scope !== 'spread')
  return pages || split ? PAGE_SCOPES : ['spread']
}
