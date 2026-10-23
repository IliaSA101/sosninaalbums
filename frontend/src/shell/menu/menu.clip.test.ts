import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { can, type Custom, type FeatureId, type Level } from '../../base/prefs/levels'
import { features } from '../../base/prefs/levelcustom'
import { buildMenu, enabledCmds, type Menu, type MenuCtx, type MenuItem } from './menu'

// Пункты буфера в «Правке» (099). Остальное меню — menu.test.ts.
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

const flat = (items: MenuItem[]): MenuItem[] => items.flatMap((i) => [i, ...(i.sub ? flat(i.sub) : [])])
const find = (menus: Menu[], id: string): MenuItem | undefined => menus.flatMap((m) => flat(m.items)).find((i) => i.id === id)
const ids = (menus: Menu[], menu: string) => flat(menus.find((m) => m.id === menu)!.items).map((i) => i.id)

describe('копировать и вставить в «Правке» (099)', () => {
  const at = (level: Level, o: Partial<MenuCtx> = {}) => ctx({ level, can: (f) => can(f, level), ...o })
  const CLIP = ['cutItems', 'copyItems', 'pasteItems', 'duplicateItems']
  it('после «Повторить»: Вырезать · Копировать · Вставить · Дублировать, с клавишами из реестра', () => {
    const m = buildMenu(at('standard', { copyable: true, pasteable: true }))
    const edit = ids(m, 'edit')
    expect(edit.slice(0, 6)).toEqual(['undo', 'redo', ...CLIP])
    expect(CLIP.map((id) => find(m, id)?.label)).toEqual(['Вырезать', 'Копировать', 'Вставить', 'Дублировать'])
    expect(CLIP.map((id) => find(m, id)?.keys)).toEqual(['Ctrl+X', 'Ctrl+C', 'Ctrl+V', 'Ctrl+D'])
    expect(find(m, 'cutItems')?.sep).toBe(true) // отдельная группа
    expect(CLIP.every((id) => find(m, id)?.disabled === undefined)).toBe(true)
  })
  it('серые: нечего копировать — три пункта; буфер пуст — «Вставить»', () => {
    const none = buildMenu(at('standard', { pasteable: true }))
    expect(['cutItems', 'copyItems', 'duplicateItems'].map((id) => find(none, id)?.disabled)).toEqual([true, true, true])
    expect(find(none, 'pasteItems')?.disabled).toBeUndefined()
    const empty = buildMenu(at('standard', { copyable: true }))
    expect(find(empty, 'pasteItems')?.disabled).toBe(true)
    expect(find(empty, 'copyItems')?.disabled).toBeUndefined()
  })
  it('зона ленты фото: пункты серые (клавиши там молчат)', () => {
    const m = buildMenu(at('standard', { zone: 'library', copyable: true, pasteable: true }))
    for (const id of CLIP) expect(find(m, id)?.disabled, id).toBe(true)
  })
  it('«Простой» — пунктов нет; «Своё» без строки clipboard — тоже', () => {
    for (const id of CLIP) expect(ids(buildMenu(at('simple')), 'edit')).not.toContain(id)
    const own: Custom = { base: 'full', on: [], off: ['clipboard'] }
    const m = buildMenu(ctx({ can: (f: FeatureId) => features('custom', own).has(f) }))
    for (const id of CLIP) expect(ids(m, 'edit')).not.toContain(id)
  })
  it('enabledCmds: серый пункт не считается включённым', () => {
    const on = [...enabledCmds(buildMenu(at('standard', { copyable: true })))].filter((id) => CLIP.includes(id))
    expect(on.sort()).toEqual(['copyItems', 'cutItems', 'duplicateItems'])
  })
})
