import { describe, expect, it } from 'vitest'
import { deleteLabel, menuPos } from './spreadmenu'

const view = { w: 1000, h: 600 }
const size = { w: 200, h: 36 }

describe('menuPos', () => {
  it('влезает — остаётся у курсора', () => {
    expect(menuPos(300, 200, size, view)).toEqual({ x: 300, y: 200 })
  })
  it('у правого и нижнего края — сдвигается внутрь с отступом', () => {
    expect(menuPos(950, 590, size, view)).toEqual({ x: 792, y: 556 })
  })
  it('у левого и верхнего края — не меньше отступа', () => {
    expect(menuPos(-5, 0, size, view)).toEqual({ x: 8, y: 8 })
  })
})

describe('deleteLabel', () => {
  it('одна группа — единственное число, несколько — с числом', () => {
    expect(deleteLabel(1)).toBe('Удалить разворот')
    expect(deleteLabel(3)).toBe('Удалить развороты (3)')
  })
})
