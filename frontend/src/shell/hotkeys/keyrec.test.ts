import { afterEach, describe, expect, it } from 'vitest'
import { setMacForTest } from '../../base/core/platform'
import { partialText, pressStep, recStep, type RecEvent } from './keyrec'

afterEach(() => setMacForTest(null))

const ev = (key: string, code: string, o: Partial<RecEvent> = {}): RecEvent => ({
  key,
  code,
  ctrlKey: false,
  shiftKey: false,
  altKey: false,
  metaKey: false,
  repeat: false,
  isComposing: false,
  ...o,
})

describe('запись сочетания: очередное нажатие', () => {
  it('буква и сочетание с модификатором → сочетание; русская раскладка — по клавише', () => {
    expect(recStep(ev('b', 'KeyB'), false)).toEqual({ kind: 'chord', chord: 'KeyB' })
    expect(recStep(ev('я', 'KeyZ', { ctrlKey: true }), false)).toEqual({ kind: 'chord', chord: 'Mod+KeyZ' })
    expect(recStep(ev('1', 'Numpad1'), false)).toEqual({ kind: 'chord', chord: 'Digit1' })
  })
  it('Esc без модификаторов — отмена записи; Esc с Ctrl — сочетание (Go откажет с причиной)', () => {
    expect(recStep(ev('Escape', 'Escape'), false)).toEqual({ kind: 'cancel' })
    expect(recStep(ev('Escape', 'Escape', { ctrlKey: true }), false)).toEqual({ kind: 'chord', chord: 'Mod+Escape' })
  })
  it('одни модификаторы — запись ждёт клавишу и показывает «Ctrl+Shift+…»', () => {
    expect(recStep(ev('Control', 'ControlLeft', { ctrlKey: true }), false)).toEqual({ kind: 'partial', text: 'Ctrl+…' })
    expect(recStep(ev('Shift', 'ShiftLeft', { ctrlKey: true, shiftKey: true }), false)).toEqual({ kind: 'partial', text: 'Ctrl+Shift+…' })
    expect(recStep(ev('Alt', 'AltLeft', { altKey: true }), false)).toEqual({ kind: 'partial', text: 'Alt+…' })
  })
  it('автоповтор и ввод IME пропускаются молча', () => {
    expect(recStep(ev('b', 'KeyB', { repeat: true }), false)).toEqual({ kind: 'skip' })
    expect(recStep(ev('б', 'KeyB', { isComposing: true }), false)).toEqual({ kind: 'skip' })
    expect(recStep(ev('Process', 'KeyB'), false)).toEqual({ kind: 'skip' })
  })
  it('Ctrl+Alt доходит до Go как Mod+Alt: отказ с причиной (AltGr) даёт Go', () => {
    expect(recStep(ev('e', 'KeyE', { ctrlKey: true, altKey: true }), false)).toEqual({ kind: 'chord', chord: 'Mod+Alt+KeyE' })
  })
  it('клавиша вне белого списка — «нельзя»', () => {
    expect(recStep(ev('AudioVolumeUp', 'AudioVolumeUp'), false)).toEqual({ kind: 'bad' })
  })
  it('Mac: ⌘ — основной модификатор; Ctrl — не наш', () => {
    setMacForTest(true)
    expect(recStep(ev('d', 'KeyD', { metaKey: true }), true)).toEqual({ kind: 'chord', chord: 'Mod+KeyD' })
    expect(recStep(ev('d', 'KeyD', { ctrlKey: true }), true)).toEqual({ kind: 'bad' })
    expect(partialText(ev('Shift', 'ShiftLeft', { metaKey: true, shiftKey: true, altKey: true }), true)).toBe('⌥⇧⌘…')
  })
  it('partialText без модификаторов — пусто', () => {
    expect(partialText(ev('Control', 'ControlLeft'), false)).toBe('')
  })
})

describe('нажатие вне записи', () => {
  it('Esc и Tab идут дальше (закроют окно, переведут фокус), F12 — инструменты разработчика', () => {
    expect(pressStep(ev('Escape', 'Escape'), false)).toEqual({ kind: 'pass' })
    expect(pressStep(ev('Tab', 'Tab'), false)).toEqual({ kind: 'pass' })
    expect(pressStep(ev('Tab', 'Tab', { shiftKey: true }), false)).toEqual({ kind: 'pass' })
    expect(pressStep(ev('F12', 'F12'), false)).toEqual({ kind: 'pass' })
  })
  it('Ctrl+Esc и Ctrl+Tab — служебные клавиши, выбираются на карте', () => {
    expect(pressStep(ev('Escape', 'Escape', { ctrlKey: true }), false)).toEqual({ kind: 'select', chord: 'Mod+Escape' })
    expect(pressStep(ev('Tab', 'Tab', { ctrlKey: true }), false)).toEqual({ kind: 'select', chord: 'Mod+Tab' })
  })
  it('обычная клавиша выбирается; Ctrl+Z — тоже клавиша карты, а не отмена', () => {
    expect(pressStep(ev('d', 'KeyD'), false)).toEqual({ kind: 'select', chord: 'KeyD' })
    expect(pressStep(ev('я', 'KeyZ', { ctrlKey: true }), false)).toEqual({ kind: 'select', chord: 'Mod+KeyZ' })
  })
  it('модификатор сам по себе меняет только слой', () => {
    expect(pressStep(ev('Control', 'ControlLeft', { ctrlKey: true }), false)).toEqual({ kind: 'mods' })
  })
})
