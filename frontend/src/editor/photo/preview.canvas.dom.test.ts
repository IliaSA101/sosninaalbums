// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Окно крупного просмотра с холста (128-6): кадр один, листания, отметок и «N из M» нет; Z в зоне холста.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ WhereUsed: vi.fn().mockResolvedValue([]), ShowPhotoInExplorer: vi.fn() }))

const { default: PhotoPreview } = await import('./PhotoPreview.svelte')
const { default: PhotoMenu } = await import('../photomenu/PhotoMenu.svelte')
const { photoui } = await import('./photoui.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { keymap } = await import('../../base/keys/keymap.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')
const { ownMarks } = await import('../../panels/marks/ownmarks.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { COMMANDS } = await import('../../shell/menu/commands')

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
const spreadView = () => ({
  id: 's1',
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
  stickers: [],
  places: [place('p1', 'a', 20), place('p2', 'b', 200)],
})

beforeEach(() => {
  useCatalog()
  vi.spyOn(prefs, 'can').mockReturnValue(true)
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(200)
  window.innerWidth = 1000
  window.innerHeight = 700
  store.view = { ...store.view, rev: 1, spreads: [spreadView()], folders: [], missing: [] } as never
  store.go(0)
  store.photos = ['a', 'b', 'c'].map((id) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })) as never
  store.zone = 'canvas'
  store.toast = null
  placesel.select([])
})
afterEach(() => {
  cleanup()
  photoui.close()
  photoui.preview = null
  placesel.select([])
  dropCatalog()
  vi.restoreAllMocks()
})

const key = (code: string, init: KeyboardEventInit = {}) => {
  const e = new KeyboardEvent('keydown', { code, key: init.key ?? code, bubbles: true, cancelable: true, ...init })
  window.dispatchEvent(e)
  flushSync()
  return e
}

describe('окно с холста', () => {
  it('пункт «Крупный просмотр» меню фото открывает одно фото; клавиша справа — из реестра', async () => {
    render(PhotoMenu)
    placesel.select(['a'])
    photoui.menu = { x: 40, y: 30, kind: 'photo' }
    flushSync()
    await vi.waitFor(() => expect(document.body.querySelector('.drop')).not.toBeNull())
    const item = screen.getByRole('menuitem', { name: /Крупный просмотр/ })
    expect(item.textContent).toContain(keymap.hint('photoPreview'))
    expect(keymap.hint('photoPreview')).toBe('Z')
    await fireEvent.click(item)
    expect(photoui.preview).toEqual({ ids: ['a'], at: 0, from: 'canvas' })
  })

  it('подпись — только имя, без «N из M» и отметок; ← → не листают, отметка не ставится', () => {
    const marks = vi.spyOn(ownMarks, 'setMarks').mockResolvedValue(true)
    render(PhotoPreview)
    photoui.openCanvas('a')
    flushSync()
    expect(document.querySelector('.cap')?.textContent?.trim()).toBe('a.jpg')
    expect(document.querySelector('.cap .mk')).toBeNull()
    const r = key('ArrowRight', { key: 'ArrowRight' })
    key('ArrowLeft', { key: 'ArrowLeft' })
    expect(r.defaultPrevented).toBe(true)
    expect(photoui.previewId).toBe('a')
    key('KeyP')
    key('Digit4')
    expect(marks).not.toHaveBeenCalled()
  })

  it('Esc, ✕ и щелчок по фону закрывают окно и освобождают стек окон', async () => {
    render(PhotoPreview)
    for (const close of [
      () => key('Escape', { key: 'Escape' }),
      () => fireEvent.click(screen.getByLabelText('Закрыть')),
      () => fireEvent.pointerDown(document.querySelector('.back')!),
    ]) {
      photoui.openCanvas('a')
      flushSync()
      expect(modalStack.any()).toBe(true)
      await close()
      flushSync()
      expect(photoui.preview).toBeNull()
      expect(modalStack.any()).toBe(false)
    }
  })
})

describe('Z в зоне холста', () => {
  it('выделено одно фото — открывает его, выделение не трогается', () => {
    placesel.select(['p1'])
    COMMANDS.photoPreview()
    expect(photoui.preview).toEqual({ ids: ['a'], at: 0, from: 'canvas' })
    expect(placesel.selected).toHaveLength(1)
    expect(store.toast).toBeNull()
  })

  it('выделено два фото — тост «Выделите одно фото на развороте…», окна нет', () => {
    placesel.select(['p1', 'p2'])
    COMMANDS.photoPreview()
    expect(photoui.preview).toBeNull()
    expect(store.toast?.text).toBe('Выделите одно фото на развороте, чтобы открыть крупно')
  })

  it('ничего не выделено — тот же тост', () => {
    COMMANDS.photoPreview()
    expect(photoui.preview).toBeNull()
    expect(store.toast?.text).toBe('Выделите одно фото на развороте, чтобы открыть крупно')
  })
})

describe('Z в зоне ленты', () => {
  beforeEach(() => {
    store.zone = 'library'
    store.selected.clear()
  })
  it('нет выделенных — тост «Выделите фото в ленте, чтобы открыть крупно»', () => {
    COMMANDS.photoPreview()
    expect(photoui.preview).toBeNull()
    expect(store.toast?.text).toBe('Выделите фото в ленте, чтобы открыть крупно')
  })
  it('якорь выделен — он; якорь снят — первое выделенное', () => {
    store.selected.add('b')
    store.selected.add('c')
    store.anchor = 'c'
    COMMANDS.photoPreview()
    expect(photoui.previewId).toBe('c')
    photoui.preview = null
    store.anchor = 'a'
    COMMANDS.photoPreview()
    expect(photoui.previewId).toBe('b')
  })
  it('выделение скрыто фильтром — открывается только видимое, невидимых нет → тост', () => {
    store.selected.add('zzz')
    COMMANDS.photoPreview()
    expect(photoui.preview).toBeNull()
    expect(store.toast).not.toBeNull()
  })
})
