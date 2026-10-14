import { describe, expect, it } from 'vitest'
import { MAX_LINES, emptyPerson, parseNames, personLine, personOf, preview, previewSlots, slotsOf, summary } from './names'

const P = (last: string, first = '', middle = '') => ({ last, first, middle })

describe('parseNames', () => {
  it('Фамилия Имя Отчество по умолчанию', () => {
    expect(parseNames('Иванов Пётр\nПетрова Анна Сергеевна', false).people).toEqual([
      P('Иванов', 'Пётр'),
      P('Петрова', 'Анна', 'Сергеевна'),
    ])
  })

  it('«Имя первым»: Имя [Отчество] Фамилия, одно слово — Имя', () => {
    const r = parseNames('Пётр Иванов\nАнна Сергеевна Петрова\nМаша', true).people
    expect(r).toEqual([P('Иванов', 'Пётр'), P('Петрова', 'Анна', 'Сергеевна'), P('', 'Маша')])
  })

  it('нумерация снимается, пустые строки и CRLF пропускаются', () => {
    const r = parseNames('1. Сидоров Иван\r\n\r\n 2) Орлова Ольга\r\n', false)
    expect(r.error).toBe('')
    expect(r.people).toEqual([P('Сидоров', 'Иван'), P('Орлова', 'Ольга')])
  })

  it('число без точки — не нумерация', () => {
    expect(parseNames('2025 Год', false).people).toEqual([P('2025', 'Год')])
  })

  it('Tab и «;» делят строку (столбцы из Excel), пробелы внутри столбца остаются', () => {
    const r = parseNames('Иванов-Петров\tПётр Алексеевич\nСмирнов;Олег', false).people
    expect(r).toEqual([P('Иванов-Петров', 'Пётр Алексеевич'), P('Смирнов', 'Олег')])
  })

  it('три слова и больше: остаток в отчество', () => {
    expect(parseNames('Али оглы Ахмед Бек', false).people).toEqual([P('Али', 'оглы', 'Ахмед Бек')])
  })

  it('слово длиннее 60 знаков обрезается', () => {
    expect(parseNames('я'.repeat(80), false).people[0].last).toHaveLength(60)
  })

  it('слишком длинный список — ошибка без имён', () => {
    const r = parseNames('а\n'.repeat(MAX_LINES + 1), false)
    expect(r.people).toEqual([])
    expect(r.error).not.toBe('')
    expect(parseNames('я'.repeat(40000), false).error).not.toBe('') // 80 КБ в UTF-8
  })

  it('пустой ввод — пустой список', () => {
    expect(parseNames('', false)).toEqual({ people: [], error: '' })
  })
})

describe('preview', () => {
  const named = (ids: string[]) => (id: string) => ids.includes(id)

  it('3 строки на 4 фото: первые три получают имена, без имени — одно', () => {
    const r = preview(['a', 'b', 'c', 'd'], [P('1'), P('2'), P('3')], named([]))
    expect(r.rows.map((x) => x.line?.last ?? null)).toEqual(['1', '2', '3', null])
    expect(r.extra).toBe(0)
    expect(r.missing).toBe(1)
    expect(summary(r)).toBe('строк лишних: 0 / фото без имени: 1')
  })

  it('лишние строки считаются, имя из файла у фото без строки — не «без имени»', () => {
    const r = preview(['a', 'b'], [P('1'), P('2'), P('3'), P('4')], named([]))
    expect(r.extra).toBe(2)
    expect(preview(['a', 'b'], [P('1')], named(['b'])).missing).toBe(0)
  })
})

describe('person', () => {
  it('personLine, personOf и emptyPerson', () => {
    expect(personLine(P('Иванов', '', 'Сергеевич'))).toBe('Иванов Сергеевич')
    expect(personOf({ last: ' Иванов ' })).toEqual(P('Иванов'))
    expect(emptyPerson({ last: ' ', first: '' })).toBe(true)
    expect(emptyPerson(P('', 'Аня'))).toBe(false)
  })
})

describe('slotsOf / previewSlots: виньетка', () => {
  const pl = (photo: string, role?: string) => ({ photo, role }) as never
  // head-t-left: ученики, групповое, учитель последним
  const vig = {
    photos: ['p1', 'g', 'p2', 't'],
    vig: {} as never,
    places: [pl('p1'), pl('g', 'group'), pl('p2', 'pupil'), pl('t', 'teacher')],
  }
  const named = () => false

  it('альбом: роль пустая, порядок photos', () => {
    expect(slotsOf({ photos: ['a', 'b'], places: null, vig: undefined })).toEqual([
      { photo: 'a', role: '' },
      { photo: 'b', role: '' },
    ])
  })
  it('виньетка: роли из мест, пустые места пропущены', () => {
    const s = slotsOf({ ...vig, places: [...vig.places, pl('')] })
    expect(s.map((x) => x.role)).toEqual(['pupil', 'group', 'pupil', 'teacher'])
  })
  it('групповое пропускается, имена идут по порядку остальных; учитель — последний', () => {
    const r = previewSlots(slotsOf(vig), [P('1'), P('2'), P('3')], named)
    expect(r.slots.map((x) => x.tag)).toEqual(['ученик 1', 'групповое', 'ученик 2', 'учитель'])
    expect(r.slots.map((x) => x.line?.last ?? null)).toEqual(['1', null, '2', '3'])
    expect(r.slots.map((x) => x.no)).toEqual([1, 0, 2, 3])
    expect(r.slots[1].skipped).toBe(true)
    expect(r.extra).toBe(0)
    expect(r.missing).toBe(0)
  })
  it('строк мало — без имени остаются учитель, групповое не считается', () => {
    const r = previewSlots(slotsOf(vig), [P('1')], named)
    expect(r.missing).toBe(2)
    expect(r.slots[3].line).toBeNull()
  })
  it('лишние строки считаются от числа получающих', () => {
    expect(previewSlots(slotsOf(vig), [P('1'), P('2'), P('3'), P('4'), P('5')], named).extra).toBe(2)
  })
  it('не виньетка: без подписей роли', () => {
    const r = previewSlots(slotsOf({ photos: ['a'], places: null, vig: undefined }), [P('1')], named)
    expect(r.slots).toEqual([{ photo: 'a', tag: '', skipped: false, line: P('1'), no: 1 }])
  })
})
