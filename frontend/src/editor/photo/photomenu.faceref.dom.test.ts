// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// «Остальные — как это лицо» (130 U2): щелчок по пункту зовёт FitFaces(лист, место, '') и показывает тот же тост,
// что кнопка карточки. Go подменён, vigFaces и store настоящие.
const m = vi.hoisted(() => ({ fit: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ FitFaces: m.fit, WhereUsed: vi.fn(async () => []) }))

const { default: PhotoMenu } = await import('./PhotoMenu.svelte')
const { photoui } = await import('./photoui.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

const place = (id: string, photo: string, x: number) => ({
  id,
  photo,
  fit: '',
  spine: false,
  z: 0,
  x,
  y: 20,
  w: 100,
  h: 100,
  rot: 0,
  shape: '',
  round: 0,
  radius: 0,
})
const sheet = (kind: string) => ({
  id: 's1',
  kind,
  ...(kind === 'vignette' ? { vig: { tpl: 'head-t-left' } } : {}),
  w: 600,
  h: 300,
  mid: 300,
  grid: true,
  photos: ['a', 'b'],
  rects: [],
  frames: [],
  texts: [],
  captions: [],
  backdrops: [],
  places: [place('p1', 'a', 20), place('p2', 'b', 200)],
})
const report = {
  fitted: 1,
  noFace: 0,
  many: 0,
  missing: 0,
  small: 0,
  big: 0,
  edge: 0,
  contain: 0,
  manual: 0,
  changed: 0,
  check: [],
  short: [],
  asked: false,
}

const open = async (...ids: string[]) => {
  render(PhotoMenu)
  placesel.select(ids)
  photoui.menu = { x: 40, y: 30, kind: 'photo' }
  flushSync()
  await vi.waitFor(() => expect(document.body.querySelector('.drop')).not.toBeNull())
  await fireEvent.pointerEnter(screen.getByRole('menuitem', { name: /Кадр/ }))
}
const setSheet = (kind: string) => {
  store.view = { ...store.view, rev: 1, spreads: [sheet(kind)], folders: [], missing: [] } as never
  store.go(0)
}

beforeEach(() => {
  vi.spyOn(prefs, 'can').mockReturnValue(true)
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(200)
  window.innerWidth = 1000
  window.innerHeight = 700
  store.photos = [
    { id: 'a', name: 'a.jpg', w: 3, h: 2 },
    { id: 'b', name: 'b.jpg', w: 3, h: 2 },
  ] as never
  setSheet('vignette')
  m.fit.mockResolvedValue({ view: { ...store.view, rev: 2 }, report })
})
afterEach(() => {
  cleanup()
  photoui.close()
  placesel.select([])
  vi.restoreAllMocks()
  vi.clearAllMocks()
  store.toast = null
})

describe('PhotoMenu: Кадр ▸ Остальные — как это лицо', () => {
  it('вход: щелчок — FitFaces(лист, место выделенного фото, «»), меню закрыто, итог тостом', async () => {
    await open('p2')
    await fireEvent.click(screen.getByRole('menuitem', { name: 'Остальные — как это лицо' }))
    expect(photoui.menu).toBeNull()
    expect(m.fit).toHaveBeenCalledExactlyOnceWith('s1', 'p2', '')
    await vi.waitFor(() => expect(store.toast?.text).toBe('Подогнано 1 из 1'))
    expect(store.toast?.kind).toBe('info')
  })

  it('подсказка пункта — слова владельца', async () => {
    await open('p1')
    expect(screen.getByRole('menuitem', { name: 'Остальные — как это лицо' }).getAttribute('title')).toBe(
      'Как у этого фото — размер лица и линия глаз у всех на листе',
    )
  })

  it('обычный разворот: пункта нет', async () => {
    setSheet('spread')
    await open('p1')
    expect(screen.queryByRole('menuitem', { name: 'Остальные — как это лицо' })).toBeNull()
  })

  it('два выделенных фото: пункта нет', async () => {
    await open('p1', 'p2')
    expect(screen.queryByRole('menuitem', { name: 'Остальные — как это лицо' })).toBeNull()
  })

  it('ошибка Go (нет лица) — тост-ошибка, без итога', async () => {
    m.fit.mockRejectedValueOnce('на этом фото не нашлось лица')
    await open('p1')
    await fireEvent.click(screen.getByRole('menuitem', { name: 'Остальные — как это лицо' }))
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
  })
})
