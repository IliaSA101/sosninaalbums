import { beforeEach, describe, expect, it } from 'vitest'
import {
  PHOTO_TAB,
  SETS,
  dropLegacyRecent,
  hexError,
  hexToHsv,
  hsvToHex,
  legacyRecent,
  loadTab,
  nameOf,
  normalizeHex,
  saveTab,
} from './palette'

// Минимальное хранилище вместо localStorage (среда тестов — node).
function fakeStorage(broken = false) {
  const m = new Map<string, string>()
  const guard = () => {
    if (broken) throw new Error('нет хранилища')
  }
  return {
    getItem: (k: string) => (guard(), m.get(k) ?? null),
    setItem: (k: string, v: string) => {
      guard()
      m.set(k, v)
    },
    removeItem: (k: string) => {
      guard()
      m.delete(k)
    },
  }
}

function useStorage(s: ReturnType<typeof fakeStorage>) {
  Object.defineProperty(globalThis, 'localStorage', { value: s, configurable: true, writable: true })
}

describe('normalizeHex и hexError — как раньше', () => {
  it('#rrggbb в любом регистре, с решёткой и без', () => {
    expect(normalizeHex('#12AB34')).toBe('#12ab34')
    expect(normalizeHex('12ab34')).toBe('#12ab34')
    expect(normalizeHex('  #FFFFFF ')).toBe('#ffffff')
  })
  it('короткое, длинное и не-hex — null', () => {
    for (const s of ['#12ab', '#fff', '#12ab345', '#gg0000', '', '#']) expect(normalizeHex(s)).toBeNull()
  })
  it('hexError: текст только для неверного', () => {
    expect(hexError('#12ab')).toMatch(/#rrggbb/)
    expect(hexError('#12ab34')).toBeNull()
  })
})

describe('наборы', () => {
  it('наборы: 5 по 8, HEX верные, имена без повторов', () => {
    expect(SETS.map((s) => s.title)).toEqual(['Нейтральные', 'Пастель', 'Пудра и нюд', 'Шампань', 'Природа'])
    const names = new Set<string>()
    const hexes = new Set<string>()
    for (const s of SETS) {
      expect(s.colors).toHaveLength(8)
      for (const e of s.colors) {
        expect(normalizeHex(e.hex)).toBe(e.hex)
        expect(e.name).not.toBe('')
        names.add(e.name)
        hexes.add(e.hex)
      }
    }
    expect(names.size).toBe(40)
    expect(hexes.size).toBe(40)
  })
  it('nameOf: имя набора, иначе «Свой цвет»', () => {
    expect(nameOf('#f1ddd6')).toBe('Пудра')
    expect(nameOf('#F1DDD6')).toBe('Пудра')
    expect(nameOf('#123456')).toBe('Свой цвет')
    expect(nameOf('мусор')).toBe('Свой цвет')
  })
})

describe('HSV', () => {
  it('hexToHsv → hsvToHex возвращает тот же цвет (сетка 16³)', () => {
    const hx = (n: number) => (n * 17).toString(16).padStart(2, '0')
    for (let r = 0; r < 16; r++)
      for (let g = 0; g < 16; g++)
        for (let b = 0; b < 16; b++) {
          const c = '#' + hx(r) + hx(g) + hx(b)
          expect(hsvToHex(hexToHsv(c))).toBe(c)
        }
  })
  it('известные точки и мусор', () => {
    expect(hexToHsv('#ff0000')).toEqual({ h: 0, s: 1, v: 1 })
    expect(hexToHsv('#00ff00').h).toBe(120)
    expect(hexToHsv('#0000ff').h).toBe(240)
    expect(hexToHsv('#808080').s).toBe(0)
    expect(hexToHsv('нет')).toEqual({ h: 0, s: 0, v: 0 })
    expect(hsvToHex({ h: 360, s: 1, v: 1 })).toBe('#ff0000')
    expect(hsvToHex({ h: -120, s: 1, v: 1 })).toBe('#0000ff')
  })
})

describe('localStorage', () => {
  beforeEach(() => useStorage(fakeStorage()))

  it('legacyRecent отбрасывает мусор и повторы', () => {
    localStorage.setItem('recentColors', JSON.stringify(['#ABCDEF', 5, 'bad', '#abcdef', '#010101']))
    expect(legacyRecent()).toEqual(['#abcdef', '#010101'])
    localStorage.setItem('recentColors', '{не json')
    expect(legacyRecent()).toEqual([])
    localStorage.setItem('recentColors', '{"a":1}')
    expect(legacyRecent()).toEqual([])
  })
  it('legacyRecent не больше 10, dropLegacyRecent стирает ключ', () => {
    localStorage.setItem('recentColors', JSON.stringify(Array.from({ length: 14 }, (_, i) => '#' + String(i).padStart(6, '0'))))
    expect(legacyRecent()).toHaveLength(10)
    dropLegacyRecent()
    expect(localStorage.getItem('recentColors')).toBeNull()
    expect(legacyRecent()).toEqual([])
  })
  it('вкладка: первая «Нейтральные», запоминается, мусор — первая', () => {
    expect(loadTab()).toBe('neutral')
    saveTab('powder')
    expect(loadTab()).toBe('powder')
    saveTab(PHOTO_TAB)
    expect(loadTab()).toBe('photo')
    saveTab('чужая')
    expect(loadTab()).toBe('neutral')
  })
  it('хранилище бросает — не падаем', () => {
    useStorage(fakeStorage(true))
    expect(legacyRecent()).toEqual([])
    expect(loadTab()).toBe('neutral')
    expect(() => saveTab('pastel')).not.toThrow()
    expect(() => dropLegacyRecent()).not.toThrow()
  })
})
