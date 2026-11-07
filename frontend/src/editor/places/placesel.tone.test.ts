import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { View } from '../../base/core/types'

// Ч/б и тон (068): «Сделать ч/б / цветным» шлёт ID выделенных фото одним вызовом, «всем или никому» решает Go
// (здесь — его модель); пустое выделение вызова не даёт. Подпись пункта берёт allGray из View.
const gray: Record<string, boolean> = {}
const tone: Record<string, { b: number; c: number }> = {}
let rev = 1
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
        photos: ['a', 'b', 'c'],
        rects: [],
        frames: ['a', 'b', 'c'].map((id) => ({
          gray: !!gray[id],
          bright: tone[id]?.b ?? 0,
          contrast: tone[id]?.c ?? 0,
          filter: gray[id] ? 'grayscale(1)' : '',
          cellRot: 0,
        })),
        texts: [],
        captions: [],
        backdrops: [],
        places: ['a', 'b', 'c'].map((id, i) => ({
          id: `p${i}`,
          photo: id,
          fit: '',
          spine: false,
          z: i,
          x: 20 + i * 60,
          y: 50,
          w: 50,
          h: 50,
          rot: 0,
          shape: '',
          round: 0,
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
  SetGray: async (...a: unknown[]) => {
    calls.push(['SetGray', ...a])
    const ids = a[0] as string[]
    const all = ids.every((id) => gray[id])
    for (const id of ids) gray[id] = !all
    rev++
    return view()
  },
  SetTone: async (...a: unknown[]) => {
    calls.push(['SetTone', ...a])
    tone[a[0] as string] = { b: a[1] as number, c: a[2] as number }
    rev++
    return view()
  },
}))
vi.mock('../text/fonts', () => ({ loadFonts: async () => {} }))

const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('./placesel.svelte')

beforeEach(() => {
  calls.length = 0
  for (const k of Object.keys(gray)) delete gray[k]
  for (const k of Object.keys(tone)) delete tone[k]
  rev = 1
  store.apply(view())
  placesel.clear()
})

describe('placesel: ч/б (068)', () => {
  it('gray() шлёт ID всех выделенных фото одним вызовом', async () => {
    placesel.select(['p0', 'p1'])
    await placesel.gray()
    expect(calls).toEqual([['SetGray', ['a', 'b']]])
    expect(placesel.allGray).toBe(true)
  })

  it('смесь (2 ч/б, 1 цветное): первый вызов — всем ч/б, второй — всем цветные', async () => {
    gray.a = true
    gray.b = true
    store.apply(view())
    placesel.select(['p0', 'p1', 'p2'])
    expect(placesel.allGray).toBe(false)
    await placesel.gray()
    expect(placesel.allGray).toBe(true)
    await placesel.gray()
    expect(placesel.allGray).toBe(false)
    expect(['a', 'b', 'c'].some((id) => gray[id])).toBe(false)
  })

  it('ничего не выделено — вызова нет, подпись «Сделать ч/б»', async () => {
    await placesel.gray()
    expect(calls).toEqual([])
    expect(placesel.allGray).toBe(false)
  })

  it('выделено пустое место (без фото) — вызова нет', async () => {
    const v = view()
    v.spreads[0].places![0].photo = ''
    store.apply(v)
    placesel.select(['p0'])
    await placesel.gray()
    expect(calls).toEqual([])
  })
})
