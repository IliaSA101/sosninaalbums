import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { View } from '../../base/core/types'

// Форма мест (спека 044): переключатель шлёт ID всех выделенных, скругление — один вызов, «Место ▸ Круглое»
// шлёт AddPlace(…, 'oval') и выделяет новое место. Go подменён моделью с формами мест.
const shapes: Record<string, string> = {}
const rounds: Record<string, number> = {}
let rev = 1
let extra = false // после AddPlace в разведе появляется место p9
const calls: unknown[][] = []

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
        places: [...['p1', 'p2', 'p3'], ...(extra ? ['p9'] : [])].map((id, i) => ({
          id,
          photo: '',
          fit: '',
          spine: false,
          z: i,
          x: 20 + i * 60,
          y: 50,
          w: 50,
          h: 50,
          rot: 0,
          shape: shapes[id] ?? '',
          round: rounds[id] ?? 0,
          radius: 0,
        })),
      },
    ],
    settings: { pageW: 300, pageH: 300, margin: 12, gap: 3, layflat: true, foldAny: false, dpi: 300, bg: '' },
    folders: [],
    rev,
    safe: [],
    backdropUsed: [],
  }) as unknown as View

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  AddPlace: async (...a: unknown[]) => {
    calls.push(['AddPlace', ...a])
    extra = true
    if (a[2] === 'oval') shapes.p9 = 'oval'
    if (a[2] === 'rounded') rounds.p9 = 10
    rev++
    return view()
  },
  SetPlaceShape: async (...a: unknown[]) => {
    calls.push(['SetPlaceShape', ...a])
    for (const id of a[1] as string[]) {
      // Как grid.SetPlaceShape: '' снимает скругление, rounded — своё или 10, oval — круг без скругления.
      shapes[id] = a[2] === 'oval' ? 'oval' : ''
      rounds[id] = a[2] === 'rounded' ? rounds[id] || 10 : 0
    }
    rev++
    return view()
  },
  SetPlaceRound: async (...a: unknown[]) => {
    calls.push(['SetPlaceRound', ...a])
    for (const id of a[1] as string[]) if (shapes[id] !== 'oval') rounds[id] = a[2] as number
    rev++
    return view()
  },
}))
vi.mock('../text/fonts', () => ({ loadFonts: async () => {} }))

const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('./placesel.svelte')

beforeEach(() => {
  calls.length = 0
  extra = false
  for (const k of Object.keys(shapes)) delete shapes[k]
  for (const k of Object.keys(rounds)) delete rounds[k]
  rev = 1
  store.apply(view())
  placesel.clear()
})

describe('placesel: форма мест (044)', () => {
  it('setShape(true) шлёт ID всех выделенных одним вызовом', async () => {
    placesel.select(['p1', 'p2'])
    await placesel.setShape('oval')
    expect(calls).toEqual([['SetPlaceShape', 's1', ['p1', 'p2'], 'oval']])
    expect(placesel.allOval).toBe(true)
    expect(placesel.allRect).toBe(false)
    expect(placesel.allRounded).toBe(false)
  })

  it('повтор той же формы — без вызова; «Прямоугольник» возвращает ""', async () => {
    placesel.select(['p1'])
    await placesel.setShape('oval')
    calls.length = 0
    await placesel.setShape('oval')
    expect(calls).toEqual([])
    await placesel.setShape('rect')
    expect(calls).toEqual([['SetPlaceShape', 's1', ['p1'], '']])
  })

  it('смесь: ни одна кнопка не подсвечена, нажатие любой доводит всех', async () => {
    shapes.p1 = 'oval'
    store.apply(view())
    placesel.select(['p1', 'p2'])
    expect(placesel.allOval || placesel.allRect || placesel.allRounded).toBe(false)
    await placesel.setShape('rect')
    expect(calls).toEqual([['SetPlaceShape', 's1', ['p1', 'p2'], '']])
  })

  it('ничего не выделено — вызова нет', async () => {
    await placesel.setShape('oval')
    await placesel.setShape('round')
    await placesel.setRound(10)
    expect(calls).toEqual([])
  })

  it('setRound — один вызов с ID выделенных; у кругов строки скругления нет', async () => {
    placesel.select(['p1', 'p2'])
    expect(placesel.hasRound).toBe(true)
    await placesel.setRound(12)
    expect(calls).toEqual([['SetPlaceRound', 's1', ['p1', 'p2'], 12]])
    expect(placesel.round).toBe(12)
    await placesel.setShape('oval')
    expect(placesel.hasRound).toBe(false)
    calls.length = 0
    await placesel.setRound(30) // у круга холостая: вызова нет
    expect(calls).toEqual([])
  })

  it('предпросмотр ползунка: радиус только у выделенных, null — прежний', () => {
    placesel.select(['p1'])
    placesel.liveRound = 10
    expect(placesel.liveRadius(['p1'], 0, 50, 50)).toBeCloseTo(5)
    expect(placesel.liveRadius(['p2'], 0, 50, 50)).toBe(0)
    placesel.liveRound = null
    expect(placesel.liveRadius(['p1'], 2, 50, 50)).toBe(2)
  })

  it('«Место ▸ Круглое»: AddPlace(…, "oval") и новое место выделено', async () => {
    await placesel.addPlace('oval')
    expect(calls[0].slice(0, 1)).toEqual(['AddPlace'])
    expect(calls[0][3]).toBe('oval')
    expect(placesel.refs).toEqual(['p9'])
  })

  it('«Место ▸ Прямоугольное» шлёт пустую форму', async () => {
    await placesel.addPlace()
    expect(calls[0][3]).toBe('')
    calls.length = 0
    await placesel.addPlace('rect')
    expect(calls[0][3]).toBe('')
  })

  it('«Место ▸ Скруглённое»: AddPlace(…, "rounded"), новое место выделено, в карточке скругление 10 %', async () => {
    await placesel.addPlace('round')
    expect(calls[0][3]).toBe('rounded')
    expect(placesel.refs).toEqual(['p9'])
    expect(placesel.allRounded).toBe(true)
    expect(placesel.hasRound).toBe(true)
    expect(placesel.round).toBe(10)
  })
})

describe('placesel: скруглённое ▢ (076)', () => {
  it('allRect / allRounded / allOval: 0 % — прямоугольник, 25 % — скруглённое, круг — своё', () => {
    rounds.p2 = 25
    shapes.p3 = 'oval'
    store.apply(view())
    placesel.select(['p1'])
    expect([placesel.allRect, placesel.allRounded, placesel.allOval]).toEqual([true, false, false])
    placesel.select(['p2'])
    expect([placesel.allRect, placesel.allRounded, placesel.allOval]).toEqual([false, true, false])
    placesel.select(['p3'])
    expect([placesel.allRect, placesel.allRounded, placesel.allOval]).toEqual([false, false, true])
    placesel.select(['p1', 'p2', 'p3'])
    expect([placesel.allRect, placesel.allRounded, placesel.allOval]).toEqual([false, false, false])
  })

  it('смесь ▭ / 25 % / ◯ → ▢: один вызов на всех, 25 % у второго остаётся', async () => {
    rounds.p2 = 25
    shapes.p3 = 'oval'
    store.apply(view())
    placesel.select(['p1', 'p2', 'p3'])
    await placesel.setShape('round')
    expect(calls).toEqual([['SetPlaceShape', 's1', ['p1', 'p2', 'p3'], 'rounded']])
    expect(rounds).toEqual({ p1: 10, p2: 25, p3: 10 })
    expect(placesel.allRounded).toBe(true)
  })

  it('все уже ▢ — вызова нет', async () => {
    rounds.p1 = 12
    rounds.p2 = 3
    store.apply(view())
    placesel.select(['p1', 'p2'])
    await placesel.setShape('round')
    expect(calls).toEqual([])
  })

  it('туда и обратно: 25 % → ◯ → ▢ даёт 10 %; ▭ снимает скругление, подсветка идёт за значением', async () => {
    rounds.p1 = 25
    store.apply(view())
    placesel.select(['p1'])
    await placesel.setShape('oval')
    await placesel.setShape('round')
    expect(placesel.round).toBe(10)
    await placesel.setShape('rect')
    expect(placesel.allRect).toBe(true)
    expect(placesel.round).toBe(0)
    await placesel.setRound(15)
    expect(placesel.allRounded).toBe(true)
    expect(placesel.allRect).toBe(false)
  })

  it('у круга строки «Скругление, %» нет, у ▢ и ▭ есть', async () => {
    placesel.select(['p1'])
    expect(placesel.hasRound).toBe(true)
    await placesel.setShape('round')
    expect(placesel.hasRound).toBe(true)
    await placesel.setShape('oval')
    expect(placesel.hasRound).toBe(false)
  })
})
