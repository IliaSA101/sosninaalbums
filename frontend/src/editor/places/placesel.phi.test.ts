import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { View } from '../../base/core/types'

// Угол рамки группы (спека 018): φ живёт в placesel, а не в проекте. Go подменён моделью с углами мест:
// RotateGroup прибавляет угол и поднимает rev; пустой ответ «паники» возвращает прежний вид без ошибки.
// Окно — настоящий EventTarget с таймерами: жест drive слушает pointermove/pointerup на window.
class FakeWindow extends EventTarget {
  setTimeout = setTimeout
  clearTimeout = clearTimeout
}
vi.stubGlobal('window', new FakeWindow())

const rots: Record<string, number> = {}
let rev = 1
let panic = false
const calls: string[] = []

const view = (): View =>
  ({
    spreads: [
      {
        id: 's1',
        w: 600,
        h: 300,
        mid: 300,
        grid: true,
        photos: [],
        rects: [],
        frames: [],
        texts: [],
        captions: [],
        backdrops: [],
        places: ['p1', 'p2', 'p3', 'p4'].map((id, i) => ({
          id,
          photo: '',
          fit: '',
          spine: false,
          z: i,
          x: 20 + i * 60,
          y: 50,
          w: 50,
          h: 50,
          rot: rots[id] ?? 0,
        })),
      },
    ],
    settings: { pageW: 300, pageH: 300, margin: 12, gap: 3, layflat: true, dpi: 300, bg: '' },
    folders: [],
    rev,
    safe: [],
    backdropUsed: [],
  }) as unknown as View

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  RotateGroup: async (_s: string, ids: string[], deg: number) => {
    calls.push('rotate:' + deg)
    await Promise.resolve() // Go отвечает не сразу
    if (!panic) {
      for (const id of ids) rots[id] = (rots[id] ?? 0) + deg
      rev++
    }
    return view()
  },
  TransformGroup: async () => {
    calls.push('transform')
    rev++
    return view()
  },
  ScaleGroupAxes: async () => {
    calls.push('axes')
    rev++
    return view()
  },
}))
vi.mock('../text/fonts', () => ({ loadFonts: async () => {} }))

const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('./placesel.svelte')
const { rotateGroup, setGroupField, startGroupScale } = await import('./groupgesture')

// Вид от Go с заданными углами (Undo/Redo приходят так же, с другим rev).
const setView = (r: Record<string, number>, v = ++rev) => {
  for (const k of ['p1', 'p2', 'p3', 'p4']) rots[k] = r[k] ?? 0
  rev = v
  store.apply(view())
}
const tick = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  calls.length = 0
  panic = false
  rev = 1
  setView({}, 1)
  placesel.clear()
})

describe('placesel: угол рамки группы φ', () => {
  it('поворот → φ = угол; Undo → общий угол членов (0), Redo → снова φ', () => {
    placesel.select(['p1', 'p2'])
    setView({ p1: 20, p2: 30 })
    placesel.noteTurn(20, placesel.key)
    expect(placesel.phi).toBe(20)
    setView({ p1: 0, p2: 10 }) // Undo
    expect(placesel.phi).toBe(0)
    expect(placesel.mixed).toBe(true)
    setView({ p1: 20, p2: 30 }) // Redo
    expect(placesel.phi).toBe(20)
  })

  it('смена refs → общий угол новых членов', () => {
    setView({ p1: 20, p2: 30, p3: 45, p4: 45 })
    placesel.select(['p1', 'p2'])
    placesel.noteTurn(20, placesel.key)
    placesel.select(['p3', 'p4'])
    expect(placesel.phi).toBe(45)
  })

  it('liveTurn = 50 → φ = 50, рамка рисуется под −40', () => {
    placesel.select(['p1', 'p2'])
    placesel.liveTurn = 50
    expect(placesel.phi).toBe(50)
    expect(placesel.frame?.rot).toBe(-40)
    expect(placesel.mixed).toBe(false)
  })

  it('clear() → 0 и без запомненного поворота', () => {
    placesel.select(['p1', 'p2'])
    setView({ p1: 20, p2: 30 })
    placesel.noteTurn(20, placesel.key)
    placesel.clear()
    expect(placesel.lastTurn).toBeNull()
    expect(placesel.phi).toBe(0)
  })

  it('новое выделение сбрасывает поворот; вернувшись к прежнему, φ — общий угол, а не старый', () => {
    placesel.select(['p1', 'p2'])
    setView({ p1: 20, p2: 30 })
    placesel.noteTurn(20, placesel.key)
    placesel.select(['p3'])
    expect(placesel.lastTurn).toBeNull()
    placesel.select(['p1', 'p2'])
    expect(placesel.phi).toBe(0)
  })

  it('Shift-щелчок меняет ключ — поворот забыт (apply)', () => {
    placesel.select(['p1', 'p2'])
    setView({ p1: 20, p2: 30 })
    placesel.noteTurn(20, placesel.key)
    placesel.pick('p3', true)
    expect(placesel.lastTurn).toBeNull()
  })

  it('noteTurn с ключом прежнего выделения игнорируется', () => {
    placesel.select(['p1', 'p2'])
    const old = placesel.key
    placesel.select(['p3', 'p4'])
    placesel.noteTurn(20, old)
    expect(placesel.lastTurn).toBeNull()
  })
})

describe('rotateGroup и поля карточки', () => {
  it('поворот запоминает φ; паника Go (rev не вырос) — нет', async () => {
    placesel.select(['p1', 'p2'])
    expect(await rotateGroup(20, 100, 100)).toBe(true)
    expect(placesel.phi).toBe(20)
    panic = true
    expect(await rotateGroup(10, 100, 100)).toBe(false)
    expect(placesel.phi).toBe(20)
  })

  it('удержанная стрелка в «Угол»: 1 и 2 подряд — итог 2°, не 3°', async () => {
    placesel.select(['p1', 'p2'])
    const a = setGroupField('a', 1)
    const b = setGroupField('a', 2)
    await Promise.all([a, b])
    expect(calls).toEqual(['rotate:1', 'rotate:1'])
    expect(rots.p1).toBe(2)
    expect(placesel.phi).toBe(2)
  })

  it('поле не-группы и мусор — без вызова', async () => {
    placesel.select(['p1'])
    await setGroupField('a', 5)
    placesel.select(['p1', 'p2'])
    await setGroupField('x', Number.NaN)
    expect(calls).toEqual([])
  })
})

describe('жест группы и чужая правка', () => {
  const ptr = (type: string, x: number, y: number) => Object.assign(new Event(type), { button: 0, clientX: x, clientY: y })
  const root = { getBoundingClientRect: () => ({ width: 600, left: 0, top: 0 }) } as HTMLElement
  const drag = async (bump: boolean) => {
    placesel.select(['p1', 'p2'])
    startGroupScale(ptr('pointerdown', 0, 0) as PointerEvent, root, 'se')
    window.dispatchEvent(ptr('pointermove', 40, 40))
    if (bump) setView({ p1: 0, p2: 0 }) // Ctrl+Z посреди жеста
    window.dispatchEvent(ptr('pointerup', 40, 40))
    await tick()
  }

  it('без чужих правок — один вызов Go', async () => {
    await drag(false)
    expect(calls).toEqual(['transform'])
  })

  it('rev сменился посреди жеста — Go не вызывается, предпросмотр сброшен', async () => {
    await drag(true)
    expect(calls).toEqual([])
    expect(placesel.live).toBeNull()
    expect(placesel.liveTurn).toBeNull()
  })
})
