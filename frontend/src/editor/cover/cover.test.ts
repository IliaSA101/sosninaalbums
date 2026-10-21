import { describe, expect, it } from 'vitest'
import type { CoverZone } from '../../base/core/types'
import {
  COVER_LAYOUTS,
  COVER_Z,
  DROP_SCOPE,
  SPINE_HIT_PX,
  boxCss,
  coverLabel,
  coverLabels,
  coverScopes,
  dropOrder,
  labeled,
  mmText,
  pctBox,
  scopeOfDrop,
  sidesOf,
  spineNoteText,
  spineZone,
  zonesOf,
} from './cover'

// Лист из спеки: Фотосфера, тонкие листы, 20 разворотов — 623 × 331 мм, сторонка 281 × 284, заворот 23,5, корешок 14.
const sheet = { w: 623, h: 331 }
const z = (kind: string, x: number, y: number, w: number, h: number, label = ''): CoverZone => ({ kind, rect: { x, y, w, h }, label })
const zones: CoverZone[] = [
  z('back', 23.5, 23.5, 281, 284, 'Задняя'),
  z('front', 318.5, 23.5, 281, 284, 'Лицевая'),
  z('spine', 304.5, 0, 14, 331, 'Корешок 14 мм'),
  z('wrap', 0, 0, 623, 23.5, 'Заворот'),
  z('wrap', 0, 307.5, 623, 23.5, 'Заворот'),
  z('trim', 23.5, 23.5, 576, 284),
]

describe('проценты зон от размеров листа', () => {
  it('лист целиком — 100 %', () => {
    expect(pctBox({ x: 0, y: 0, w: 623, h: 331 }, sheet)).toEqual({ left: 0, top: 0, width: 100, height: 100 })
  })
  it('корешок: ширина 14/623 листа', () => {
    const b = pctBox(spineZone({ zones })!.rect, sheet)
    expect(b.left).toBeCloseTo((304.5 / 623) * 100, 6)
    expect(b.width).toBeCloseTo((14 / 623) * 100, 6)
    expect(b.top).toBe(0)
    expect(b.height).toBe(100)
  })
  it('нулевой лист не даёт NaN', () => {
    expect(pctBox({ x: 1, y: 1, w: 1, h: 1 }, { w: 0, h: 0 })).toEqual({ left: 0, top: 0, width: 0, height: 0 })
  })
  it('строка style округлена до тысячных', () => {
    expect(boxCss({ x: 311.5, y: 0, w: 311.5, h: 165.5 }, sheet)).toBe('left:50%;top:0%;width:50%;height:50%')
    expect(boxCss({ x: 1, y: 1, w: 1, h: 1 }, { w: 3, h: 3 })).toBe('left:33.333%;top:33.333%;width:33.333%;height:33.333%')
  })
})

describe('зоны броска и раскладки', () => {
  it('зона -> область фона', () => {
    expect(scopeOfDrop('back')).toBe('left')
    expect(scopeOfDrop('front')).toBe('right')
    expect(scopeOfDrop('wrap')).toBe('spread')
    expect(scopeOfDrop('spine')).toBe('spine')
    expect(scopeOfDrop('hinge')).toBeNull()
    expect(DROP_SCOPE.spine).toBe('spine')
  })
  it('три кнопки раскладки в порядке и с подписями спеки 061', () => {
    expect(COVER_LAYOUTS.map((l) => [l.kind, l.label])).toEqual([
      ['front', 'Лицевая'],
      ['wrap', 'Одно фото через корешок'],
      ['both', 'Задняя + корешок + лицевая'],
    ])
  })
  it('dropOrder: корешок последним', () => {
    const d = [z('back', 0, 0, 1, 1), z('spine', 1, 0, 1, 1), z('front', 2, 0, 1, 1), z('wrap', 0, 0, 3, 1)]
    expect(dropOrder(d).map((x) => x.kind)).toEqual(['back', 'front', 'wrap', 'spine'])
    expect(dropOrder(d.filter((x) => x.kind !== 'spine')).map((x) => x.kind)).toEqual(['back', 'front', 'wrap'])
    expect(dropOrder([])).toEqual([])
    expect(d[1].kind).toBe('spine') // вход не меняется
    expect(SPINE_HIT_PX).toBe(24)
  })
  it('zonesOf отбирает по виду', () => {
    expect(zonesOf({ zones }, 'wrap')).toHaveLength(2)
    expect(zonesOf({ zones }, 'safe')).toHaveLength(0)
  })
})

// Области фото фона раскладок — как отдаёт Go (CoverView.scopes)
const SCOPES = {
  front: [{ scope: 'right', label: 'Лицевая' }],
  wrap: [{ scope: 'spread', label: 'Вся обложка' }],
  both: [
    { scope: 'left', label: 'Задняя' },
    { scope: 'spine', label: 'Корешок' },
    { scope: 'right', label: 'Лицевая' },
  ],
} as const

describe('области фона из cover.scopes', () => {
  it('порядок и состав — от Go, фронт ничего не добавляет', () => {
    expect(coverScopes({ scopes: [...SCOPES.front] })).toEqual(['right'])
    expect(coverScopes({ scopes: [...SCOPES.wrap] })).toEqual(['spread'])
    expect(coverScopes({ scopes: [...SCOPES.both] })).toEqual(['left', 'spine', 'right'])
  })
  it('обложки нет — областей нет', () => {
    expect(coverScopes(null)).toEqual([])
    expect(coverScopes(undefined)).toEqual([])
  })
  it('подписи берутся у Go; область вне раскладки — запасная подпись сторонки', () => {
    const cv = { scopes: [...SCOPES.both] }
    expect(coverLabel(cv, 'spine')).toBe('Корешок')
    expect(coverLabel({ scopes: [{ scope: 'left', label: 'Тыл' }] }, 'left')).toBe('Тыл')
    expect(coverLabel({ scopes: [...SCOPES.front] }, 'left')).toBe('Задняя')
    expect(coverLabel(null, 'spread')).toBe('Вся обложка')
    expect(coverLabels(cv)).toEqual({ spread: 'Вся обложка', left: 'Задняя', right: 'Лицевая', spine: 'Корешок' })
  })
})

describe('порядок слоёв обложки', () => {
  it('плашка < фото корешка <= зоны броска < .content (z 3)', () => {
    expect(COVER_Z.plate).toBeLessThan(COVER_Z.spine)
    expect(COVER_Z.spine).toBeLessThanOrEqual(COVER_Z.drop)
    expect(COVER_Z.drop).toBeLessThan(3)
  })
})

describe('подписи зон', () => {
  it('«Заворот» подписан один раз, остальные зоны — как есть', () => {
    const l = labeled(zones).map((x) => x.label)
    expect(l).toEqual(['Задняя', 'Лицевая', 'Корешок 14 мм', 'Заворот', '', ''])
  })
  it('«Расстав» тоже один раз', () => {
    const h = [z('hinge', 0, 0, 9, 1, 'Расстав 9 мм'), z('hinge', 20, 0, 9, 1, 'Расстав 9 мм')]
    expect(labeled(h).map((x) => x.label)).toEqual(['Расстав 9 мм', ''])
  })
})

describe('сторонки для прилипания', () => {
  it('задняя и лицевая по X, по возрастанию', () => {
    expect(sidesOf({ zones: [...zones].reverse() })).toEqual([
      [23.5, 304.5],
      [318.5, 599.5],
    ])
  })
  it('не обложка или нет сторонок — undefined (линии по шву)', () => {
    expect(sidesOf(null)).toBeUndefined()
    expect(sidesOf({ zones: [z('spine', 0, 0, 1, 1)] })).toBeUndefined()
  })
})

describe('текст отметки корешка', () => {
  it('числа с запятой без хвостов', () => {
    expect(mmText(14)).toBe('14')
    expect(mmText(13.2)).toBe('13,2')
    expect(mmText(13.2000001)).toBe('13,2')
    expect(spineNoteText(12, 14)).toBe('Корешок 12 → 14 мм')
  })
})
