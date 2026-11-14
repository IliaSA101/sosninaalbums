import { describe, expect, it } from 'vitest'
import { VIG_CLEAR, VIG_SAVE, vigMore } from './vigmenu'

describe('vigMore: «⋯» группы «Виньетка»', () => {
  it('со «Своими шаблонами» — два пункта в прежнем порядке и с прежними словами', () => {
    const list = vigMore('разворот', true)
    expect(list.map((n) => [n.id, n.label])).toEqual([
      [VIG_SAVE, 'Сохранить как шаблон…'],
      [VIG_CLEAR, 'Обычный разворот'],
    ])
    expect(list.every((n) => !!n.title)).toBe(true)
  })

  it('без функции «templates» остаётся «Обычный …»; на листе — «Обычный лист»', () => {
    expect(vigMore('разворот', false).map((n) => n.id)).toEqual([VIG_CLEAR])
    expect(vigMore('лист', false)[0].label).toBe('Обычный лист')
  })
})
