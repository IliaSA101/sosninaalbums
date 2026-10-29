// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Размер плиток окон оформления (133 U1): ползунок, колесо, память по виду, адреса картинок. Go подменён.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  LibraryPacks: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))

const { default: DecorDialog } = await import('../decor/DecorDialog.svelte')
const { decor } = await import('../decor/decor.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { screenDpr } = await import('../../base/ui/dpr.svelte')

const item = (id: string, url: string) => ({ id, theme: 'basic', name: id, url, own: false, aspect: 0, w: 0, h: 0 })
const cat = (kind: string) => ({
  kind,
  themes: [{ id: 'all', name: 'Все', count: 2 }],
  items: [item('sa:x', '/media/gen/art?k=x&v=1'), item('my:a', '/media/lib/t/abc?v=3')],
  recent: [],
  readOnly: false,
})
const sheet = (id: string) => ({
  id,
  photos: [],
  w: 600,
  h: 300,
  mid: 300,
  frames: [],
  texts: [],
  captions: [],
  bg: ['#fff', '#fff'],
  bgSet: ['', ''],
  backdrops: null,
  n: 0,
})
type Kind = 'bg' | 'sticker' | 'frame' | 'spread'
const open = async (kind: Kind) => {
  api.DecorCatalog.mockResolvedValue(cat(kind))
  decor.show(kind, 's1', 'spread')
  await vi.waitFor(() => expect(decor.cat?.kind).toBe(kind))
  flushSync()
}
const tiles = () => document.querySelector<HTMLElement>('.tiles')!
const slider = () => document.querySelector<HTMLInputElement>('input[aria-label="Размер миниатюр"]')!
const tile = () => tiles().style.getPropertyValue('--tile')
const srcs = () => Array.from(document.querySelectorAll<HTMLImageElement>('.tiles img'), (i) => i.getAttribute('src'))
const wheel = (init: WheelEventInit) => {
  const e = new WheelEvent('wheel', { bubbles: true, cancelable: true, deltaMode: 0, ...init })
  // happy-dom не переносит модификаторы в WheelEvent
  for (const k of ['ctrlKey', 'altKey', 'metaKey'] as const) Object.defineProperty(e, k, { value: !!init[k] })
  tiles().dispatchEvent(e)
  return e
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.LibraryPacks.mockResolvedValue({ packs: [], net: {}, busy: false, readOnly: false })
  localStorage.clear()
  modes.decorTile = {}
  decor.close()
  decor.cat = null
  decor.theme = ''
  store.view = { rev: 1, folders: [], settings: { pageW: 300, pageH: 300 }, spreads: [sheet('s1')] } as never
  render(DecorDialog)
})
afterEach(() => {
  cleanup()
  decor.close()
})

describe('ползунок «Размер миниатюр»', () => {
  it('умолчания: стикеры и рамочки 120, фоны и развороты 168; слова — как у ленты', async () => {
    for (const [kind, px] of [
      ['sticker', '120px'],
      ['frame', '120px'],
      ['bg', '168px'],
    ] as const) {
      await open(kind)
      expect(tile()).toBe(px)
      expect(slider().title).toBe('Размер миниатюр')
      decor.close()
      flushSync()
    }
  })

  it('протяжка меняет --tile и запоминается в настройках окна', async () => {
    await open('sticker')
    slider().value = '150'
    await fireEvent.input(slider())
    expect(tile()).toBe('150px')
    expect(JSON.parse(localStorage.getItem('viewMode')!).decorTile).toEqual({ sticker: 150 })
  })

  it('память по виду окна: фоны не берут размер стикеров', async () => {
    await open('sticker')
    slider().value = '200'
    await fireEvent.input(slider())
    decor.close()
    flushSync()
    await open('bg')
    expect(tile()).toBe('168px')
    decor.close()
    flushSync()
    await open('sticker')
    expect(tile()).toBe('200px')
  })

  it('пределы: 96…220, у «Разворотов» от 128', async () => {
    await open('sticker')
    expect([slider().min, slider().max]).toEqual(['96', '220'])
    decor.close()
    flushSync()
    await open('spread')
    expect([slider().min, slider().max]).toEqual(['128', '220'])
  })

  it('мусор в хранилище — умолчание вида; размер вне пределов вида тоже', async () => {
    modes.decorTile = { sticker: 10, bg: 999 }
    await open('sticker')
    expect(tile()).toBe('120px')
    decor.close()
    flushSync()
    await open('bg')
    expect(tile()).toBe('168px')
    decor.close()
    flushSync()
    modes.decorTile = { spread: 100 } // меньше 128 для «Разворотов»
    await open('spread')
    expect(tile()).toBe('168px')
  })
})

describe('Ctrl/Alt+колесо над плитками', () => {
  it('Ctrl+колесо вверх увеличивает, вниз уменьшает; страницу не масштабирует', async () => {
    await open('sticker')
    const e = wheel({ deltaY: -100, ctrlKey: true })
    expect(e.defaultPrevented).toBe(true)
    await vi.waitFor(() => expect(modes.decorTile.sticker).toBeGreaterThan(120))
    const up = modes.decorTile.sticker!
    wheel({ deltaY: 100, ctrlKey: true })
    await vi.waitFor(() => expect(modes.decorTile.sticker).toBeLessThan(up))
  })

  it('Alt+колесо — то же; колесо без клавиши прокручивает как обычно', async () => {
    await open('sticker')
    expect(wheel({ deltaY: -100 }).defaultPrevented).toBe(false)
    expect(modes.decorTile.sticker).toBeUndefined()
    expect(wheel({ deltaY: -100, altKey: true }).defaultPrevented).toBe(true)
    await vi.waitFor(() => expect(modes.decorTile.sticker).toBeGreaterThan(120))
  })

  it('упирается в пределы вида: стикеры 96…220, «Развороты» не мельче 128', async () => {
    await open('sticker')
    for (let i = 0; i < 40; i++) wheel({ deltaY: -100, ctrlKey: true })
    await vi.waitFor(() => expect(modes.decorTile.sticker).toBe(220))
    for (let i = 0; i < 80; i++) wheel({ deltaY: 100, ctrlKey: true })
    await vi.waitFor(() => expect(modes.decorTile.sticker).toBe(96))
    decor.close()
    flushSync()
    await open('spread')
    for (let i = 0; i < 80; i++) wheel({ deltaY: 100, ctrlKey: true })
    await vi.waitFor(() => expect(modes.decorTile.spread).toBe(128))
  })
})

// 134: щипок Mac (GestureEvent; happy-dom его не знает — событие собираем руками) меняет размер плиток так же, как колесо
const gesture = (type: string, init: { scale?: number; clientX?: number; clientY?: number }) => {
  const e = new Event(type, { bubbles: true, cancelable: true })
  for (const [k, v] of Object.entries(init)) Object.defineProperty(e, k, { value: v })
  tiles().dispatchEvent(e)
  return e
}

describe('щипок над плитками (134)', () => {
  it('развёл пальцы — плитки крупнее, погашен; свёл — мельче; ползунок идёт следом', async () => {
    await open('sticker')
    expect(gesture('gesturestart', { scale: 1, clientX: 5, clientY: 5 }).defaultPrevented).toBe(true)
    expect(gesture('gesturechange', { scale: 2, clientX: 5, clientY: 5 }).defaultPrevented).toBe(true)
    await vi.waitFor(() => expect(modes.decorTile.sticker).toBe(Math.round(120 * Math.SQRT2)))
    flushSync()
    expect(slider().value).toBe(String(modes.decorTile.sticker))
    gesture('gesturechange', { scale: 1, clientX: 5, clientY: 5 })
    await vi.waitFor(() => expect(modes.decorTile.sticker).toBe(120))
    gesture('gestureend', { scale: 1 })
  })

  it('упирается в пределы вида: «Развороты» не мельче 128, стикеры не крупнее 220', async () => {
    await open('spread')
    gesture('gesturestart', { scale: 1 })
    let s = 1
    for (let i = 0; i < 30; i++) gesture('gesturechange', { scale: (s *= 0.6) })
    await vi.waitFor(() => expect(modes.decorTile.spread).toBe(128))
    decor.close()
    flushSync()
    await open('sticker')
    gesture('gesturestart', { scale: 1 })
    s = 1
    for (let i = 0; i < 30; i++) gesture('gesturechange', { scale: (s *= 1.6) })
    await vi.waitFor(() => expect(modes.decorTile.sticker).toBe(220))
  })

  it('Ctrl-колесо посреди щипка размер не меняет (старый WebKit дублирует жест)', async () => {
    await open('sticker')
    gesture('gesturestart', { scale: 1 })
    const e = wheel({ deltaY: -100, ctrlKey: true })
    expect(e.defaultPrevented).toBe(true)
    await new Promise((r) => setTimeout(r, 50))
    expect(modes.decorTile.sticker).toBeUndefined()
    gesture('gestureend', { scale: 1 })
  })

  it('после закрытия окна щипок плиток не слушается', async () => {
    await open('sticker')
    const box = tiles()
    decor.close()
    flushSync()
    const e = new Event('gesturestart', { bubbles: true, cancelable: true })
    box.dispatchEvent(e)
    expect(e.defaultPrevented).toBe(false)
  })

  it('колесо и щипок до кадра копятся вместе', async () => {
    await open('sticker')
    wheel({ deltaY: -100, ctrlKey: true })
    gesture('gesturestart', { scale: 1 })
    gesture('gesturechange', { scale: 1.3 })
    // отдельно колесо дало бы 120·1,13 ≈ 136, щипок отдельно — 120·1,14 ≈ 137; вместе заметно больше
    await vi.waitFor(() => expect(modes.decorTile.sticker).toBeGreaterThan(150))
  })

  it('прокрутка держит плитку под курсором', async () => {
    await open('sticker')
    const box = tiles()
    const el = box.firstElementChild as HTMLElement
    // happy-dom геометрии не считает: плитка 100 px, после смены размера (появилась запись в decorTile) она на 40 px ниже
    const top = () => (modes.decorTile.sticker ? 90 : 50)
    box.getBoundingClientRect = () => ({ left: 0, top: 0, right: 400, bottom: 400, width: 400, height: 400 }) as DOMRect
    el.getBoundingClientRect = () => ({ left: 0, top: top(), right: 100, bottom: top() + 100, width: 100, height: 100 }) as DOMRect
    box.scrollTop = 0
    gesture('gesturestart', { scale: 1, clientX: 20, clientY: 70 }) // курсор на 20 px ниже верха плитки
    gesture('gesturechange', { scale: 1.5, clientX: 20, clientY: 70 })
    await vi.waitFor(() => expect(modes.decorTile.sticker).toBeGreaterThan(120))
    await vi.waitFor(() => expect(box.scrollTop).toBe(40))
  })
})

describe('адреса картинок', () => {
  it('/media/lib/ — без px (протяжка не перезапрашивает), встроенные — с px', async () => {
    await open('sticker')
    const dpr = screenDpr.value > 0 ? screenDpr.value : 1
    const px = Math.ceil((120 * dpr) / 64) * 64
    expect(srcs()).toEqual([`/media/gen/art?k=x&v=1&px=${px}`, '/media/lib/t/abc?v=3'])
    slider().value = '200'
    await fireEvent.input(slider())
    expect(srcs()[1]).toBe('/media/lib/t/abc?v=3')
  })

  it('px встроенных не уменьшается, пока окно открыто; после закрытия счёт заново', async () => {
    await open('sticker')
    slider().value = '200'
    await fireEvent.input(slider())
    const big = srcs()[0]
    slider().value = '100'
    await fireEvent.input(slider())
    expect(srcs()[0]).toBe(big)
    decor.close()
    flushSync()
    await open('sticker')
    expect(srcs()[0]).not.toBe(big)
  })
})
