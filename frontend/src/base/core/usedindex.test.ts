import { describe, expect, it } from 'vitest'
import { UsedIndex, keepSet, useCountOf, usesOf } from './usedindex'
import vector from '../../../../testdata/parity/uses.json'
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

describe('UsedIndex: копии (098)', () => {
  it('фото, стоящее только копией, — в наборе по ID файла и по ID записи', () => {
    const u = new UsedIndex()
    const set = u.update([sp('a', 'f1-2')])
    expect(sorted(set)).toEqual(['f1', 'f1-2'])
    expect(u.places('f1')).toBe(1)
    expect(u.places('f1-2')).toBe(1)
  })

  it('уходит последняя запись семьи — файл уходит из набора; пока есть другая — остаётся', () => {
    const u = new UsedIndex()
    const a = sp('a', 'f1')
    const b = sp('b', 'f1-2')
    u.update([a, b])
    expect(u.places('f1')).toBe(2)
    expect(sorted(u.update([b]))).toEqual(['f1', 'f1-2'])
    expect(sorted(u.update([]))).toEqual([])
    expect(u.places('f1')).toBe(0)
  })

  it('правка без смены состава оставляет прежний набор и прежнюю карту счёта', () => {
    const u = new UsedIndex()
    const a = sp('a', 'f1', 'f2-3')
    const b = sp('b', 'f4')
    const set = u.update([a, b])
    const uses = u.uses
    expect(u.update([{ ...a }, b])).toBe(set)
    expect(u.uses).not.toBe(uses) // карта пересоздаётся при любой правке листа, набор — нет
    expect([...u.uses.entries()].sort()).toEqual([...uses.entries()].sort())
  })

  it('стикер и фон меняют счёт, но не набор «в альбоме»', () => {
    const u = new UsedIndex()
    const plain = { id: 'a', photos: ['f1'] } as unknown as SpreadView
    const set = u.update([plain])
    const withBd = { id: 'a', photos: ['f1'], backdrops: [{ photo: 'f9' }] } as unknown as SpreadView
    expect(u.update([withBd])).toBe(set)
    expect(u.uses.get('f9')).toBe(1)
    expect(set.has('f9')).toBe(false)
  })
})

describe('useCountOf', () => {
  it('фон альбома добавляется один раз по ID файла', () => {
    const uses = new Map([['f1', 2]])
    expect(useCountOf(uses, 'f1-3', 'f1')).toBe(3)
    expect(useCountOf(uses, undefined, 'f1-2')).toBe(2)
    expect(useCountOf(uses, '', 'f2')).toBe(0)
  })
})

// Паритет с Go (uses_parity_test.go, countByView): тот же вектор, тот же счёт.
describe('паритет с Go: testdata/parity/uses.json', () => {
  const sheets = vector.sheets as unknown as SpreadView[]
  const u = new UsedIndex()
  u.update(sheets)
  it('в векторе есть файлы и фон альбома', () => {
    expect(vector.files.length).toBeGreaterThan(5)
    expect(vector.albumBg).not.toBe('')
  })
  for (const f of vector.files) {
    it(`файл ${f.id}: вхождений ${f.uses}, мест ${f.places}`, () => {
      expect(useCountOf(u.uses, vector.albumBg, f.id)).toBe(f.uses)
      expect(u.places(f.id)).toBe(f.places)
    })
  }
  it('фон листа с album: true не вхождение листа', () => {
    expect(usesOf(sheets[1])).not.toContain('ada021bb4a112acb')
  })
})
