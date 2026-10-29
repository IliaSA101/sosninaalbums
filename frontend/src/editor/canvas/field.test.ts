import { describe, expect, it } from 'vitest'
import { emptyField } from './field'

describe('emptyField', () => {
  // Минимальный двойник элемента: селекторы ищем по набору «классов предков».
  const el = (cls: string[], self: string[] = cls) =>
    ({
      closest: (sel: string) => (sel.split(',').some((x) => cls.includes(x.trim().replace(/^[.[\]]|\]$/g, ''))) ? {} : null),
      matches: (sel: string) => sel.split(',').some((x) => self.includes(x.trim().replace(/^\./, ''))),
    }) as unknown as Element
  it('пустая страница и корешок — поле', () => {
    expect(emptyField(el(['spread'], ['page']))).toBe(true)
    expect(emptyField(el(['spread'], ['content']))).toBe(true)
    expect(emptyField(el(['spread'], ['spine']))).toBe(true)
  })
  it('объект на развороте — не поле', () => expect(emptyField(el(['spread'], ['cell']))).toBe(false))
  it('сосед ([data-peer]) — не поле', () => {
    expect(emptyField(el(['spread', 'data-peer'], ['page']))).toBe(false)
    expect(emptyField(el(['data-peer']))).toBe(false)
  })
  it('серое поле — поле; полоса, панели и пунктиры new/first — нет', () => {
    expect(emptyField(el([]))).toBe(true)
    expect(emptyField(el(['data-chrome']))).toBe(false)
    expect(emptyField(el(['new']))).toBe(false)
    expect(emptyField(el(['first']))).toBe(false)
  })
  it('null — не поле', () => expect(emptyField(null)).toBe(false))
})
