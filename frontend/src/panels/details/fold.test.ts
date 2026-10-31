import { describe, expect, it } from 'vitest'
import { FOLD_ITEMS, foldChangeNote, foldFields, foldNote, foldOf, type FoldMode } from './fold'

const MODES: FoldMode[] = ['none', 'wide', 'any']

describe('положение из пары полей', () => {
  it('!layflat — нет; layflat без foldAny — широкие; обе — любые', () => {
    expect(foldOf({ layflat: false, foldAny: false })).toBe('none')
    expect(foldOf({ layflat: true, foldAny: false })).toBe('wide')
    expect(foldOf({ layflat: true, foldAny: true })).toBe('any')
  })
  it('foldAny без layflat — «нет»; нет поля foldAny (Go ещё не отдал) — строгое правило', () => {
    expect(foldOf({ layflat: false, foldAny: true })).toBe('none')
    expect(foldOf({ layflat: true })).toBe('wide')
    expect(foldOf({})).toBe('none')
  })
  it('туда и обратно для каждого положения', () => {
    for (const m of MODES) expect(foldOf(foldFields(m))).toBe(m)
    expect(foldFields('none')).toEqual({ layflat: false, foldAny: false })
    expect(foldFields('wide')).toEqual({ layflat: true, foldAny: false })
    expect(foldFields('any')).toEqual({ layflat: true, foldAny: true })
  })
})

describe('переключатель и строки', () => {
  it('три пункта в порядке «Нет · Только широкие · Любые фото», у каждого подсказка', () => {
    expect(FOLD_ITEMS.map((i) => [i.id, i.label])).toEqual([
      ['none', 'Нет'],
      ['wide', 'Только широкие'],
      ['any', 'Любые фото'],
    ])
    expect(FOLD_ITEMS.every((i) => !!i.title)).toBe(true)
  })
  it('три разные строки-пояснения', () => {
    const notes = MODES.map(foldNote)
    expect(new Set(notes).size).toBe(3)
    expect(notes[1]).toContain('1,2 раза')
    expect(notes[1]).toContain('20 %')
    expect(notes[2]).toContain('Лицо может оказаться на сгибе')
  })
  it('вторая строка: пусто при равенстве, для «Нет» — про перекладку, иначе — про свёрстанное', () => {
    for (const m of MODES) expect(foldChangeNote(m, m), m).toBe('')
    for (const a of ['wide', 'any'] as const) expect(foldChangeNote(a, 'none')).toContain('разложатся заново')
    expect(foldChangeNote('none', 'wide')).toContain('останутся как есть')
    expect(foldChangeNote('wide', 'any')).toContain('останутся как есть')
    expect(foldChangeNote('any', 'wide')).toContain('останутся как есть')
    expect(foldChangeNote('none', 'any')).toContain('останутся как есть')
  })
})
