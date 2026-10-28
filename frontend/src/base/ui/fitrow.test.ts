import { describe, expect, it } from 'vitest'
import { fitGroups, type RowGroup } from './fitrow'

const g = (id: string, w: number | undefined, prio: number): RowGroup => ({ id, w, prio })
const list = [g('a', 100, 10), g('b', 100, 20), g('auto', 100, Infinity), g('c', 100, 30)]

describe('fitGroups', () => {
  it('всё помещается — ничего не прячет', () => {
    // 4·100 + 3·8 = 424
    expect(fitGroups(list, 424, 8, 30).size).toBe(0)
  })

  it('прячет по приоритету с учётом ширины «⋯»', () => {
    // Без a: 300 + 16 = 316, плюс «⋯» 38 = 354
    expect([...fitGroups(list, 360, 8, 30)]).toEqual(['a'])
    // 353 не хватает на «⋯» — уходит и b
    expect([...fitGroups(list, 353, 8, 30)].sort()).toEqual(['a', 'b'])
  })

  it('«Автосборка» (Infinity) не уходит даже при нулевой ширине', () => {
    const h = fitGroups(list, 0, 8, 30)
    expect(h.has('auto')).toBe(false)
    expect([...h].sort()).toEqual(['a', 'b', 'c'])
  })

  it('неизвестная ширина — показать', () => {
    const u = [g('a', undefined, 10), g('b', undefined, 20)]
    expect(fitGroups(u, 10, 8, 30).size).toBe(0)
  })

  it('при равном приоритете правее уходит раньше', () => {
    const e = [g('l', 100, 5), g('r', 100, 5)]
    expect([...fitGroups(e, 150, 8, 30)]).toEqual(['r'])
  })
})
