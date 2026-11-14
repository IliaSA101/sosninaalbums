import { describe, expect, it } from 'vitest'
import { canvasList, caption, closeSelection, nearestVisible, openList, shownId, stepList, type PreviewState } from './previewlist'

// Список окна крупного просмотра (128): заморозка, шаги, ближайшее видимое, выделение на закрытии, подпись.
const all = () => true
const lib = (ids: string[], at: number): PreviewState => ({ ids, at, from: 'library' })

describe('openList', () => {
  it('замораживает копию видимой ленты: правка исходного массива список не меняет', () => {
    const vis = ['a', 'b', 'c']
    const s = openList(vis, 'b')
    vis.splice(0, 3, 'x')
    expect(s).toEqual({ ids: ['a', 'b', 'c'], at: 1, from: 'library' })
    expect(shownId(s)).toBe('b')
  })
  it('фото нет среди видимых (скрыто фильтром) — список из него одного', () => {
    expect(openList(['a', 'b'], 'z')).toEqual({ ids: ['z'], at: 0, from: 'library' })
  })
  it('с холста — одно фото, листания нет', () => {
    const s = canvasList('p')
    expect(s).toEqual({ ids: ['p'], at: 0, from: 'canvas' })
    expect(stepList(s, 1, all)).toBe(s)
  })
})

describe('stepList', () => {
  const s = lib(['a', 'b', 'c'], 1)
  it('шаг вперёд и назад', () => {
    expect(stepList(s, 1, all).at).toBe(2)
    expect(stepList(s, -1, all).at).toBe(0)
  })
  it('края без кольца: на краю то же состояние', () => {
    expect(stepList(lib(['a', 'b'], 1), 1, all)).toEqual(lib(['a', 'b'], 1))
    expect(stepList(lib(['a', 'b'], 0), -1, all)).toEqual(lib(['a', 'b'], 0))
    const one = lib(['a'], 0)
    expect(stepList(one, 1, all)).toBe(one)
  })
  it('ушедшее из альбома фото пропускается; за ним никого — остаёмся', () => {
    const alive = (id: string) => id !== 'b'
    expect(stepList(lib(['a', 'b', 'c'], 0), 1, alive).at).toBe(2)
    expect(stepList(lib(['a', 'b', 'c'], 2), -1, alive).at).toBe(0)
    expect(stepList(lib(['a', 'b'], 0), 1, alive)).toEqual(lib(['a', 'b'], 0))
  })
  it('список не мутируется', () => {
    const before = lib(['a', 'b', 'c'], 1)
    stepList(before, 1, all)
    expect(before.at).toBe(1)
  })
})

describe('nearestVisible', () => {
  const s = lib(['a', 'b', 'c', 'd', 'e'], 2)
  it('показанное видно — оно', () => {
    expect(nearestVisible(s, new Set(['a', 'c', 'e']))).toBe('c')
  })
  it('скрыто фильтром (X при «Без отклонённых») — ближайшее видимое по списку', () => {
    expect(nearestVisible(s, new Set(['a', 'b', 'e']))).toBe('b')
    expect(nearestVisible(s, new Set(['a', 'e']))).toBe('e') // a и e в двух шагах — берётся следующее
    expect(nearestVisible(s, new Set(['a']))).toBe('a')
  })
  it('при равенстве расстояния — следующее', () => {
    expect(nearestVisible(lib(['a', 'b', 'c'], 1), new Set(['a', 'c']))).toBe('c')
  })
  it('в ленте не осталось никого из списка — null', () => {
    expect(nearestVisible(s, new Set(['zz']))).toBeNull()
    expect(nearestVisible(s, new Set())).toBeNull()
  })
})

describe('closeSelection', () => {
  it('показанное в выделении — выделение не трогаем (null)', () => {
    expect(closeSelection(['a', 'b', 'c', 'd'], 'c')).toBeNull()
  })
  it('вне выделения — выделяется только оно', () => {
    expect(closeSelection(['a', 'b'], 'z')).toEqual(['z'])
    expect(closeSelection([], 'z')).toEqual(['z'])
  })
})

describe('caption', () => {
  it('из ленты — «имя · N из M»', () => {
    expect(caption('IMG_1.jpg', lib(['a', 'b', 'c'], 1))).toBe('IMG_1.jpg · 2 из 3')
    expect(caption('IMG_1.jpg', openList(['x'], 'x'))).toBe('IMG_1.jpg · 1 из 1')
  })
  it('с холста — только имя', () => {
    expect(caption('IMG_1.jpg', canvasList('a'))).toBe('IMG_1.jpg')
  })
})
