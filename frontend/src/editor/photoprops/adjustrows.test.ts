import { describe, expect, it } from 'vitest'
import { clampRow, needsGray, pendingGate, signed, toneArgs, toneOf, withRow } from './adjustrows'

describe('adjustrows: подпись числа', () => {
  it('со знаком, минус U+2212, ноль без знака', () => {
    expect(signed(12)).toBe('+12')
    expect(signed(-5)).toBe('−5')
    expect(signed(0)).toBe('0')
    expect(signed(50)).toBe('+50')
    expect(signed(-50)).toBe('−50')
  })
})

describe('adjustrows: тон из вида', () => {
  it('нет вида или полей — без тона', () => {
    expect(toneOf(undefined)).toEqual({ gray: false, bright: 0, contrast: 0 })
    expect(toneOf({})).toEqual({ gray: false, bright: 0, contrast: 0 })
  })
  it('берёт значения из FrameView', () => {
    expect(toneOf({ gray: true, bright: 10, contrast: -20 })).toEqual({ gray: true, bright: 10, contrast: -20 })
  })
})

describe('adjustrows: что уходит в SetTone', () => {
  const cur = { gray: true, bright: 10, contrast: -20 }
  it('меняют яркость — второе число из вида', () => {
    expect(toneArgs(cur, 'bright', 30)).toEqual([30, -20])
  })
  it('меняют контраст — яркость из вида', () => {
    expect(toneArgs(cur, 'contrast', 5)).toEqual([10, 5])
  })
  it('⊗: строка в 0, вторая не тронута', () => {
    expect(toneArgs(cur, 'bright', 0)).toEqual([0, -20])
    expect(toneArgs(cur, 'contrast', 0)).toEqual([10, 0])
  })
  it('то же значение — null: Go не зовём, шага отмены нет', () => {
    expect(toneArgs(cur, 'bright', 10)).toBeNull()
    expect(toneArgs({ gray: false, bright: 0, contrast: 0 }, 'contrast', 0)).toBeNull()
  })
  it('значение за пределами зажимается', () => {
    expect(toneArgs(cur, 'bright', 900)).toEqual([50, -20])
    expect(toneArgs(cur, 'contrast', -1e9)).toEqual([10, -50])
    expect(toneArgs({ gray: false, bright: 50, contrast: 0 }, 'bright', 70)).toBeNull() // 50 → 50
  })
  it('мусор вместо числа — 0', () => {
    expect(clampRow(NaN)).toBe(0)
    expect(withRow(cur, 'bright', Infinity).bright).toBe(0)
  })
})

describe('adjustrows: «Цвет | Ч/б»', () => {
  it('SetGray — переключатель, поэтому звать только при смене состояния', () => {
    const color = { gray: false, bright: 0, contrast: 0 }
    expect(needsGray(color, true)).toBe(true)
    expect(needsGray(color, false)).toBe(false) // повторный щелчок по «Цвет»
    expect(needsGray({ ...color, gray: true }, false)).toBe(true)
    expect(needsGray({ ...color, gray: true }, true)).toBe(false) // повторный щелчок по «Ч/б»
  })
})

describe('adjustrows: защита от повторного вызова', () => {
  it('двойной щелчок по «Ч/б» до ответа Go — Go зовётся один раз', async () => {
    const gate = pendingGate()
    let calls = 0
    let answer: () => void = () => {}
    const call = () => (calls++, new Promise<void>((r) => (answer = r)))
    const first = gate.run(call)
    expect(gate.busy).toBe(true)
    expect(await gate.run(call)).toBe(false) // второй щелчок
    expect(calls).toBe(1)
    answer()
    expect(await first).toBe(true)
    expect(gate.busy).toBe(false)
  })
  it('после ответа следующий вызов проходит; ошибка вызова замок снимает', async () => {
    const gate = pendingGate()
    await expect(gate.run(() => Promise.reject(new Error('x')))).rejects.toThrow('x')
    expect(gate.busy).toBe(false)
    expect(await gate.run(async () => {})).toBe(true)
  })
})
