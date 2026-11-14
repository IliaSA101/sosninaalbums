import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go и store подменены: проверяем, какой метод зовут поля имени и что они показывают.
vi.mock('../../base/core/store.svelte', () => ({
  store: { run: vi.fn(async () => true) },
  Api: {
    Person: vi.fn(async () => ({ last: 'Файлов' })),
    PlacePerson: vi.fn(async () => ({ last: 'Орлова', place: 'p1' })),
    SetPerson: vi.fn(() => 'setPerson'),
    SetPlacePerson: vi.fn(() => 'setPlacePerson'),
  },
}))

import { Api, store } from '../../base/core/store.svelte'
import { cardOf, loadPerson, savePerson, seatNoteText, SEAT_NOTE, SEAT_SUBJECT_NOTE } from './personapi'
import type { PersonView } from '../../base/core/types'

beforeEach(() => vi.clearAllMocks())

describe('loadPerson', () => {
  it('фото — Person(photo)', async () => {
    expect(await loadPerson({ photo: 'a' })).toMatchObject({ last: 'Файлов' })
    expect(Api.Person).toHaveBeenCalledWith('a')
    expect(Api.PlacePerson).not.toHaveBeenCalled()
  })
  it('пустое место — PlacePerson(разворот, место)', async () => {
    expect(await loadPerson({ photo: '', place: { spread: 's1', id: 'p1' } })).toMatchObject({ last: 'Орлова' })
    expect(Api.PlacePerson).toHaveBeenCalledWith('s1', 'p1')
    expect(Api.Person).not.toHaveBeenCalled()
  })
})

describe('savePerson', () => {
  const p = { last: 'Орлова', first: '', middle: '', subj: '' }
  it('фото — SetPerson с сессией (Go сам направит в табличку места)', async () => {
    await savePerson({ photo: 'a' }, p, 'sess1')
    expect(Api.SetPerson).toHaveBeenCalledWith('a', p, 'sess1')
    expect(Api.SetPlacePerson).not.toHaveBeenCalled()
    expect(store.run).toHaveBeenCalledWith('setPerson')
  })
  it('пустое место — SetPlacePerson с той же сессией', async () => {
    await savePerson({ photo: '', place: { spread: 's1', id: 'p1' } }, p, 'sess2')
    expect(Api.SetPlacePerson).toHaveBeenCalledWith('s1', 'p1', p, 'sess2')
    expect(Api.SetPerson).not.toHaveBeenCalled()
    expect(store.run).toHaveBeenCalledWith('setPlacePerson')
  })
  it('очистка всех полей уходит как есть — табличку снимает Go', async () => {
    const none = { last: '', first: '', middle: '', subj: '' }
    await savePerson({ photo: '', place: { spread: 's1', id: 'p1' } }, none, 's')
    expect(Api.SetPlacePerson).toHaveBeenCalledWith('s1', 'p1', none, 's')
  })
})

const view = (o: Partial<PersonView>): PersonView => ({
  last: '',
  first: '',
  middle: '',
  subject: '',
  fromFile: false,
  teacher: false,
  place: '',
  seatNamed: false,
  ...o,
})

describe('seatNoteText', () => {
  it('фото на месте с табличкой-именем — «Имя места»', () => {
    const pv = view({ last: 'Петров', first: 'Ваня', place: 'p1', seatNamed: true })
    expect(seatNoteText({ photo: 'a' }, pv)).toBe(SEAT_NOTE)
    expect(SEAT_NOTE).toContain('Имя места')
  })
  it('табличка только с предметом, имя из файла — «Предмет места»', () => {
    const pv = view({ last: 'Сидорова', first: 'Ольга', subject: 'Математика', fromFile: true, place: 'p1' })
    expect(seatNoteText({ photo: 'a' }, pv)).toBe(SEAT_SUBJECT_NOTE)
    expect(SEAT_SUBJECT_NOTE).toContain('Предмет места')
  })
  it('табличка только с предметом, имени нет совсем — «Предмет места»', () => {
    expect(seatNoteText({ photo: 'a' }, view({ subject: 'Физика', place: 'p1' }))).toBe(SEAT_SUBJECT_NOTE)
  })
  it('своё имя фото «Козлов Олег» на табличке только с предметом — «Предмет места»', () => {
    const pv = view({ last: 'Козлов', first: 'Олег', subject: 'Физика', place: 'p1', seatNamed: false })
    expect(seatNoteText({ photo: 'a' }, pv)).toBe(SEAT_SUBJECT_NOTE)
  })
  it('фото без таблички и само пустое место — нет подсказки', () => {
    expect(seatNoteText({ photo: 'a' }, view({ last: 'Файлов', fromFile: true }))).toBe('')
    expect(seatNoteText({ photo: '', place: { spread: 's1', id: 'p1' } }, view({ last: 'Орлова', place: 'p1' }))).toBe('')
  })
})

describe('cardOf', () => {
  it('под табличкой только с предметом пометка «из имени файла» остаётся, как без таблички', () => {
    const base = { last: 'Сидорова', first: 'Ольга', subject: '', fromFile: true }
    const withSeat = cardOf({ photo: 'a' }, view({ ...base, subject: 'Математика', place: 'p1' }))
    const noSeat = cardOf({ photo: 'a' }, view(base))
    expect(withSeat.fromFile).toBe(true)
    expect(noSeat.fromFile).toBe(true)
    expect(withSeat.draft).toEqual({ last: 'Сидорова', first: 'Ольга', middle: '', subj: 'Математика' })
  })
  it('табличка с именем — без пометки, подсказка «Имя места»', () => {
    const c = cardOf({ photo: 'a' }, view({ last: 'Петров', first: 'Ваня', place: 'p1', seatNamed: true }))
    expect(c.fromFile).toBe(false)
    expect(c.note).toBe(SEAT_NOTE)
  })
  it('сама табличка пустого места — подсказки нет, предмет в draft.subj', () => {
    const c = cardOf({ photo: '', place: { spread: 's1', id: 'p1' } }, view({ subject: 'Музыка', place: 'p1' }))
    expect(c.note).toBe('')
    expect(c.fromFile).toBe(false)
    expect(c.draft.subj).toBe('Музыка')
  })
})
