import { describe, expect, it } from 'vitest'
import { emptyPerson, isPasteKey, personLine, personOf } from './names'

const P = (last: string, first = '', middle = '') => ({ last, first, middle })

describe('person', () => {
  it('personLine, personOf и emptyPerson', () => {
    expect(personLine(P('Иванов', '', 'Сергеевич'))).toBe('Иванов Сергеевич')
    expect(personOf({ last: ' Иванов ' })).toEqual({ ...P('Иванов'), subj: '' })
    expect(emptyPerson({ last: ' ', first: '' })).toBe(true)
    expect(emptyPerson(P('', 'Аня'))).toBe(false)
  })

  it('предмет (049): уходит в Go ключом subj с обрезанными краями; предмет без имени — не пустое имя', () => {
    expect(personOf({ last: 'Петрова', subj: ' Математика ' })).toEqual({ last: 'Петрова', first: '', middle: '', subj: 'Математика' })
    expect(emptyPerson({ subj: 'Математика' })).toBe(false)
    expect(personLine({ last: 'Петрова', subj: 'Математика' })).toBe('Петрова')
  })
})

describe('isPasteKey', () => {
  const inside = {} as EventTarget
  const outside = {} as EventTarget
  const box = { contains: (n: unknown) => n === inside } as unknown as Element
  const ev = (target: EventTarget | null, over = {}) => ({ key: 'Enter', ctrlKey: true, metaKey: false, target, ...over })
  it('Ctrl/Cmd+Enter внутри окна, в том числе на самом окне без фокуса в поле', () => {
    expect(isPasteKey(ev(inside), box)).toBe(true)
    expect(isPasteKey(ev(inside, { ctrlKey: false, metaKey: true }), box)).toBe(true)
  })
  it('не наше: чужое окно, голый Enter, другая клавиша, окна нет', () => {
    expect(isPasteKey(ev(outside), box)).toBe(false)
    expect(isPasteKey(ev(inside, { ctrlKey: false }), box)).toBe(false)
    expect(isPasteKey(ev(inside, { key: 'a' }), box)).toBe(false)
    expect(isPasteKey(ev(inside), null)).toBe(false)
    expect(isPasteKey(ev(null), box)).toBe(false)
  })
})
