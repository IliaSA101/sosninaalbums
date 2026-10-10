import { describe, expect, it } from 'vitest'
import { cardBoxes, copyTargets, defaultN, fits, planQuestion, previewRects, touched, tplError } from './gridgallery'
import type { Box, GridChoice, PlaceView } from '../../base/core/types'

const r = (x: number, y: number, w: number, h: number) => ({ x, y, w, h })
const page = (boxes: Box[] = [r(0.1, 0.2, 0.5, 0.4)]): GridChoice => ({ id: 'a', name: '', scope: 'page', boxes, drop: 0 })
const whole = (boxes = [r(0.1, 0.2, 0.3, 0.4)]): GridChoice => ({ id: 'b', name: '', scope: 'spread', boxes, drop: 0 })

describe('fits', () => {
  it('шаблон разворота — только на разворот, страницы — куда угодно', () => {
    expect(fits(whole(), 'spread')).toBe(true)
    expect(fits(whole(), 'left')).toBe(false)
    expect(fits(page(), 'right')).toBe(true)
    expect(fits(page(), 'spread')).toBe(true)
  })
})

describe('defaultN', () => {
  const rects = [r(0, 0, 100, 100), r(120, 0, 50, 50), r(200, 0, 50, 50)]
  it('считает ячейки области по центру', () => {
    expect(defaultN({ rects, places: null }, 150, 'spread')).toBe(3)
    expect(defaultN({ rects, places: null }, 150, 'left')).toBe(2)
    expect(defaultN({ rects, places: null }, 150, 'right')).toBe(1)
  })
  it('у сетки берёт места, в том числе пустые', () => {
    const places = [r(0, 0, 10, 10), r(200, 0, 10, 10)].map((p, i) => ({
      ...p,
      id: `p${i}`,
      photo: '',
      fit: '',
      spine: false,
    })) as PlaceView[]
    expect(defaultN({ rects: [], places }, 150, 'spread')).toBe(2)
  })
  it('зажимает 1..7', () => {
    expect(defaultN({ rects: [], places: null }, 150, 'spread')).toBe(1)
    const many = Array.from({ length: 12 }, (_, i) => r(i, 0, 1, 1))
    expect(defaultN({ rects: many, places: null }, 150, 'spread')).toBe(7)
  })
})

describe('cardBoxes и previewRects', () => {
  it('шаблон страницы на разворот встаёт на обе страницы', () => {
    const b = cardBoxes(page(), 'spread')
    expect(b).toEqual([r(0.05, 0.2, 0.25, 0.4), r(0.55, 0.2, 0.25, 0.4)])
  })
  it('мм: разворот', () => {
    expect(previewRects(whole(), 'spread', 200, 100)).toEqual([r(40, 20, 120, 40)])
  })
  it('мм: правая страница сдвинута на страницу', () => {
    expect(previewRects(page(), 'right', 200, 100)).toEqual([r(220, 20, 100, 40)])
  })
  it('мм: шаблон страницы на разворот — две копии', () => {
    const [a, b] = previewRects(page(), 'spread', 200, 100)
    expect(a).toEqual(r(20, 20, 100, 40))
    expect(b.x).toBeCloseTo(220)
    expect([b.y, b.w, b.h]).toEqual([20, 100, 40])
  })
})

describe('угол коробки', () => {
  // Повёрнутая у края: центр внутри области, x < 0 — допустимо.
  const turned = (): GridChoice => page([{ x: -0.1, y: 0.2, w: 0.5, h: 0.4, rot: 30 }, r(0.5, 0.2, 0.3, 0.4)])
  it('cardBoxes: шаблон страницы на разворот — угол на обеих копиях, у неповёрнутой поля нет', () => {
    const b = cardBoxes(turned(), 'spread')
    expect(b).toHaveLength(4)
    expect(b.map((x) => x.rot)).toEqual([30, undefined, 30, undefined])
    expect(b[0]).toEqual({ x: -0.05, y: 0.2, w: 0.25, h: 0.4, rot: 30 })
    expect('rot' in b[1]).toBe(false)
  })
  it('cardBoxes: область как есть — боксы с углом не копируются', () => {
    const ch = turned()
    expect(cardBoxes(ch, 'left')).toBe(ch.boxes)
  })
  it('previewRects: угол сохраняется, центр не меняется', () => {
    const [a, b] = previewRects(turned(), 'right', 200, 100)
    expect(a).toEqual({ x: 180, y: 20, w: 100, h: 40, rot: 30 })
    expect('rot' in b).toBe(false)
  })
  it('previewRects: шаблон страницы на разворот — угол на обеих страницах', () => {
    const rects = previewRects(turned(), 'spread', 200, 100)
    expect(rects.map((x) => x.rot)).toEqual([30, undefined, 30, undefined])
    expect(rects[2].x).toBeCloseTo(180)
  })
})

describe('copyTargets и touched', () => {
  const base = { src: 's1', spreadIds: ['s2', 's3'], allIds: ['s1', 's2', 's3'], side: 'right' as const }
  it('соседняя страница — другая сторона того же разворота', () => {
    expect(copyTargets({ ...base, from: 'left', where: 'next' })).toEqual([{ spread: 's1', scope: 'right' }])
    expect(copyTargets({ ...base, from: 'right', where: 'next' })).toEqual([{ spread: 's1', scope: 'left' }])
    expect(copyTargets({ ...base, from: 'spread', where: 'next' })).toEqual([])
  })
  it('со страницы — выбранная сторона, с разворота — разворот', () => {
    expect(copyTargets({ ...base, from: 'left', where: 'spreads' })).toEqual([
      { spread: 's2', scope: 'right' },
      { spread: 's3', scope: 'right' },
    ])
    expect(copyTargets({ ...base, from: 'spread', where: 'all' }).map((t) => t.scope)).toEqual(['spread', 'spread', 'spread'])
  })
  it('новые развороты — не цели', () => {
    expect(copyTargets({ ...base, from: 'spread', where: 'new' })).toEqual([])
  })
  it('touched пропускает ту же область источника', () => {
    const t = copyTargets({ ...base, from: 'spread', where: 'all' })
    expect(touched(t, 's1', 'spread')).toBe(2)
    const p = copyTargets({ ...base, from: 'left', where: 'all', side: 'left' })
    expect(touched(p, 's1', 'left')).toBe(2)
    expect(touched([{ spread: 's1', scope: 'right' }], 's1', 'left')).toBe(1)
  })
})

describe('planQuestion', () => {
  it('без последствий — без вопроса', () => {
    expect(planQuestion({ frozen: 0, left: 0, texts: 0 })).toBeNull()
    expect(planQuestion({ frozen: 1, left: 0, texts: 0 })).toBeNull()
  })
  it('собирает фразу из ненулевых частей', () => {
    expect(planQuestion({ frozen: 12, left: 37, texts: 4 })).toBe(
      '12 разворотов станут ручными, 37 фото вернутся в ленту, 4 блока текста заменятся. Копировать?',
    )
    expect(planQuestion({ frozen: 0, left: 1, texts: 0 })).toBe('1 фото вернётся в ленту. Копировать?')
    expect(planQuestion({ frozen: 2, left: 0, texts: 1 })).toBe('2 разворота станут ручными, 1 блок текста заменится. Копировать?')
  })
})

describe('tplError', () => {
  it('«новой версией» объясняется', () => {
    expect(tplError('шаблоны сохранены более новой версией программы')).toContain('Обновите программу')
    expect(tplError('шаблон не найден')).toBe('шаблон не найден')
  })
})
