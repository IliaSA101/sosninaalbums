import { beforeEach, describe, expect, it } from 'vitest'
import { DEFAULT_PREFS, buildOptions, estimateBytes, formatSize, loadPrefs, sampleName, savePrefs } from './exportopts'

const album = { pageW: 300, pageH: 300, dpi: 300 }
const MB = 1024 * 1024

describe('estimateBytes', () => {
  it('разворот 2×300 мм, 300 dpi, 95 — пиксели × 0,45', () => {
    // 3543 px страница: 7086 × 3543
    expect(estimateBytes(album, 300, 95, false)).toBeCloseTo(7086 * 3543 * 0.45, 0)
  })
  it('страница вдвое меньше разворота', () => {
    expect(estimateBytes(album, 300, 95, true)).toBeCloseTo(estimateBytes(album, 300, 95, false) / 2, -3)
  })
  it('dpi 0 берёт из альбома', () => {
    expect(estimateBytes(album, 0, 80, false)).toBe(estimateBytes(album, 300, 80, false))
    expect(estimateBytes({ ...album, dpi: 150 }, 0, 80, false)).toBe(estimateBytes(album, 150, 80, false))
  })
  it('качество монотонно, dpi квадратично', () => {
    const q = ([100, 95, 80] as const).map((v) => estimateBytes(album, 300, v, false))
    expect(q[0]).toBeGreaterThan(q[1])
    expect(q[1]).toBeGreaterThan(q[2])
    expect(estimateBytes(album, 150, 95, false) / estimateBytes(album, 300, 95, false)).toBeCloseTo(0.25, 2)
  })
})

describe('formatSize', () => {
  it('десятые до 10 МБ, потом целые', () => {
    expect(formatSize(2.5 * MB)).toBe('≈ 2,5 МБ')
    expect(formatSize(24.6 * MB)).toBe('≈ 25 МБ')
    expect(formatSize(1000)).toBe('< 0,1 МБ')
  })
})

describe('buildOptions', () => {
  const p = { quality: 80 as const, dpi: 150, pages: true }
  it('«Все» — пустой диапазон', () => {
    expect(buildOptions('all', 3, '1-2', p)).toEqual({ range: '', quality: 80, dpi: 150, pages: true })
  })
  it('«Текущий» — номер с 1', () => {
    expect(buildOptions('current', 4, '', DEFAULT_PREFS)).toEqual({ range: '4', quality: 95, dpi: 0, pages: false })
  })
  it('«Диапазон» — текст без пробелов по краям', () => {
    expect(buildOptions('range', 1, '  3-5, 7п ', p).range).toBe('3-5, 7п')
  })
})

// Минимальное хранилище вместо localStorage (среда тестов — node).
function useStorage(broken = false) {
  const m = new Map<string, string>()
  const guard = () => {
    if (broken) throw new Error('нет хранилища')
  }
  const s = {
    getItem: (k: string) => (guard(), m.get(k) ?? null),
    setItem: (k: string, v: string) => {
      guard()
      m.set(k, v)
    },
  }
  Object.defineProperty(globalThis, 'localStorage', { value: s, configurable: true, writable: true })
}

describe('prefs в localStorage', () => {
  beforeEach(() => useStorage())
  it('по умолчанию 95, из альбома, разворотами', () => {
    expect(loadPrefs()).toEqual({ quality: 95, dpi: 0, pages: false })
  })
  it('сохранённое читается', () => {
    savePrefs({ quality: 100, dpi: 150, pages: true })
    expect(loadPrefs()).toEqual({ quality: 100, dpi: 150, pages: true })
  })
  it('порченое и чужое отбрасывается', () => {
    localStorage.setItem('exportOpts', JSON.stringify({ quality: 50, dpi: 5, pages: 'да' }))
    expect(loadPrefs()).toEqual(DEFAULT_PREFS)
    localStorage.setItem('exportOpts', '{не json')
    expect(loadPrefs()).toEqual(DEFAULT_PREFS)
  })
  it('без хранилища не падает', () => {
    useStorage(true)
    expect(() => savePrefs(DEFAULT_PREFS)).not.toThrow()
    expect(loadPrefs()).toEqual(DEFAULT_PREFS)
  })
})

describe('sampleName', () => {
  it('две цифры, как у Go', () => {
    expect(sampleName(7, 12, false)).toBe('(07.jpg)')
    expect(sampleName(3, 12, true)).toBe('(03-1.jpg, 03-2.jpg)')
  })
  it('при 100+ разворотах три цифры', () => {
    expect(sampleName(7, 120, false)).toBe('(007.jpg)')
    expect(sampleName(120, 120, true)).toBe('(120-1.jpg, 120-2.jpg)')
  })
  it('пустой альбом — номер 01', () => {
    expect(sampleName(0, 0, false)).toBe('(01.jpg)')
  })
})
