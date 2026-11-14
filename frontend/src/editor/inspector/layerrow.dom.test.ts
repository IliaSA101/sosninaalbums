// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import LayerRow from './LayerRow.svelte'

// «Расположение» на общем Seg (action, сетка 2×2): щелчок по пункту — своя операция слоя; фокус холста не отнимается.
afterEach(cleanup)

describe('LayerRow', () => {
  it('щелчок по четырём пунктам зовёт onpick(front/back/up/down) в порядке подписей', async () => {
    const onpick = vi.fn()
    render(LayerRow, { onpick })
    for (const n of ['Вперёд', 'Назад', 'Выше', 'Ниже']) await fireEvent.click(screen.getByRole('button', { name: n }))
    expect(onpick.mock.calls).toEqual([['front'], ['back'], ['up'], ['down']])
  })

  it('это действия, не переключатель: aria-pressed нет; подсказки и значки на месте; mousedown не берёт фокус', async () => {
    render(LayerRow, { onpick: () => {} })
    const b = screen.getByRole('button', { name: 'Вперёд' })
    expect(b.hasAttribute('aria-pressed')).toBe(false)
    expect(b.getAttribute('title')).toBe('На передний план')
    expect(b.querySelector('svg')).not.toBeNull()
    expect(await fireEvent.mouseDown(b)).toBe(false)
    expect(screen.getByRole('group', { name: 'Расположение' }).parentElement?.getAttribute('data-guide')).toBe('layer-row')
  })
})
