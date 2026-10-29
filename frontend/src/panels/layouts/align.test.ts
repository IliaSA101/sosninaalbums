import { describe, expect, it } from 'vitest'
import {
  ALIGN_OPS,
  ALIGN_TO,
  alignBlock,
  alignNeed,
  alignSlot,
  alignToBlock,
  isAlignOp,
  isDist,
  normAlignTo,
  normAlignToOne,
} from './align'

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

describe('две опоры (спека 100 п. 4)', () => {
  it('слот по числу выделенных: 0 и 1 — одна, от 2 — много', () => {
    expect(alignSlot(0)).toBe('one')
    expect(alignSlot(1)).toBe('one')
    expect(alignSlot(2)).toBe('many')
    expect(alignSlot(9)).toBe('many')
  })
  it('«Выделения» у одного — блок с подсказкой, у нескольких и прочие опоры — можно', () => {
    expect(alignToBlock('sel', 1)).toBe('Относительно выделения — от 2 объектов: Shift+щелчок добавит ещё')
    expect(alignToBlock('sel', 0)).not.toBeNull()
    expect(alignToBlock('sel', 2)).toBeNull()
    expect(alignToBlock('page', 1)).toBeNull()
    expect(alignToBlock('spread', 1)).toBeNull()
  })
  it('опора одного: по умолчанию «Страницы», sel и чужое из хранилища — тоже', () => {
    expect(normAlignToOne(null)).toBe('page')
    expect(normAlignToOne('sel')).toBe('page')
    expect(normAlignToOne('чужое')).toBe('page')
    expect(normAlignToOne('spread')).toBe('spread')
  })
})
