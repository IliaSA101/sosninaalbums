// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Редактор надписи встаёт на строки Go (156-4): зонд базы вне правки, отступы, translateY, перемер по загрузке шрифтов.
// measureBase подменён частичным мок-модулем: в happy-dom раскладки нет, а editorShift остаётся настоящим.
const measure = vi.hoisted(() => vi.fn((): number | null => 40))
vi.mock('./baseline', async (orig) => ({ ...(await orig<typeof import('./baseline')>()), measureBase: measure }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: TextEditor } = await import('./TextEditor.svelte')
const { edit } = await import('./edit.svelte')

const K = 1.52
const SIZE_MM = (80 * 25.4) / 72
const style = { font: 'badscript', size: 80, color: '#000000', align: 'left' as const }
// Bad Script 80 pt: asc 1,324 кегля, desc 0,648; y блока = 10 мм, три строки.
const lines = (n: number, asc = 1.324) => Array.from({ length: n }, (_, i) => ({ y: 10 + asc * SIZE_MM + i * 1.2 * SIZE_MM }))
const box = (n: number, rot = 0) => ({ x: 20, y: 10, w: 100, h: (1.324 + 0.648) * SIZE_MM + (n - 1) * 1.2 * SIZE_MM, rot })

const props = (o: Record<string, unknown> = {}) => ({
  box: box(3),
  auto: false,
  maxW: 200,
  text: 'а\nб\nв',
  style,
  lines: lines(3),
  sw: 300,
  ph: 200,
  k: K,
  ...o,
})
const mount = (o: Record<string, unknown> = {}) => render(TextEditor, props(o))
const editor = (c: HTMLElement) => c.querySelector<HTMLElement>('.editor')!
const flush = async () => {
  await Promise.resolve()
  flushSync()
}
// Подставной document.fonts: EventTarget с готовым ready.
function fakeFonts() {
  const fake = Object.assign(new EventTarget(), { ready: Promise.resolve() })
  Object.defineProperty(document, 'fonts', { value: fake, configurable: true })
  return fake
}

beforeEach(() => {
  measure.mockReset()
  measure.mockReturnValue(40)
  edit.clear()
})
afterEach(() => {
  cleanup()
  // Убираем подставной document.fonts
  delete (document as unknown as Record<string, unknown>).fonts
})

describe('texteditor: поправка базы', () => {
  it('зонд вне правки и aria-hidden', () => {
    const { container } = mount()
    const probe = container.querySelector('.probe')!
    expect(probe).not.toBeNull()
    expect(probe.getAttribute('aria-hidden')).toBe('true')
    expect(editor(container).contains(probe)).toBe(false)
    expect(editor(container).querySelector('.pb')).toBeNull()
    expect(probe.querySelector('.pb')).not.toBeNull()
  })

  it('отступы и min-height из editorshift', async () => {
    const { container } = mount()
    await flush()
    const st = editor(container).style
    const sizePx = SIZE_MM * K
    const top = 1.324 * SIZE_MM * K - 40
    const bottom = 0.648 * SIZE_MM * K - (1.2 * sizePx - 40)
    expect(parseFloat(st.paddingTop)).toBeCloseTo(top, 6)
    expect(parseFloat(st.paddingBottom)).toBeCloseTo(bottom, 6)
    expect(parseFloat(st.minHeight)).toBeCloseTo(1.2 * sizePx + top + bottom, 6)
  })

  it('translatey после rotate при отрицательном сдвиге', async () => {
    // Old Standard: asc + desc = 1,0 кегля — база Go выше базы редактора, сдвиг уходит в translateY.
    const os = { ...style, font: 'oldstandard' }
    const b = { x: 20, y: 10, w: 100, h: 1.0 * SIZE_MM, rot: 15 }
    const { container } = mount({ box: b, style: os, lines: lines(1, 0.762) })
    await flush()
    const t = editor(container).style.transform
    expect(t).toMatch(/^rotate\(15deg\) translateY\(-[\d.]+px\)$/)
    expect(editor(container).style.paddingTop).toBe('0px')
  })

  it('ввод без знаков зонда', async () => {
    const input = vi.spyOn(edit, 'input').mockImplementation(() => {})
    const { container } = mount({ text: '' })
    const el = editor(container)
    // append, а не textContent: присвоение стёрло бы всех детей редактора, а с ними и зонд, если он внутри.
    el.append('a')
    await fireEvent.input(el)
    // Зонд внутри редактора добавил бы к вводу свой «x» (innerText = «ax»).
    expect(input).toHaveBeenCalledWith('a')
    expect(input.mock.calls.every(([v]) => !String(v).includes('x'))).toBe(true)
    input.mockRestore()
  })

  it('без lines и без замера — прежний вид, без ошибки', async () => {
    measure.mockReturnValue(null)
    const { container } = mount({ lines: [] })
    await flush()
    const st = editor(container).style
    expect(st.paddingTop).toBe('')
    expect(st.minHeight).toBe('')
    expect(st.transform).toBe('')
  })
})

describe('texteditor: перемер по загрузке шрифтов', () => {
  it('без document.fonts — один замер, без ошибки', async () => {
    // В happy-dom document.fonts нет: вызов без ?. уронил бы onMount.
    expect((document as unknown as { fonts?: unknown }).fonts).toBeUndefined()
    mount()
    await flush()
    expect(measure).toHaveBeenCalledTimes(1)
  })

  it('замер, затем перемер после fonts.ready', async () => {
    fakeFonts()
    mount()
    await flush()
    await flush()
    expect(measure).toHaveBeenCalledTimes(2)
  })

  it('loadingdone во время правки — перемер, после закрытия слушатель снят', async () => {
    const fake = fakeFonts()
    const rm = vi.spyOn(fake, 'removeEventListener')
    const { unmount } = mount()
    await flush()
    await flush()
    const n = measure.mock.calls.length
    fake.dispatchEvent(new Event('loadingdone'))
    expect(measure).toHaveBeenCalledTimes(n + 1)
    unmount()
    expect(rm).toHaveBeenCalledWith('loadingdone', expect.any(Function))
    fake.dispatchEvent(new Event('loadingdone'))
    expect(measure).toHaveBeenCalledTimes(n + 1)
  })

  it('смена кегля — перемер, смена цвета — нет', async () => {
    const { rerender } = mount()
    await flush()
    const n = measure.mock.calls.length
    await rerender(props({ style: { ...style, color: '#ff0000' } }))
    await flush()
    expect(measure).toHaveBeenCalledTimes(n)
    await rerender(props({ style: { ...style, size: 40 } }))
    await flush()
    expect(measure).toHaveBeenCalledTimes(n + 1)
  })
})
