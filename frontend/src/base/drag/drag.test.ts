import { beforeEach, describe, expect, it, vi } from 'vitest'

// 075-9: данные переноса снимаются при нажатии (источник к первому сдвигу может быть размонтирован),
// а побочное (begin: выделение плитки) — только когда мышь сдвинулась.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const ls = vi.hoisted(() => ({ move: null as ((e: unknown) => void) | null, up: null as (() => void) | null }))
vi.stubGlobal('window', {
  addEventListener: (t: string, f: never) => {
    if (t === 'pointermove') ls.move = f
    if (t === 'pointerup') ls.up = f
  },
  removeEventListener: () => {},
})
vi.stubGlobal('document', {
  body: { classList: { add: () => {}, remove: () => {} } },
  elementsFromPoint: () => [],
})

const { drag } = await import('./drag.svelte')

const down = { button: 0, ctrlKey: false, shiftKey: false, metaKey: false, altKey: false, clientX: 10, clientY: 10 } as never
const moved = (dx: number) => ({ clientX: 10 + dx, clientY: 10 })

beforeEach(() => {
  ls.move = null
  ls.up = null
})

describe('drag.start', () => {
  it('make зовётся один раз — при нажатии, до сдвига', () => {
    const make = vi.fn(() => ({ kind: 'photos' as const, ids: ['a'] }))
    drag.start(down, make)
    expect(make).toHaveBeenCalledTimes(1)
    ls.move!(moved(20))
    ls.move!(moved(40))
    expect(make).toHaveBeenCalledTimes(1)
    expect(drag.payload).toEqual({ kind: 'photos', ids: ['a'] })
    ls.up!()
  })

  it('клик без сдвига: begin не зовётся, перенос не начался', () => {
    const begin = vi.fn()
    drag.start(down, () => ({ kind: 'photos', ids: ['a'] }), begin)
    ls.move!(moved(2)) // меньше порога
    expect(begin).not.toHaveBeenCalled()
    expect(drag.active).toBe(false)
    ls.up!()
    expect(drag.active).toBe(false)
  })

  it('сдвиг за порог: begin один раз, перенос активен', () => {
    const begin = vi.fn()
    drag.start(down, () => ({ kind: 'photos', ids: ['a'] }), begin)
    ls.move!(moved(20))
    ls.move!(moved(30))
    expect(begin).toHaveBeenCalledTimes(1)
    expect(drag.active).toBe(true)
    ls.up!()
    expect(drag.active).toBe(false)
  })
})
