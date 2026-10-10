import { beforeEach, describe, expect, it } from 'vitest'
import { tip, tipped } from './tip.svelte'

// Узел без DOM: EventTarget с размерами. Действие знает только addEventListener и getBoundingClientRect.
function fakeNode(): HTMLElement {
  const n = new EventTarget() as unknown as HTMLElement
  n.getBoundingClientRect = () => ({ left: 10, right: 30, top: 40, height: 20, width: 20 }) as DOMRect
  return n
}
const fire = (n: HTMLElement, type: string) => n.dispatchEvent(new Event(type))

describe('tipped', () => {
  beforeEach(() => tip.hide())

  it('показывается по наведению и гаснет при уходе курсора', () => {
    const n = fakeNode()
    tipped(n, { text: 'Корзина' })
    fire(n, 'pointerenter')
    expect(tip.state).toEqual({ text: 'Корзина', x: 20, y: 40, side: 'top' })
    fire(n, 'pointerleave')
    expect(tip.state).toBeNull()
  })

  it('side right: справа по середине', () => {
    const n = fakeNode()
    tipped(n, { text: 'Текст', side: 'right' })
    fire(n, 'pointerenter')
    expect(tip.state).toEqual({ text: 'Текст', x: 30, y: 50, side: 'right' })
  })

  it('подсказка гаснет при удалении элемента', () => {
    const n = fakeNode()
    const a = tipped(n, { text: 'Корзина' })
    fire(n, 'pointerenter')
    expect(tip.state).not.toBeNull()
    a.destroy() // узел исчез под курсором: pointerleave не придёт
    expect(tip.state).toBeNull()
  })

  it('чужая подсказка при удалении элемента не гаснет', () => {
    const a = fakeNode()
    const b = fakeNode()
    const ta = tipped(a, { text: 'A' })
    tipped(b, { text: 'B' })
    fire(a, 'pointerenter')
    fire(b, 'pointerenter') // теперь подсказка у b
    ta.destroy()
    expect(tip.state?.text).toBe('B')
  })

  it('после destroy наведение ничего не показывает', () => {
    const n = fakeNode()
    tipped(n, { text: 'X' }).destroy()
    fire(n, 'pointerenter')
    expect(tip.state).toBeNull()
  })

  it('пустой текст — подсказки нет; update меняет текст на лету и гасит при пустом', () => {
    const n = fakeNode()
    const a = tipped(n, { text: '' })
    fire(n, 'pointerenter')
    expect(tip.state).toBeNull()
    a.update({ text: 'Склеить' })
    fire(n, 'pointerenter')
    expect(tip.state?.text).toBe('Склеить')
    a.update({ text: 'Нельзя' })
    expect(tip.state?.text).toBe('Нельзя')
    a.update({ text: '' })
    expect(tip.state).toBeNull()
  })

  it('update с тем же текстом не заменяет tip.state (иначе цикл эффекта в Svelte)', () => {
    const n = fakeNode()
    const a = tipped(n, { text: 'Зеркало', side: 'right' })
    fire(n, 'pointerenter')
    const before = tip.state
    a.update({ text: 'Зеркало', side: 'right' })
    expect(tip.state).toBe(before)
    a.update({ text: 'Иначе', side: 'right' })
    expect(tip.state).not.toBe(before)
    expect(tip.state?.text).toBe('Иначе')
  })
})
