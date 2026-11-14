import { describe, expect, it } from 'vitest'
import type { PatternView } from '../../base/core/types'
import { fadeLive, patFadeLive, patScaleLive } from '../backdrop/fade.svelte'
import {
  PATTERN_NEWER,
  PatSession,
  inksOf,
  layerPx,
  newSeed,
  patKey,
  patSession,
  patched,
  patternCaption,
  patternOf,
  patternPx,
  patternSrc,
  pctScale,
  scalePct,
} from './pattern'

const pv = (over: Partial<PatternView> = {}): PatternView => ({
  scope: 'spread',
  area: { x: 0, y: 0, w: 600, h: 300 },
  pat: { kind: 'dots', inks: ['#112233', '#445566', '#778899'], scale: 1.5, seed: 7, fade: 0.2 },
  name: 'Горох',
  inks: 2,
  random: true,
  known: true,
  url: '/media/gen/pat?k=dots&w=600&h=300&v=1',
  ...over,
})

describe('patternPx: размер запроса узора', () => {
  it('шаг 256 по ширине на экране × dpr', () => {
    expect(patternPx(600, 1)).toBe(768)
    expect(patternPx(600, 2)).toBe(1280)
    expect(patternPx(256, 1)).toBe(256)
    expect(patternPx(257, 1)).toBe(512)
  })
  it('не больше 4096 и не меньше шага', () => {
    expect(patternPx(5000, 2)).toBe(4096)
    expect(patternPx(10, 1)).toBe(256)
  })
  it('мусор и нулевой dpr — безопасно', () => {
    expect(patternPx(0, 1)).toBe(256)
    expect(patternPx(NaN, 1)).toBe(256)
    expect(patternPx(-5, 2)).toBe(256)
    expect(patternPx(512, 0)).toBe(512) // dpr 0 — как 1
  })
  it('зум в одном шаге не меняет адрес', () => {
    expect(patternPx(600, 1)).toBe(patternPx(700, 1))
  })
})

describe('patternSrc', () => {
  it('px дописывается к адресу Go с «?»', () => {
    expect(patternSrc('/media/gen/pat?k=dots&v=1', 768)).toBe('/media/gen/pat?k=dots&v=1&px=768')
  })
  it('адрес без «?» — с «?»', () => {
    expect(patternSrc('/x', 256)).toBe('/x?px=256')
  })
})

describe('масштаб: доля ↔ проценты 50…200', () => {
  it('перевод и зажим окном ползунка', () => {
    expect(scalePct(1.5)).toBe(150)
    expect(pctScale(150)).toBe(1.5)
    expect(scalePct(4)).toBe(200)
    expect(scalePct(0.25)).toBe(50)
    expect(pctScale(10)).toBe(0.5)
    expect(pctScale(999)).toBe(2)
  })
  it('нет поля (0 в модели) и мусор — 100 %', () => {
    expect(scalePct(undefined)).toBe(100)
    expect(scalePct(0)).toBe(100)
    expect(scalePct(NaN)).toBe(100)
    expect(pctScale(NaN)).toBe(1)
  })
})

describe('данные узора', () => {
  it('patternOf ищет по области', () => {
    const sp = { patterns: [pv({ scope: 'left' }), pv({ scope: 'right' })] }
    expect(patternOf(sp, 'right')?.scope).toBe('right')
    expect(patternOf(sp, 'spread')).toBeUndefined()
    expect(patternOf({ patterns: undefined }, 'spread')).toBeUndefined()
    expect(patternOf(undefined, 'spread')).toBeUndefined()
  })
  it('пикеров столько, сколько у генератора', () => {
    expect(inksOf(pv())).toEqual(['#112233', '#445566'])
    expect(inksOf(pv({ inks: 0 }))).toEqual([])
    expect(inksOf(pv({ pat: { kind: 'dots' } }))).toEqual([])
  })
  it('patched: правка поверх прежнего узора, цвета — копия', () => {
    const p = pv()
    const out = patched(p, { fade: 0.5 })
    expect(out).toEqual({ ...p.pat, fade: 0.5 })
    expect(out.inks).not.toBe(p.pat.inks)
    expect(patched(p, { inks: ['#000000'] }).inks).toEqual(['#000000'])
    expect(patched(pv({ pat: { kind: 'grid' } }), { scale: 2 })).toEqual({ kind: 'grid', scale: 2 })
  })
  it('подпись области: «Узор» или с областью, когда узоров два', () => {
    const l = { spread: 'Весь разворот', left: 'Левая страница', right: 'Правая страница', spine: 'Корешок' }
    expect(patternCaption('spread', 1, l)).toBe('Узор')
    expect(patternCaption('left', 2, l)).toBe('Узор — левая страница')
    expect(PATTERN_NEWER).toBe('Нужна новая версия программы')
  })
})

describe('«Перемешать» и сессия', () => {
  it('новое число отличается от прежнего и не 0', () => {
    expect(newSeed(5, () => 0.5)).toBe(2147483648)
    expect(newSeed(2147483648, () => 0.5)).not.toBe(2147483648)
    expect(newSeed(undefined, () => 0)).not.toBe(0)
  })
  it('случайное число в пределах uint32', () => {
    for (let i = 0; i < 50; i++) {
      const n = newSeed(i)
      expect(n).toBeGreaterThan(0)
      expect(n).toBeLessThan(2 ** 32)
    }
  })
  it('сессия merge — по грамматике Go: [A-Za-z0-9-], ≤ 32 знаков', () => {
    expect(patSession(1_700_000_000_000)).toMatch(/^[A-Za-z0-9-]{1,32}$/)
    expect(patSession(1)).not.toBe(patSession(2))
  })
})

describe('живой просмотр узора не путается с фото фона', () => {
  it('прозрачность и масштаб — отдельные экземпляры', () => {
    fadeLive.clear()
    patFadeLive.clear()
    patScaleLive.clear()
    patFadeLive.set('s1', 'spread', 0.4)
    expect(patFadeLive.of('s1', 'spread', 0)).toBe(0.4)
    expect(fadeLive.of('s1', 'spread', 0)).toBe(0) // фото фона той же области не тронуто
    expect(patScaleLive.of('s1', 'spread', 1)).toBe(1)
    patScaleLive.set('s1', 'spread', 1.5)
    expect(patScaleLive.of('s1', 'spread', 1)).toBe(1.5)
    expect(patFadeLive.of('s1', 'left', 0)).toBe(0) // другая область
    patFadeLive.clear()
    patScaleLive.clear()
  })
})

describe('layerPx: узор слоя при зуме холста', () => {
  // Область — весь лист 600 мм; ширина листа на экране приходит из Stage уже с зумом.
  it('на 100 % — от ширины листа на экране', () => {
    expect(layerPx(600, 600, 900, 1)).toBe(1024)
  })
  it('зум 348 % и 800 % просит картинку крупнее, шаг 256, потолок 4096', () => {
    expect(layerPx(600, 600, 900 * 3.48, 1)).toBe(3328)
    expect(layerPx(600, 600, 900 * 8, 1)).toBe(4096)
    expect(layerPx(600, 600, 900 * 3.48, 2)).toBe(4096)
  })
  it('область — доля листа: левая страница вдвое меньше', () => {
    expect(layerPx(300, 600, 900 * 3.48, 1)).toBe(1792)
  })
  it('нет ширины листа — минимальный шаг', () => {
    expect(layerPx(600, 0, 900, 1)).toBe(256)
  })
})

describe('сессия merge ползунков узора', () => {
  const p = pv()
  it('новый View с тем же листом, областью и генератором — сессия та же', () => {
    const s = new PatSession()
    s.begin(patKey('s1', p), 1_700_000_000_000)
    const first = s.merge(patKey('s1', p))
    expect(first).not.toBe('')
    // ответ Go: новый объект узора, но тот же узор той же области
    const next = pv({ pat: { ...p.pat, scale: 2 } })
    expect(patKey('s1', next)).toBe(patKey('s1', p))
    expect(s.merge(patKey('s1', next))).toBe(first)
    expect(s.merge(patKey('s1', next))).toBe(first) // и на следующее нажатие тоже
  })
  it('другая область, лист или генератор — сессии нет', () => {
    const s = new PatSession()
    s.begin(patKey('s1', p))
    expect(s.merge(patKey('s1', pv({ scope: 'left' })))).toBe('')
    expect(s.merge(patKey('s2', p))).toBe('')
    expect(s.merge(patKey('s1', pv({ pat: { ...p.pat, kind: 'stripes' } })))).toBe('')
  })
  it('без фокуса и после ухода фокуса — пусто (холостая правка не клеится к чужой)', () => {
    const s = new PatSession()
    expect(s.merge(patKey('s1', p))).toBe('')
    s.begin(patKey('s1', p))
    s.end()
    expect(s.merge(patKey('s1', p))).toBe('')
  })
})
