import { afterEach, describe, expect, it, vi } from 'vitest'
import { reducedMotion } from './spotstate.svelte'
import {
  GAP,
  PAD,
  SPOT_HINT_MS,
  SPOT_MS,
  isCandidate,
  joinToast,
  missToast,
  pickTarget,
  placeLabel,
  revealShift,
  ringBox,
  visibleArea,
  type Box,
  type Probe,
} from './spot'
import { ARTICLES } from './articles'

const win = { w: 1280, h: 760 }
const box = (left: number, top: number, width: number, height: number): Box => ({ left, top, width, height })
const probe = (o: Partial<Probe> = {}): Probe => ({ rects: 1, w: 40, h: 20, inert: false, hidden: false, ...o })

describe('кандидаты', () => {
  it('видимый элемент годится', () => {
    expect(isCandidate(probe())).toBe(true)
  })
  it('нулевой размер, display:none, inert и visibility:hidden — нет', () => {
    expect(isCandidate(probe({ w: 0 }))).toBe(false)
    expect(isCandidate(probe({ h: 0 }))).toBe(false)
    expect(isCandidate(probe({ rects: 0 }))).toBe(false)
    expect(isCandidate(probe({ inert: true }))).toBe(false)
    expect(isCandidate(probe({ hidden: true }))).toBe(false)
  })
})

describe('выбор из нескольких', () => {
  it('видимый с наибольшей площадью в окне', () => {
    const r = [box(10, 10, 20, 20), box(100, 100, 300, 200), box(2000, 10, 500, 500)]
    expect(pickTarget(r, win)).toEqual({ index: 1, needScroll: false })
  })
  it('площадь считается по видимой части', () => {
    expect(visibleArea(box(1200, 700, 200, 200), win)).toBe(80 * 60)
    expect(visibleArea(box(-50, -50, 100, 100), win)).toBe(50 * 50)
    expect(visibleArea(box(2000, 0, 10, 10), win)).toBe(0)
  })
  it('все вне окна — самый большой, с прокруткой', () => {
    const r = [box(0, 900, 50, 50), box(0, 1000, 200, 100)]
    expect(pickTarget(r, win)).toEqual({ index: 1, needScroll: true })
  })
  it('кандидатов нет — null', () => {
    expect(pickTarget([], win)).toBeNull()
  })
})

describe('кольцо и подпись', () => {
  it('кольцо шире элемента на PAD', () => {
    expect(ringBox(box(100, 100, 50, 30))).toEqual(box(100 - PAD, 100 - PAD, 50 + 2 * PAD, 30 + 2 * PAD))
  })
  it('подпись снизу, если там есть место', () => {
    const ring = box(500, 100, 100, 40)
    const p = placeLabel(ring, { w: 120, h: 30 }, win)
    expect(p.top).toBe(140 + GAP)
  })
  it('внизу окна — сверху', () => {
    const ring = box(500, 720, 100, 30)
    const p = placeLabel(ring, { w: 120, h: 30 }, win)
    expect(p.top).toBe(720 - 30 - GAP)
  })
  it('не выходит за окно ни по одному краю', () => {
    for (const ring of [box(0, 0, 10, 10), box(1270, 0, 10, 10), box(0, 750, 10, 10), box(1270, 750, 10, 10), box(0, 0, 1280, 760)]) {
      const size = { w: 200, h: 40 }
      const p = placeLabel(ring, size, win)
      expect(p.left).toBeGreaterThanOrEqual(GAP)
      expect(p.top).toBeGreaterThanOrEqual(GAP)
      expect(p.left + size.w).toBeLessThanOrEqual(win.w - GAP)
      expect(p.top + size.h).toBeLessThanOrEqual(win.h - GAP)
    }
  })
  it('длительность: ≈ 4 с, с подсказкой ≈ 6 с (аудит 2026-10-14: 2,4 с мало)', () => {
    expect(SPOT_MS).toBe(4000)
    expect(SPOT_HINT_MS).toBe(6000)
  })
})

describe('прокрутка в вид', () => {
  const view = [100, 500] as const // поле зрения предка: 400 px
  it('внутри поля зрения — не двигаем', () => {
    expect(revealShift(150, 250, ...view)).toBe(0)
  })
  it('ниже — сдвиг вперёд, с отступом GAP', () => {
    expect(revealShift(600, 700, ...view)).toBe(700 - 500 + GAP)
  })
  it('выше — сдвиг назад, с отступом GAP', () => {
    expect(revealShift(20, 60, ...view)).toBe(20 - 100 - GAP)
  })
  it('крупнее поля зрения — по началу', () => {
    expect(revealShift(900, 1500, ...view)).toBe(900 - 100 - GAP)
  })
  it('у края внутри отступа — подвинуть, чтобы кольцо не прилипало к краю', () => {
    expect(revealShift(104, 140, ...view)).toBe(104 - 100 - GAP)
  })
})

describe('запасной тост «Название: где»', () => {
  it('подсказка статьи главнее where, where главнее how', () => {
    expect(missToast({ title: 'Т', where: 'здесь', how: 'так' }, 'ещё нет')).toBe('«Т»: ещё нет')
    expect(missToast({ title: 'Т', where: 'здесь', how: 'так' })).toBe('«Т»: здесь')
    expect(missToast({ title: 'Т', how: 'так' })).toBe('«Т»: так')
  })
  it('у статьи без where и how текст всё равно есть', () => {
    expect(missToast({ title: 'Т' })).toMatch(/^«Т»: .+/)
  })
  it('у каждой статьи тост непустой, а подсказка miss не дублирует название', () => {
    for (const a of ARTICLES) {
      const t = missToast(a, a.miss?.hint)
      expect(t.length, a.id).toBeGreaterThan(a.title.length + 4)
    }
  })
  it('с тостом о смене уровня — одно сообщение', () => {
    expect(joinToast('Уровень «Стандартный» — для «Кадр»', '«Т»: тут')).toBe('Уровень «Стандартный» — для «Кадр». «Т»: тут')
    expect(joinToast(undefined, '«Т»: тут')).toBe('«Т»: тут')
  })
})

describe('без движения', () => {
  afterEach(() => vi.unstubAllGlobals())
  it('prefers-reduced-motion: reduce → кольцо и прокрутка без анимации', () => {
    vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce') }))
    expect(reducedMotion()).toBe(true)
  })
  it('без настройки — анимация есть', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    expect(reducedMotion()).toBe(false)
  })
})
