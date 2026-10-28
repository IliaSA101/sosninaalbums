// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Go подменён: из ленты при разрезе зовётся SplitSpread (ответ — тот же вид с новой ревизией).
const split = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SplitSpread: split }))

// Плитки фото монтируются только «рядом с видимым» (Nearby): подставной IntersectionObserver сразу говорит «видно».
class SeenIO {
  constructor(private cb: IntersectionObserverCallback) {}
  observe(target: Element) {
    this.cb([{ target, isIntersecting: true } as IntersectionObserverEntry], this as never)
  }
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal('IntersectionObserver', SeenIO)

const { default: Timeline } = await import('./Timeline.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { cutKey } = await import('./cutkey.svelte')
const { naming } = await import('./naming.svelte')

const rect = { x: 0, y: 0, w: 100, h: 100 }
const sp = (id: string, photos: string[]) => ({ id, name: '', photos, rects: photos.map(() => rect), w: 400, h: 200, kind: '' })

beforeEach(() => {
  cutKey.held = false
  naming.stop()
  prefs.level = 'full'
  store.view = { ...store.view, rev: 1, spreads: [sp('a', ['a1', 'a2']), sp('b', ['b1'])], settings: {} } as never
  store.currentId = 'a'
  split.mockImplementation(async () => ({ ...store.view, rev: store.view.rev + 1 }))
})
afterEach(() => {
  cleanup()
  split.mockReset()
})

// Плитки появляются после микрозадач Nearby (корень привязывается чуть позже первого монтирования).
async function mount() {
  const r = render(Timeline)
  await tick()
  await tick()
  return r
}
const strip = (c: HTMLElement) => c.querySelector('.timeline')!
const cuts = (c: HTMLElement) => c.querySelectorAll<HTMLElement>('button.cut')
const join = (c: HTMLElement) => c.querySelector('button.join')!

// Ctrl — на окно, как у реального браузера: зона разреза проступает по keydown, не по щелчку.
const ctrl = (type: 'keydown' | 'keyup', held: boolean) => {
  window.dispatchEvent(new KeyboardEvent(type, { key: 'Control', ctrlKey: held }))
  flushSync()
}

describe('зона разреза в ленте (Ctrl, 094 §1)', () => {
  it('без Ctrl — зоны в DOM есть, но лента не «режет» и черта не вооружена', async () => {
    const { container } = await mount()
    expect(cuts(container)).toHaveLength(1) // перед вторым фото первой группы
    expect(strip(container).classList.contains('cutting')).toBe(false)
    expect(join(container).classList.contains('armed')).toBe(false)
  })

  it('Ctrl зажат — класс .cutting и вооружённая черта; отпущен — оба сняты', async () => {
    const { container } = await mount()
    ctrl('keydown', true)
    expect(strip(container).classList.contains('cutting')).toBe(true)
    expect(join(container).classList.contains('armed')).toBe(true)
    ctrl('keyup', false)
    expect(strip(container).classList.contains('cutting')).toBe(false)
    expect(join(container).classList.contains('armed')).toBe(false)
  })

  it('окно потеряло фокус при зажатом Ctrl — подсказка гаснет (keyup к нам не придёт)', async () => {
    const { container } = await mount()
    ctrl('keydown', true)
    window.dispatchEvent(new Event('blur'))
    flushSync()
    expect(strip(container).classList.contains('cutting')).toBe(false)
  })

  it('Ctrl был зажат до фокуса окна: первое же движение мыши с Ctrl включает подсказку', async () => {
    const { container } = await mount()
    window.dispatchEvent(new MouseEvent('pointermove', { ctrlKey: true }))
    flushSync()
    expect(strip(container).classList.contains('cutting')).toBe(true)
  })

  it('Ctrl+щелчок по зоне режет разворот ровно один раз; щелчок без Ctrl — нет', async () => {
    const { container } = await mount()
    await fireEvent.click(cuts(container)[0])
    expect(split).not.toHaveBeenCalled()
    await fireEvent.click(cuts(container)[0], { ctrlKey: true })
    expect(split).toHaveBeenCalledExactlyOnceWith('a', 1)
  })

  it('лента ушла из DOM — слушатели сняты, зажатый Ctrl не залипает', async () => {
    const { container, unmount } = await mount()
    ctrl('keydown', true)
    expect(strip(container).classList.contains('cutting')).toBe(true)
    unmount()
    expect(cutKey.held).toBe(false)
  })
})
