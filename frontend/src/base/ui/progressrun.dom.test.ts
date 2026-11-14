// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import ProgressRun from './ProgressRun.svelte'

// Общая полоса хода (PackRun, VideoWindow): строка, полоса, «Отменить».
afterEach(cleanup)

describe('ProgressRun', () => {
  it('строка, полоса с долей и «Отменить» зовёт oncancel', async () => {
    const oncancel = vi.fn()
    render(ProgressRun, { text: 'Скачиваем… 3 из 10 МБ', percent: 30, label: 'Ход', oncancel })
    expect(screen.getByRole('status').textContent).toBe('Скачиваем… 3 из 10 МБ')
    expect(document.querySelector('progress')!.getAttribute('value')).toBe('30')
    await fireEvent.click(screen.getByRole('button', { name: 'Отменить' }))
    expect(oncancel).toHaveBeenCalledOnce()
  })

  it('доля null — неопределённая полоса; отмена идёт — «Отменяем…» и кнопка серая', () => {
    render(ProgressRun, { text: 'Скачиваем…', percent: null, label: 'Ход', cancelling: true, oncancel: () => {} })
    expect(document.querySelector('progress')!.hasAttribute('value')).toBe(false)
    expect(screen.getByRole('status').textContent).toBe('Отменяем…')
    expect((screen.getByRole('button', { name: 'Отменить' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('autofocus ставит data-autofocus для Modal; без него — нет', () => {
    render(ProgressRun, { text: 'x', percent: 1, label: 'Ход', autofocus: true, oncancel: () => {} })
    expect(screen.getByRole('button', { name: 'Отменить' }).hasAttribute('data-autofocus')).toBe(true)
    cleanup()
    render(ProgressRun, { text: 'x', percent: 1, label: 'Ход', oncancel: () => {} })
    expect(screen.getByRole('button', { name: 'Отменить' }).hasAttribute('data-autofocus')).toBe(false)
  })
})
