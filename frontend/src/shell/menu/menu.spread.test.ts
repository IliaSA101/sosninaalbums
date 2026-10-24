import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { can, type Level } from '../../base/prefs/levels'
import { buildMenu, type Menu, type MenuCtx, type MenuItem } from './menu'

// Меню «Разворот» (владелец 2026-10-24, 0.26-124-1): 11 пунктов, четыре группы; «Правка» — 14. Тесты порядка
// пунктов 041, 048, 049, 067, 079, 094 переехали сюда из menu.test.ts вместе с пунктами.
beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

const base: MenuCtx = {
  spreads: 3,
  hasSpread: true,
  canUndo: true,
  canRedo: true,
  zone: 'canvas',
  libraryPhotos: 10,
  anySelection: true,
  deletable: true,
  flippable: true,
  varyable: true,
  layerable: true,
  missing: 2,
  big: false,
  details: false,
  safe: false,
  rulers: true,
  snap: true,
}
const ctx = (o: Partial<MenuCtx> = {}): MenuCtx => ({ ...base, ...o })
const at = (level: Level) => ctx({ level, can: (f) => can(f, level) })

const flat = (items: MenuItem[]): MenuItem[] => items.flatMap((i) => [i, ...(i.sub ? flat(i.sub) : [])])
const find = (menus: Menu[], id: string): MenuItem | undefined => menus.flatMap((m) => flat(m.items)).find((i) => i.id === id)
// Меню нет — ошибка: иначе проверка «пункта нет в меню» прошла бы и для несуществующего (0.26-124-20).
const ids = (menus: Menu[], menu: string) => {
  const m = menus.find((x) => x.id === menu)
  if (!m) throw new Error(`меню «${menu}» нет в строке`)
  return flat(m.items).map((i) => i.id)
}
const top = (menus: Menu[], menu: string) => menus.find((m) => m.id === menu)!.items
// Группы по чертам: [[id, id], [id…]]
const groups = (items: MenuItem[]): string[][] =>
  items.reduce<string[][]>(
    (out, it) => (it.sep || out.length === 0 ? [...out, [it.id]] : [...out.slice(0, -1), [...out[out.length - 1], it.id]]),
    [],
  )

const SPREAD = [
  'addText',
  'addSticker',
  'bgGallery',
  'borderGallery',
  'copyDecor',
  'spreadTpl',
  'renameSpread',
  'pasteNames',
  'varyLayouts',
  'makeVignette',
  'vigSetup',
]
const EDIT = [
  'undo',
  'redo',
  'cutItems',
  'copyItems',
  'pasteItems',
  'duplicateItems',
  'selectAll',
  'clearSelection',
  'removeCell',
  'layer',
  'turn',
  'flip',
  'gray',
  'align',
]

describe('«Правка» 14 и «Разворот» 11 (владелец 2026-10-24)', () => {
  const m = buildMenu(ctx({ level: 'full', can: () => true }))

  it('«Правка» — 14 пунктов верхнего уровня, группы с чертами по макету', () => {
    expect(top(m, 'edit').map((i) => i.id)).toEqual(EDIT)
    expect(groups(top(m, 'edit'))).toEqual([
      ['undo', 'redo'],
      ['cutItems', 'copyItems', 'pasteItems', 'duplicateItems'],
      ['selectAll', 'clearSelection', 'removeCell'],
      ['layer', 'turn', 'flip', 'gray', 'align'],
    ])
  })

  it('«Разворот» — 11 пунктов, группы с чертами по макету', () => {
    expect(top(m, 'spread').map((i) => i.id)).toEqual(SPREAD)
    expect(groups(top(m, 'spread'))).toEqual([
      ['addText', 'addSticker'],
      ['bgGallery', 'borderGallery', 'copyDecor', 'spreadTpl'],
      ['renameSpread', 'pasteNames', 'varyLayouts'],
      ['makeVignette', 'vigSetup'],
    ])
  })

  it('каждый пункт ровно в одном меню: 11 — в «Разворот», 14 — в «Правка»', () => {
    const where = (id: string) => m.filter((x) => top([x], x.id).some((i) => i.id === id)).map((x) => x.id)
    for (const id of SPREAD) expect(where(id), id).toEqual(['spread'])
    for (const id of EDIT) expect(where(id), id).toEqual(['edit'])
    const all = m.flatMap((x) => flat(x.items).map((i) => i.id))
    expect(new Set(all).size, 'id пунктов уникальны во всех меню').toBe(all.length)
  })

  it('название «Разворот» всегда — и в проекте «виньетка-лист» (0.26-124-1)', () => {
    for (const o of [
      {},
      { spreads: 1 },
      { hasSpread: false },
      { level: 'simple' as const, can: (f: string) => can(f as never, 'simple') },
    ]) {
      expect(buildMenu(ctx(o)).find((x) => x.id === 'spread')?.label).toBe('Разворот')
    }
  })

  it('пустое меню скрыто: нет видимых на уровне пунктов — «Разворота» в строке нет', () => {
    const none = buildMenu(ctx({ can: () => false }))
    expect(none.map((x) => x.id)).toEqual(['file', 'edit', 'view', 'help'])
    expect(() => ids(none, 'spread')).toThrow('нет в строке')
  })

  it('«Простой»: «Разворот» в строке, пунктов уровня меньше; id как у «Полного»', () => {
    const simple = buildMenu(at('simple'))
    expect(simple.map((x) => x.label)).toEqual(['Файл', 'Правка', 'Разворот', 'Вид', 'Справка'])
    expect(ids(simple, 'spread').length).toBeLessThan(SPREAD.length)
    for (const id of ids(simple, 'spread')) expect(SPREAD, id).toContain(id)
  })

  it('первый пункт «Разворота» не начинается с черты', () => {
    for (const l of ['simple', 'standard', 'full'] as const) expect(top(buildMenu(at(l)), 'spread')[0].sep, l).toBeUndefined()
  })
})

describe('пункты «Разворота»: уровень, серое, порядок', () => {
  it('«Разворот ▸ Сделать разворот виньеткой» — со «Стандартного», серое без обычного разворота (отзыв 14.10)', () => {
    expect(ids(buildMenu(at('simple')), 'spread')).not.toContain('makeVignette')
    const std = buildMenu({ ...at('standard'), vigable: true })
    expect(find(std, 'makeVignette')).toMatchObject({ label: 'Сделать разворот виньеткой', cmd: 'makeVignette' })
    expect(find(std, 'makeVignette')?.disabled).toBeUndefined()
    expect(find(buildMenu(at('standard')), 'makeVignette')?.disabled).toBe(true)
    expect(find(buildMenu({ ...at('full'), vigable: false }), 'makeVignette')?.disabled).toBe(true)
  })

  it('Разворот ▸ Название разворота…: F2, серое без разворотов и на обложке', () => {
    expect(ids(buildMenu(at('simple')), 'spread')).not.toContain('renameSpread') // «Названия» — со «Стандартного»
    const std = buildMenu({ ...at('standard'), decorable: true })
    expect(find(std, 'renameSpread')).toMatchObject({ label: 'Название разворота…', cmd: 'renameSpread', keys: 'F2' })
    expect(find(std, 'renameSpread')?.disabled).toBeUndefined()
    // Первым в третьей группе: после «Готовых разворотов…», перед «Имена из списка…».
    const edit = ids(std, 'spread')
    expect(edit.indexOf('renameSpread')).toBe(edit.indexOf('spreadTpl') + 1)
    // decorable не задан — обложка или нет листа.
    expect(find(buildMenu(at('standard')), 'renameSpread')?.disabled).toBe(true)
    expect(find(buildMenu({ ...at('full'), decorable: false, spreads: 0, hasSpread: false }), 'renameSpread')?.disabled).toBe(true)
  })

  it('«Разворот ▸ Разнообразить раскладки» (041) — с уровня «Простой», серое, когда менять нечего', () => {
    expect(find(buildMenu(at('simple')), 'varyLayouts')).toMatchObject({ label: 'Разнообразить раскладки', cmd: 'varyLayouts' })
    expect(find(buildMenu(at('full')), 'varyLayouts')?.disabled).toBeUndefined()
    expect(find(buildMenu({ ...at('full'), varyable: false }), 'varyLayouts')?.disabled).toBe(true)
    expect(ids(buildMenu(at('full')), 'spread')).toContain('varyLayouts')
  })

  it('«Разворот ▸ Настроить виньетку…» — со «Стандартного», серое, пока лист не виньетка', () => {
    expect(ids(buildMenu(at('simple')), 'spread')).not.toContain('vigSetup')
    const on = buildMenu({ ...at('standard'), vigSetup: true })
    expect(find(on, 'vigSetup')).toMatchObject({ label: 'Настроить виньетку…', cmd: 'vigSetup' })
    expect(find(on, 'vigSetup')?.disabled).toBeUndefined()
    expect(find(buildMenu(at('standard')), 'vigSetup')?.disabled).toBe(true)
  })

  it('«Разворот ▸ Галерея фонов…» (048) — после «Добавить стикер…», со «Стандартного», серое без листа', () => {
    expect(ids(buildMenu(at('simple')), 'spread')).not.toContain('bgGallery')
    const std = buildMenu(at('standard'))
    expect(find(std, 'bgGallery')).toMatchObject({ label: 'Галерея фонов…', cmd: 'bgGallery' })
    expect(find(std, 'bgGallery')?.disabled).toBeUndefined()
    const edit = ids(std, 'spread')
    expect(edit.indexOf('bgGallery')).toBe(edit.indexOf('addSticker') + 1)
    expect(find(buildMenu({ ...at('full'), hasSpread: false }), 'bgGallery')?.disabled).toBe(true)
  })

  it('«Разворот ▸ Рамочки…» (079) — сразу после «Галереи фонов…», со «Стандартного», серое без листа, клавиши по умолчанию нет', () => {
    expect(ids(buildMenu(at('simple')), 'spread')).not.toContain('borderGallery')
    const std = buildMenu(at('standard'))
    expect(find(std, 'borderGallery')).toMatchObject({ label: 'Рамочки…', cmd: 'borderGallery' })
    expect(find(std, 'borderGallery')?.disabled).toBeUndefined()
    expect(find(std, 'borderGallery')?.keys).toBeFalsy()
    const edit = ids(std, 'spread')
    expect(edit.indexOf('borderGallery')).toBe(edit.indexOf('bgGallery') + 1)
    expect(find(buildMenu({ ...at('full'), hasSpread: false }), 'borderGallery')?.disabled).toBe(true)
    // «Свой» без функции «Фоны, стикеры и рамочки»: пункта нет
    expect(ids(buildMenu({ ...at('full'), can: (f) => f !== 'border' }), 'spread')).not.toContain('borderGallery')
  })

  it('«Разворот ▸ Копировать оформление…» (049 Ф2) — после «Галереи фонов…», со «Стандартного», серое на обложке и без листа', () => {
    expect(ids(buildMenu(at('simple')), 'spread')).not.toContain('copyDecor')
    const std = buildMenu({ ...at('standard'), decorable: true })
    expect(find(std, 'copyDecor')).toMatchObject({ label: 'Копировать оформление…', cmd: 'copyDecor' })
    expect(find(std, 'copyDecor')?.disabled).toBeUndefined()
    const edit = ids(std, 'spread')
    expect(edit.indexOf('copyDecor')).toBe(edit.indexOf('borderGallery') + 1)
    expect(find(buildMenu(at('standard')), 'copyDecor')?.disabled).toBe(true) // обложка / нет листа: decorable не задан
    expect(find(buildMenu({ ...at('full'), decorable: false }), 'copyDecor')?.disabled).toBe(true)
  })

  it('«Разворот ▸ Готовые развороты…» (067) — сразу после «Копировать оформление…», со «Стандартного», серое на обложке и без листа', () => {
    expect(ids(buildMenu(at('simple')), 'spread')).not.toContain('spreadTpl')
    const std = buildMenu({ ...at('standard'), decorable: true })
    expect(find(std, 'spreadTpl')).toMatchObject({ label: 'Готовые развороты…', cmd: 'spreadTpl' })
    expect(find(std, 'spreadTpl')?.disabled).toBeUndefined()
    const edit = ids(std, 'spread')
    expect(edit.indexOf('spreadTpl')).toBe(edit.indexOf('copyDecor') + 1)
    expect(find(buildMenu(at('standard')), 'spreadTpl')?.disabled).toBe(true) // обложка / нет листа: decorable не задан
    expect(find(buildMenu({ ...at('full'), decorable: false }), 'spreadTpl')?.disabled).toBe(true)
  })

  it('«Разворот ▸ Добавить стикер…» (048 Ф2) — сразу после «Добавить текст», со «Стандартного», серое без листа', () => {
    expect(ids(buildMenu(at('simple')), 'spread')).not.toContain('addSticker')
    const std = buildMenu(at('standard'))
    expect(find(std, 'addSticker')).toMatchObject({ label: 'Добавить стикер…', cmd: 'addSticker' })
    expect(find(std, 'addSticker')?.disabled).toBeUndefined()
    const edit = ids(std, 'spread')
    expect(edit.indexOf('addSticker')).toBe(edit.indexOf('addText') + 1)
    expect(find(buildMenu({ ...at('full'), hasSpread: false }), 'addSticker')?.disabled).toBe(true)
  })
})
