// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Enter в окне «Настроить виньетку» применяет и при фокусе на самом окне (после щелчка в пустое место предпросмотра).
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
const act = vi.hoisted(() => ({ apply: vi.fn(async () => true) }))
vi.mock('./vigact', () => ({
  previewVignette: async () => ({ spread: null, error: '' }),
  vigChoices: async () => [],
  applySetup: act.apply,
}))

const { default: VigDialog } = await import('./VigDialog.svelte')
const { vigDialog } = await import('./vigsession.svelte')
const { store } = await import('../../base/core/store.svelte')

const vig = { tpl: 't', teachers: 1, pupils: 24, ratio: '3:4', manual: false, max: 64 }
const sp = { id: 's1', kind: 'vignette', grid: true, photos: [], places: [], rects: [], w: 420, h: 297, vig } as never

afterEach(() => {
  cleanup()
  vigDialog.close()
  act.apply.mockClear()
})

// Окно открыто; change — число учителей изменено (иначе «Применить» закрыто: «Ничего не изменилось»).
async function open(change = true) {
  store.view = { rev: 1, folders: [], spreads: [sp], settings: {} } as never
  vigDialog.open('s1')
  render(VigDialog)
  flushSync()
  if (change) {
    const t = document.querySelector<HTMLInputElement>("input[type='number']")!
    t.value = '2'
    await fireEvent.input(t)
    flushSync()
  }
}
const box = () => document.querySelector<HTMLElement>('.box')!
const key = (el: Element) => fireEvent.keyDown(el, { key: 'Enter' })

describe('VigDialog: Enter', () => {
  it('фокус внутри тела окна — применяет', async () => {
    await open()
    await key(document.querySelector('.pv')!)
    expect(act.apply).toHaveBeenCalledTimes(1)
  })

  it('фокус на самом окне (щелчок в пустое поле) — применяет', async () => {
    await open()
    box().focus()
    await key(box())
    expect(act.apply).toHaveBeenCalledTimes(1)
  })

  it('«Применить» закрыто (ничего не менялось) — Enter молчит', async () => {
    await open(false)
    box().focus()
    await key(box())
    expect(act.apply).not.toHaveBeenCalled()
  })

  it('на кнопке окна Enter не применяет (кнопка делает своё)', async () => {
    await open()
    const btn = document.querySelector<HTMLElement>('header button')!
    btn.focus()
    await key(btn)
    expect(act.apply).not.toHaveBeenCalled()
  })
})
