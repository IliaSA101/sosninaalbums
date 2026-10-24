// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { resetStamps, stamp } from './clip'
import { domCommand, installClipDom, type ClipDomCtx } from './clipdom'

// Второй путь буфера (099-13): события copy/cut/paste документа.
const ctx = (o: Partial<ClipDomCtx> = {}): ClipDomCtx => ({
  inTextField: false,
  dialog: false,
  nativeSel: false,
  canvas: true,
  clipboardOn: true,
  off: false,
  copyable: true,
  pasteable: true,
  now: 10_000,
  ...o,
})

afterEach(() => resetStamps())

describe('domCommand', () => {
  it('на холсте: copy, cut, paste -> команды буфера', () => {
    expect(domCommand('copy', ctx())).toBe('copyItems')
    expect(domCommand('cut', ctx())).toBe('cutItems')
    expect(domCommand('paste', ctx())).toBe('pasteItems')
  })
  it.each([
    ['поле ввода', { inTextField: true }],
    ['окно', { dialog: true }],
    ['выделен текст вне холста', { nativeSel: true }],
    ['зона — не холст (лента фото)', { canvas: false }],
    ['функции нет на уровне', { clipboardOn: false }],
    ['режим фона, галерея или «Во весь экран»', { off: true }],
  ])('молчит: %s', (_n, o) => {
    for (const k of ['copy', 'cut', 'paste'] as const) expect(domCommand(k, ctx(o))).toBeNull()
  })
  it('копировать нечего / буфер пуст — событие браузеру', () => {
    expect(domCommand('copy', ctx({ copyable: false }))).toBeNull()
    expect(domCommand('cut', ctx({ copyable: false }))).toBeNull()
    expect(domCommand('paste', ctx({ copyable: false }))).toBe('pasteItems') // вставка от copyable не зависит
    expect(domCommand('paste', ctx({ pasteable: false }))).toBeNull()
  })
  it('сразу после клавиши (≤ 300 мс) — не второй раз; позже — можно; чужая команда не мешает', () => {
    stamp('pasteItems', 9_800)
    expect(domCommand('paste', ctx())).toBeNull()
    expect(domCommand('copy', ctx())).toBe('copyItems')
    expect(domCommand('paste', ctx({ now: 10_101 }))).toBe('pasteItems')
  })
})

describe('installClipDom', () => {
  it('событие -> preventDefault и run; отказ — событие не тронуто; снятие слушателей', () => {
    const run = vi.fn()
    let c = ctx()
    const off = installClipDom(document, () => c, run)
    const fire = (type: string) => {
      const e = new Event(type, { cancelable: true, bubbles: true })
      document.body.dispatchEvent(e)
      return e.defaultPrevented
    }
    expect(fire('paste')).toBe(true)
    expect(run).toHaveBeenCalledWith('pasteItems')
    c = ctx({ inTextField: true })
    expect(fire('copy')).toBe(false)
    expect(run).toHaveBeenCalledTimes(1)
    off()
    c = ctx()
    expect(fire('cut')).toBe(false)
    expect(run).toHaveBeenCalledTimes(1)
  })
})
