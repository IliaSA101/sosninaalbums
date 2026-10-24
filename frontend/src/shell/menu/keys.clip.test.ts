import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { isTextField, keyCommand, preventsDefault, type KeyContext, type KeyEvent } from './keys'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { setMacForTest } from '../../base/core/platform'

// Клавиши буфера на холсте (099): Ctrl+C, X, V, D. Остальное — keys.test.ts.
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
const canvasZone = { zone: 'canvas' as const }

describe('копировать и вставить (099)', () => {
  const sel = { ...canvasZone, hasSelection: true, copyable: true, pasteable: true }
  const KEYS = [
    ['c', 'copyItems'],
    ['x', 'cutItems'],
    ['v', 'pasteItems'],
    ['d', 'duplicateItems'],
  ] as const
  it('Ctrl+C, X, V, D на холсте; по-русски (С, Ч, М, В на тех же клавишах) — то же', () => {
    for (const [k, id] of KEYS) expect(cmd(ev(k, ctrl), ctx(sel)), k).toBe(id)
    for (const [k, id] of [
      ['с', 'copyItems'],
      ['ч', 'cutItems'],
      ['м', 'pasteItems'],
      ['в', 'duplicateItems'],
    ] as const)
      expect(cmd(ev(k, ctrl), ctx(sel)), k).toBe(id)
  })
  it('Cmd на Mac', () => {
    setMacForTest(true)
    for (const [k, id] of KEYS) expect(cmd(ev(k, { metaKey: true }), ctx(sel)), k).toBe(id)
    expect(cmd(ev('c', ctrl), ctx(sel))).toBeNull()
  })
  it('браузер гасится: Ctrl+C/X/V/D (закладка Ctrl+D) не доходят до страницы', () => {
    for (const [k, id] of KEYS) expect(preventsDefault(id, ev(k, ctrl)), k).toBe(true)
  })
  it('в поле ввода и при вводе надписи (contenteditable) — родное поведение: команда не находится', () => {
    for (const [k] of KEYS) expect(cmd(ev(k, ctrl), ctx({ ...sel, inTextField: true })), k).toBeNull()
    // App ставит inTextField по isTextField(цель): поле, textarea и contenteditable (редактор надписи) — одно и то же
    expect(isTextField({ closest: () => ({}) })).toBe(true)
  })
  it('нет выделения: копировать, вырезать, дублировать нечего, а вставка идёт, пока буфер не пуст', () => {
    const none = { ...canvasZone, pasteable: true }
    for (const k of ['c', 'x', 'd']) expect(cmd(ev(k, ctrl), ctx(none)), k).toBeNull()
    expect(cmd(ev('v', ctrl), ctx(none))).toBe('pasteItems')
    expect(cmd(ev('v', ctrl), ctx({ ...canvasZone, copyable: true }))).toBeNull() // буфер пуст
  })
  it('молчат: режим фона, крупный просмотр, галерея раскладок, окно, выделен текст страницы', () => {
    for (const o of [{ bgMode: true }, { big: true }, { galleryOpen: true }, { dialog: 'shut' as const }, { nativeSel: true }])
      for (const [k] of KEYS) expect(cmd(ev(k, ctrl), ctx({ ...sel, ...o })), `${k} ${JSON.stringify(o)}`).toBeNull()
  })
  it('лента фото: Ctrl+C ничего не делает (не наша область)', () => {
    for (const [k] of KEYS) expect(cmd(ev(k, ctrl), ctx({ ...sel, zone: 'library' })), k).toBeNull()
    expect(cmd(ev('c', ctrl), ctx({ ...sel, zone: undefined }))).toBeNull()
  })
  it('удержание клавиши не повторяет вставку', () => {
    for (const [k] of KEYS) expect(cmd(ev(k, { ...ctrl, repeat: true }), ctx(sel)), k).toBeNull()
  })
  it('Ctrl+Shift+C и Ctrl+Alt+V — другие сочетания, команда молчит', () => {
    expect(cmd(ev('C', { ctrlKey: true, shiftKey: true }), ctx(sel))).toBeNull()
    expect(cmd(ev('v', { ctrlKey: true, altKey: true }), ctx(sel))).toBeNull()
  })
})
