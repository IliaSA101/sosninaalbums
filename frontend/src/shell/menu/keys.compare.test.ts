import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { keyCommand, type KeyContext, type KeyEvent } from './keys'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'

// Сравнение 2–4 кадров (138): клавиши C и Shift+C, отметки под окном сравнения.
beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

const ev = (key: string, o: Partial<KeyEvent> = {}): KeyEvent => ({
  key,
  code: '',
  ctrlKey: false,
  metaKey: false,
  altKey: false,
  shiftKey: false,
  repeat: false,
  ...o,
})
const ctx = (o: Partial<KeyContext> = {}): KeyContext => ({
  inTextField: false,
  onCheckbox: false,
  galleryOpen: false,
  hasSelection: false,
  ...o,
})
const cmd = (e: KeyEvent, c: KeyContext = ctx()) => keyCommand(e, c)
const C = { code: 'KeyC' }
const shiftC = { code: 'KeyC', shiftKey: true }

describe('C и Shift+C', () => {
  it('в ленте — сравнить и похожие; по-русски (С на той же клавише) — то же', () => {
    expect(cmd(ev('c', C))).toBe('photoCompare')
    expect(cmd(ev('с', C))).toBe('photoCompare')
    expect(cmd(ev('C', shiftC))).toBe('photoSimilar')
    expect(cmd(ev('c', C), ctx({ zone: 'library' }))).toBe('photoCompare')
  })
  it('на холсте, в «Во весь экран», при галерее, в поле ввода и под окном молчат', () => {
    for (const c of [
      ctx({ zone: 'canvas' }),
      ctx({ big: true }),
      ctx({ galleryOpen: true }),
      ctx({ inTextField: true }),
      ctx({ dialog: 'shut' }),
    ]) {
      expect(cmd(ev('c', C), c)).toBeNull()
      expect(cmd(ev('C', shiftC), c)).toBeNull()
    }
  })
  it('под окном сравнения они молчат (C закрывает окно само)', () => {
    expect(cmd(ev('c', C), ctx({ compare: true, dialog: 'undo' }))).toBeNull()
    expect(cmd(ev('c', C), ctx({ compare: true }))).toBeNull()
  })
  it('назначается в «Горячих клавишах»: снятая молчит, новая работает', () => {
    useCatalog({ photoCompare: [] })
    expect(cmd(ev('c', C))).toBeNull()
    useCatalog({ photoCompare: ['KeyJ'] })
    expect(cmd(ev('j', { code: 'KeyJ' }))).toBe('photoCompare')
    expect(cmd(ev('c', C))).toBeNull()
  })
})

describe('C и Shift+C под окном 128', () => {
  it('из ленты проходят (окно передаёт кадры сравнению); с холста и под другими окнами молчат', () => {
    const under = ctx({ preview: 'library', dialog: 'undo', zone: 'canvas' })
    expect(cmd(ev('c', C), under)).toBe('photoCompare')
    expect(cmd(ev('C', shiftC), under)).toBe('photoSimilar')
    expect(cmd(ev('c', C), ctx({ preview: 'canvas', dialog: 'undo', zone: 'canvas' }))).toBeNull()
    expect(cmd(ev('c', C), ctx({ preview: 'library', dialog: 'shut' }))).toBeNull()
    expect(cmd(ev('c', C), ctx({ preview: 'library', dialog: 'undo', big: true }))).toBeNull()
  })
})

describe('отметки под окном сравнения', () => {
  const under = ctx({ compare: true, dialog: 'undo', zone: 'canvas' })
  it('проходят, даже когда последнее нажатие мыши было на холсте', () => {
    expect(cmd(ev('p', { code: 'KeyP' }), under)).toBe('markPick')
    expect(cmd(ev('4', { code: 'Digit4' }), under)).toBe('markRating4')
    expect(cmd(ev('x', { code: 'KeyX' }), under)).toBe('markReject')
  })
  it('0 остаётся окну, отмена проходит, остальное молчит', () => {
    expect(cmd(ev('0', { code: 'Digit0' }), under)).toBeNull()
    expect(cmd(ev('z', { code: 'KeyZ', ctrlKey: true }), under)).toBe('undo')
    expect(cmd(ev('b', { code: 'KeyB' }), under)).toBeNull()
    expect(cmd(ev('Delete', { code: 'Delete' }), under)).toBeNull()
  })
  it('без окна сравнения отметки под другим окном молчат, как раньше', () => {
    expect(cmd(ev('p', { code: 'KeyP' }), ctx({ dialog: 'undo', zone: 'canvas' }))).toBeNull()
  })
})
