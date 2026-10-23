import { describe, expect, it } from 'vitest'
import { cutState, joinState, VIG_CUT_WHY, VIG_WHY } from './timeline'
import type { SpreadView } from '../../base/core/types'
import { canVary, deleteLabel, insertItems, setupItem, stripItems, varyLabel, vignetteItem } from './spreadmenu'

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

describe('stripItems (094)', () => {
  const all = () => true
  const cut = { show: true, ok: true, why: '' }
  const join = { ok: true, why: '' }
  const grayCut = { show: true, ok: false, why: VIG_CUT_WHY }

  it('Название… / Разрезать перед этим фото / Склеить со следующим: одна группа, по уровню, серые с причиной', () => {
    expect(stripItems(1, all, cut, join)).toEqual({ rename: true, cut: { ok: true, why: '' }, join: { ok: true, why: '' } })
    // Без «Разреза и склейки» — только название; без «Названий» — только разрез и склейка.
    expect(stripItems(1, (f) => f === 'names', cut, join)).toEqual({ rename: true, cut: null, join: null })
    expect(stripItems(1, (f) => f === 'timeline', cut, join).rename).toBe(false)
    // Серые несут причину черты.
    expect(stripItems(1, all, grayCut, { ok: false, why: 'Нельзя склеить: вместе 70 фото, максимум 64' })).toEqual({
      rename: true,
      cut: { ok: false, why: VIG_CUT_WHY },
      join: { ok: false, why: 'Нельзя склеить: вместе 70 фото, максимум 64' },
    })
    // Первое фото (show = нет) и последняя группа (join не задан) — пунктов нет.
    expect(stripItems(1, all, { ...cut, show: false }, undefined)).toEqual({ rename: true, cut: null, join: null })
  })

  it('ручной разворот (103): «Склеить» и «Разрезать» активны, виньетка — серые с причиной', () => {
    const sp = (extra: object) => ({ id: 'a', photos: ['p1', 'p2', 'p3'], rects: [], ...extra }) as unknown as SpreadView
    const spreads = [sp({ grid: true }), sp({ id: 'b' })]
    const items = stripItems(1, all, cutState(spreads[0], 1), joinState(spreads, 0, 64))
    expect(items.cut).toEqual({ ok: true, why: '' })
    expect(items.join).toEqual({ ok: true, why: '' })
    const vig = [sp({ grid: true, kind: 'vignette' }), sp({ id: 'b' })]
    const gray = stripItems(1, all, cutState(vig[0], 1), joinState(vig, 0, 64))
    expect(gray.cut).toEqual({ ok: false, why: VIG_CUT_WHY })
    expect(gray.join).toEqual({ ok: false, why: VIG_WHY })
  })

  it('при нескольких выделенных их нет', () => {
    expect(stripItems(2, all, cut, join)).toEqual({ rename: false, cut: null, join: null })
    expect(stripItems(5, all, cut, join)).toEqual({ rename: false, cut: null, join: null })
  })
})

describe('insertItems (103)', () => {
  const spread = { one: 'разворот' }
  it('одна группа — «перед» и «после», по порядку', () => {
    expect(insertItems(1, spread)).toEqual([
      { where: 'before', label: 'Вставить разворот перед' },
      { where: 'after', label: 'Вставить разворот после' },
    ])
  })
  it('у альбома листов — «лист»', () => {
    expect(insertItems(1, { one: 'лист' }).map((i) => i.label)).toEqual(['Вставить лист перед', 'Вставить лист после'])
  })
  it('несколько выделенных групп — пунктов нет', () => {
    expect(insertItems(2, spread)).toEqual([])
    expect(insertItems(0, spread)).toEqual([])
  })
})
