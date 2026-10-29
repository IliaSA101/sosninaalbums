// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { ALL_LEVELS, can } from '../../base/prefs/levels'
import { buildMenu, enabledCmds, type MenuCtx, type MenuItem } from './menu'

// Go подменён: пункт «Файл» зовёт SetAssoc и берёт ответ как новое состояние.
const setAssoc = vi.hoisted(() => vi.fn(async (on: boolean) => ({ can: true, on, ask: false })))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SetAssoc: setAssoc }))

const { COMMANDS } = await import('./commands')
const { menuCtx } = await import('./menuctx')
const { store } = await import('../../base/core/store.svelte')
const { assocUi, ASSOC_OFF, ASSOC_ON } = await import('../dialogs/assoc.svelte')

beforeEach(() => {
  useCatalog()
  store.toast = null
  store.assoc = { can: true, on: false, ask: false }
  assocUi.reset()
  setAssoc.mockClear()
})
afterEach(() => dropCatalog())

const base: MenuCtx = {
  spreads: 0,
  hasSpread: false,
  canUndo: false,
  canRedo: false,
  zone: 'canvas',
  libraryPhotos: 0,
  anySelection: false,
  deletable: false,
  flippable: false,
  varyable: false,
  layerable: false,
  missing: 0,
  big: false,
  details: false,
  safe: false,
  rulers: false,
  snap: false,
}
const file = (c: Partial<MenuCtx>): MenuItem[] => buildMenu({ ...base, ...c }).find((m) => m.id === 'file')!.items
const ids = (items: MenuItem[]) => items.map((i) => i.id)

describe('«Файл ▸ Открывать .salbum этой программой» (112)', () => {
  it('assocCan нет (Mac, Linux, dev, стенд) — пункта нет', () => {
    expect(ids(file({}))).not.toContain('assoc')
    expect(ids(file({ assocCan: false, assocOn: true }))).not.toContain('assoc')
  })

  it('assocCan: пункт сразу после «Открыть папку данных», команда toggleAssoc, без значка', () => {
    const items = file({ assocCan: true })
    const i = ids(items).indexOf('showData')
    expect(items[i + 1]).toMatchObject({ id: 'assoc', label: 'Открывать .salbum этой программой', cmd: 'toggleAssoc' })
    expect(items[i + 1].icon).toBeUndefined()
    expect(items[i + 1].disabled).toBeFalsy()
  })

  it('галочка по assocOn: связь есть — checked true, нет — false (а не пункт-кнопка без отметки)', () => {
    expect(file({ assocCan: true, assocOn: true }).find((i) => i.id === 'assoc')?.checked).toBe(true)
    expect(file({ assocCan: true, assocOn: false }).find((i) => i.id === 'assoc')?.checked).toBe(false)
    expect(file({ assocCan: true }).find((i) => i.id === 'assoc')?.checked).toBe(false)
  })

  it('виден на всех уровнях (feature не задан)', () => {
    for (const level of ALL_LEVELS) {
      const items = file({ assocCan: true, level, can: (f) => can(f, level) })
      expect(ids(items), level).toContain('assoc')
    }
  })

  it('команда без клавиши учтена в enabledCmds (074-8)', () => {
    const menus = buildMenu({ ...base, assocCan: true })
    expect(enabledCmds(menus).has('toggleAssoc')).toBe(true)
  })

  it('menuCtx берёт can и on из store.assoc', () => {
    store.assoc = { can: true, on: true, ask: false }
    expect(menuCtx()).toMatchObject({ assocCan: true, assocOn: true })
    store.assoc = { can: false, on: false, ask: true }
    expect(menuCtx()).toMatchObject({ assocCan: false, assocOn: false })
  })
})

describe('команда toggleAssoc', () => {
  it('связи нет — включает (SetAssoc(true)), галочка встаёт, тост', async () => {
    await COMMANDS.toggleAssoc()
    expect(setAssoc).toHaveBeenCalledExactlyOnceWith(true)
    expect(store.assoc.on).toBe(true)
    expect(store.toast?.text).toBe(ASSOC_ON)
  })

  it('связь есть — снимает (SetAssoc(false)), повторный щелчок снова включает', async () => {
    store.assoc = { can: true, on: true, ask: false }
    await COMMANDS.toggleAssoc()
    expect(setAssoc).toHaveBeenLastCalledWith(false)
    expect(store.assoc.on).toBe(false)
    expect(store.toast?.text).toBe(ASSOC_OFF)
    await COMMANDS.toggleAssoc()
    expect(setAssoc).toHaveBeenLastCalledWith(true)
    expect(store.assoc.on).toBe(true)
  })

  it('assoc.can нет (клавиша без пункта) — Go не зовётся', async () => {
    store.assoc = { can: false, on: false, ask: false }
    await COMMANDS.toggleAssoc()
    expect(setAssoc).not.toHaveBeenCalled()
  })

  it('ошибка SetAssoc — тост, галочка не меняется', async () => {
    setAssoc.mockRejectedValueOnce(new Error('не удалось записать связь'))
    await COMMANDS.toggleAssoc()
    expect(store.assoc.on).toBe(false)
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Не удалось записать связь' })
  })
})
