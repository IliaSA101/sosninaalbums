import { describe, expect, it } from 'vitest'
import { ALIGN_OPS, ALIGN_TO, alignBlock, alignNeed, isAlignOp, isDist, normAlignTo } from './align'

describe('выравнивание: минимум как в Go', () => {
  it('к выделению: выравнивание от 2, распределение от 3', () => {
    expect(alignNeed('left', 'sel')).toBe(2)
    expect(alignNeed('vcenter', 'sel')).toBe(2)
    expect(alignNeed('distH', 'sel')).toBe(3)
    expect(alignNeed('distV', 'sel')).toBe(3)
  })
  it('к странице и развороту: выравнивание от 1, распределение от 2', () => {
    for (const to of ['page', 'spread'] as const) {
      expect(alignNeed('right', to), to).toBe(1)
      expect(alignNeed('distH', to), to).toBe(2)
    }
  })
  it('подсказка неактивного — сколько выделить', () => {
    expect(alignBlock('left', 'sel', 1)).toBe('Выделите 2 объекта')
    expect(alignBlock('distH', 'sel', 2)).toBe('Выделите 3 объекта')
    expect(alignBlock('distV', 'page', 1)).toBe('Выделите 2 объекта')
    expect(alignBlock('top', 'page', 0)).toBe('Выделите 1 объект')
    expect(alignBlock('left', 'sel', 2)).toBeNull()
    expect(alignBlock('distH', 'sel', 3)).toBeNull()
    expect(alignBlock('hcenter', 'spread', 1)).toBeNull()
  })
})

describe('состав и опора', () => {
  it('6 выравниваний и 2 распределения, op — те, что принимает Go', () => {
    expect(ALIGN_OPS.map((a) => a.op)).toEqual(['left', 'hcenter', 'right', 'top', 'vcenter', 'bottom', 'distH', 'distV'])
    expect(ALIGN_OPS.filter((a) => isDist(a.op))).toHaveLength(2)
    expect(ALIGN_TO.map((t) => t.to)).toEqual(['sel', 'page', 'spread'])
  })
  it('чужое из localStorage и меню отбрасывается', () => {
    expect(normAlignTo('page')).toBe('page')
    expect(normAlignTo('spread')).toBe('spread')
    expect(normAlignTo(null)).toBe('sel')
    expect(normAlignTo('всё')).toBe('sel')
    expect(isAlignOp('distV')).toBe(true)
    expect(isAlignOp('center')).toBe(false)
    expect(isAlignOp(undefined)).toBe(false)
  })
})
