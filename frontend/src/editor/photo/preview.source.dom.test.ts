// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Что показывает окно (128-7): из ленты — исходник без поворота, наклона и тона, даже если фото стоит на текущем
// развороте; с холста — как в альбоме, как раньше.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: PhotoPreview } = await import('./PhotoPreview.svelte')
const { photoui } = await import('./photoui.svelte')
const { adjlive } = await import('./adjlive.svelte')
const { store } = await import('../../base/core/store.svelte')

// Кадр фото a на развороте: четверть и наклон (полный угол 100°), ч/б.
const frame = { s: 1, x: 0, y: 0, r: 100, turn: 1, tilt: 10, filter: 'grayscale(1)', img: { x: 0, y: 0, w: 1, h: 1 } }
const spreadView = () => ({
  id: 's1',
  w: 600,
  h: 300,
  mid: 300,
  grid: true,
  photos: ['a'],
  rects: [],
  frames: [frame],
  texts: [],
  captions: [],
  backdrops: [],
  stickers: [],
  places: [],
})

beforeEach(() => {
  vi.spyOn(HTMLImageElement.prototype, 'naturalWidth', 'get').mockReturnValue(2048)
  vi.spyOn(HTMLImageElement.prototype, 'naturalHeight', 'get').mockReturnValue(1365)
  store.view = { ...store.view, rev: 1, spreads: [spreadView()], folders: [], missing: [] } as never
  store.go(0)
  store.photos = ['a', 'b'].map((id) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })) as never
})
afterEach(() => {
  cleanup()
  photoui.preview = null
  adjlive.clear()
  vi.restoreAllMocks()
})

const img = () => document.querySelector<HTMLImageElement>('img.ph')!
const shown = () => {
  flushSync()
  fireEvent.load(img()) // размер картинки приходит по load
  flushSync()
}

describe('источник кадра', () => {
  it('из ленты: фото на текущем развороте показано исходником — без поворота, наклона и ч/б', () => {
    render(PhotoPreview)
    photoui.openLibrary('a')
    shown()
    const el = img()
    expect(el.getAttribute('src')).toContain('/media/p/a')
    expect(el.style.filter).toBe('')
    expect(el.style.transform).toBe('')
    expect(document.querySelector('.turned')).toBeNull()
    expect(el.classList.contains('raw')).toBe(false)
  })

  it('из ленты: живой тон карточки (adjlive) тоже не применяется', () => {
    render(PhotoPreview)
    adjlive.set('a', { bright: 20, contrast: 0 } as never)
    photoui.openLibrary('a')
    shown()
    expect(img().style.filter).toBe('')
  })

  it('с холста: как в альбоме — поворот на четверть и наклон, ч/б', () => {
    render(PhotoPreview)
    photoui.openCanvas('a')
    shown()
    const el = img()
    expect(document.querySelector('.turned')).not.toBeNull()
    expect(el.classList.contains('raw')).toBe(true)
    expect(el.style.transform).toContain('rotate(100deg)')
    expect(el.style.filter).toBe('grayscale(1)')
  })

  it('фото не на развороте: и из ленты, и с холста — исходник', () => {
    render(PhotoPreview)
    for (const open of [() => photoui.openLibrary('b'), () => photoui.openCanvas('b')]) {
      open()
      shown()
      expect(img().style.filter).toBe('')
      expect(img().style.transform).toBe('')
      photoui.preview = null
      flushSync()
    }
  })

  it('листание из ленты: каждый кадр — исходник, адрес меняется на соседний', () => {
    render(PhotoPreview)
    photoui.openLibrary('a')
    shown()
    photoui.step(1)
    shown()
    expect(img().getAttribute('src')).toContain('/media/p/b')
    expect(img().style.transform).toBe('')
  })
})
