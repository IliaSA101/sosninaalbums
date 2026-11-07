import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { keyCommand, type KeyContext, type KeyEvent } from './keys'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { setMacForTest } from '../../base/core/platform'

// Крупный просмотр фото (128): клавиша Z и клавиши под окном.
beforeEach(() => useCatalog())
afterEach(() => {
  dropCatalog()
  setMacForTest(null)
})

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
const ctrl = { ctrlKey: true }

describe('Z — крупный просмотр фото (0.26-128-2)', () => {
  const Z = { code: 'KeyZ' }
  it('в ленте и на холсте; по-русски (Я на той же клавише) — то же', () => {
    for (const zone of ['library', 'canvas'] as const) {
      expect(cmd(ev('z', Z), ctx({ zone }))).toBe('photoPreview')
      expect(cmd(ev('я', Z), ctx({ zone }))).toBe('photoPreview')
    }
    expect(cmd(ev('z', Z))).toBe('photoPreview') // зоны ещё нет: лента
  })
  it('лента: в «Во весь экран» и при галерее молчит; холст: молчит при галерее и в режиме фона, а в «Во весь экран» работает', () => {
    expect(cmd(ev('z', Z), ctx({ zone: 'library', big: true }))).toBeNull()
    expect(cmd(ev('z', Z), ctx({ zone: 'library', galleryOpen: true }))).toBeNull()
    expect(cmd(ev('z', Z), ctx({ zone: 'canvas', galleryOpen: true }))).toBeNull()
    expect(cmd(ev('z', Z), ctx({ zone: 'canvas', bgMode: true }))).toBeNull()
    expect(cmd(ev('z', Z), ctx({ zone: 'canvas', big: true }))).toBe('photoPreview')
  })
  it('поле ввода, окно и Ctrl+Z: молчит; Ctrl+Z — по-прежнему «Отменить»', () => {
    expect(cmd(ev('z', Z), ctx({ inTextField: true }))).toBeNull()
    expect(cmd(ev('z', Z), ctx({ dialog: 'shut' }))).toBeNull()
    expect(cmd(ev('z', { ...Z, ctrlKey: true }))).toBe('undo')
  })
  it('назначается в «Горячих клавишах»: снятая клавиша молчит, новая работает', () => {
    useCatalog({ photoPreview: [] })
    expect(cmd(ev('z', Z))).toBeNull()
    useCatalog({ photoPreview: ['KeyJ'] })
    expect(cmd(ev('z', Z))).toBeNull()
    expect(cmd(ev('j', { code: 'KeyJ' }))).toBe('photoPreview')
  })
})

describe('под окном крупного просмотра (0.26-128-6)', () => {
  const win = (o: Partial<KeyContext> = {}) => ctx({ zone: 'library', hasSelection: true, dialog: 'undo', preview: 'library', ...o })
  const P = { code: 'KeyP' }
  it('из ленты проходят отметки, кроме «Звёзды: убрать»; зона не важна', () => {
    expect(cmd(ev('p', P), win())).toBe('markPick')
    expect(cmd(ev('3', { code: 'Digit3' }), win())).toBe('markRating3')
    expect(cmd(ev('p', P), win({ zone: 'canvas' }))).toBe('markPick')
    expect(cmd(ev('0', { code: 'Digit0' }), win())).toBeNull()
  })
  it('из ленты проходят «Отменить» и «Повторить»', () => {
    expect(cmd(ev('z', { ...ctrl, code: 'KeyZ' }), win())).toBe('undo')
    expect(cmd(ev('y', { ...ctrl, code: 'KeyY' }), win())).toBe('redo')
  })
  it('с холста отметки молчат, отмена проходит', () => {
    expect(cmd(ev('p', P), win({ preview: 'canvas' }))).toBeNull()
    expect(cmd(ev('z', { ...ctrl, code: 'KeyZ' }), win({ preview: 'canvas' }))).toBe('undo')
  })
  it('подборки, буфер, F, I, Пробел, стрелки и сама Z молчат', () => {
    const silent: [string, Partial<KeyEvent>][] = [
      ['b', { code: 'KeyB' }],
      ['b', { code: 'KeyB', shiftKey: true }],
      ['Delete', { code: 'Delete' }],
      ['v', { code: 'KeyV', ctrlKey: true }],
      ['f', { code: 'KeyF' }],
      ['i', { code: 'KeyI' }],
      [' ', { code: 'Space' }],
      ['ArrowRight', { code: 'ArrowRight' }],
      ['z', { code: 'KeyZ' }],
    ]
    for (const [k, o] of silent) expect(cmd(ev(k, o), win({ pasteable: true, copyable: true })), `${k} ${JSON.stringify(o)}`).toBeNull()
  })
  it('«Во весь экран» и галерея отметки по-прежнему гасят', () => {
    expect(cmd(ev('p', P), win({ big: true }))).toBeNull()
    expect(cmd(ev('p', P), win({ galleryOpen: true }))).toBeNull()
  })
})
