import { describe, expect, it } from 'vitest'
import { SHAPE_BTNS, SHAPE_ROW, pressedOf } from './shapetoggle'

// Строка «Форма» (076): подпись, три значка с подсказками и состояние aria-pressed по placesel.allRect/allRounded/allOval.
describe('строка «Форма»', () => {
  it('подпись и три значка ▭ ▢ ◯ с подсказками', () => {
    expect(SHAPE_ROW).toBe('Форма')
    expect(SHAPE_BTNS.map((b) => [b.key, b.glyph, b.title])).toEqual([
      ['rect', '▭', 'Прямоугольник'],
      ['round', '▢', 'Скруглённые углы'],
      ['oval', '◯', 'Круг; при разных Ш и В — овал'],
    ])
  })
  it('нажат ровно тот значок, чей флаг поднят; смесь — ни один', () => {
    const f = (allRect: boolean, allRounded: boolean, allOval: boolean) =>
      SHAPE_BTNS.map((b) => pressedOf(b.key, { allRect, allRounded, allOval }))
    expect(f(true, false, false)).toEqual([true, false, false])
    expect(f(false, true, false)).toEqual([false, true, false])
    expect(f(false, false, true)).toEqual([false, false, true])
    expect(f(false, false, false)).toEqual([false, false, false])
  })
})
