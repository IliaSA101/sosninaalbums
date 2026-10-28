import { beforeEach, describe, expect, it, vi } from 'vitest'

// 075-9: данные переноса снимаются при нажатии (источник к первому сдвигу может быть размонтирован),
// а побочное (begin: выделение плитки) — только когда мышь сдвинулась.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

// Слушатели окна и документа по типу: последний повешенный; снятый — убирается (так видно, что жест кончился).
const ls = vi.hoisted(() => ({ win: new Map<string, (e?: unknown) => void>(), doc: new Map<string, (e?: unknown) => void>() }))
const ev = vi.hoisted(() => ({ hidden: false, stack: [] as unknown[] }))
const bag = (m: Map<string, (e?: unknown) => void>) => ({
  addEventListener: (t: string, f: never) => m.set(t, f),
  removeEventListener: (t: string, f: never) => m.get(t) === f && m.delete(t),
})
vi.stubGlobal('window', bag(ls.win))
vi.stubGlobal('document', {
  ...bag(ls.doc),
  body: { classList: { add: () => {}, remove: () => {} } },
  elementsFromPoint: vi.fn(() => ev.stack),
  get visibilityState() {
    return ev.hidden ? 'hidden' : 'visible'
  },
})
vi.stubGlobal('requestAnimationFrame', () => 1)
vi.stubGlobal('cancelAnimationFrame', () => {})

const dropOn = vi.hoisted(() => vi.fn(async () => true))
vi.mock('./dropact', () => ({ dropOn }))

const { drag } = await import('./drag.svelte')
const { edges, edgeScroll } = await import('./edgescroll')

const down = { button: 0, ctrlKey: false, shiftKey: false, metaKey: false, altKey: false, clientX: 10, clientY: 10 } as never
const moved = (dx: number) => ({ clientX: 10 + dx, clientY: 10 })
const ls2 = {
  get move() {
    return ls.win.get('pointermove') ?? null
  },
  get up() {
    return ls.win.get('pointerup') ?? null
  },
}

// Цель под курсором — элемент с data-drop="spread" (чужой лист): бросок пошёл бы в dropOn.
const sheet = (id: string) =>
  ({
    dataset: { drop: 'spread', spread: id },
    closest: (sel: string) => (sel === '[data-drop]' ? sheet(id) : null),
    getBoundingClientRect: () => ({ left: 0, top: 0, width: 100, height: 100 }),
  }) as unknown

beforeEach(async () => {
  await new Promise((r) => setTimeout(r, 0)) // гашение щелчка после прежнего броска (setTimeout 0)
  drag.cancel()
  ls.win.clear()
  ls.doc.clear()
  ev.hidden = false
  ev.stack = []
  dropOn.mockClear()
})

describe('drag.start', () => {
  it('make зовётся один раз — при нажатии, до сдвига', () => {
    const make = vi.fn(() => ({ kind: 'photos' as const, ids: ['a'] }))
    drag.start(down, make)
    expect(make).toHaveBeenCalledTimes(1)
    ls2.move!(moved(20))
    ls2.move!(moved(40))
    expect(make).toHaveBeenCalledTimes(1)
    expect(drag.payload).toEqual({ kind: 'photos', ids: ['a'] })
    ls2.up!()
  })

  it('клик без сдвига: begin не зовётся, перенос не начался', () => {
    const begin = vi.fn()
    drag.start(down, () => ({ kind: 'photos', ids: ['a'] }), begin)
    ls2.move!(moved(2)) // меньше порога
    expect(begin).not.toHaveBeenCalled()
    expect(drag.active).toBe(false)
    ls2.up!()
    expect(drag.active).toBe(false)
  })

  it('сдвиг за порог: begin один раз, перенос активен', () => {
    const begin = vi.fn()
    drag.start(down, () => ({ kind: 'photos', ids: ['a'] }), begin)
    ls2.move!(moved(20))
    ls2.move!(moved(30))
    expect(begin).toHaveBeenCalledTimes(1)
    expect(drag.active).toBe(true)
    ls2.up!()
    expect(drag.active).toBe(false)
  })
})

// 092 U1: перенос, потерявший отпускание, не должен «застрять» и бросить старое фото следующим жестом.
describe('сброс переноса (092)', () => {
  const photo = () => ({ kind: 'photos' as const, ids: ['a'], fromSpread: 's1' })
  const begin = () => {
    ev.stack = [sheet('s2')]
    drag.start(down, photo)
    ls2.move!({ ...moved(20), buttons: 1 })
    expect(drag.active).toBe(true)
    expect(drag.target).toEqual({ kind: 'spread', spread: 's2', at: 0.3 })
  }
  // Жест снят целиком: ни одного слушателя, перенос не активен, бросок не ушёл.
  const gone = () => {
    expect(drag.active).toBe(false)
    expect(drag.target).toBeNull()
    expect([...ls.win.keys(), ...ls.doc.keys()]).toEqual([])
    expect(dropOn).not.toHaveBeenCalled()
    expect(drag.clickAllowed()).toBe(true)
  }

  it('blur отменяет перенос: бросок не идёт', () => {
    begin()
    ls.win.get('blur')!()
    gone()
  })
  it('pointercancel отменяет перенос', () => {
    begin()
    ls.win.get('pointercancel')!()
    gone()
  })
  it('скрытие окна отменяет перенос', () => {
    begin()
    ls.doc.get('visibilitychange')!() // видно — ничего
    expect(drag.active).toBe(true)
    ev.hidden = true
    ls.doc.get('visibilitychange')!()
    gone()
  })
  it('движение без нажатой кнопки отменяет перенос', () => {
    begin()
    ls2.move!({ ...moved(30), buttons: 0 })
    gone()
  })
  it('Esc отменяет перенос', () => {
    begin()
    ls.win.get('keydown')!({ key: 'Escape', stopPropagation: () => {}, preventDefault: () => {} })
    gone()
  })
  it('новый жест снимает застрявший: старое фото не бросается', () => {
    begin()
    const staleUp = ls2.up!
    drag.start(down, () => ({ kind: 'photos', ids: ['b'] }))
    expect(drag.active).toBe(false)
    staleUp() // запоздалый up прежнего жеста — уже снят
    ls2.up!() // up нового жеста без сдвига — клик
    expect(dropOn).not.toHaveBeenCalled()
  })
  it('обычный бросок по-прежнему идёт', () => {
    begin()
    ls2.up!()
    expect(dropOn).toHaveBeenCalledWith(photo(), { kind: 'spread', spread: 's2', at: 0.3 })
    expect(drag.clickAllowed()).toBe(false)
  })
})

// 095: автопрокрутка у края — точка конца переноса одна (reset), прокрутка скроллера пересчитывает цель.
describe('автопрокрутка (095)', () => {
  it('reset останавливает автопрокрутку', () => {
    const feed = vi.spyOn(edges, 'feed')
    const stop = vi.spyOn(edges, 'stop')
    drag.start(down, () => ({ kind: 'photos', ids: ['a'] }))
    ls2.move!({ ...moved(20), buttons: 1 })
    expect(feed).toHaveBeenLastCalledWith(30, 10)
    stop.mockClear()
    ls.win.get('blur')!()
    expect(stop).toHaveBeenCalled()
    // и при броске
    drag.start(down, () => ({ kind: 'photos', ids: ['a'] }))
    ls2.move!({ ...moved(20), buttons: 1 })
    stop.mockClear()
    ls2.up!()
    expect(stop).toHaveBeenCalled()
    feed.mockRestore()
    stop.mockRestore()
  })

  it('прокрутка скроллера — rehit', () => {
    const scroll = new Map<string, () => void>()
    const node = {
      addEventListener: (t: string, f: () => void) => scroll.set(t, f),
      removeEventListener: () => {},
      getBoundingClientRect: () => ({ left: 0, top: 500, width: 1000, height: 80 }),
      closest: () => null,
      scrollLeft: 0,
      scrollWidth: 1000,
      clientWidth: 1000,
    }
    const act = edgeScroll(node as never, { axis: 'x' })
    drag.start(down, () => ({ kind: 'photos', ids: ['a'] }))
    ls2.move!({ ...moved(20), buttons: 1 })
    expect(drag.target).toBeNull()
    // лента уехала: под неподвижным курсором теперь чужой лист
    ev.stack = [sheet('s2')]
    scroll.get('scroll')!()
    expect(drag.target).toEqual({ kind: 'spread', spread: 's2', at: 0.3 })
    // после конца переноса прокрутка цель не трогает
    drag.cancel()
    ev.stack = [sheet('s3')]
    scroll.get('scroll')!()
    expect(drag.target).toBeNull()
    act.destroy()
  })
})

// 096: источник со своей сессией (живая перестановка) — цель считает она, а не DOM под курсором.
describe('сессия переноса (096)', () => {
  const spread = () => ({ kind: 'spread' as const, id: 's1', index: 0 })
  const gap = { kind: 'gap' as const, at: 3, to: 2 }
  const fake = () => ({ hit: vi.fn(() => gap), drop: vi.fn(), settle: vi.fn(), cancel: vi.fn() })

  it('разворот без сессии цели не получает: щели ведёт только сессия', () => {
    const el = {
      dataset: { drop: 'new', at: '1' },
      // [data-chrome] над листом нет; всё остальное — сам лист
      closest: (sel: string) => (sel === '[data-chrome]' ? null : el),
      getBoundingClientRect: () => ({ left: 0, top: 0, width: 10, height: 10 }),
    }
    ev.stack = [el as unknown]
    drag.start(down, spread)
    ls2.move!({ ...moved(20), buttons: 1 })
    expect(drag.active).toBe(true)
    expect(drag.target).toBeNull()
    expect(drag.over).toBe('')
    ls2.up!()
    expect(dropOn).not.toHaveBeenCalled()
  })

  it('сессия: hit, drop и cancel вызываются по ходу жеста', async () => {
    const s = fake()
    const make = vi.fn(() => s)
    ev.stack = [sheet('s2')] // DOM под курсором не спрашивается
    drag.start(down, spread, undefined, make)
    ls2.move!(moved(2)) // до порога сессии нет
    expect(make).not.toHaveBeenCalled()
    ls2.move!({ ...moved(20), buttons: 1 })
    expect(make).toHaveBeenCalledWith(10, 10) // точка нажатия
    expect(s.hit).toHaveBeenLastCalledWith(30, 10)
    expect(drag.target).toEqual(gap)
    expect(drag.over).toBe('') // общие подсветки молчат
    drag.rehit()
    expect(s.hit).toHaveBeenCalledTimes(2)
    ls2.up!()
    expect(s.drop).toHaveBeenCalledWith(gap)
    expect(s.cancel).not.toHaveBeenCalled()
    expect(dropOn).toHaveBeenCalledWith(spread(), gap)
    await Promise.resolve()
    await Promise.resolve()
    expect(s.settle).toHaveBeenCalledWith(true)

    // ошибка Go — settle(false)
    const s2 = fake()
    dropOn.mockResolvedValueOnce(false)
    drag.start(down, spread, undefined, () => s2)
    ls2.move!({ ...moved(20), buttons: 1 })
    ls2.up!()
    await Promise.resolve()
    await Promise.resolve()
    expect(s2.settle).toHaveBeenCalledWith(false)

    // Esc — cancel, без drop и броска
    const s3 = fake()
    dropOn.mockClear()
    drag.start(down, spread, undefined, () => s3)
    ls2.move!({ ...moved(20), buttons: 1 })
    ls.win.get('keydown')!({ key: 'Escape', stopPropagation: () => {}, preventDefault: () => {} })
    expect(s3.cancel).toHaveBeenCalledOnce()
    expect(s3.drop).not.toHaveBeenCalled()
    expect(dropOn).not.toHaveBeenCalled()
    expect(drag.active).toBe(false)
  })

  it('отпускание без цели: drop(null), броска нет', () => {
    const s = { ...fake(), hit: vi.fn(() => null) }
    drag.start(down, spread, undefined, () => s)
    ls2.move!({ ...moved(20), buttons: 1 })
    ls2.up!()
    expect(s.drop).toHaveBeenCalledWith(null)
    expect(s.settle).not.toHaveBeenCalled()
    expect(dropOn).not.toHaveBeenCalled()
  })

  it('без сессии — прежний hit', () => {
    ev.stack = [sheet('s2')]
    drag.start(down, () => ({ kind: 'photos', ids: ['a'], fromSpread: 's1' }))
    ls2.move!({ ...moved(20), buttons: 1 })
    expect(drag.target).toEqual({ kind: 'spread', spread: 's2', at: 0.3 })
    expect(drag.over).toBe('spread:s2')
    drag.cancel()
  })
})
