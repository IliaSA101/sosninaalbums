// Alt+щелчок, когда верхний объект сам начинает жест (drive): стикер, фото на «Ручном» (спека 015, QA 0.12).
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { StickerView, View } from '../../base/core/types'

class FakeWindow extends EventTarget {
  setTimeout = setTimeout
  clearTimeout = clearTimeout
}
vi.stubGlobal('window', new FakeWindow())
vi.stubGlobal('localStorage', { getItem: () => null, setItem: () => {} })

const api = vi.hoisted(() => ({ SetStickerBox: vi.fn(), SetPlaces: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../text/fonts', () => ({ loadFonts: async () => {} }))

const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { edit } = await import('../text/edit.svelte')
const { sticker } = await import('../sticker/sticker.svelte')
const { moveSelected } = await import('../places/groupgesture')
const { pickUnder } = await import('./pickunder.svelte')

const stk = (z: number): StickerView =>
  ({
    id: 'k1',
    x: 0,
    y: 0,
    w: 100,
    h: 100,
    rot: 0,
    flipH: false,
    flipV: false,
    fade: 0,
    z,
    photo: '',
    stretch: false,
    name: 'x',
    inks: 1,
    url: '',
    gen: true,
    aspect: 1,
    dpi: 0,
    known: true,
    art: { kind: 'heart', inks: [] },
  }) as StickerView

// Лист «Ручной» 600×300 мм: место pl1 с фото p1 (z=3) и стикер k1 (z задаётся), оба накрывают точку (50,50).
const view = (stZ: number): View =>
  ({
    spreads: [
      {
        id: 's1',
        w: 600,
        h: 300,
        mid: 300,
        grid: true,
        photos: ['p1'],
        rects: [{ x: 0, y: 0, w: 200, h: 200 }],
        frames: [{ z: 3, cellRot: 0 }],
        texts: [],
        captions: [],
        backdrops: [],
        bg: ['#fff', '#fff'],
        places: [{ id: 'pl1', photo: 'p1', x: 0, y: 0, w: 200, h: 200, z: 3, rot: 0, fit: '', spine: false }],
        stickers: [stk(stZ)],
      },
    ],
    settings: { pageW: 300, pageH: 300, margin: 12, gap: 3, layflat: true, dpi: 300, bg: '' },
    folders: [],
    rev: 1,
    safe: [],
    backdropUsed: [],
    minDPI: 200,
  }) as unknown as View

// .content: EventTarget с closest; рамка 600 px = 600 мм.
class FakeEl extends EventTarget {
  closest() {
    return this
  }
  getBoundingClientRect() {
    return { left: 0, top: 0, width: 600, height: 300 }
  }
}
const node = new FakeEl()
const root = node as unknown as HTMLElement
pickUnder(node as unknown as HTMLElement, { spread: () => store.spread!, root: () => root, tg: {} as never, on: true })

const ev = (type: string, alt: boolean) =>
  Object.assign(new Event(type, { cancelable: true }), {
    clientX: 50,
    clientY: 50,
    button: 0,
    altKey: alt,
    shiftKey: false,
    ctrlKey: false,
    metaKey: false,
  })

// Alt+щелчок: pointerdown — сначала захват pickUnder на .content, потом обработчик верхнего объекта (own);
// pointerup на window; click сразу за ним (в браузере — в той же задаче, таймеры между ними не идут).
function altClick(own: (e: PointerEvent) => void) {
  const d = ev('pointerdown', true)
  node.dispatchEvent(d)
  own(d as unknown as PointerEvent)
  window.dispatchEvent(ev('pointerup', true))
  node.dispatchEvent(ev('click', true))
}

beforeEach(() => {
  prefs.level = 'full'
  store.plug({ ...store.hooks, selectedRefs: () => placesel.refs, select: (ids) => placesel.select(ids), applied: () => placesel.prune() })
  placesel.clear()
  edit.clear()
})

describe('Alt+щелчок через верхний объект с жестом drive', () => {
  it('стикер сверху (выделен) — Alt+щелчок выбирает фото под ним', () => {
    store.apply(view(12))
    placesel.pickSticker('k1', false)
    altClick((e) => sticker.move(e, root, store.spread!.stickers![0])) // как StickerLayer.down
    expect(placesel.refs).toEqual(['p1'])
  })
  it('фото сверху (выделено), стикер под ним — Alt+щелчок выбирает стикер', () => {
    store.apply(view(1))
    placesel.pick('p1', false)
    altClick((e) => moveSelected(e, root)) // как Cell.startCell на «Ручном»
    expect(placesel.stickers).toEqual(['k1'])
  })
  it('Alt-нажатие ушло дальше порога и вернулось (перенос) — щелчок не выбирает', () => {
    store.apply(view(12))
    placesel.pickSticker('k1', false)
    const d = ev('pointerdown', true)
    node.dispatchEvent(d)
    window.dispatchEvent(Object.assign(new Event('pointermove'), { clientX: 80, clientY: 50 }))
    window.dispatchEvent(Object.assign(new Event('pointermove'), { clientX: 50, clientY: 50 }))
    window.dispatchEvent(ev('pointerup', true))
    node.dispatchEvent(ev('click', true))
    expect(placesel.stickers).toEqual(['k1'])
  })
  it('Esc во время Alt-нажатия — щелчок не выбирает', () => {
    store.apply(view(12))
    placesel.pickSticker('k1', false)
    node.dispatchEvent(ev('pointerdown', true))
    window.dispatchEvent(Object.assign(new Event('keydown'), { key: 'Escape' }))
    window.dispatchEvent(ev('pointerup', true))
    node.dispatchEvent(ev('click', true))
    expect(placesel.stickers).toEqual(['k1'])
  })
})
