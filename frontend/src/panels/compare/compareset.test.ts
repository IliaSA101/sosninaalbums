import { describe, expect, it } from 'vitest'
import {
  activeId,
  cellCaption,
  closeSelection,
  dropGone,
  isLastLeft,
  openBySelection,
  openIds,
  openPair,
  pickSelected,
  removeCell,
  stepCell,
  title,
  type CompareState,
} from './compareset'

// Список и ячейки окна сравнения (138-5): открытие, шаг, удаление, выделение на выходе.
const list = ['a', 'b', 'c', 'd', 'e', 'f']
const always = () => true
const st = (cells: string[], active = 0): CompareState => ({ list, cells, active })

describe('открытие по выделению', () => {
  it('2–4 выделенных — они в порядке ленты, активный первый; список заморожен копией', () => {
    for (const sel of [
      ['d', 'b'],
      ['d', 'b', 'a'],
      ['f', 'a', 'd', 'c'],
    ]) {
      const s = openBySelection(list, sel)
      expect(s).not.toBeTypeOf('string')
      const st = s as CompareState
      expect(st.cells).toEqual(list.filter((id) => sel.includes(id)))
      expect(st.active).toBe(0)
      expect(st.list).toEqual(list)
      expect(st.list).not.toBe(list)
    }
  })
  it('0 выделенных — none, больше 4 — many', () => {
    expect(openBySelection(list, [])).toBe('none')
    expect(openBySelection(list, ['a', 'b', 'c', 'd', 'e'])).toBe('many')
  })
  it('невидимые выделенные не считаются: 5 с двумя скрытыми — это 3; одни скрытые — none', () => {
    const vis = ['a', 'b', 'c']
    expect((openBySelection(vis, ['a', 'b', 'c', 'x', 'y']) as CompareState).cells).toEqual(['a', 'b', 'c'])
    expect(openBySelection(vis, ['x', 'y'])).toBe('none')
    expect(pickSelected(vis, ['c', 'x', 'a'])).toEqual(['a', 'c'])
  })
  it('один выделенный — он и следующий; у последнего — предыдущий, активен выделенный', () => {
    expect(openBySelection(list, ['b'])).toEqual({ list, cells: ['b', 'c'], active: 0 })
    expect(openBySelection(list, ['f'])).toEqual({ list, cells: ['e', 'f'], active: 1 })
  })
  it('в ленте одно фото — alone, даже без выделения; пустая лента — none', () => {
    expect(openBySelection(['a'], ['a'])).toBe('alone')
    expect(openBySelection(['a'], [])).toBe('alone')
    expect(openBySelection([], [])).toBe('none')
  })
  it('ровно 4 и ровно 2 — граница открывается', () => {
    expect((openBySelection(list, ['a', 'b', 'c', 'd']) as CompareState).cells).toHaveLength(4)
    expect((openBySelection(list, ['a', 'b']) as CompareState).cells).toHaveLength(2)
  })
})

describe('из окна 128 и из «Похожих»', () => {
  it('openPair: показанный и следующий; у последнего — предыдущий', () => {
    expect(openPair(list, 'c')).toEqual({ list, cells: ['c', 'd'], active: 0 })
    expect(openPair(list, 'f')).toEqual({ list, cells: ['e', 'f'], active: 1 })
    expect(openPair(list, 'a')).toEqual({ list, cells: ['a', 'b'], active: 0 })
  })
  it('openPair: список из одного или кадр не из списка — alone', () => {
    expect(openPair(['a'], 'a')).toBe('alone')
    expect(openPair(list, 'zz')).toBe('alone')
  })
  it('openIds: порядок ленты, активный — исходный кадр, не больше 4', () => {
    const s = openIds(list, ['d', 'b', 'c'], 'c') as CompareState
    expect(s.cells).toEqual(['b', 'c', 'd'])
    expect(activeId(s)).toBe('c')
    expect((openIds(list, list, 'a') as CompareState).cells).toEqual(['a', 'b', 'c', 'd'])
  })
  it('openIds: чужие id и повторы отброшены; меньше двух — alone; исходный не из ответа — первая ячейка', () => {
    expect((openIds(list, ['b', 'b', 'x', 'e'], 'b') as CompareState).cells).toEqual(['b', 'e'])
    expect(openIds(list, ['b', 'x'], 'b')).toBe('alone')
    expect(openIds(list, [], 'b')).toBe('alone')
    expect((openIds(list, ['b', 'e'], 'zz') as CompareState).active).toBe(0)
  })
})

describe('шаг ячейки ← →', () => {
  it('кадр в активной ячейке меняется на соседний по списку, место ячейки то же', () => {
    const s = stepCell(st(['b', 'e'], 1), 1, always)
    expect(s.cells).toEqual(['b', 'f'])
    expect(s.active).toBe(1)
    expect(stepCell(st(['b', 'e'], 1), -1, always).cells).toEqual(['b', 'd'])
  })
  it('кадры других ячеек пропускаются', () => {
    expect(stepCell(st(['b', 'c', 'e'], 0), 1, always).cells).toEqual(['d', 'c', 'e'])
    expect(stepCell(st(['b', 'c', 'd'], 2), -1, always).cells).toEqual(['b', 'c', 'a'])
  })
  it('ушедшие из альбома пропускаются', () => {
    expect(stepCell(st(['a', 'f'], 0), 1, (id) => id !== 'b' && id !== 'c').cells).toEqual(['d', 'f'])
  })
  it('край и некуда идти — то же состояние', () => {
    const edge = st(['a', 'f'], 0)
    expect(stepCell(edge, -1, always)).toBe(edge)
    const end = st(['a', 'f'], 1)
    expect(stepCell(end, 1, always)).toBe(end)
    const blocked = st(['a', 'b'], 0)
    expect(stepCell(blocked, 1, (id) => id === 'a' || id === 'b')).toBe(blocked)
    const lost = st(['x', 'a'], 0)
    expect(stepCell(lost, 1, always)).toBe(lost)
  })
  it('список не меняется', () => {
    expect(stepCell(st(['a', 'b']), 1, always).list).toBe(list)
  })
})

describe('удаление ячейки', () => {
  it('убрана активная: активной становится следующая, у последней — предыдущая', () => {
    expect(removeCell(st(['a', 'b', 'c'], 1), 1)).toEqual({ list, cells: ['a', 'c'], active: 1 })
    expect(removeCell(st(['a', 'b', 'c'], 2), 2)).toEqual({ list, cells: ['a', 'b'], active: 1 })
  })
  it('убрана ячейка до активной — активный кадр остаётся тем же; после — не меняется', () => {
    const before = removeCell(st(['a', 'b', 'c'], 2), 0)
    expect(activeId(before)).toBe('c')
    const after = removeCell(st(['a', 'b', 'c'], 0), 2)
    expect(activeId(after)).toBe('a')
  })
  it('до одного: остаётся одна ячейка, isLastLeft — окно должно закрыться; из 4 — три', () => {
    const one = removeCell(st(['a', 'b'], 0), 0)
    expect(one.cells).toEqual(['b'])
    expect(isLastLeft(one)).toBe(true)
    expect(isLastLeft(st(['a', 'b']))).toBe(false)
    expect(removeCell(st(['a', 'b', 'c', 'd']), 3).cells).toHaveLength(3)
  })
  it('номер вне ячеек — то же состояние', () => {
    const s = st(['a', 'b'])
    expect(removeCell(s, 5)).toBe(s)
    expect(removeCell(s, -1)).toBe(s)
  })
  it('dropGone: ушедший кадр пропадает, активный остаётся; ушёл активный — соседняя', () => {
    const s = st(['a', 'b', 'c'], 2)
    expect(dropGone(s, always)).toBe(s)
    expect(dropGone(s, (id) => id !== 'a')).toEqual({ list, cells: ['b', 'c'], active: 1 })
    expect(dropGone(s, (id) => id !== 'c')).toEqual({ list, cells: ['a', 'b'], active: 1 })
    expect(dropGone(st(['a', 'b', 'c'], 1), (id) => id !== 'b')).toEqual({ list, cells: ['a', 'c'], active: 1 })
  })
})

describe('выделение на закрытии', () => {
  it('оставшиеся видимые, якорь — активный', () => {
    const s = st(['a', 'c', 'e'], 1)
    expect(closeSelection(s, new Set(list))).toEqual({ ids: ['a', 'c', 'e'], anchor: 'c' })
  })
  it('скрытые фильтром не выделяются; активный скрыт — якорь первый видимый', () => {
    const s = st(['a', 'c', 'e'], 1)
    expect(closeSelection(s, new Set(['a', 'e']))).toEqual({ ids: ['a', 'e'], anchor: 'a' })
  })
  it('ни одного видимого — пусто и без якоря', () => {
    expect(closeSelection(st(['a', 'b']), new Set())).toEqual({ ids: [], anchor: null })
  })
  it('один оставшийся (последний) — выделен он', () => {
    expect(closeSelection(st(['b'], 0), new Set(list))).toEqual({ ids: ['b'], anchor: 'b' })
  })
})

describe('подписи', () => {
  it('ячейка: «имя · N из M» по замороженному списку', () => {
    const s = st(['b', 'e'])
    expect(cellCaption(s, 0, 'IMG_1.jpg')).toBe('IMG_1.jpg · 2 из 6')
    expect(cellCaption(s, 1, 'IMG_4.jpg')).toBe('IMG_4.jpg · 5 из 6')
  })
  it('окно: склонение по числу кадров', () => {
    expect(title(2)).toBe('Сравнение · 2 кадра')
    expect(title(3)).toBe('Сравнение · 3 кадра')
    expect(title(4)).toBe('Сравнение · 4 кадра')
    expect(title(1)).toBe('Сравнение · 1 кадр')
  })
})
