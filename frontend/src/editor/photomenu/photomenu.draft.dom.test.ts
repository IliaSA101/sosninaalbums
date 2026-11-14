// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// 131: путь «заготовка выделена → меню надписи → «Сделать обычной надписью» → Api.SetTextDraft(…, false)». Go подменён.
const m = vi.hoisted(() => ({ setTextDraft: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SetTextDraft: m.setTextDraft, WhereUsed: vi.fn(async () => []) }))

const { default: PhotoMenu } = await import('./PhotoMenu.svelte')
const { photoui } = await import('../photo/photoui.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { edit } = await import('../text/edit.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

const text = (id: string, draft?: boolean) => ({
  id,
  x: 10,
  y: 10,
  w: 80,
  h: 20,
  rot: 0,
  z: 0,
  text: '[Имена]',
  ...(draft ? { draft } : {}),
})
const sheet = {
  id: 's1',
  w: 600,
  h: 300,
  mid: 300,
  grid: true,
  photos: [],
  rects: [],
  frames: [],
  texts: [text('d1', true), text('t1')],
  captions: [],
  backdrops: [],
  places: [],
}
const open = (id: string) => {
  edit.select({ kind: 'text', spread: 's1', id })
  photoui.menu = { x: 40, y: 30, kind: 'text' }
  flushSync()
}
const item = () => screen.queryByRole('menuitem', { name: 'Сделать обычной надписью' })

beforeEach(() => {
  vi.spyOn(prefs, 'can').mockReturnValue(true)
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(200)
  window.innerWidth = 1000
  window.innerHeight = 700
  store.view = { ...store.view, rev: 1, spreads: [sheet], folders: [], missing: [] } as never
  store.go(0)
  m.setTextDraft.mockResolvedValue({ ...store.view, rev: 2 })
  render(PhotoMenu)
})
afterEach(() => {
  cleanup()
  photoui.close()
  edit.clear()
  placesel.select([])
  vi.restoreAllMocks()
  vi.clearAllMocks()
})

describe('PhotoMenu: заготовка', () => {
  it('выделена заготовка — пункт есть; щелчок зовёт SetTextDraft(s1, d1, false) и закрывает меню', async () => {
    open('d1')
    await vi.waitFor(() => expect(item()).not.toBeNull())
    await fireEvent.click(item() as HTMLElement)
    await vi.waitFor(() => expect(m.setTextDraft).toHaveBeenCalledExactlyOnceWith('s1', 'd1', false))
    expect(photoui.menu).toBeNull()
  })
  it('обычная надпись — пункта нет', async () => {
    open('t1')
    await vi.waitFor(() => expect(screen.queryByRole('menuitem', { name: /Копировать/ })).not.toBeNull())
    expect(item()).toBeNull()
  })
})
