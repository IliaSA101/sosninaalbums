// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Слои фона альбома (098 §3 п. 2, 4): те же слои, что у своего фона, но не объект — метка своего фона на них не ставится,
// прозрачность идёт от общего просмотра альбома, а не от листа.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: BackdropLayer } = await import('./BackdropLayer.svelte')
const { default: PatternLayer } = await import('../pattern/PatternLayer.svelte')
const { albumFadeLive, ALBUM_FADE, fadeLive } = await import('./fade.svelte')

const rect = { x: 0, y: 0, w: 600, h: 300 }
const bd = (o: Record<string, unknown> = {}) => ({
  scope: 'spread',
  photo: 'p1',
  area: rect,
  box: { ...rect, rot: 0 },
  img: rect,
  frame: { r: 0 },
  flipH: false,
  flipV: false,
  fade: 0,
  ...o,
})
const pat = (o: Record<string, unknown> = {}) => ({
  scope: 'spread',
  area: rect,
  known: true,
  url: '/media/gen/x?k=1',
  pat: { kind: 'x', fade: 0 },
  ...o,
})
const sp = (o: Record<string, unknown>) => ({ id: 's1', w: 600, h: 300, ...o }) as never

afterEach(() => {
  cleanup()
  albumFadeLive.clear()
  fadeLive.clear()
})

describe('фото фона альбома', () => {
  it('рисуется, но без data-backdrop (по нему находят свой фон листа); метка — data-album-bg', () => {
    render(BackdropLayer, { spread: sp({ backdrops: [bd({ album: true })] }) })
    expect(document.querySelector('[data-backdrop]')).toBeNull()
    expect(document.querySelector('[data-album-bg="spread"] img')).not.toBeNull()
  })

  it('свой фон листа — как раньше', () => {
    render(BackdropLayer, { spread: sp({ backdrops: [bd()] }) })
    expect(document.querySelector('[data-backdrop="spread"]')).not.toBeNull()
    expect(document.querySelector('[data-album-bg]')).toBeNull()
  })

  it('прозрачность: просмотр ползунка альбома, а не листа; на своём фоне просмотр альбома не действует', () => {
    albumFadeLive.set(ALBUM_FADE, ALBUM_FADE, 0.5)
    render(BackdropLayer, { spread: sp({ backdrops: [bd({ album: true })] }) })
    expect(document.querySelector<HTMLElement>('[data-album-bg] .box')?.style.opacity).toBe('0.5')
    cleanup()
    render(BackdropLayer, { spread: sp({ backdrops: [bd()] }) })
    expect(document.querySelector<HTMLElement>('[data-backdrop] .box')?.style.opacity).toBe('')
  })
})

describe('узор фона альбома', () => {
  it('рисуется с data-album-bg и без data-pattern', () => {
    render(PatternLayer, { spread: sp({ patterns: [pat({ album: true })] }), width: 600 })
    expect(document.querySelector('[data-pattern]')).toBeNull()
    expect(document.querySelector('img[data-album-bg="spread"]')).not.toBeNull()
  })

  it('прозрачность узора альбома — от общего просмотра', () => {
    albumFadeLive.set(ALBUM_FADE, ALBUM_FADE, 0.25)
    render(PatternLayer, { spread: sp({ patterns: [pat({ album: true })] }), width: 600 })
    flushSync()
    expect(document.querySelector<HTMLElement>('img[data-album-bg]')?.style.opacity).toBe('0.75')
  })
})
