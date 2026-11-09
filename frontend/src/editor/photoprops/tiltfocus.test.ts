import { describe, expect, it } from 'vitest'
import { fieldKey, leftRow } from './tiltfocus'

// Строка из двух элементов: ползунок и поле; всё остальное — снаружи.
const slider = {} as Node
const num = {} as Node
const outside = {} as Node
const row = { contains: (n: Node | null) => n === slider || n === num }

describe('leftRow: когда кончается сессия наклона', () => {
  it('ползунок → поле той же строки: сессия продолжается', () => {
    expect(leftRow(row, num as unknown as EventTarget)).toBe(false)
  })
  it('поле → ползунок той же строки: сессия продолжается', () => {
    expect(leftRow(row, slider as unknown as EventTarget)).toBe(false)
  })
  it('фокус ушёл на другой элемент: сессия кончилась', () => {
    expect(leftRow(row, outside as unknown as EventTarget)).toBe(true)
  })
  it('фокуса нет (blur после отпускания, окно потеряло фокус): сессия кончилась', () => {
    expect(leftRow(row, null)).toBe(true)
  })
})

describe('fieldKey: клавиши числового поля строки', () => {
  it('Enter — применить (blur пошлёт change), Esc — откат', () => {
    expect(fieldKey('Enter')).toBe('apply')
    expect(fieldKey('Escape')).toBe('cancel')
  })
  it('остальные клавиши, в том числе стрелки и минус, поле не трогают', () => {
    for (const k of ['ArrowUp', 'ArrowDown', '-', '5', 'Tab', 'Backspace']) expect(fieldKey(k)).toBeNull()
  })
})
