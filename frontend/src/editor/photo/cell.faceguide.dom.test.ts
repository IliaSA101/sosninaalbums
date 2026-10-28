// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Силуэт под лицо в ячейке (130): на выделенной ячейке виньетки при подгонке кадра он заменяет сетку третей;
// на обычном листе, в мини-развороте, в галерее, при выключенной галочке и при наклоне — прежнее.
const m = vi.hoisted(() => ({ guide: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ FaceGuide: m.guide }))

const { default: Cell } = await import('./Cell.svelte')
const { store } = await import('../../base/core/store.svelte')
const { framelive } = await import('./framelive.svelte')
const { modes } = await import('../../base/prefs/view.svelte')

const F = { s: 1.5, x: 0, y: 0, r: 0 }
const frameView = { s: 1, x: 0, y: 0, r: 0, img: { x: 0, y: 0, w: 40, h: 60 }, cellRot: 0, dpi: 300 }
const spread = (kind: '' | 'vignette') =>
  ({
    id: 's1',
    kind,
    w: 600,
    h: 300,
    mid: 300,
    photos: ['a'],
    rects: [{ x: 0, y: 0, w: 40, h: 60 }],
    frames: [frameView],
    texts: [],
    captions: [],
    backdrops: [],
  }) as never
const r = { x: 0, y: 0, w: 40, h: 60 }
const answer = {
  on: true,
  w: 40,
  h: 60,
  eyeY: 19.2,
  path: [
    [20, 10],
    [30, 20],
    [50, 60],
    [-10, 60],
    [10, 20],
  ],
}
const guide = () => document.querySelector('[data-testid="face-guide"]')
const thirds = () => document.querySelectorAll('.third').length
const ninths = () => document.querySelectorAll('.ninth').length
const settle = async () => {
  await Promise.resolve()
  await tick()
  flushSync()
}
const mount = async (kind: '' | 'vignette', o: { mini?: boolean; preview?: boolean } = {}) => {
  const view = render(Cell, { spread: spread(kind), i: 0, r, ...o })
  await settle()
  return view
}

beforeEach(() => {
  m.guide.mockReset()
  m.guide.mockResolvedValue(answer)
  modes.faceGuide = true
  vi.spyOn(store, 'selectedCell', 'get').mockReturnValue('a')
  framelive.clear()
})
afterEach(() => {
  cleanup()
  framelive.clear()
  vi.restoreAllMocks()
})

describe('Cell: силуэт под лицо', () => {
  it('виньетка, сдвиг кадра — силуэт без сетки третей; отпустили — ничего', async () => {
    await mount('vignette')
    expect(guide()).toBeNull() // фото выделено, но кадр не тянут
    framelive.set('a', F, false, true)
    await settle()
    expect(guide()).not.toBeNull()
    expect(thirds()).toBe(0)
    framelive.clear()
    await settle()
    expect(guide()).toBeNull()
    expect(thirds()).toBe(0)
  })

  it('масштаб на виньетке — силуэт, отпускание гасит', async () => {
    await mount('vignette')
    framelive.setZoom('a', F)
    await settle()
    expect(guide()).not.toBeNull()
    framelive.set('a', F)
    await settle()
    expect(guide()).toBeNull()
  })

  it('обычный лист: сдвиг даёт сетку третей, силуэта нет и Go не зовётся', async () => {
    await mount('')
    framelive.set('a', F, false, true)
    await settle()
    expect(guide()).toBeNull()
    expect(thirds()).toBe(4)
    expect(m.guide).not.toHaveBeenCalled()
  })

  it('наклон на виньетке — сетка 9×9, силуэта нет', async () => {
    await mount('vignette')
    framelive.set('a', F, true, false, 2)
    await settle()
    expect(guide()).toBeNull()
    expect(thirds() + ninths()).toBe(16)
  })

  it('галочка «Силуэт под лицо» выключена — на виньетке снова сетка третей, Go не зовётся', async () => {
    modes.faceGuide = false
    await mount('vignette')
    framelive.set('a', F, false, true)
    await settle()
    expect(guide()).toBeNull()
    expect(thirds()).toBe(4)
    expect(m.guide).not.toHaveBeenCalled()
  })

  it('галочку выключили на ходу — силуэт уходит, сетка возвращается', async () => {
    await mount('vignette')
    framelive.set('a', F, false, true)
    await settle()
    expect(guide()).not.toBeNull()
    modes.faceGuide = false
    await settle()
    expect(guide()).toBeNull()
    expect(thirds()).toBe(4)
  })

  it('Go ответил «силуэта нет» (групповое, «Целиком») — прежняя сетка третей', async () => {
    m.guide.mockResolvedValue({ on: false, w: 0, h: 0, eyeY: 0, path: [] })
    await mount('vignette')
    framelive.set('a', F, false, true)
    await settle()
    expect(guide()).toBeNull()
    expect(thirds()).toBe(4)
  })

  it('мини-разворот и галерея раскладок — силуэта нет', async () => {
    for (const o of [{ mini: true }, { preview: true }]) {
      await mount('vignette', o)
      framelive.set('a', F, false, true)
      await settle()
      expect(guide()).toBeNull()
      expect(m.guide).not.toHaveBeenCalled()
      cleanup()
      framelive.clear()
    }
  })

  it('фото не выделено — силуэта нет, Go не зовётся', async () => {
    vi.spyOn(store, 'selectedCell', 'get').mockReturnValue(null)
    await mount('vignette')
    framelive.set('a', F, false, true)
    await settle()
    expect(guide()).toBeNull()
    expect(m.guide).not.toHaveBeenCalled()
  })

  it('кадр тянут у другого фото — силуэт не появляется', async () => {
    await mount('vignette')
    framelive.set('b', F, false, true)
    await settle()
    expect(guide()).toBeNull()
  })
})
