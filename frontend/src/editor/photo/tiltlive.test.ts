import { describe, expect, it } from 'vitest'
import { containImage, placeImage } from '../../base/geom/frame'
import type { SpreadView } from '../../base/core/types'
import { liveImage, liveTilt, liveTiltBox, liveTransform, TiltSession } from './tiltlive'

const cell = { x: 10, y: 20, w: 150, h: 100 }

describe('live rotation and contain', () => {
  it('поворот берётся из живого кадра, а не из View', () => {
    expect(liveTransform({ s: 1, x: 0, y: 0, r: 3 })).toBe('rotate(3deg)')
    expect(liveTransform({ s: 1, x: 0, y: 0 })).toBeUndefined()
  })
  it('зеркало из View остаётся при повороте: зеркало в осях фото, затем поворот', () => {
    expect(liveTransform({ s: 1, x: 0, y: 0, r: -4.5 }, { flipH: true })).toBe('rotate(-4.5deg) scale(-1, 1)')
    expect(liveTransform({ s: 1, x: 0, y: 0 }, { flipV: true })).toBe('scale(1, -1)')
  })
  it('с обрезкой: cover × S по живому углу', () => {
    const f = { s: 1.5, x: 0.5, y: 0, r: 3 }
    expect(liveImage(cell, 1.5, f, false)).toEqual(placeImage(cell, 1.5, f))
  })
  it('«Целиком»: вписано в место, масштаб и сдвиг кадра не влияют', () => {
    const a = liveImage(cell, 1.5, { s: 1, x: 0, y: 0, r: 3 }, true)
    const b = liveImage(cell, 1.5, { s: 3, x: 1, y: -1, r: 3 }, true)
    expect(a).toEqual(b)
    expect(a).toEqual(containImage(cell, 1.5, 3))
    // вписанное фото не больше места и не растёт как cover
    expect(a.w).toBeLessThanOrEqual(cell.w)
    expect(a.h).toBeLessThanOrEqual(cell.h)
  })
  it('«Целиком» с наклоном: угол живого кадра меняет размер вписанного фото', () => {
    const flat = liveImage(cell, 1.5, { s: 1, x: 0, y: 0 }, true)
    const tilted = liveImage(cell, 1.5, { s: 1, x: 0, y: 0, r: 10 }, true)
    expect(tilted.w).toBeLessThan(flat.w)
  })
  it('форма: овал «Целиком» уже прямоугольника', () => {
    const rect = liveImage(cell, 1.5, { s: 1, x: 0, y: 0, r: 5 }, true)
    const oval = liveImage(cell, 1.5, { s: 1, x: 0, y: 0, r: 5 }, true, { oval: true })
    expect(oval.w).toBeLessThan(rect.w)
  })
})

// Разворот с одним фото 3:2 в месте 150×100 (кадр из View — для turn, формы и пропорций).
const spread = (opts: { fit?: string; turn?: number; shape?: string } = {}): SpreadView =>
  ({
    photos: ['a'],
    rects: [cell],
    grid: !!opts.fit,
    places: opts.fit ? [{ id: 'p', photo: 'a', fit: opts.fit }] : undefined,
    frames: [{ img: { x: 0, y: 0, w: 150, h: 100 }, s: 1.5, x: 0, y: 0, r: 0, turn: opts.turn ?? 0, shape: opts.shape ?? '', radius: 0 }],
  }) as unknown as SpreadView

describe('live tilt rule', () => {
  it('приближение 150 % держится: 3° даёт S ≈ 139 %, ширина фото на экране та же', () => {
    const f = liveTilt(spread(), 'a', { s: 1.5, x: 0, y: 0 }, 3)
    expect(f).toEqual({ s: 1.393, x: 0, y: 0, r: 3 })
    const w0 = placeImage(cell, 1.5, { s: 1.5, x: 0, y: 0 }).w
    expect(placeImage(cell, 1.5, f!).w).toBeCloseTo(w0, 0)
  })
  it('без приближения фото растёт ровно настолько, чтобы закрыть место, и S остаётся 1', () => {
    const f = liveTilt(spread(), 'a', { s: 1, x: 0, y: 0 }, 3)!
    expect(f.s).toBe(1)
    expect(placeImage(cell, 1.5, f).w).toBeGreaterThan(150)
  })
  it('сессия: 0 → 3 → 0 от базы начала возвращает кадр точно', () => {
    const base = { s: 1.5, x: 0.4, y: -0.2 }
    liveTilt(spread(), 'a', base, 3)
    expect(liveTilt(spread(), 'a', base, 0)).toEqual({ ...base })
  })
  it('четверти берутся из turn: r = 90 + наклон', () => {
    expect(liveTilt(spread({ turn: 1 }), 'a', { s: 1, x: 0, y: 0, r: 90 }, 5)?.r).toBe(95)
  })
  it('«Целиком»: меняется только R, S, X, Y стоят', () => {
    const base = { s: 2, x: 0.3, y: -0.3 }
    expect(liveTilt(spread({ fit: 'contain' }), 'a', base, 7)).toEqual({ ...base, r: 7 })
  })
  it('нет фото на листе — null; форма-овал даёт другой кадр, чем прямоугольник', () => {
    expect(liveTilt(spread(), 'zzz', { s: 1.5, x: 0, y: 0 }, 3)).toBeNull()
    const base = { s: 1.5, x: 0, y: 0 }
    expect(liveTilt(spread({ shape: 'oval' }), 'a', base, 10)?.s).not.toBe(liveTilt(spread(), 'a', base, 10)?.s)
  })
  it('фон: рамка — прямоугольник w×h, масштаб держится', () => {
    const f = liveTiltBox({ w: 150, h: 100 }, 1.5, { s: 1.5, x: 0, y: 0 }, 3)
    expect(f).toEqual({ s: 1.393, x: 0, y: 0, r: 3 })
  })
})

describe('session', () => {
  const frame = { s: 1.5, x: 0, y: 0 }
  it('id подходит Go: [A-Za-z0-9-], 1–32 знака', () => {
    expect(new TiltSession().keep('a', frame, 1).id).toMatch(/^tl-[a-z0-9]{1,29}$/)
  })
  it('первая правка открывает, следующие того же фото идут с тем же ключом и базой', () => {
    const s = new TiltSession()
    const a = s.keep('a', frame, 1)
    s.settle(2)
    const b = s.keep('a', { s: 1.2, x: 0, y: 0, r: 3 }, 2)
    expect(b).toEqual(a)
    expect(b.base).toBe(frame)
  })
  it('другое фото, уход фокуса и чужая правка начинают новую сессию', () => {
    const s = new TiltSession()
    const a = s.keep('a', frame, 1)
    expect(s.keep('b', frame, 1).id).not.toBe(a.id)
    const c = s.keep('b', frame, 1)
    s.end()
    expect(s.keep('b', frame, 1).id).not.toBe(c.id)
    const d = s.keep('b', frame, 1)
    s.settle(5)
    const e = s.keep('b', { s: 2, x: 0, y: 0 }, 6) // rev сменил кто-то ещё
    expect(e.id).not.toBe(d.id)
    expect(e.base).toEqual({ s: 2, x: 0, y: 0 })
  })
  it('settle без открытой сессии ничего не открывает', () => {
    const s = new TiltSession()
    s.settle(3)
    expect(s.base).toBeNull()
  })
})
