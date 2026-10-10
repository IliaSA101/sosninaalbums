import { beforeEach, describe, expect, it } from 'vitest'
import { hexError, loadRecent, normalizeHex, pushRecent, RECENT_MAX, rememberColor, SWATCHES } from './palette'

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
  }
}

function useStorage(s: ReturnType<typeof fakeStorage>) {
  Object.defineProperty(globalThis, 'localStorage', { value: s, configurable: true, writable: true })
}

describe('normalizeHex', () => {
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

describe('образцы', () => {
  it('12 штук, все верные и без повторов', () => {
    expect(SWATCHES).toHaveLength(12)
    expect(new Set(SWATCHES).size).toBe(12)
    for (const c of SWATCHES) expect(normalizeHex(c)).toBe(c)
  })
})

describe('pushRecent', () => {
  it('новый в начало, повтор поднимается без дубля', () => {
    expect(pushRecent(['#111111', '#222222'], '#333333')).toEqual(['#333333', '#111111', '#222222'])
    expect(pushRecent(['#111111', '#222222'], '#222222')).toEqual(['#222222', '#111111'])
    expect(pushRecent(['#111111'], '#111111')).toEqual(['#111111'])
  })
  it('не больше 8, старейший вытесняется', () => {
    let l: string[] = []
    for (let i = 0; i < 12; i++) l = pushRecent(l, '#' + String(i).padStart(6, '0'))
    expect(l).toHaveLength(RECENT_MAX)
    expect(l[0]).toBe('#000011')
    expect(l).not.toContain('#000000')
  })
  it('верхний регистр приводится, мусор игнорируется', () => {
    expect(pushRecent([], '#ABCDEF')).toEqual(['#abcdef'])
    expect(pushRecent(['#111111'], 'oops')).toEqual(['#111111'])
  })
})

describe('localStorage', () => {
  beforeEach(() => useStorage(fakeStorage()))

  it('запоминает и читает', () => {
    rememberColor('#AA0000')
    expect(rememberColor('#00bb00')).toEqual(['#00bb00', '#aa0000'])
    expect(loadRecent()).toEqual(['#00bb00', '#aa0000'])
  })
  it('битые данные -> пустой список', () => {
    localStorage.setItem('recentColors', '{не json')
    expect(loadRecent()).toEqual([])
    localStorage.setItem('recentColors', '{"a":1}')
    expect(loadRecent()).toEqual([])
  })
  it('в списке остаются только верные цвета без повторов', () => {
    localStorage.setItem('recentColors', JSON.stringify(['#ABCDEF', 5, 'bad', '#abcdef', '#010101']))
    expect(loadRecent()).toEqual(['#abcdef', '#010101'])
  })
  it('хранилище бросает — не падаем', () => {
    useStorage(fakeStorage(true))
    expect(loadRecent()).toEqual([])
    expect(rememberColor('#123456')).toEqual(['#123456'])
  })
})
