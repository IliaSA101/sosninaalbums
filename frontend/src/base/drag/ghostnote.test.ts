import { describe, expect, it } from 'vitest'
import type { Payload, Target } from '../core/types'
import { ghostNote, REPLACE_NOTE, replaceKeyNote, type NoteIn } from './ghostnote'
import { HINT_TEXT } from './wheelhint.svelte'

const one: Payload = { kind: 'photos', ids: ['a'] }
const cell = (side: 'before' | 'after' | 'center', many?: boolean): Target => ({
  kind: 'cell',
  spread: 's',
  index: 0,
  photo: 'x',
  side,
  many,
})
const base: NoteIn = { target: null, payload: one, keep: false, mac: false, used: false, wheel: false }
const note = (o: Partial<NoteIn>) => ghostNote({ ...base, ...o })

describe('ghostNote (168)', () => {
  it('замена на «Авто» (Ctrl, центр) — «Заменить фото»', () => {
    expect(note({ target: cell('center'), keep: true })).toBe('Заменить фото')
    expect(REPLACE_NOTE).toBe('Заменить фото')
  })
  it('занятое место ручного из ленты — то же слово; пустое — нет', () => {
    expect(note({ target: { kind: 'place', spread: 's', place: 'p', photo: 'x' } })).toBe(REPLACE_NOTE)
    expect(note({ target: { kind: 'place', spread: 's', place: 'p', photo: null } })).toBe('')
  })
  it('фото из альбома — ни замены, ни подсказки клавиши', () => {
    const p: Payload = { kind: 'photos', ids: ['a'], fromSpread: 's1' }
    expect(note({ payload: p, target: cell('center') })).toBe('')
    expect(note({ payload: p, target: { kind: 'place', spread: 's', place: 'p', photo: 'x' } })).toBe('')
  })
  it('подсказка клавиши: одно фото из ленты над фото «Авто» без клавиши', () => {
    expect(note({ target: cell('before') })).toBe('Ctrl — заменить фото')
    expect(note({ target: cell('after'), mac: true })).toBe('⌘ — заменить фото')
    expect(replaceKeyNote(true)).toBe('⌘ — заменить фото')
  })
  it('после первой замены подсказка клавиши молчит', () => {
    expect(note({ target: cell('before'), used: true })).toBe('')
  })
  it('несколько фото: клавиши не подсказываем, с Ctrl (many) строки нет', () => {
    const two: Payload = { kind: 'photos', ids: ['a', 'b'] }
    expect(note({ payload: two, target: cell('before') })).toBe('')
    expect(note({ payload: two, target: cell('center', true), keep: true })).toBe('')
  })
  it('не над фото «Авто» — подсказки клавиши нет', () => {
    expect(note({ target: { kind: 'spread', spread: 's' } })).toBe('')
    expect(note({ target: null })).toBe('')
  })
  it('колесо идёт третьим: при строке про замену не видно', () => {
    expect(note({ wheel: true })).toBe(HINT_TEXT)
    expect(note({ wheel: true, target: cell('center'), keep: true })).toBe(REPLACE_NOTE)
    expect(note({ wheel: true, target: cell('before') })).toBe('Ctrl — заменить фото')
    expect(note({ wheel: true, target: cell('before'), used: true })).toBe(HINT_TEXT)
  })
  it('перенос разворота — только колесо', () => {
    expect(note({ payload: { kind: 'spread', id: 's', index: 0 }, wheel: true })).toBe(HINT_TEXT)
    expect(note({ payload: null })).toBe('')
  })
})
