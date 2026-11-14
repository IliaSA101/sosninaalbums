// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { createRawSnippet } from 'svelte'

// Оболочка крупного просмотра (163 U1): запись в modalStack, Esc, фон и ПКМ, поверх неё вопрос (isTop) молчит,
// ← → и Enter в полях ввода — их собственные. Содержимое — сырые сниппеты, без компонента-обёртки.
const { default: PeekShell } = await import('./PeekShell.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')

const snip = (html: string) => createRawSnippet(() => ({ render: () => html }))
const view = createRawSnippet(() => ({ render: () => '<div class="v"></div>' }))
const strip = snip(
  '<div class="strip"><input class="in" /><select class="sel"><option>a</option></select><textarea class="ta"></textarea><div class="rd" role="radio" tabindex="0"></div></div>',
)
const caption = snip('<i>Подпись</i>')
const hint = snip('<i>Клавиши</i>')

function mount(over: Record<string, unknown> = {}) {
  const onclose = vi.fn()
  const onkey = vi.fn(() => null as (() => void) | null)
  const r = render(PeekShell, { tileId: 'a', onclose, onkey, view, strip, caption, hint, ...over })
  const q = <T extends Element>(s: string) => document.querySelector<T>(s)!
  return { ...r, onclose, onkey, q }
}
// Клавиша, что дошла до окна: вернуть, погасило ли её окно (preventDefault)
async function press(target: Element, key: string, init: KeyboardEventInit = {}) {
  const e = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init })
  target.dispatchEvent(e)
  await Promise.resolve()
  return e.defaultPrevented
}

afterEach(() => {
  cleanup()
  document.body.innerHTML = ''
})

describe('PeekShell: окно в стеке', () => {
  it('стоит в modalStack, пока смонтировано; фокус — в коробке', () => {
    const m = mount()
    expect(modalStack.count()).toBe(1)
    expect(document.activeElement).toBe(m.q('[role=dialog]'))
    m.unmount()
    expect(modalStack.count()).toBe(0)
  })
  it('без strip три ряда, со strip четыре', () => {
    const a = mount({ strip: undefined })
    expect(a.q<HTMLElement>('[role=dialog]').style.gridTemplateRows).toBe('1fr 36px auto')
    a.unmount()
    const b = mount()
    expect(b.q<HTMLElement>('[role=dialog]').style.gridTemplateRows).toBe('1fr auto 36px auto')
  })
})

describe('PeekShell: назад', () => {
  it('Esc закрывает один раз и возвращает фокус плитке', async () => {
    const tile = document.createElement('button')
    tile.dataset.decorId = 'a'
    document.body.append(tile)
    const m = mount()
    expect(await press(m.q('[role=dialog]'), 'Escape')).toBe(true)
    await vi.waitFor(() => expect(document.activeElement).toBe(tile))
    expect(m.onclose).toHaveBeenCalledTimes(1)
  })
  it('tileSel сужает поиск плитки', async () => {
    const other = document.createElement('button')
    other.dataset.decorId = 'a'
    const mine = document.createElement('button')
    mine.className = 'mine'
    mine.dataset.decorId = 'a'
    document.body.append(other, mine)
    const m = mount({ tileSel: '.mine' })
    await press(m.q('[role=dialog]'), 'Escape')
    await vi.waitFor(() => expect(document.activeElement).toBe(mine))
  })
  it('✕, правый щелчок и щелчок по фону закрывают; щелчок внутри коробки — нет', async () => {
    const m = mount()
    await fireEvent.pointerDown(m.q('[role=dialog]'))
    expect(m.onclose).not.toHaveBeenCalled()
    await fireEvent.pointerDown(m.q('.back'), { button: 0 })
    expect(m.onclose).toHaveBeenCalledTimes(1)
    await fireEvent.pointerDown(m.q('.back'), { button: 2 })
    expect(m.onclose).toHaveBeenCalledTimes(1) // правая кнопка по pointerdown не закрывает: закроет contextmenu
    const ctx = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 2 })
    m.q('.back').dispatchEvent(ctx)
    expect(ctx.defaultPrevented).toBe(true)
    expect(m.onclose).toHaveBeenCalledTimes(2)
    await fireEvent.click(m.q('.x'))
    expect(m.onclose).toHaveBeenCalledTimes(3)
  })
})

describe('PeekShell: своя клавиша и фокус', () => {
  it('клавиша без действия идёт дальше, с действием — гасится и выполняется', async () => {
    const run = vi.fn()
    const m = mount()
    expect(await press(m.q('[role=dialog]'), 'ArrowRight')).toBe(false)
    expect(m.onkey).toHaveBeenCalledTimes(1)
    m.onkey.mockReturnValueOnce(run)
    expect(await press(m.q('[role=dialog]'), 'ArrowRight')).toBe(true)
    expect(run).toHaveBeenCalledTimes(1)
  })
  it('фокус ушёл из окна: клавиша гасится, фокус возвращается, действие не зовётся', async () => {
    const out = document.createElement('button')
    document.body.append(out)
    const m = mount()
    out.focus()
    expect(await press(out, 'Enter')).toBe(true)
    expect(document.activeElement).toBe(m.q('[role=dialog]'))
    expect(m.onkey).not.toHaveBeenCalled()
  })
})

describe('PeekShell: поверх лежит вопрос (isTop)', () => {
  it('Esc, ← →, Enter молчат, пока над просмотром другое окно; фокус не отбирается', async () => {
    const m = mount()
    const ask = modalStack.push()
    const field = document.createElement('input')
    document.body.append(field)
    field.focus()
    for (const key of ['Escape', 'ArrowLeft', 'ArrowRight', 'Enter']) expect(await press(field, key)).toBe(false)
    expect(m.onkey).not.toHaveBeenCalled()
    expect(m.onclose).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(field)
    modalStack.remove(ask)
    m.q<HTMLElement>('[role=dialog]').focus() // фокус вернуло закрывшееся окно вопроса
    expect(await press(m.q('[role=dialog]'), 'Escape')).toBe(true)
    expect(m.onclose).toHaveBeenCalledTimes(1)
  })
})

describe('PeekShell: поля ввода', () => {
  it.each(['.in', '.sel', '.ta', '.rd'])('← → и Enter в %s окно не берёт', async (sel) => {
    const m = mount()
    const el = m.q(sel)
    for (const key of ['ArrowLeft', 'ArrowRight', 'Enter']) {
      expect(await press(el, key)).toBe(false)
      expect(await press(el, key, { shiftKey: true })).toBe(false) // Shift+← выделяет текст — тоже поля
    }
    expect(m.onkey).not.toHaveBeenCalled()
  })
  it('Esc в поле закрывает окно: поле — не повод держать его', async () => {
    const m = mount()
    expect(await press(m.q('.in'), 'Escape')).toBe(true)
    expect(m.onclose).toHaveBeenCalledTimes(1)
  })
  it('другие клавиши в поле идут в onkey (команды лупы, отмена)', async () => {
    const m = mount()
    await press(m.q('.in'), 'z', { ctrlKey: true })
    expect(m.onkey).toHaveBeenCalledTimes(1)
  })
})
