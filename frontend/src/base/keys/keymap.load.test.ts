import { afterEach, describe, expect, it, vi } from 'vitest'
import parity from '../../../../testdata/parity/keymap.json'
import type { KeymapView } from '../types/keys'

// Загрузка из State.keymap: store.init → хук keymapLoaded → keymap (подключает wire()).
let state: unknown = {}
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  GetState: () => Promise.resolve(state),
  MissingNames: () => Promise.resolve({}),
}))
vi.stubGlobal('window', globalThis)

const { store } = await import('../core/store.svelte')
const { wire } = await import('../../shell/boot/wire')
const { keymap } = await import('./keymap.svelte')
const { EMPTY_KEYMAP } = await import('./keyindex')

const view = parity.view as unknown as KeymapView
const base = { view: { spreads: [], folders: [] }, photos: [], missing: [] }

afterEach(() => {
  keymap.view = EMPTY_KEYMAP
})

describe('State.keymap', () => {
  it('wire подключает загрузку: вид попадает в реестр', async () => {
    wire()
    state = { ...base, keymap: view }
    await store.init()
    expect(keymap.view.commands.length).toBe(view.commands.length)
    expect(keymap.keys('undo')).toEqual(['Mod+KeyZ'])
  })

  it('Go без поля (до G2) или с null — реестр пуст, запуск не падает', async () => {
    wire()
    for (const keymapField of [undefined, null]) {
      state = { ...base, keymap: keymapField }
      await store.init()
      expect(keymap.view.commands.length).toBe(0)
      expect(store.loaded).toBe(true)
    }
  })
})
