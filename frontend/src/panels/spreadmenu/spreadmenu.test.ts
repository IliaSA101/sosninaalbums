import { describe, expect, it } from 'vitest'
import { cutState, joinState, VIG_CUT_WHY, VIG_WHY } from '../timeline/timeline'
import type { SpreadView } from '../../base/core/types'
import {
  canVary,
  deleteLabel,
  insertItems,
  setupItem,
  spreadMenu,
  stripItems,
  varyLabel,
  vignetteItem,
  type SpreadMenuSpec,
} from './spreadmenu'

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

describe('spreadMenu: дерево для MenuList (108)', () => {
  const base: SpreadMenuSpec = {
    count: 1,
    sheet: false,
    unit: { one: 'разворот' },
    can: () => true,
    rename: true,
    cut: { show: true, ok: true, why: '' },
    join: { ok: false, why: 'мало места' },
    insert: true,
    vignette: true,
    newVignette: false,
    vary: true,
    varies: true,
    del: true,
    how: 'Alt+щелчок',
  }
  const labels = (o: Partial<SpreadMenuSpec>) => spreadMenu({ ...base, ...o }).map((n) => n.label)
  const none = { rename: false, cut: undefined, join: undefined, insert: false, vignette: false, vary: false, del: false }

  it('порядок и тексты прежние: название, разрез, склейка, вставка, виньетка, разнообразить, удалить', () => {
    expect(labels({})).toEqual([
      'Название…',
      'Разрезать перед этим фото',
      'Склеить со следующим',
      'Вставить разворот перед',
      'Вставить разворот после',
      'Сделать виньеткой',
      'Разнообразить раскладки',
      'Удалить разворот',
    ])
  })
  it('серый пункт несёт причину в title, способ мышью — в hint', () => {
    const [, cut, join] = spreadMenu(base)
    expect(cut).toMatchObject({ id: 'cut', hint: 'Alt+щелчок', disabled: false, title: undefined })
    expect(join).toMatchObject({ id: 'join', disabled: true, title: 'мало места' })
  })
  it('удаление — danger; «Новая виньетка» только на плитке «+»', () => {
    expect(spreadMenu(base).at(-1)).toMatchObject({ id: 'delete', danger: true })
    expect(labels({ ...none, newVignette: true })).toEqual(['Новая виньетка'])
  })
  it('«Разворот макета «…»» (131): после «Новой виньетки», только с именем макета', () => {
    expect(labels({ ...none, newVignette: true, newLook: 'Светлая' })).toEqual(['Новая виньетка', 'Разворот макета «Светлая»'])
    expect(labels({ ...none, newLook: 'Светлая' })).toEqual(['Разворот макета «Светлая»']) // меню есть и без виньетки
    expect(labels({ ...none, newVignette: true })).toEqual(['Новая виньетка'])
    expect(labels({ ...none, newLook: '' })).toEqual([])
    expect(spreadMenu({ ...base, ...none, newLook: 'С' })[0]).toMatchObject({ id: 'newLook', icon: 'spread-add' })
  })
  it('виньетка: «Настроить…» перед «Обычный разворот»; arg несёт act', () => {
    const nodes = spreadMenu({ ...base, kind: 'vignette', cut: undefined, join: undefined, insert: false })
    expect(nodes.filter((n) => n.id === 'vignette').map((n) => [n.arg, n.label])).toEqual([
      ['setup', 'Настроить виньетку…'],
      ['clear', 'Обычный разворот'],
    ])
  })
  it('«Разнообразить»: на листе проекта и без функции пункта нет, без «Авто» с фото — серый', () => {
    expect(labels({ sheet: true })).not.toContain('Разнообразить раскладки')
    expect(labels({ can: (f) => f !== 'variants' })).not.toContain('Разнообразить раскладки')
    expect(spreadMenu({ ...base, varies: false }).find((n) => n.id === 'vary')?.disabled).toBe(true)
  })
  it('несколько групп: только «Разнообразить (N)» и «Удалить развороты (N)»', () => {
    expect(labels({ count: 3 })).toEqual(['Разнообразить раскладки (3)', 'Удалить развороты (3)'])
  })
  it('плитка обложки: свой текст удаления и ничего больше', () => {
    expect(labels({ ...none, del: true, label: 'Удалить обложку' })).toEqual(['Удалить обложку'])
  })
})

describe('spreadMenu: значки (0.25-menu-icons)', () => {
  const spec: SpreadMenuSpec = {
    count: 1,
    sheet: false,
    unit: { one: 'разворот' },
    can: () => true,
    rename: true,
    cut: { show: true, ok: true, why: '' },
    join: { ok: true, why: '' },
    insert: true,
    vignette: true,
    newVignette: true,
    vary: true,
    varies: true,
    del: true,
    how: 'Alt+щелчок',
  }
  it('у каждого пункта есть значок', () => {
    for (const o of [spec, { ...spec, kind: 'vignette' }]) {
      const nodes = spreadMenu(o)
      expect(nodes.length).toBeGreaterThan(6)
      for (const n of nodes) expect(n.icon, n.label).toBeTruthy()
    }
  })
  it('«перед» и «после» — разные значки, у листа и у разворота одни и те же', () => {
    const pick = (o: Partial<SpreadMenuSpec>) => spreadMenu({ ...spec, ...o }).filter((n) => n.id === 'insert')
    const [before, after] = pick({})
    expect(before.arg).toBe('before')
    expect(after.arg).toBe('after')
    expect(before.icon).toBe('insert-before')
    expect(after.icon).toBe('insert-after')
    expect(before.icon).not.toBe(after.icon)
    expect(pick({ sheet: true, unit: { one: 'лист' } }).map((n) => n.icon)).toEqual([before.icon, after.icon])
  })
  it('плитка «+» и плитка обложки — со значками', () => {
    const none = { rename: false, cut: undefined, join: undefined, insert: false, vignette: false, vary: false, del: false }
    expect(spreadMenu({ ...spec, ...none, newVignette: true }).map((n) => n.icon)).toEqual(['vignette'])
    expect(spreadMenu({ ...spec, ...none, newVignette: false, del: true, label: 'Удалить обложку' }).map((n) => n.icon)).toEqual([
      'spread-delete',
    ])
  })
})
