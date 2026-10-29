import { describe, expect, it } from 'vitest'
import { shapeArg } from '../../base/geom/shape'
import { PLACE_ADD_ITEMS } from './placeadd'

// Меню «Место ▾» (076): три пункта в порядке прямоугольное · скруглённое · круглое; выбор шлёт ключ формы в AddPlace
// (сам вызов и выделение нового места — placesel.shape.test.ts).
describe('«Место ▾»: пункты', () => {
  it('порядок и подписи', () => {
    expect(PLACE_ADD_ITEMS.map((i) => [i.shape, i.label])).toEqual([
      ['rect', 'Прямоугольное'],
      ['round', 'Скруглённое'],
      ['oval', 'Круглое'],
    ])
  })
  it('у каждого свой значок', () => {
    expect(PLACE_ADD_ITEMS.map((i) => i.icon)).toEqual(['place-add', 'place-rounded', 'place-round'])
  })
  it('в Go уходит "" / "rounded" / "oval"', () => {
    expect(PLACE_ADD_ITEMS.map((i) => shapeArg(i.shape))).toEqual(['', 'rounded', 'oval'])
  })
})
