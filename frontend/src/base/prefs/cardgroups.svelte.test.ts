import { beforeAll, beforeEach, describe, expect, it } from 'vitest'
import { CardGroups, GROUP_IDS, normGroups } from './cardgroups.svelte'

// Минимальное хранилище вместо localStorage (среда тестов — node); пишет в общую Map, читать можно из «нового запуска».
const mem = new Map<string, string>()
let broken = false
function useStorage() {
  mem.clear()
  broken = false
  const s = {
    getItem: (k: string) => {
      if (broken) throw new Error('нет хранилища')
      return mem.get(k) ?? null
    },
    setItem: (k: string, v: string) => {
      if (broken) throw new Error('нет хранилища')
      mem.set(k, v)
    },
  }
  Object.defineProperty(globalThis, 'localStorage', { value: s, configurable: true, writable: true })
}

describe('normGroups: разбор хранилища', () => {
  it('мусор и не-массивы — всё открыто', () => {
    for (const x of [null, undefined, 5, 'x', [], true, { d: 'photo.frame', f: 7 }, { d: { 0: 'photo.frame' } }]) {
      expect(normGroups(x)).toEqual({ d: [], f: [] })
    }
  })

  it('чужие id и не-строки отбрасываются, повторы схлопываются, порядок — как в GROUP_IDS', () => {
    const r = normGroups({ d: ['spread.bg', 'нет такой', 7, null, 'photo.frame', 'photo.frame'], f: ['cover.size'], x: ['photo.tone'] })
    expect(r).toEqual({ d: ['photo.frame', 'spread.bg'], f: ['cover.size'] })
  })
})

describe('CardGroups: память свёрнутых', () => {
  beforeAll(() => useStorage())
  beforeEach(useStorage)

  it('первый запуск: всё открыто на обоих хостах', () => {
    const g = new CardGroups()
    for (const id of GROUP_IDS) {
      expect(g.isOpen('d', id)).toBe(true)
      expect(g.isOpen('f', id)).toBe(true)
    }
    expect(mem.has('cardGroups')).toBe(false) // читать — не значит писать
  })

  it('d и f раздельны', () => {
    const g = new CardGroups()
    expect(g.toggle('f', 'photo.tone')).toBe(false)
    expect(g.isOpen('f', 'photo.tone')).toBe(false)
    expect(g.isOpen('d', 'photo.tone')).toBe(true)
    expect(JSON.parse(mem.get('cardGroups')!)).toEqual({ d: [], f: ['photo.tone'] })
  })

  it('переживает перезапуск; повторный щелчок возвращает открытой', () => {
    const g = new CardGroups()
    g.toggle('d', 'spread.bg')
    g.toggle('d', 'photo.frame')
    const again = new CardGroups()
    expect(again.isOpen('d', 'spread.bg')).toBe(false)
    expect(again.isOpen('d', 'photo.frame')).toBe(false)
    expect(again.toggle('d', 'spread.bg')).toBe(true)
    expect(JSON.parse(mem.get('cardGroups')!)).toEqual({ d: ['photo.frame'], f: [] })
  })

  it('open: раскрывает с записью; уже открытую не трогает и не пишет', () => {
    const g = new CardGroups()
    expect(g.open('d', 'photo.about')).toBe(false)
    expect(mem.has('cardGroups')).toBe(false)
    g.toggle('d', 'photo.about')
    expect(g.open('d', 'photo.about')).toBe(true)
    expect(g.isOpen('d', 'photo.about')).toBe(true)
    expect(JSON.parse(mem.get('cardGroups')!)).toEqual({ d: [], f: [] })
  })

  it('повреждённый JSON и чужие значения: всё открыто, без исключения', () => {
    for (const raw of ['{', 'null', '"x"', '[1,2]', '{"d":"photo.frame"}']) {
      mem.set('cardGroups', raw)
      const g = new CardGroups()
      expect(g.isOpen('d', 'photo.frame')).toBe(true)
    }
  })

  it('чужие id в хранилище не оживают после записи', () => {
    mem.set('cardGroups', JSON.stringify({ d: ['photo.frame', 'old.group'], f: [] }))
    const g = new CardGroups()
    g.toggle('d', 'photo.tone')
    expect(JSON.parse(mem.get('cardGroups')!)).toEqual({ d: ['photo.frame', 'photo.tone'], f: [] })
  })

  it('reset: сброс памяти — всё открыто и записано', () => {
    const g = new CardGroups()
    g.toggle('d', 'photo.frame')
    g.toggle('f', 'cover.bg')
    g.reset()
    expect(g.isOpen('d', 'photo.frame')).toBe(true)
    expect(new CardGroups().isOpen('f', 'cover.bg')).toBe(true)
  })

  it('хранилище недоступно: работает в памяти окна, исключений нет', () => {
    broken = true
    const g = new CardGroups()
    expect(g.isOpen('d', 'photo.frame')).toBe(true)
    g.toggle('d', 'photo.frame')
    expect(g.isOpen('d', 'photo.frame')).toBe(false)
  })
})
