import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  additive,
  foldPath,
  isLeft,
  isMac,
  os,
  primaryPress,
  revealLabel,
  secondaryPress,
  setMacForTest,
  setOsForTest,
  toggleKey,
} from './platform'

const press = (button: number, ctrlKey = false) => ({ button, ctrlKey })
const mods = (o: Partial<{ shiftKey: boolean; ctrlKey: boolean; metaKey: boolean }> = {}) => ({
  shiftKey: false,
  ctrlKey: false,
  metaKey: false,
  ...o,
})

afterEach(() => {
  setMacForTest(null)
  vi.unstubAllGlobals()
})

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

describe('os / revealLabel (101)', () => {
  const ua = (userAgent: string) => vi.stubGlobal('navigator', { userAgent })
  it('по userAgent: Mac, Linux (не Android), остальное и пустое — Windows', () => {
    ua('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15')
    expect(os()).toBe('mac')
    ua('Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15')
    expect(os()).toBe('linux')
    ua('Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36')
    expect(os()).toBe('windows')
    ua('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36')
    expect(os()).toBe('windows')
    vi.stubGlobal('navigator', undefined)
    expect(os()).toBe('windows')
  })
  it('Linux не Mac: Ctrl там — Ctrl', () => {
    setOsForTest('linux')
    expect(isMac()).toBe(false)
    expect(secondaryPress(press(0, true))).toBe(false)
    expect(toggleKey(mods({ ctrlKey: true }))).toBe(true)
  })
  it('подпись «Показать…» по ОС', () => {
    setOsForTest('windows')
    expect(revealLabel()).toBe('Показать в Проводнике')
    setOsForTest('mac')
    expect(revealLabel()).toBe('Показать в Finder')
    setOsForTest('linux')
    expect(revealLabel()).toBe('Показать в папке')
  })
  it('подмена снимается: null — снова по userAgent', () => {
    setOsForTest('linux')
    setOsForTest(null)
    vi.stubGlobal('navigator', { userAgent: 'Macintosh' })
    expect(os()).toBe('mac')
  })
})

describe('foldPath', () => {
  it('Linux — точно, Windows и macOS — нижний регистр', () => {
    setOsForTest('linux')
    expect(foldPath('/x/Весна')).toBe('/x/Весна')
    setOsForTest('windows')
    expect(foldPath('D:/Весна')).toBe('d:/весна')
    setOsForTest('mac')
    expect(foldPath('/X/Весна')).toBe('/x/весна')
    setOsForTest(null)
  })
})
