import { describe, expect, it } from 'vitest'
import { snapLines } from '../../base/geom/snap'
import { clampSize, cleanInput, isBlank, moveBox, resizeBox, sessionId } from './textedit'

const lines = snapLines({ pageW: 280, pageH: 280, margin: 12, rects: [] })
const box = { x: 100, y: 100, w: 60, h: 20 }

describe('sessionId', () => {
  it('до 32 знаков, только [A-Za-z0-9-]', () => {
    const s = sessionId(Date.now(), 7)
    expect(s).toMatch(/^[A-Za-z0-9-]{1,32}$/)
    expect(sessionId(1e18, 1e18).length).toBeLessThanOrEqual(32)
  })
  it('разные счётчики — разные сессии', () => expect(sessionId(5, 1)).not.toBe(sessionId(5, 2)))
})

describe('isBlank / cleanInput / clampSize', () => {
  it('пробелы и переводы строк — пусто', () => {
    expect(isBlank(' \n\t')).toBe(true)
    expect(isBlank('a')).toBe(false)
  })
  it('CRLF → LF, NUL убирается', () => expect(cleanInput('a\r\nb\rc\u0000')).toBe('a\nb\nc'))
  it('кегль зажимается', () => {
    expect(clampSize(3)).toBe(6)
    expect(clampSize(999)).toBe(200)
    expect(clampSize(NaN)).toBe(14)
    expect(clampSize(11.6)).toBe(12)
  })
})

describe('moveBox', () => {
  it('без прилипания — просто сдвиг', () => {
    const r = moveBox({ ...box, x: 63, y: 33 }, 3, 3, lines, 1, 560, 280)
    expect(r.x).toBeCloseTo(66)
    expect(r.guideX).toBeNull()
  })
  it('центр блока липнет к центру страницы, направляющая задана', () => {
    // центр блока x+30 = 100+30+dx; цель 140 → dx=10; смещение 9.5 в пределах порога
    const r = moveBox(box, 9.5, 0, lines, 2, 560, 280)
    expect(r.x).toBeCloseTo(110)
    expect(r.guideX).toBe(140)
  })
  it('не выходит за разворот', () => {
    const r = moveBox(box, -500, 900, lines, 0, 560, 280)
    expect(r.x).toBe(0)
    expect(r.y).toBe(260)
  })
})

describe('resizeBox', () => {
  it('правая ручка меняет ширину, x стоит', () => {
    const r = resizeBox(box, 'r', 20, lines, 0, 560)
    expect(r).toEqual({ x: 100, w: 80, guide: null })
  })
  it('левая ручка двигает левый край, правый стоит', () => {
    const r = resizeBox(box, 'l', -20, lines, 0, 560)
    expect(r.x).toBe(80)
    expect(r.x + r.w).toBe(160)
  })
  it('не уже минимальной ширины и не за край разворота', () => {
    expect(resizeBox(box, 'r', -500, lines, 0, 560).w).toBe(10)
    expect(resizeBox(box, 'l', 500, lines, 0, 560).w).toBe(10)
    expect(resizeBox(box, 'r', 9999, lines, 0, 560).x + resizeBox(box, 'r', 9999, lines, 0, 560).w).toBe(560)
  })
  it('край липнет к линии', () => {
    const r = resizeBox(box, 'r', 119, lines, 2, 560) // край 279 → 280
    expect(r.w).toBe(180)
    expect(r.guide).toBe(280)
  })
})
