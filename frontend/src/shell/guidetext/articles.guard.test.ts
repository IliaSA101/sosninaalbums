import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { menuPath } from '../guide/show'
import { ARTICLES, GUIDE_GROUPS, type Article } from './articles'
import { articleText } from './articletext'
import { GUIDE_ORDER } from './order'
import { RENAMED } from './renamed'

// Страж текстов справочника (спека 136, design §6): порядок, «Где» без дублей, абзацы, «См. также», замороженные id.
beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

// id статей на 0.29: с этой версии они ключи отметок «изучено» (0.30). Переименовывать нельзя; убирать — только с записью в decisions.md.
// prettier-ignore
const FROZEN_IDS = [
  'add-photos', 'adjust', 'align', 'all-spreads', 'autobuild', 'autobuild-redo', 'auto-gray', 'backups', 'bg-color', 'bg-photo',
  'big', 'bins', 'border', 'caption-fields', 'captions', 'clipboard', 'collect', 'color-profile', 'copy-decor', 'copy-grid',
  'cover', 'cover-export', 'cover-markup', 'cover-settings', 'cover-spine', 'crop', 'data-folder', 'decor', 'delete-spreads',
  'details', 'dpi', 'drop-places', 'export', 'fill-empty', 'filter', 'first-steps', 'fit', 'flip', 'fold', 'folder-filter',
  'folders', 'frame', 'fsfera', 'group-select', 'group-xf', 'hotkeys', 'insert-spread', 'layers', 'layout-gallery', 'levels',
  'library-packs', 'looks', 'looks-built', 'looks-draft', 'looks-format', 'looks-save', 'looks-step', 'loose-photos', 'manual',
  'marks', 'mini-panel', 'mirror', 'names', 'name-search', 'new-album', 'new-spread', 'open', 'own-fonts', 'pan', 'panels',
  'person', 'photo-about', 'photo-fade', 'photo-menu', 'photo-preview', 'pick-under', 'place-add', 'place-fields', 'print-px',
  'relink', 'rotate', 'rulers', 'safe', 'save', 'settings', 'shape', 'similar', 'snap', 'split', 'spread-names', 'spread-nav',
  'spread-order', 'spread-tpl', 'start-window', 'straighten', 'templates', 'text', 'text-rotate', 'text-style', 'thumb-size',
  'tilt', 'timeline', 'undo', 'updates', 'variants', 'variety', 'vig-faces', 'vig-fields', 'vignette', 'vig-own', 'vig-settings',
  'vig-sheet', 'vig-silhouette', 'vig-sort', 'vig-templates', 'zoom', 'compare',
  'guide', 'print-profile', 'same-size', 'photo-uses', 'replace-photo', 'my-colors',
]

// Замороженные id, которых нет среди статей и которые не объяснены записью в RENAMED (переименование законно).
const lostIds = (frozen: readonly string[], have: ReadonlySet<string>, renamed: Record<string, string>): string[] =>
  frozen.filter((id) => !have.has(id) && !(id in renamed))

const MAX_PARA = 500
const MAX_TOTAL = 1600
const MAX_SEE = 4

// Клавиша, написанная руками («; I.», «, F2»): её место — {key:…} или поле keys.
const BARE_KEY = /(?:^|[;,]\s*)(?:[A-Z]|F\d{1,2})(?=[.;,]|$)/m
const noKeys = (s: string): string => s.replace(/\{key:[A-Za-z0-9]+(?:\|[^}]*)?\}/g, '')
const flat = (s: string): string => s.replace(/\s*▸\s*/g, '▸')
// Текст как его видит пользователь с самым длинным набором функций: все куски {can} на месте.
const shown = (s: string | undefined): string => articleText(s ?? '', () => true)

// «Где» и «Как» повторяют путь из строки «Меню»: карточка уже печатает его из menu.
function menuDupes(a: Article): string[] {
  const paths = (a.menu ?? []).map((m) => menuPath(m)).filter((p): p is string => !!p)
  return (['where', 'how'] as const).flatMap((f) => {
    const t = flat(shown(a[f]))
    return paths.filter((p) => t.includes(flat(p))).map((p) => `${a.id}: «${f === 'where' ? 'Где' : 'Как'}» повторяет «Меню» (${p})`)
  })
}

// Голые клавиши, ссылки «(статья «…»)», пустые и длинные абзацы, длина статьи.
function textIssues(a: Article): string[] {
  const bad: string[] = []
  for (const f of ['what', 'where', 'how'] as const) {
    const raw = a[f]
    if (!raw) continue
    if (BARE_KEY.test(noKeys(raw))) bad.push(`${a.id}.${f}: голая клавиша`)
    if (/стать[яеюи] «/.test(raw)) bad.push(`${a.id}.${f}: «(статья «…»)» вместо see`)
    for (const p of shown(raw).split('\n')) {
      if (!p.trim()) bad.push(`${a.id}.${f}: пустой абзац`)
      else if (p.length > MAX_PARA) bad.push(`${a.id}.${f}: абзац ${p.length} знаков (> ${MAX_PARA})`)
    }
  }
  const total = shown(a.what).length + shown(a.where).length + shown(a.how).length
  if (total > MAX_TOTAL) bad.push(`${a.id}: статья ${total} знаков (> ${MAX_TOTAL})`)
  return bad
}

// «См. также»: существующие id, не своя, без повторов, не больше MAX_SEE.
function seeIssues(a: Article, ids: ReadonlySet<string>): string[] {
  const see = a.see ?? []
  const bad: string[] = []
  if (see.length > MAX_SEE) bad.push(`${a.id}: see длиннее ${MAX_SEE}`)
  if (new Set(see).size !== see.length) bad.push(`${a.id}: see с повтором`)
  for (const id of see) {
    if (id === a.id) bad.push(`${a.id}: see указывает на себя`)
    else if (!ids.has(id)) bad.push(`${a.id}: see → нет статьи «${id}»`)
  }
  return bad
}

// Каждая статья ровно один раз в списке своего раздела; лишних id в списках нет.
function orderIssues(list: readonly Article[], order: Record<string, readonly string[]>): string[] {
  const bad: string[] = []
  for (const g of GUIDE_GROUPS) {
    const inGroup = list.filter((a) => a.group === g.id).map((a) => a.id)
    const listed = order[g.id] ?? []
    for (const id of inGroup) {
      const n = listed.filter((x) => x === id).length
      if (n !== 1) bad.push(`${id}: в порядке раздела «${g.id}» ${n} раз`)
    }
    for (const id of listed) if (!inGroup.includes(id)) bad.push(`${id}: в порядке «${g.id}», но статьи нет в разделе`)
  }
  return bad
}

// Все нарушения списка статей; пусто — страж зелёный. Отдельная функция, чтобы проверить сам страж на подмене.
function violations(list: readonly Article[], order: Record<string, readonly string[]> = GUIDE_ORDER): string[] {
  const ids = new Set(list.map((a) => a.id))
  return [...list.flatMap((a) => [...menuDupes(a), ...textIssues(a), ...seeIssues(a, ids)]), ...orderIssues(list, order)]
}

describe('тексты справочника: страж', () => {
  it('нарушений нет', () => {
    expect(violations(ARTICLES)).toEqual([])
  })
  it('id прежние на месте, повторов нет', () => {
    const have = new Set(ARTICLES.map((a) => a.id))
    expect(have.size).toBe(ARTICLES.length)
    expect(lostIds(FROZEN_IDS, have, RENAMED), 'переименован или убран id без записи в RENAMED — это ключи отметок 0.30').toEqual([])
    expect(new Set(FROZEN_IDS).size).toBe(FROZEN_IDS.length)
  })
  it('самопроверка: id, ушедший с записью в RENAMED, не провал; без записи — провал', () => {
    const have = new Set(['a', 'b'])
    expect(lostIds(['a', 'b', 'c'], have, {})).toEqual(['c'])
    expect(lostIds(['a', 'b', 'c'], have, { c: 'a' })).toEqual([])
    expect(lostIds(['a', 'b', 'c'], have, { c: '' })).toEqual([])
  })
  it('«Автосборка» разделена: старый id цел, новая часть — в «Развороты»', () => {
    expect(ARTICLES.find((a) => a.id === 'autobuild')?.see).toContain('autobuild-redo')
    expect(ARTICLES.find((a) => a.id === 'autobuild-redo')?.group).toBe('spreads')
  })
  it('порядок в разделе «Начало» — по ходу работы: создать, открыть, сохранить, выгрузить', () => {
    expect(GUIDE_ORDER.start.slice(0, 4)).toEqual(['new-album', 'open', 'save', 'export'])
  })
})

describe('самопроверка стража: подмена роняет', () => {
  const base: Article = { ...ARTICLES.find((a) => a.id === 'export')!, menu: undefined, where: 'Кнопка.', how: undefined, see: undefined }
  const order = Object.fromEntries(GUIDE_GROUPS.map((g) => [g.id, g.id === base.group ? [base.id] : []]))
  const only = (patch: Partial<Article>): string => violations([{ ...base, ...patch }], order).join('; ')

  it('чистая подменная статья проходит', () => {
    expect(only({})).toBe('')
  })
  it('«Где» с путём меню', () => {
    expect(only({ menu: ['export'], where: 'Меню «Файл ▸ Экспорт…» рядом.' })).toContain('повторяет «Меню»')
    expect(only({ menu: ['export'], where: 'Меню «Файл▸Экспорт…».' })).toContain('повторяет «Меню»')
    expect(only({ menu: ['export'], where: 'Кнопка в шапке.' })).toBe('')
  })
  it('«Как» с путём меню (141): тоже дубль', () => {
    expect(only({ menu: ['export'], how: 'Нажмите; «Файл ▸ Экспорт…».' })).toBe('export: «Как» повторяет «Меню» (Файл ▸ Экспорт…)')
    expect(only({ menu: ['export'], how: 'Нажмите кнопку.' })).toBe('')
  })
  it('голая клавиша; клавиша в {key:} не считается', () => {
    expect(only({ where: 'Кнопка; I.' })).toContain('голая клавиша')
    expect(only({ where: 'Кнопка, F2' })).toContain('голая клавиша')
    expect(only({ where: 'Кнопка{key:undo| (%)}.' })).toBe('')
  })
  it('длинный абзац и длинная статья', () => {
    expect(only({ how: 'а'.repeat(MAX_PARA + 1) })).toContain('абзац')
    const part = 'б'.repeat(480)
    expect(only({ how: `${part}\n${part}\n${part}\n${part}` })).toContain('статья')
  })
  it('пустой абзац', () => {
    expect(only({ how: 'Раз.\n\nДва.' })).toContain('пустой абзац')
  })
  it('see: нет статьи, на себя, повтор, больше четырёх', () => {
    expect(only({ see: ['нет-такой'] })).toContain('нет статьи')
    expect(only({ see: [base.id] })).toContain('на себя')
    expect(only({ see: ['a', 'a'] })).toContain('повтор')
    expect(only({ see: ['a', 'b', 'c', 'd', 'e'] })).toContain('длиннее')
  })
  it('«(статья «…»)» вместо see', () => {
    expect(only({ how: 'Смотрите (статья «Ч/б развороты»).' })).toContain('статья «')
  })
  it('порядок: статьи нет в списке, дважды, лишняя', () => {
    const none = Object.fromEntries(GUIDE_GROUPS.map((g) => [g.id, [] as string[]]))
    expect(violations([base], none).join()).toContain('0 раз')
    expect(violations([base], { ...none, [base.group]: [base.id, base.id] }).join()).toContain('2 раз')
    expect(violations([base], { ...none, [base.group]: [base.id, 'призрак'] }).join()).toContain('статьи нет в разделе')
  })
})
