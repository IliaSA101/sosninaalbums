// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, waitFor } from '@testing-library/svelte'
import { createRawSnippet } from 'svelte'

// Modal: [data-autofocus] берёт фокус при открытии; при смене содержимого на месте — только цель с data-autofocus="screen".
const { default: Modal } = await import('./Modal.svelte')

afterEach(cleanup)
const body = createRawSnippet(() => ({ render: () => '<div id="host"><button id="a" data-autofocus>A</button></div>' }))
const open = () => render(Modal, { title: 'Окно', onclose: () => {}, children: body })
const host = () => document.getElementById('host')!
const swap = (id: string, mark = 'screen') => {
  const el = document.createElement('button')
  el.id = id
  el.setAttribute('data-autofocus', mark)
  host().replaceChildren(el)
}

describe('Modal.autofocus', () => {
  it('при монтировании фокус на поле с data-autofocus', async () => {
    open()
    await waitFor(() => expect(document.activeElement?.id).toBe('a'))
  })

  it('кнопка с фокусом исчезла, пришла новая с пометкой screen — фокус на ней', async () => {
    open()
    await waitFor(() => expect(document.activeElement?.id).toBe('a'))
    swap('b')
    await waitFor(() => expect(document.activeElement?.id).toBe('b'))
  })

  it('фокус на самом окне, пришла цель с пометкой screen — фокус на ней', async () => {
    open()
    host().replaceChildren()
    document.querySelector<HTMLElement>('.box')!.focus()
    swap('c')
    await waitFor(() => expect(document.activeElement?.id).toBe('c'))
  })

  it('кнопка с фокусом исчезла, пришла новая без пометки screen — фокус не переходит на неё', async () => {
    open()
    await waitFor(() => expect(document.activeElement?.id).toBe('a'))
    swap('b', '')
    await new Promise((r) => setTimeout(r, 20))
    expect(document.activeElement?.id).not.toBe('b')
  })

  it('фокус на самом окне, пришёл узел без цели с пометкой — фокус остаётся на окне', async () => {
    open()
    host().replaceChildren()
    const box = document.querySelector<HTMLElement>('.box')!
    box.focus()
    swap('d', '')
    await new Promise((r) => setTimeout(r, 20))
    expect(document.activeElement).toBe(box)
  })

  it('фокус в поле ввода: новое содержимое его не отбирает', async () => {
    open()
    const inp = document.createElement('input')
    host().append(inp)
    inp.focus()
    const btn = document.createElement('button')
    btn.setAttribute('data-autofocus', '')
    host().append(btn)
    await new Promise((r) => setTimeout(r, 20))
    expect(document.activeElement).toBe(inp)
  })
})
