// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Оригиналы в окне сравнения (138-10): /media/o только выше 1:1 и в бюджете (все sRGB, вместе ≤ 100 Мп),
// вне бюджета — причина в значке (Т9), 415 у одного кадра — превью только у него.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: PhotoCompare } = await import('./PhotoCompare.svelte')
const { compare } = await import('./compare.svelte')
const { store } = await import('../../base/core/store.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')
const { ORIG_DELAY } = await import('../../editor/photo/original.svelte')
const { BUDGET_TEXT } = await import('./synczoom')
const { ORIG_WHY_TEXT } = await import('../../editor/photo/loupe')

// Превью 2048×1365; оригиналы вдвое больше (max = 2).
const PREVIEW: [number, number] = [2048, 1365]
const sizeOf = (el: HTMLImageElement): [number, number] => (el.getAttribute('src')?.includes('/o/') ? [4096, 2730] : PREVIEW)

const set = (extra: Record<string, unknown> = {}) => {
  store.photos = [
    { id: 'a', name: 'a.jpg', w: 4096, h: 2730 },
    { id: 'b', name: 'b.jpg', w: 4096, h: 2730 },
    { id: 'c', name: 'c.jpg', w: 4096, h: 2730 },
    { id: 'd', name: 'd.jpg', w: 4096, h: 2730 },
  ].map((p) => ({ ...p, ...(extra[p.id] as object) })) as never
}

beforeEach(() => {
  vi.useFakeTimers()
  useCatalog()
  vi.spyOn(HTMLImageElement.prototype, 'naturalWidth', 'get').mockImplementation(function (this: HTMLImageElement) {
    return sizeOf(this)[0]
  })
  vi.spyOn(HTMLImageElement.prototype, 'naturalHeight', 'get').mockImplementation(function (this: HTMLImageElement) {
    return sizeOf(this)[1]
  })
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(700)
  store.view = { ...store.view, rev: 1, spreads: [], folders: [], missing: [] } as never
  store.selected.clear()
  for (const id of ['a', 'b', 'c']) store.selected.add(id)
  set()
})
afterEach(() => {
  compare.close()
  cleanup()
  dropCatalog()
  vi.restoreAllMocks()
  vi.useRealTimers()
})

const origs = () => [...document.querySelectorAll<HTMLImageElement>('img.orig')].map((i) => i.getAttribute('src'))
const badge = () => document.querySelector('.cap .zoom')?.textContent
const note = () => document.querySelector('.cap .note')?.textContent
const tick = (ms: number) => {
  vi.advanceTimersByTime(ms)
  flushSync()
}
const open = () => {
  render(PhotoCompare)
  compare.openByKey()
  flushSync()
  for (const img of document.querySelectorAll('img.ph:not(.orig)')) fireEvent.load(img)
  flushSync()
}
const key = (code: string, init: KeyboardEventInit = {}) => {
  document
    .querySelector('[role=dialog]')!
    .dispatchEvent(new KeyboardEvent('keydown', { code, key: init.key ?? code, bubbles: true, cancelable: true, ...init }))
  flushSync()
}
// 1:1 у всех (Z), затем шаг зума выше 1:1 (Mod+=)
const beyond = () => {
  key('KeyZ')
  key('Equal', { ctrlKey: true })
}

describe('оригинал только выше 1:1', () => {
  it('вписано и 1:1 по превью оригиналов не запрашивают, сколько ни жди', () => {
    open()
    tick(ORIG_DELAY * 3)
    key('KeyZ')
    tick(ORIG_DELAY * 3)
    expect(origs()).toEqual([])
    expect(badge()).toBe('1:1')
  })

  it('выше 1:1 после паузы оригинал грузится у всех ячеек сразу', () => {
    open()
    beyond()
    expect(origs()).toEqual([])
    tick(ORIG_DELAY)
    expect(origs()).toHaveLength(3)
    expect(origs().every((s) => s?.includes('/media/o/'))).toBe(true)
  })

  it('0 вписывает всех: значок «Вписано»', () => {
    open()
    beyond()
    tick(ORIG_DELAY)
    key('Digit0')
    expect(badge()).toMatch(/^Вписано/)
  })
})

describe('бюджет 100 Мп и профиль', () => {
  it('кадры вместе больше 100 Мп: оригиналов нет ни у кого, в значке причина (Т9)', () => {
    set({ c: { w: 12000, h: 9000 } })
    open()
    beyond()
    tick(ORIG_DELAY * 3)
    expect(origs()).toEqual([])
    expect(note()).toBe(BUDGET_TEXT)
  })

  it('у одного кадра Adobe RGB: оригиналов нет ни у кого, причина — профиль', () => {
    set({ b: { color: 'adobe' } })
    open()
    beyond()
    tick(ORIG_DELAY * 3)
    expect(origs()).toEqual([])
    expect(note()).toBe(ORIG_WHY_TEXT.adobe)
  })
})

describe('отказ сервера у одного кадра', () => {
  it('415 у одного: у него превью, у остальных оригинал', () => {
    open()
    beyond()
    tick(ORIG_DELAY)
    const first = document.querySelector('img.orig')!
    fireEvent.error(first)
    flushSync()
    expect(origs()).toHaveLength(2)
  })

  it('пока оригинал не пришёл — значок «Оригинал загружается…»', () => {
    open()
    beyond()
    tick(ORIG_DELAY)
    expect(badge()).toBe('Оригинал загружается…')
    for (const img of document.querySelectorAll('img.orig')) fireEvent.load(img)
    flushSync()
    expect(badge()).toMatch(/%$/)
  })
})
