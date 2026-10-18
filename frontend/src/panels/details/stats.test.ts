import { describe, expect, it } from 'vitest'
import { albumStats, colorLine, nextLowSheet, orientation } from './stats'
import type { Photo, SpreadView } from '../../base/core/types'

const photo = (id: string, w: number, h: number): Photo => ({ id, path: id, name: id, w, h })
const spread = (photos: string[], layout: string, rects: { w: number; h: number }[] = []): SpreadView =>
  ({ id: photos.join(), photos, layout, rects: rects.map((r) => ({ x: 0, y: 0, ...r })) }) as SpreadView

describe('orientation', () => {
  it('горизонт, вертикаль, квадрат с допуском 2 %', () => {
    expect(orientation(3, 2)).toBe('h')
    expect(orientation(2, 3)).toBe('v')
    expect(orientation(100, 101)).toBe('sq')
    expect(orientation(100, 105)).toBe('v')
  })
  it('без размеров — квадрат', () => {
    expect(orientation(0, 0)).toBe('sq')
    expect(orientation(5, 0)).toBe('sq')
  })
})

describe('albumStats', () => {
  it('считает развороты, страницы, ключи и ориентации', () => {
    const byId = new Map([photo('a', 3, 2), photo('b', 2, 3), photo('c', 1, 1)].map((p) => [p.id, p]))
    const st = albumStats([spread(['a', 'b'], 'H(0,1)'), spread(['c'], 'H(0,1)')], byId, 10)
    expect(st).toMatchObject({ spreads: 2, pages: 4, layouts: 1, photos: 3, total: 10, h: 1, v: 1, sq: 1 })
  })
  it('обложка: её фото считаются, в развороты, страницы и раскладки не входит', () => {
    const byId = new Map([photo('a', 3, 2), photo('c', 1, 1)].map((p) => [p.id, p]))
    const st = albumStats([spread(['c'], 'cover'), spread(['a'], 'H(0,1)')], byId, 5, 1)
    expect(st).toMatchObject({ spreads: 1, pages: 2, layouts: 1, photos: 2, sq: 1, h: 1 })
  })
  it('фото вне библиотеки берёт аспект ячейки', () => {
    const st = albumStats([spread(['x'], 'k', [{ w: 4, h: 2 }])], new Map(), 0)
    expect(st.h).toBe(1)
  })
  it('считает фото в Adobe RGB и с другим профилем', () => {
    const byId = new Map(
      [
        { ...photo('a', 3, 2), color: 'adobe' },
        { ...photo('b', 2, 3), color: 'other', profile: 'Display P3' },
        { ...photo('c', 1, 1), color: 'srgb' },
        { ...photo('d', 1, 1), color: 'adobe' },
      ].map((p) => [p.id, p as Photo]),
    )
    const st = albumStats([spread(['a', 'b', 'c'], 'k')], byId, 4)
    expect(st).toMatchObject({ adobe: 1, other: 1 })
  })
  it('кэш по листу: тот же лист при новой таблице фото считается заново', () => {
    const s = spread(['a'], 'k')
    const v1 = new Map([photo('a', 3, 2)].map((p) => [p.id, p]))
    expect(albumStats([s], v1, 1)).toMatchObject({ h: 1, v: 0 })
    expect(albumStats([s], v1, 1)).toMatchObject({ h: 1, v: 0 }) // из кэша
    const v2 = new Map([{ ...photo('a', 2, 3), color: 'adobe' } as Photo].map((p) => [p.id, p]))
    expect(albumStats([s], v2, 1)).toMatchObject({ h: 0, v: 1, adobe: 1 })
  })
  it('пустой альбом', () => {
    expect(albumStats([], new Map(), 0)).toMatchObject({ spreads: 0, photos: 0, layouts: 0 })
  })
})

describe('colorLine', () => {
  it('нет цветных фото — строки нет', () => {
    expect(colorLine({ adobe: 0, other: 0 })).toBeNull()
  })
  it('только Adobe, только другой, оба', () => {
    expect(colorLine({ adobe: 3, other: 0 })).toBe('Цвет: Adobe RGB — 3 (будут пересчитаны)')
    expect(colorLine({ adobe: 0, other: 2 })).toBe('Цвет: другой профиль — 2')
    expect(colorLine({ adobe: 3, other: 2 })).toBe('Цвет: Adobe RGB — 3 (будут пересчитаны), другой профиль — 2')
  })
})

describe('nextLowSheet', () => {
  const sp = (...low: number[]) => low.map((lowDPI) => ({ lowDPI }))
  it('идёт вперёд и замыкается по кругу', () => {
    const s = sp(1, 0, 2, 0, 0)
    expect(nextLowSheet(s, 0)).toBe(2)
    expect(nextLowSheet(s, 1)).toBe(2)
    expect(nextLowSheet(s, 2)).toBe(0)
    expect(nextLowSheet(s, 4)).toBe(0)
  })
  it('обложка (первый лист) входит в круг: до неё доходят после последнего разворота', () => {
    const s = sp(2, 1, 0, 0) // обложка и разворот 1 с малым разрешением
    expect(nextLowSheet(s, 1)).toBe(0) // с разворота 1 — на обложку, а не снова на разворот 1
    expect(nextLowSheet(s, 0)).toBe(1)
    expect(nextLowSheet(s, 3)).toBe(0)
  })
  it('единственный разворот — он же', () => {
    expect(nextLowSheet(sp(0, 3, 0), 1)).toBe(1)
  })
  it('нет таких разворотов или альбом пуст — −1', () => {
    expect(nextLowSheet(sp(0, 0), 0)).toBe(-1)
    expect(nextLowSheet([], 0)).toBe(-1)
  })
})
