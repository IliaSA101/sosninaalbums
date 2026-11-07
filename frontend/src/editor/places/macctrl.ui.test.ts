import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// 073 п. 4: на Mac Ctrl+щелчок — правый щелчок. Жест (drag, места, рамка) не начинается, указатель не захватывается,
// выделение Ctrl не собирает. Компоненты без DOM не монтируем — проверяем общие входы, через которые идут Cell,
// PlaceLayer, SelFrame, Thumb и лента.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const listeners = vi.hoisted(() => ({ add: [] as string[] }))
vi.stubGlobal('window', {
  addEventListener: (t: string) => listeners.add.push(t),
  removeEventListener: () => {},
})
// 092 U1: жест слушает и скрытие окна на документе.
vi.stubGlobal('document', {
  addEventListener: (t: string) => listeners.add.push(`document:${t}`),
  removeEventListener: () => {},
})
// Полный набор слушателей жеста: перенос, бросок, отмена (092 U1: pointercancel, blur, скрытие), Esc, колесо (095).
const GESTURE = ['pointermove', 'pointerup', 'pointercancel', 'blur', 'keydown', 'wheel', 'document:visibilitychange']

const { setMacForTest } = await import('../../base/core/platform')
const { drag } = await import('../../base/drag/drag.svelte')
const { store } = await import('../../base/core/store.svelte')
const { drive } = await import('./placegesture')
const { canvas } = await import('../canvas/canvas.svelte')
const { isSelectClick } = await import('../../panels/timeline/spreadsel')
const { additive } = await import('./gesture')
const { pressPlan } = await import('../canvas/pickunder')

const down = (o: Partial<{ button: number; ctrlKey: boolean; shiftKey: boolean; metaKey: boolean }> = {}) =>
  ({ button: 0, ctrlKey: false, shiftKey: false, metaKey: false, altKey: false, clientX: 10, clientY: 10, pointerId: 1, ...o }) as never
const captured = vi.fn()
const root = { getBoundingClientRect: () => ({ width: 100 }), setPointerCapture: captured } as never

beforeEach(() => {
  listeners.add.length = 0
  captured.mockClear()
  store.view = { rev: 1, marksRev: 0, folders: [], spreads: [{ id: 's', w: 200, h: 100, photos: [], rects: [] }], settings: {} } as never
})
afterEach(() => setMacForTest(null))

describe('drag.start (Thumb, Cell, Timeline, SpreadChips)', () => {
  it('Mac: Ctrl+левая — жест не начинается, слушатели не вешаются', () => {
    setMacForTest(true)
    drag.start(down({ ctrlKey: true }), () => ({ kind: 'photos', ids: ['a'] }))
    expect(listeners.add).toEqual([])
  })
  it('Mac: правая кнопка — тоже нет', () => {
    setMacForTest(true)
    drag.start(down({ button: 2 }), () => ({ kind: 'photos', ids: ['a'] }))
    expect(listeners.add).toEqual([])
  })
  it('Mac: обычная левая — жест есть', () => {
    setMacForTest(true)
    drag.start(down(), () => ({ kind: 'photos', ids: ['a'] }))
    expect(listeners.add).toEqual(GESTURE)
  })
  it('Windows: Ctrl+левая — жест есть (без изменений)', () => {
    setMacForTest(false)
    drag.start(down({ ctrlKey: true }), () => ({ kind: 'photos', ids: ['a'] }))
    expect(listeners.add).toEqual(GESTURE)
  })
})

describe('drive (PlaceLayer, SelFrame, GroupFrame, текст)', () => {
  it('Mac: Ctrl+левая — перенос не начинается, холст не занят', () => {
    setMacForTest(true)
    drive(
      down({ ctrlKey: true }),
      root,
      () => 1,
      () => {},
      async () => {},
    )
    expect(listeners.add).toEqual([])
    expect(canvas.busy).toBe(false)
  })
  it('Windows: Ctrl+левая — перенос начинается', () => {
    setMacForTest(false)
    drive(
      down({ ctrlKey: true }),
      root,
      () => 1,
      () => {},
      async () => {},
    )
    expect(listeners.add).toContain('pointermove')
    expect(canvas.busy).toBe(true)
    canvas.busy = false
  })
})

describe('выделение Ctrl', () => {
  it('Mac: Ctrl не добавляет, Cmd и Shift добавляют', () => {
    setMacForTest(true)
    expect(additive(down({ ctrlKey: true }))).toBe(false)
    expect(additive(down({ metaKey: true }))).toBe(true)
    expect(additive(down({ shiftKey: true }))).toBe(true)
    expect(isSelectClick(down({ ctrlKey: true }))).toBe(false)
    expect(isSelectClick(down({ metaKey: true }))).toBe(true)
  })
  it('лента фото: Ctrl на Mac заменяет выделение, Cmd — добавляет', () => {
    setMacForTest(true)
    store.selected.clear()
    store.click('a', down() as MouseEvent)
    store.click('b', down({ ctrlKey: true }) as MouseEvent)
    expect([...store.selected]).toEqual(['b'])
    store.click('c', down({ metaKey: true }) as MouseEvent)
    expect([...store.selected].sort()).toEqual(['b', 'c'])
  })
  it('лента фото, Windows: Ctrl добавляет', () => {
    setMacForTest(false)
    store.selected.clear()
    store.click('a', down() as MouseEvent)
    store.click('b', down({ ctrlKey: true }) as MouseEvent)
    expect([...store.selected].sort()).toEqual(['a', 'b'])
  })
})

describe('выбор нижнего объекта (Alt+щелчок)', () => {
  it('приведённая кнопка 2 (Ctrl на Mac) — план «уступить»', () => {
    const plan = pressPlan({ button: 2, alt: true, additive: false, stack: [], cur: null, picked: null })
    expect(plan).toEqual({ press: 'pass', click: 'pass', hint: false })
  })
})
