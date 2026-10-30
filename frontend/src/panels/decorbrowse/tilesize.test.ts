import { describe, expect, it } from 'vitest'
import { loadDecorTile } from '../../base/prefs/view.svelte'
import { holdSize, tileDefault, tileRange, tileUrl, tileValue } from './tilesize'

describe('пределы и умолчания (133 §7)', () => {
  it('96…220, у «Разворотов» от 128', () => {
    expect(tileRange('sticker')).toEqual({ lo: 96, hi: 220 })
    expect(tileRange('bg')).toEqual({ lo: 96, hi: 220 })
    expect(tileRange('spread')).toEqual({ lo: 128, hi: 220 })
  })
  it('умолчание: стикеры и рамочки 120, фоны и развороты 168', () => {
    expect(['sticker', 'frame', 'bg', 'spread'].map((k) => tileDefault(k as never))).toEqual([120, 120, 168, 168])
  })
  it('tileValue: целое в пределах вида — как есть, иначе умолчание', () => {
    expect(tileValue(150, 'sticker')).toBe(150)
    expect(tileValue(96, 'sticker')).toBe(96)
    expect(tileValue(220, 'bg')).toBe(220)
    for (const bad of [undefined, null, 'x', NaN, 120.5, 95, 221, -1, 0]) expect(tileValue(bad, 'sticker')).toBe(120)
    expect(tileValue(127, 'spread')).toBe(168)
    expect(tileValue(128, 'spread')).toBe(128)
  })
})

describe('loadDecorTile', () => {
  it('берёт только известные виды и целые положительные', () => {
    expect(loadDecorTile({ sticker: 150, bg: 1.5, frame: 'x', spread: -3, other: 200, __proto__: { bg: 9 } })).toEqual({ sticker: 150 })
    expect(loadDecorTile(JSON.parse('{"__proto__":{"bg":200}}'))).toEqual({})
    expect(loadDecorTile(null)).toEqual({})
    expect(loadDecorTile('x')).toEqual({})
    expect(loadDecorTile(7)).toEqual({})
  })
})

describe('адрес картинки', () => {
  it('/media/lib/ — без px, остальные — с px', () => {
    expect(tileUrl('/media/lib/t/abc?v=3', 192)).toBe('/media/lib/t/abc?v=3')
    expect(tileUrl('/media/gen/art?k=x&v=1', 192)).toBe('/media/gen/art?k=x&v=1&px=192')
    expect(tileUrl('/media/tpl/a', 64)).toBe('/media/tpl/a?px=64')
  })
  it('holdSize не уменьшается', () => {
    expect(holdSize(0, 120)).toBe(120)
    expect(holdSize(200, 120)).toBe(200)
    expect(holdSize(120, 200)).toBe(200)
  })
})
