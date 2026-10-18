import { afterEach, describe, expect, it } from 'vitest'
import { additive, isLeft, isMac, primaryPress, secondaryPress, setMacForTest, toggleKey } from './platform'

const press = (button: number, ctrlKey = false) => ({ button, ctrlKey })
const mods = (o: Partial<{ shiftKey: boolean; ctrlKey: boolean; metaKey: boolean }> = {}) => ({
  shiftKey: false,
  ctrlKey: false,
  metaKey: false,
  ...o,
})

afterEach(() => setMacForTest(null))

describe('isMac', () => {
  it('подмена для тестов и возврат к userAgent (в node его нет — не Mac)', () => {
    setMacForTest(true)
    expect(isMac()).toBe(true)
    setMacForTest(false)
    expect(isMac()).toBe(false)
    setMacForTest(null)
    expect(isMac()).toBe(false)
  })
})

describe('primaryPress / secondaryPress', () => {
  it('Windows: левая — жест, правая — меню, Ctrl ничего не меняет', () => {
    setMacForTest(false)
    expect(primaryPress(press(0))).toBe(true)
    expect(primaryPress(press(0, true))).toBe(true)
    expect(primaryPress(press(2))).toBe(false)
    expect(primaryPress(press(1))).toBe(false)
    expect(secondaryPress(press(2))).toBe(true)
    expect(secondaryPress(press(0, true))).toBe(false)
    expect(secondaryPress(press(0))).toBe(false)
  })

  it('Mac: Ctrl+левая — правый щелчок, жеста нет', () => {
    setMacForTest(true)
    expect(primaryPress(press(0))).toBe(true)
    expect(primaryPress(press(0, true))).toBe(false)
    expect(secondaryPress(press(0, true))).toBe(true)
    expect(secondaryPress(press(0))).toBe(false)
  })

  it('Mac: настоящая правая кнопка (в том числе с Ctrl) — меню, не жест; средняя — ни то ни другое', () => {
    setMacForTest(true)
    expect(secondaryPress(press(2))).toBe(true)
    expect(secondaryPress(press(2, true))).toBe(true)
    expect(primaryPress(press(2))).toBe(false)
    expect(primaryPress(press(1, true))).toBe(false)
    expect(secondaryPress(press(1, true))).toBe(false)
  })

  it('isLeft — номер кнопки без события', () => {
    expect(isLeft(0)).toBe(true)
    expect(isLeft(2)).toBe(false)
  })
})

describe('additive / toggleKey', () => {
  it('Windows: Shift, Ctrl и Cmd добавляют к выделению', () => {
    setMacForTest(false)
    expect(additive(mods({ shiftKey: true }))).toBe(true)
    expect(additive(mods({ ctrlKey: true }))).toBe(true)
    expect(additive(mods({ metaKey: true }))).toBe(true)
    expect(additive(mods())).toBe(false)
    expect(toggleKey(mods({ ctrlKey: true }))).toBe(true)
  })

  it('Mac: Shift и Cmd добавляют, Ctrl — нет', () => {
    setMacForTest(true)
    expect(additive(mods({ shiftKey: true }))).toBe(true)
    expect(additive(mods({ metaKey: true }))).toBe(true)
    expect(additive(mods({ ctrlKey: true }))).toBe(false)
    expect(additive(mods())).toBe(false)
    expect(toggleKey(mods({ ctrlKey: true }))).toBe(false)
    expect(toggleKey(mods({ metaKey: true }))).toBe(true)
  })
})
