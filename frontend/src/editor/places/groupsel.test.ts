import { describe, expect, it } from 'vitest'
import { EMPTY, count, isGroup, normalize, pruneSel, pruneTexts, selectAll, toggleRef, toggleText, zoneOf, type GroupSel } from './groupsel'
import type { SpreadView } from '../../base/core/types'

const sel = (o: Partial<GroupSel>): GroupSel => ({ ...EMPTY, ...o })
const spread = (o: object) => o as unknown as SpreadView

describe('toggleRef', () => {
  it('добавляет и убирает фото', () => {
    const a = toggleRef(sel({ refs: ['p1'] }), 'p2')
    expect(a.refs).toEqual(['p1', 'p2'])
    expect(toggleRef(a, 'p1').refs).toEqual(['p2'])
    expect(isGroup(a)).toBe(true)
  })
  it('одиночный текст переходит в группу', () => {
    const a = toggleRef(sel({ text: 't1' }), 'p1')
    expect(a).toEqual({ refs: ['p1'], texts: ['t1'], text: null })
  })
  it('убрали последнее фото из группы с одним текстом — текст снова одиночный', () => {
    expect(toggleRef(sel({ refs: ['p1'], texts: ['t1'] }), 'p1')).toEqual({ refs: [], texts: [], text: 't1' })
  })
  it('убрали единственное — пусто', () => {
    expect(count(toggleRef(sel({ refs: ['p1'] }), 'p1'))).toBe(0)
  })
})

describe('toggleText', () => {
  it('текст при выделенных фото уходит в группу', () => {
    expect(toggleText(sel({ refs: ['p1'] }), 't1')).toEqual({ refs: ['p1'], texts: ['t1'], text: null })
  })
  it('без выделения — одиночный', () => {
    expect(toggleText(EMPTY, 't1')).toEqual(sel({ text: 't1' }))
  })
  it('второй текст при одиночном — группа из двух текстов', () => {
    expect(toggleText(sel({ text: 't1' }), 't2')).toEqual({ refs: [], texts: ['t1', 't2'], text: null })
  })
  it('убрать текст из группы; остался один текст без фото — одиночный', () => {
    expect(toggleText(sel({ texts: ['t1', 't2'] }), 't1')).toEqual(sel({ text: 't2' }))
    expect(toggleText(sel({ refs: ['p1'], texts: ['t1'] }), 't1')).toEqual(sel({ refs: ['p1'] }))
  })
  it('повторный Shift по одиночному снимает выделение', () => {
    expect(count(toggleText(sel({ text: 't1' }), 't1'))).toBe(0)
  })
})

describe('normalize и счётчики', () => {
  it('два текста — группа, фото + текст — группа, один текст — нет', () => {
    expect(isGroup(sel({ texts: ['a', 'b'] }))).toBe(true)
    expect(isGroup(sel({ refs: ['p'], texts: ['a'] }))).toBe(true)
    expect(isGroup(sel({ text: 'a' }))).toBe(false)
    expect(normalize(sel({ refs: ['p'], text: 'a' }))).toEqual({ refs: ['p'], texts: ['a'], text: null })
  })
})

describe('selectAll (Ctrl+A на холсте)', () => {
  const auto = spread({ grid: false, photos: ['a', 'b'], texts: [{ id: 't1' }], captions: [{ photo: 'a' }] })
  const grid = spread({
    grid: true,
    photos: ['a'],
    places: [
      { id: 'pl1', photo: 'a' },
      { id: 'pl2', photo: '' },
    ],
    texts: [],
  })
  it('«Авто»: фото и тексты, без подписей', () => {
    expect(selectAll(auto)).toEqual({ refs: ['a', 'b'], texts: ['t1'], text: null })
  })
  it('«Ручная»: занятое место по фото, пустое по ID', () => {
    expect(selectAll(grid).refs).toEqual(['a', 'pl2'])
  })
  it('на развороте только один текст — он одиночный', () => {
    expect(selectAll(spread({ grid: false, photos: [], texts: [{ id: 't1' }] }))).toEqual(sel({ text: 't1' }))
  })
  it('нет разворота — пусто', () => {
    expect(selectAll(undefined)).toEqual(EMPTY)
  })
})

describe('pruneTexts', () => {
  it('исчезнувшие тексты выпадают, целый список возвращается как есть', () => {
    const s = spread({ texts: [{ id: 'a' }] })
    expect(pruneTexts(['a', 'b'], s)).toEqual(['a'])
    const same = ['a']
    expect(pruneTexts(same, s)).toBe(same)
  })
})

describe('pruneSel', () => {
  it('из группы из двух текстов выпал один — оставшийся одиночный', () => {
    const s = spread({ texts: [{ id: 'a' }] })
    expect(pruneSel({ refs: [], texts: ['a', 'b'], text: null }, s)).toEqual({ refs: [], texts: [], text: 'a' })
  })
  it('с фото остаток остаётся в группе', () => {
    const s = spread({ texts: [{ id: 'a' }] })
    expect(pruneSel({ refs: ['p'], texts: ['a', 'b'], text: null }, s)).toEqual({ refs: ['p'], texts: ['a'], text: null })
  })
  it('выпали все тексты — выделение пустеет', () => {
    expect(pruneSel({ refs: [], texts: ['a', 'b'], text: null }, spread({ texts: [] }))).toEqual(EMPTY)
  })
})

describe('zoneOf', () => {
  const el = (match: string) => ({ closest: (q: string) => (q.split(',').some((x) => x.trim() === match) ? {} : null) })
  it('библиотека, холст и лента альбома; прочее — null', () => {
    expect(zoneOf(el('.library'))).toBe('library')
    expect(zoneOf(el('.stage'))).toBe('canvas')
    expect(zoneOf(el('.tl'))).toBe('canvas')
    expect(zoneOf(el('.other'))).toBeNull()
    expect(zoneOf(null)).toBeNull()
  })
})
