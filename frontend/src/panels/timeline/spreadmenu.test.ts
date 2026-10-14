import { describe, expect, it } from 'vitest'
import { canVary, deleteLabel, menuPos, setupItem, varyLabel, vignetteItem } from './spreadmenu'

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
  it('у листа виньетки — «лист»', () => {
    expect(deleteLabel(1, true)).toBe('Удалить лист')
    expect(deleteLabel(3, true)).toBe('Удалить листы (3)')
  })
})

describe('varyLabel (041)', () => {
  it('одна группа — без числа, несколько — с числом, как «Удалить развороты (N)»', () => {
    expect(varyLabel(1)).toBe('Разнообразить раскладки')
    expect(varyLabel(3)).toBe('Разнообразить раскладки (3)')
  })
})

describe('canVary (041)', () => {
  it('есть что менять (anyVaries), не лист проекта, функция на уровне — да', () => {
    expect(canVary(true, false, true)).toBe(true)
  })
  it('все цели — сетки/виньетки (varies = false), лист проекта и уровень без функции — нет', () => {
    expect(canVary(false, false, true)).toBe(false)
    expect(canVary(true, true, true)).toBe(false)
    expect(canVary(true, false, false)).toBe(false)
  })
})

describe('setupItem', () => {
  it('только у одной виньетки на уровне с функцией', () => {
    expect(setupItem(1, 'vignette')).toEqual({ act: 'setup', label: 'Настроить виньетку…' })
    expect(setupItem(1, undefined)).toBeNull()
    expect(setupItem(2, 'vignette')).toBeNull()
    expect(setupItem(1, 'vignette', false)).toBeNull()
  })
})

describe('vignetteItem', () => {
  it('обычный лист — «Сделать виньеткой», виньетка — «Обычный разворот»', () => {
    expect(vignetteItem(1, undefined)).toEqual({ act: 'make', label: 'Сделать виньеткой' })
    expect(vignetteItem(1, '')).toEqual({ act: 'make', label: 'Сделать виньеткой' })
    expect(vignetteItem(1, 'vignette')).toEqual({ act: 'clear', label: 'Обычный разворот' })
  })
  it('в проекте-листе — «Обычный лист»', () => {
    expect(vignetteItem(1, 'vignette', true)?.label).toBe('Обычный лист')
  })
  it('на «Простом» (функции нет) «Сделать виньеткой» не предлагается, готовую виньетку вернуть можно', () => {
    expect(vignetteItem(1, undefined, false, false)).toBeNull()
    expect(vignetteItem(1, 'vignette', false, false)).toEqual({ act: 'clear', label: 'Обычный разворот' })
  })
  it('несколько выделенных групп — пункта нет', () => {
    expect(vignetteItem(2, 'vignette')).toBeNull()
    expect(vignetteItem(3, undefined)).toBeNull()
  })
})
