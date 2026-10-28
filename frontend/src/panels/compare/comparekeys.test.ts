import { describe, expect, it } from 'vitest'
import { compareHint, ownKey, type KeyLike } from './compareset'

// Строка клавиш и свои клавиши окна сравнения (138): чистая логика.
const k = (key: string, o: Partial<KeyLike> = {}): KeyLike => ({
  key,
  ctrlKey: false,
  metaKey: false,
  altKey: false,
  shiftKey: false,
  ...o,
})

describe('compareHint', () => {
  it('с «Отметками» — лучший и отметки; mod и метки клавиш на месте', () => {
    const full = compareHint('Ctrl', true)
    expect(full).toContain('Ctrl+колесо — зум')
    expect(full).toContain('Enter — лучший · отметки — клавишами ленты')
    expect(full).toContain('{key:photoPreview| или %}')
    expect(full).toContain('{key:fitZoom| · % — вписать}')
    expect(full.endsWith('Esc — закрыть')).toBe(true)
  })
  it('без «Отметок» нет ни «лучший», ни «отметки»; Mac — ⌘', () => {
    const none = compareHint('⌘', false)
    expect(none).toContain('⌘+колесо')
    expect(none).not.toContain('лучший')
    expect(none).not.toContain('отметки')
  })
})

describe('ownKey', () => {
  it('Esc, стрелки, Tab и Enter — свои клавиши окна', () => {
    expect(ownKey(k('Escape'), false)).toBe('close')
    expect(ownKey(k('ArrowLeft'), false)).toBe('prev')
    expect(ownKey(k('ArrowRight'), false)).toBe('next')
    expect(ownKey(k('Tab'), false)).toBe('tab')
    expect(ownKey(k('Tab', { shiftKey: true }), false)).toBe('shiftTab')
    expect(ownKey(k('Enter'), false)).toBe('best')
    expect(ownKey(k('Delete'), false)).toBeNull() // Delete — команда реестра binRemove
  })
  it('с Ctrl/Alt/⌘ и Shift+стрелки — не свои; Enter на кнопке — щелчок кнопки; Esc всегда', () => {
    expect(ownKey(k('ArrowLeft', { shiftKey: true }), false)).toBeNull()
    expect(ownKey(k('ArrowLeft', { ctrlKey: true }), false)).toBeNull()
    expect(ownKey(k('Enter', { altKey: true }), false)).toBeNull()
    expect(ownKey(k('Enter'), true)).toBeNull()
    expect(ownKey(k('Escape', { ctrlKey: true }), true)).toBe('close')
    expect(ownKey(k('a'), false)).toBeNull()
  })
})
