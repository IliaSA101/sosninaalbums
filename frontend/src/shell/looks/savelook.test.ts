import { describe, expect, it } from 'vitest'
import { SAVE_HINT } from '../../panels/decor/spreadtpl'
import {
  DEFAULT_THEME,
  NO_REPEAT,
  SAVE_LOOK_HINT,
  SAVE_LOOK_TITLE,
  clampPos,
  clampTo,
  countOf,
  defaultRepeat,
  defaultScope,
  hasRepeat,
  saveOpts,
  savedText,
  scopeItems,
  themeItems,
} from './savelook'

// «Сохранить альбом как макет…» (131 U3, сц. 12): слова, зажимы, сборка параметров SaveLook.
describe('умолчания', () => {
  it('повторяемые — со второго по предпоследний', () => {
    expect(defaultRepeat(10)).toEqual({ from: 2, to: 9 })
    expect(defaultRepeat(3)).toEqual({ from: 2, to: 2 })
  })
  it('меньше трёх разворотов — повторяемых нет (с больше по)', () => {
    for (const n of [1, 2]) {
      const r = defaultRepeat(n)
      expect(hasRepeat(r.from, r.to), String(n)).toBe(false)
    }
    expect(hasRepeat(2, 9)).toBe(true)
    expect(hasRepeat(4, 4)).toBe(true)
  })
  it('«Выделенные» по умолчанию — от двух выделенных; один случайный щелчок не в счёт', () => {
    expect(defaultScope(0)).toBe('all')
    expect(defaultScope(1)).toBe('all')
    expect(defaultScope(2)).toBe('selected')
  })
  it('тема по умолчанию — «Мои»', () => expect(DEFAULT_THEME).toBe('mine'))
})

describe('число разворотов и позиции', () => {
  it('«Все» — все развороты, «Выделенные» — выделенные', () => {
    expect(countOf('all', 12, 3)).toBe(12)
    expect(countOf('selected', 12, 3)).toBe(3)
    expect(countOf('selected', 12, 0)).toBe(0)
  })
  it('позиция зажата в 1…N; мусор — 1', () => {
    expect(clampPos(0, 9)).toBe(1)
    expect(clampPos(99, 9)).toBe(9)
    expect(clampPos(4.7, 9)).toBe(4)
    expect(clampPos(NaN, 9)).toBe(1)
    expect(clampPos(5, 0)).toBe(1) // выбрано ноль — предел 1, окно «Сохранить» всё равно серое
  })
  it('«по» зажато в 0…N; мусор и пусто — 0 (повторяемых нет)', () => {
    expect(clampTo(NaN, 9)).toBe(0)
    expect(clampTo(-3, 9)).toBe(0)
    expect(clampTo(99, 9)).toBe(9)
    expect(clampTo(4.7, 9)).toBe(4)
  })
  it('«Выделенные» серые, пока в ленте ничего не выделено, с подсказкой что делать', () => {
    const [all, sel] = scopeItems(0)
    expect(all).toMatchObject({ id: 'all', label: 'Все' })
    expect(sel).toMatchObject({ id: 'selected', label: 'Выделенные', off: true })
    expect(sel.title).toBe('Выделите развороты в ленте (Ctrl+щелчок, на Mac — ⌘)')
    expect(scopeItems(2)[1].off).toBe(false)
  })
})

describe('темы', () => {
  it('«Недавние» и «Все» не темы для сохранения; «Мои» есть всегда', () => {
    const t = [
      { id: 'recent', name: 'Недавние', count: 0 },
      { id: 'all', name: 'Все', count: 0 },
      { id: 'wedding', name: 'Свадьба', count: 0 },
    ]
    expect(themeItems({ themes: t }).map((x) => x.id)).toEqual(['wedding', 'mine'])
    expect(themeItems({ themes: [{ id: 'mine', name: 'Мои', count: 0 }] }).map((x) => x.id)).toEqual(['mine'])
    expect(themeItems(null)).toEqual([{ id: 'mine', name: 'Мои' }])
  })
})

describe('параметры SaveLook', () => {
  const form = { theme: 'mine', name: '  Мой макет ', ids: ['a', 'b', 'c'], from: 2, to: 2, places: true, drafts: false, cover: true }
  it('имя обрезается по краям; поля по design §8', () => {
    expect(saveOpts(form, true)).toEqual({
      theme: 'mine',
      name: 'Мой макет',
      ids: ['a', 'b', 'c'],
      repFrom: 2,
      repTo: 2,
      places: true,
      drafts: false,
      cover: true,
    })
  })
  it('нет обложки в альбоме — флажок уходит выключенным, как бы он ни стоял', () => {
    expect(saveOpts(form, false).cover).toBe(false)
  })
  it('подсказка окна начинается подсказкой «Сохранить разворот» и не повторяет её дословно', () => {
    expect(SAVE_LOOK_HINT.startsWith(SAVE_HINT)).toBe(true)
    expect(SAVE_LOOK_HINT).toContain('шагов не больше 24')
    // Сливаются только повторяемые (0.28-131-151); текст заготовок входит как есть (0.28-131-152).
    expect(SAVE_LOOK_HINT).toContain('Одинаковые повторяемые развороты сольются')
    expect(SAVE_LOOK_HINT).toContain('«[Имена]», «[Дата]»')
  })
  it('слова', () => {
    expect(SAVE_LOOK_TITLE).toBe('Сохранить альбом как макет')
    expect(savedText('Мой')).toBe('Макет «Мой» сохранён в библиотеку')
    expect(NO_REPEAT).toContain('Повторяемых нет')
  })
})
