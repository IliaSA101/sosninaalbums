import { describe, expect, it } from 'vitest'
import { outlineD, polyD, validGuide, veilD, type FaceGuideData } from './faceguide'

const g = (o: Partial<FaceGuideData> = {}): FaceGuideData => ({
  on: true,
  w: 40,
  h: 60,
  eyeY: 19.2,
  path: [
    [20, 10],
    [30, 20],
    [50, 60], // плечо за правым краем окна
    [-10, 60], // и за левым: звено по низу окна
    [10, 20],
  ],
  ...o,
})

describe('validGuide', () => {
  it('нормальный ответ годен', () => expect(validGuide(g())).toBe(true))
  it('on=false, пусто и null — нет', () => {
    expect(validGuide(g({ on: false }))).toBe(false)
    expect(validGuide(null)).toBe(false)
    expect(validGuide(undefined)).toBe(false)
    expect(validGuide(g({ path: [] }))).toBe(false)
  })
  it('окно не положительное или не число — нет', () => {
    expect(validGuide(g({ w: 0 }))).toBe(false)
    expect(validGuide(g({ h: -1 }))).toBe(false)
    expect(validGuide(g({ w: NaN }))).toBe(false)
    expect(validGuide(g({ eyeY: Infinity }))).toBe(false)
  })
  it('мусор в точках — нет', () => {
    expect(validGuide(g({ path: [[1, 2], [3], [5, 6]] }))).toBe(false)
    expect(
      validGuide(
        g({
          path: [
            [1, 2],
            [NaN, 4],
            [5, 6],
          ],
        }),
      ),
    ).toBe(false)
    expect(
      validGuide(
        g({
          path: [
            [1, 2],
            [3, 4],
          ],
        }),
      ),
    ).toBe(false) // двух точек на фигуру мало
  })
})

describe('пути', () => {
  it('polyD замыкает контур и округляет до сотых мм', () => {
    expect(
      polyD([
        [1.234, 2],
        [3, 4.5],
        [5, 6],
      ]),
    ).toBe('M1.23 2L3 4.5L5 6Z')
  })
  it('veilD — окно, затем контур (evenodd вычитает силуэт)', () => {
    expect(
      veilD(
        g({
          path: [
            [1, 1],
            [2, 1],
            [2, 2],
          ],
        }),
      ),
    ).toBe('M0 0H40V60H0ZM1 1L2 1L2 2Z')
  })
  it('outlineD не рисует звено по низу окна, но замыкает остальное', () => {
    // звено (50,60)→(-10,60) лежит на низу окна; остальные четыре, включая замыкающее (10,20)→(20,10), — одной линией
    expect(outlineD(g())).toBe('M-10 60L10 20L20 10L30 20L50 60')
  })
  it('силуэт целиком внутри окна — замкнутая линия без пропусков', () => {
    const d = outlineD(
      g({
        path: [
          [5, 5],
          [10, 5],
          [10, 10],
        ],
      }),
    )
    expect(d).toBe('M5 5L10 5L10 10L5 5Z')
  })
  it('звенья ниже окна тоже не рисуются', () => {
    expect(
      outlineD(
        g({
          path: [
            [5, 5],
            [10, 70],
            [20, 70],
          ],
        }),
      ),
    ).toBe('M20 70L5 5L10 70')
  })
})
