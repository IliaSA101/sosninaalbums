// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { ALL_LEVELS, can } from '../../base/prefs/levels'
import { buildMenu, keyId, type MenuCtx, type MenuItem } from './menu'

// «Файл ▸ ИИ-агенты (MCP)…» (143, У1): пункт на всех уровнях, команда открывает окно, клавиши и каталога Go у неё нет.
const bridgeInfo = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { COMMANDS } = await import('./commands')
const { agents } = await import('../agents/agents.svelte')
const { bridge, emptyInfo } = await import('../agents/agents')

beforeEach(() => {
  useCatalog()
  agents.close()
  vi.spyOn(bridge, 'info').mockImplementation(async () => {
    bridgeInfo()
    return emptyInfo()
  })
})
afterEach(() => {
  dropCatalog()
  agents.close()
  vi.restoreAllMocks()
})

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

describe('«Файл ▸ ИИ-агенты (MCP)…» (143)', () => {
  it('пункт после «Открыть папку данных», подпись с многоточием, команда agents, без значка', () => {
    const items = file({})
    const ids = items.map((i) => i.id)
    const it = items[ids.indexOf('agents')]
    expect(ids.indexOf('agents')).toBeGreaterThan(ids.indexOf('showData'))
    expect(it).toMatchObject({ label: 'ИИ-агенты (MCP)…', cmd: 'agents' })
    expect(it.icon).toBeUndefined()
    expect(it.disabled).toBeFalsy()
  })

  it('виден на всех уровнях и с assocCan (Windows-выпуск)', () => {
    for (const level of ALL_LEVELS)
      for (const assocCan of [false, true]) {
        const items = file({ level, can: (f) => can(f, level), assocCan })
        expect(
          items.map((i) => i.id),
          `${level} assoc=${assocCan}`,
        ).toContain('agents')
      }
  })

  it('команда открывает окно и читает состояние Go', async () => {
    expect(agents.open).toBe(false)
    await COMMANDS.agents()
    expect(agents.open).toBe(true)
    expect(bridgeInfo).toHaveBeenCalled()
  })

  // Временно, до Г5: команды нет в internal/keymap/catalog.go, см. design §12 (Г5 убирает исключение и этот тест).
  it('своей клавиши нет: keyId пуст, значит каталог Go команду не требует', () => {
    expect(keyId('agents')).toBeUndefined()
  })
})
