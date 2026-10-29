// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Узор и окно цвета (109): наведение на цвет подменяет i= в адресе Go после паузы 120 мс; свой узор и фон альбома — раздельно.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), MyColors: vi.fn(async () => []) }
})

const { default: PatternLayer } = await import('./PatternLayer.svelte')
const { default: InspectorPattern } = await import('./InspectorPattern.svelte')
const { colorLive, INK_DELAY } = await import('../../base/color/colorlive.svelte')
const { store } = await import('../../base/core/store.svelte')

const URL = '/media/gen/pat?k=bands&i=c8201e,203040&w=200&h=100&v=3'
const pat = (scope: string, over: object = {}) => ({
  scope,
  known: true,
  url: URL,
  name: 'Полосы',
  inks: 2,
  album: false,
  area: { x: 0, y: 0, w: 200, h: 100 },
  pat: { kind: 'bands', inks: ['#c8201e', '#203040'] },
  ...over,
})
const sheet = (patterns: object[]) =>
  ({ id: 's1', w: 200, h: 100, bg: ['#fff', '#fff'], bgSet: ['', ''], backdrops: [], patterns, places: [] }) as never
const srcs = (c: HTMLElement) => Array.from(c.querySelectorAll('img.pat')).map((i) => i.getAttribute('src') ?? '')

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  cleanup()
  colorLive.clear()
  vi.useRealTimers()
})

describe('PatternLayer: показ краски', () => {
  it('своя краска i подменяется через 120 мс, чужой лист и фон альбома — нет', () => {
    const own = render(PatternLayer, { spread: sheet([pat('left'), pat('right', { album: true })]), width: 400 })
    colorLive.set(['pat:s1:left', 'pat:s1:right'], '#AABBCC', 1)
    flushSync()
    expect(srcs(own.container)[0]).toContain('i=c8201e,203040')
    vi.advanceTimersByTime(INK_DELAY)
    flushSync()
    const [left, right] = srcs(own.container)
    expect(left).toContain('i=c8201e,aabbcc')
    expect(right).toContain('i=c8201e,203040') // фон альбома (098) — не узор листа
    const foreign = render(PatternLayer, { spread: { ...(sheet([pat('left')]) as object), id: 's2' } as never, width: 400 })
    expect(srcs(foreign.container)[0]).toContain('i=c8201e,203040')
  })

  it('узор без i= (краски по умолчанию) — адрес как был', () => {
    const plain = '/media/gen/pat?k=bands&w=200&h=100&v=3'
    const r = render(PatternLayer, { spread: sheet([pat('left', { url: plain })]), width: 400 })
    colorLive.set(['pat:s1:left'], '#aabbcc', 0)
    vi.advanceTimersByTime(INK_DELAY)
    flushSync()
    expect(srcs(r.container)[0]).not.toContain('aabbcc')
  })
})

describe('InspectorPattern: наведение на цвет узора', () => {
  it('ключ узора и номер краски; плитка в карточке тоже перекрашивается; rev прежний', async () => {
    vi.useRealTimers()
    store.view = { ...store.view, rev: 9, settings: { pageW: 300, pageH: 300 }, spreads: [sheet([pat('left')])] } as never
    store.go(0)
    const sp = store.spread!
    const r = render(InspectorPattern, { sp })
    await fireEvent.click(screen.getByRole('button', { name: 'Цвет узора 2' }))
    flushSync()
    await fireEvent.pointerEnter(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    expect(colorLive.color('pat:s1:left', '')).toBe('#4a4643')
    await new Promise((res) => setTimeout(res, INK_DELAY + 30))
    flushSync()
    const tile = r.container.querySelector('.have img')?.getAttribute('src') ?? ''
    expect(tile).toContain('i=c8201e,4a4643')
    expect(store.view.rev).toBe(9)
    await fireEvent.pointerLeave(screen.getAllByRole('button', { name: /#4a4643/ })[0])
    flushSync()
    expect(r.container.querySelector('.have img')?.getAttribute('src')).toContain('i=c8201e,203040')
  })
})
