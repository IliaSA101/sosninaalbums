// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { createRawSnippet } from 'svelte'

// Modal.onkey: клавиши окна, пока оно верхнее и фокус в нём (в том числе на самом окне).
const { default: Modal } = await import('./Modal.svelte')

afterEach(cleanup)
const body = createRawSnippet(() => ({ render: () => '<div><input id="inp" /></div>' }))
const open = (onkey: (e: KeyboardEvent) => void, title = 'Окно') => render(Modal, { title, onclose: () => {}, onkey, children: body })
const boxes = () => document.querySelectorAll<HTMLElement>('.box')

describe('Modal.onkey', () => {
  it('зовётся и при фокусе на самом окне, и из поля внутри', async () => {
    const onkey = vi.fn()
    open(onkey)
    boxes()[0].focus()
    await fireEvent.keyDown(boxes()[0], { key: 'Enter' })
    await fireEvent.keyDown(document.getElementById('inp')!, { key: 'Enter' })
    expect(onkey).toHaveBeenCalledTimes(2)
  })

  it('нижнее окно молчит, пока сверху другое', async () => {
    const low = vi.fn()
    const high = vi.fn()
    open(low, 'Нижнее')
    open(high, 'Верхнее')
    await fireEvent.keyDown(boxes()[1], { key: 'Enter' })
    expect(low).not.toHaveBeenCalled()
    expect(high).toHaveBeenCalledTimes(1)
  })

  it('клавиша вне окна не доходит до onkey', async () => {
    const onkey = vi.fn()
    open(onkey)
    await fireEvent.keyDown(document.body, { key: 'Enter' })
    expect(onkey).not.toHaveBeenCalled()
  })
})
