import { describe, expect, it } from 'vitest'
import { isSelectClick, prune, range, toggle } from './spreadsel'

const all = ['a', 'b', 'c', 'd', 'e']

describe('toggle (Ctrl+щелчок)', () => {
  it('добавляет и убирает', () => {
    expect(toggle([], 'b')).toEqual(['b'])
    expect(toggle(['b', 'd'], 'd')).toEqual(['b'])
  })
  it('не меняет исходный список', () => {
    const sel = ['a']
    toggle(sel, 'b')
    expect(sel).toEqual(['a'])
  })
})

describe('range (Shift+щелчок)', () => {
  it('от опоры вперёд и назад, включая обе', () => {
    expect(range(all, 'b', 'd')).toEqual(['b', 'c', 'd'])
    expect(range(all, 'd', 'b')).toEqual(['b', 'c', 'd'])
  })
  it('опора = цель — одна группа', () => {
    expect(range(all, 'c', 'c')).toEqual(['c'])
  })
  it('нет опоры или она исчезла — только щёлкнутая', () => {
    expect(range(all, null, 'c')).toEqual(['c'])
    expect(range(all, 'zz', 'c')).toEqual(['c'])
  })
  it('цели нет в альбоме — пусто', () => {
    expect(range(all, 'a', 'zz')).toEqual([])
  })
})

describe('prune (чистка исчезнувших)', () => {
  it('убирает удалённые', () => {
    expect(prune(['a', 'c', 'x'], all)).toEqual(['a', 'c'])
  })
  it('ничего не исчезло — тот же массив', () => {
    const sel = ['a', 'b']
    expect(prune(sel, all)).toBe(sel)
  })
})

describe('isSelectClick', () => {
  const e = (o: object) => ({ ctrlKey: false, metaKey: false, shiftKey: false, ...o })
  it('Ctrl, Cmd и Shift собирают выделение, простой щелчок — нет', () => {
    expect(isSelectClick(e({}))).toBe(false)
    expect(isSelectClick(e({ ctrlKey: true }))).toBe(true)
    expect(isSelectClick(e({ metaKey: true }))).toBe(true)
    expect(isSelectClick(e({ shiftKey: true }))).toBe(true)
  })
})
