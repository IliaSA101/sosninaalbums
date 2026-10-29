import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Спека 100 п. 6: Ctrl-перенос фото на ручном листе идёт тем же drag.start, что ⇄. Порог сдвига 5 px:
// меньше — щелчок (Ctrl+щелчок идёт в выделение), больше — перенос, после него щелчок гасится.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const ls = vi.hoisted(() => ({ win: new Map<string, (e?: unknown) => void>() }))
vi.stubGlobal('window', {
  addEventListener: (t: string, f: never) => ls.win.set(t, f),
  removeEventListener: (t: string, f: never) => ls.win.get(t) === f && ls.win.delete(t),
})
vi.stubGlobal('document', {
  addEventListener: () => {},
  removeEventListener: () => {},
  body: { classList: { add: () => {}, remove: () => {} } },
  elementsFromPoint: () => [],
  visibilityState: 'visible',
})
vi.stubGlobal('requestAnimationFrame', () => 1)
vi.stubGlobal('cancelAnimationFrame', () => {})

const { drag } = await import('../../base/drag/drag.svelte')
const { setMacForTest, toggleKey } = await import('../../base/core/platform')
const { bodyAction } = await import('./gesture')

const down = (o: object = {}) =>
  ({ button: 0, ctrlKey: false, shiftKey: false, metaKey: false, altKey: false, clientX: 10, clientY: 10, ...o }) as never
const moveTo = (dx: number) => ls.win.get('pointermove')!({ clientX: 10 + dx, clientY: 10, buttons: 1 })

// Как Cell.startCell на ручном листе: действие по нажатию; 'drag' — drag.start с фото.
function press(e: ReturnType<typeof down>) {
  const act = bodyAction({ grid: true, selected: false, inGroup: false, swap: toggleKey(e) })
  if (act === 'drag') drag.start(e, () => ({ kind: 'photos', ids: ['a'], fromSpread: 's1' }))
  return act
}

beforeEach(async () => {
  await new Promise((r) => setTimeout(r, 0)) // гашение щелчка после прежнего переноса
  drag.cancel()
  ls.win.clear()
})
afterEach(() => setMacForTest(null))

describe('Ctrl-обмен на ручном листе', () => {
  it('Windows: Ctrl и сдвиг дальше порога — перенос фото, место не двигается', () => {
    setMacForTest(false)
    expect(press(down({ ctrlKey: true }))).toBe('drag')
    moveTo(20)
    expect(drag.active).toBe(true)
    expect(drag.payload).toEqual({ kind: 'photos', ids: ['a'], fromSpread: 's1' })
    ls.win.get('pointerup')!()
    expect(drag.clickAllowed()).toBe(false) // щелчок после переноса не выделяет
  })
  it('Ctrl без сдвига (меньше 5 px) — не перенос, щелчок идёт в выделение', () => {
    setMacForTest(false)
    expect(press(down({ ctrlKey: true }))).toBe('drag')
    moveTo(4)
    expect(drag.active).toBe(false)
    ls.win.get('pointerup')!()
    expect(drag.clickAllowed()).toBe(true)
  })
  it('без Ctrl — двигается место, drag не стартует', () => {
    setMacForTest(false)
    expect(press(down())).toBe('move')
    expect(ls.win.size).toBe(0)
  })
  it('Mac: ⌘ — перенос; Ctrl — правый щелчок, жеста нет', () => {
    setMacForTest(true)
    expect(press(down({ metaKey: true }))).toBe('drag')
    moveTo(20)
    expect(drag.active).toBe(true)
    drag.cancel()
    ls.win.clear()
    expect(press(down({ ctrlKey: true }))).toBe('move')
    expect(ls.win.size).toBe(0)
  })
})
