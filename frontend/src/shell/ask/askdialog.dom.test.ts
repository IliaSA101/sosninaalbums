// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Go подменён: бэкенда в DOM-тесте нет; SetGrid — единственный вызов, который окно «Вернуть авто?» может вызвать.
const setGrid = vi.hoisted(() => vi.fn(async () => ({ rev: 2, folders: [], spreads: [], settings: {} })))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SetGrid: setGrid, SetLayout: vi.fn() }))

const { default: AutoAskDialog } = await import('./AutoAskDialog.svelte')
const { gridAct } = await import('../../panels/layouts/gridact.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')

const grid = { id: 's1', grid: true, photos: [], rects: [] }

afterEach(() => {
  cleanup()
  gridAct.cancelAuto()
  setGrid.mockClear()
})

function open() {
  store.view = { ...store.view, rev: 1, spreads: [grid] } as never
  void gridAct.backToAuto('s1')
  flushSync()
}

describe('AutoAskDialog: «Вернуть авто?»', () => {
  it('нет вопроса — окна в DOM нет', () => {
    render(AutoAskDialog)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('открыть → «Отмена» → окно ушло, Go не звали; снова открыть — окно снова есть', async () => {
    render(AutoAskDialog)
    open()
    expect(screen.getByRole('dialog').getAttribute('aria-label')).toBe('Вернуть авто?')
    await fireEvent.click(screen.getByText('Отмена'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(setGrid).not.toHaveBeenCalled()
    expect(modalStack.any()).toBe(false)
    open()
    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(modalStack.any()).toBe(true)
  })

  it('безопасный выбор «Отмена» получает фокус', () => {
    render(AutoAskDialog)
    open()
    expect(document.activeElement?.textContent).toBe('Отмена')
  })

  it('Esc закрывает как «Отмена» и не доходит до страницы', async () => {
    const page = vi.fn()
    window.addEventListener('keydown', page)
    render(AutoAskDialog)
    open()
    await fireEvent.keyDown(document.activeElement as Element, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(setGrid).not.toHaveBeenCalled()
    expect(page).not.toHaveBeenCalled()
    window.removeEventListener('keydown', page)
  })

  it('щелчок по фону закрывает, щелчок внутри окна — нет', async () => {
    render(AutoAskDialog)
    open()
    const box = screen.getByRole('dialog')
    await fireEvent.pointerDown(box)
    expect(screen.queryByRole('dialog')).not.toBeNull()
    await fireEvent.pointerDown(box.parentElement as Element)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('«Вернуть авто» зовёт Go один раз и закрывает окно', async () => {
    render(AutoAskDialog)
    open()
    const ok = screen.getByText('Вернуть авто', { selector: 'button' })
    await fireEvent.click(ok)
    expect(setGrid).toHaveBeenCalledExactlyOnceWith('s1', false)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('разворот пропал из вида (prune) — окно закрывается само', () => {
    render(AutoAskDialog)
    open()
    gridAct.prune([])
    flushSync()
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
