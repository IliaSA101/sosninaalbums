// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Enter в окне «Автосборка» собирает и при фокусе на самом окне (после щелчка в пустое место), и в поле числа.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
const go = vi.hoisted(() => ({ build: vi.fn(async () => {}) }))
vi.mock('../autoalbum/autogo', () => ({ goBuild: go.build }))
const plan = {
  photos: 20,
  spreads: 5,
  minSpreads: 1,
  maxSpreads: 20,
  avg: 4,
  min: 1,
  max: 4,
  reuse: 0,
  from: 1,
  to: 4,
  cuts: 0,
}
vi.mock('./autoapi', () => ({
  planFn: () => async () => plan,
  previewFn: async () => ({ samples: [], spreads: [], total: 0 }),
  dropsFn: async () => [],
  sigsFn: async () => ({}),
  savedOf: () => ({ spreads: 5 }),
}))

const { default: AutoBuildDialog } = await import('./AutoBuildDialog.svelte')
const { store } = await import('../../base/core/store.svelte')

afterEach(() => {
  cleanup()
  store.autoDraft = null
  go.build.mockClear()
})

function open() {
  store.autoDraft = { ids: Array.from({ length: 20 }, (_, i) => `p${i}`), skipped: 0, from: 'selected', inAlbum: 0, filtered: false }
  render(AutoBuildDialog)
  flushSync()
}
const box = () => document.querySelector<HTMLElement>('.box')!
const key = (el: Element) => fireEvent.keyDown(el, { key: 'Enter' })

describe('AutoBuildDialog: Enter', () => {
  it('фокус в поле числа — собирает', async () => {
    open()
    const n = document.getElementById('auto-n')!
    n.focus()
    await key(n)
    expect(go.build).toHaveBeenCalledTimes(1)
  })

  it('фокус на самом окне (щелчок в пустое место) — собирает', async () => {
    open()
    box().focus()
    await key(box())
    expect(go.build).toHaveBeenCalledTimes(1)
  })

  it('на кнопке окна Enter не собирает (кнопка делает своё)', async () => {
    open()
    const btn = document.querySelector<HTMLElement>('header button')!
    btn.focus()
    await key(btn)
    expect(go.build).not.toHaveBeenCalled()
  })

  it('окно закрыто — Enter молчит', async () => {
    open()
    store.autoDraft = null
    flushSync()
    await key(document.body)
    expect(go.build).not.toHaveBeenCalled()
  })
})
