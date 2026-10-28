// Порядок статей в разделах справочника F1 (спека 136, design §2): данные, а не склейка файлов articles-*.ts.
// Каждая статья — ровно один раз в списке своего раздела (страж articles.guard.test.ts). Порядок «по ходу работы»:
// создать или открыть → основная работа раздела → тонкая настройка → обслуживание; родственные статьи (looks-*,
// vig-*, cover-*) стоят подряд.
import { GUIDE_GROUPS, type GroupId } from './articles'

export const GUIDE_ORDER: Record<GroupId, readonly string[]> = {
  start: [
    'new-album',
    'open',
    'save',
    'export',
    'fsfera',
    'print-px',
    'print-profile',
    'settings',
    'fold',
    'crop',
    'undo',
    'levels',
    'start-window',
    'first-steps',
    'guide',
    'backups',
    'collect',
    'data-folder',
    'library-packs',
    'updates',
  ],
  photos: [
    'folders',
    'loose-photos',
    'folder-filter',
    'relink',
    'marks',
    'filter',
    'bins',
    'name-search',
    'thumb-size',
    'photo-preview',
    'compare',
    'photo-about',
    'photo-uses',
    'color-profile',
    'dpi',
    'add-photos',
    'drop-places',
    'fill-empty',
  ],
  spreads: [
    'autobuild',
    'autobuild-redo',
    'similar',
    'auto-gray',
    'new-spread',
    'insert-spread',
    'variants',
    'layout-gallery',
    'variety',
    'spread-order',
    'timeline',
    'spread-names',
    'all-spreads',
    'spread-nav',
    'delete-spreads',
    'big',
    'looks',
    'looks-built',
    'looks-step',
    'looks-format',
    'looks-draft',
    'looks-save',
  ],
  onspread: [
    'mini-panel',
    'group-select',
    'pick-under',
    'clipboard',
    'frame',
    'fit',
    'shape',
    'flip',
    'tilt',
    'straighten',
    'adjust',
    'photo-fade',
    'photo-menu',
    'replace-photo',
    'bg-color',
    'my-colors',
    'bg-photo',
    'decor',
    'border',
    'spread-tpl',
    'copy-decor',
  ],
  text: ['text', 'text-style', 'own-fonts', 'text-rotate', 'captions', 'caption-fields', 'person', 'names', 'vig-fields'],
  cover: [
    'cover',
    'cover-settings',
    'cover-spine',
    'cover-markup',
    'cover-export',
    'vignette',
    'vig-settings',
    'vig-templates',
    'vig-own',
    'vig-sort',
    'vig-faces',
    'vig-silhouette',
    'vig-sheet',
  ],
  view: ['details', 'panels', 'zoom', 'pan', 'rulers', 'safe', 'hotkeys'],
  manual: [
    'manual',
    'place-add',
    'place-fields',
    'split',
    'rotate',
    'align',
    'same-size',
    'group-xf',
    'layers',
    'snap',
    'templates',
    'copy-grid',
    'mirror',
  ],
}

// Статьи раздела по GUIDE_ORDER; не названные в нём (новые, ещё не расставленные) — в конце, в порядке массива.
export function inOrder<T extends { id: string }>(items: readonly T[], group: GroupId): T[] {
  const rank = new Map(GUIDE_ORDER[group].map((id, i) => [id, i]))
  const at = (a: T) => rank.get(a.id) ?? Infinity
  return [...items].sort((x, y) => (at(x) === at(y) ? 0 : at(x) < at(y) ? -1 : 1))
}

// Место статьи во всём справочнике: разделы по GUIDE_GROUPS, внутри — по GUIDE_ORDER; нет в списке — в самый конец.
// Нужно поиску: при равном весе выше та, что раньше в окне.
let ranks: Map<string, number> | null = null
export function orderRank(id: string): number {
  if (!ranks) {
    ranks = new Map()
    for (const g of GUIDE_GROUPS) for (const x of GUIDE_ORDER[g.id]) ranks.set(x, ranks.size)
  }
  return ranks.get(id) ?? Infinity
}
