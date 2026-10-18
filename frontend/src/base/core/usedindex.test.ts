import { describe, expect, it } from 'vitest'
import { UsedIndex, keepSet } from './usedindex'
import type { SpreadView } from './types'

const sp = (id: string, ...photos: string[]) => ({ id, photos }) as unknown as SpreadView
const sorted = (s: Set<string>) => [...s].sort()

describe('keepSet', () => {
  it('тот же состав — тот же объект, иной — новый', () => {
    const a = new Set(['x', 'y'])
    expect(keepSet(a, ['y', 'x'])).toBe(a)
    expect(keepSet(a, ['x'])).not.toBe(a)
    expect([...keepSet(a, ['x', 'z'])].sort()).toEqual(['x', 'z'])
    expect(keepSet(new Set(), [])).toBeInstanceOf(Set)
  })
})

describe('UsedIndex', () => {
  it('собирает фото всех листов', () => {
    const u = new UsedIndex()
    expect(sorted(u.update([sp('a', 'x', 'y'), sp('b', 'z')]))).toEqual(['x', 'y', 'z'])
  })

  it('пусто и ровно один разворот', () => {
    const u = new UsedIndex()
    expect(sorted(u.update([]))).toEqual([])
    expect(sorted(u.update([sp('a', 'x')]))).toEqual(['x'])
  })

  it('photos: null от Go не роняет подсчёт', () => {
    const u = new UsedIndex()
    const bad = { id: 'n', photos: null } as unknown as SpreadView
    expect(sorted(u.update([bad, sp('a', 'x')]))).toEqual(['x'])
    expect(sorted(u.update([sp('a', 'x')]))).toEqual(['x'])
  })

  it('тот же массив — тот же набор', () => {
    const u = new UsedIndex()
    const list = [sp('a', 'x')]
    expect(u.update(list)).toBe(u.update(list))
  })

  it('правка без смены состава (другой объект разворота, те же фото) — прежний набор', () => {
    const u = new UsedIndex()
    const a = sp('a', 'x', 'y')
    const b = sp('b', 'z')
    const first = u.update([a, b])
    expect(u.update([{ ...a }, b])).toBe(first)
  })

  it('фото ушло, пришло, переехало между листами', () => {
    const u = new UsedIndex()
    const a = sp('a', 'x', 'y')
    const b = sp('b', 'z')
    u.update([a, b])
    expect(sorted(u.update([sp('a', 'x'), b]))).toEqual(['x', 'z'])
    expect(sorted(u.update([sp('a', 'x', 'q'), sp('b')]))).toEqual(['q', 'x'])
    // x переехало с a на b: состав тот же, набор прежний
    const cur = u.update([sp('a', 'x', 'q'), sp('b')])
    expect(sorted(u.update([sp('a', 'q'), sp('b', 'x')]))).toEqual(sorted(cur))
  })

  it('одно фото на двух листах: уходит с последнего', () => {
    const u = new UsedIndex()
    const a = sp('a', 'x')
    const b = sp('b', 'x')
    u.update([a, b])
    expect(sorted(u.update([b]))).toEqual(['x'])
    expect(sorted(u.update([]))).toEqual([])
  })

  it('удаление, вставка и перестановка листов', () => {
    const u = new UsedIndex()
    const a = sp('a', '1')
    const b = sp('b', '2')
    const c = sp('c', '3')
    u.update([a, b, c])
    expect(sorted(u.update([c, a]))).toEqual(['1', '3'])
    expect(sorted(u.update([c, b, a]))).toEqual(['1', '2', '3'])
  })

  it('совпадает с полным пересчётом на случайных правках', () => {
    const u = new UsedIndex()
    let list: SpreadView[] = []
    let seed = 7
    const rnd = (n: number) => (seed = (seed * 1103515245 + 12345) & 0x7fffffff) % n
    for (let step = 0; step < 300; step++) {
      const next = list.slice()
      if (rnd(3) === 0 && next.length) next.splice(rnd(next.length), 1)
      else next.splice(rnd(next.length + 1), 0, sp('s' + step, ...Array.from({ length: rnd(4) }, () => 'p' + rnd(12))))
      list = next
      expect(sorted(u.update(list))).toEqual(sorted(new Set(list.flatMap((s) => s.photos))))
    }
  })
})
