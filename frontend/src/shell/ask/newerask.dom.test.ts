// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import type { NewerInfo } from '../../base/core/types'

// Окно «Альбом из новой версии» (112): кнопки и порядок, фокус, Esc, вызовы Go, ошибка — тостом, путь через store.
const openInNewer = vi.hoisted(() => vi.fn())
const openLink = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ OpenInNewer: openInNewer, OpenLink: openLink }))

const { default: NewerAskDialog } = await import('./NewerAskDialog.svelte')
const { newerAsk } = await import('./newerask.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { wire } = await import('../boot/wire')

const info = (p: Partial<NewerInfo> = {}): NewerInfo => ({
  name: 'Свадьба.salbum',
  draft: false,
  need: '0.26',
  mine: '0.25',
  have: '',
  ...p,
})
const dialog = () => screen.queryByRole('dialog')
const labels = () => [...screen.getByRole('dialog').querySelectorAll('.actions button')].map((b) => b.textContent)
const shown = (i: NewerInfo = info()) => {
  render(NewerAskDialog)
  newerAsk.show(i)
  return vi.waitFor(() => expect(dialog()).not.toBeNull())
}

afterEach(() => {
  newerAsk.close()
  newerAsk.busy = false
  cleanup()
  openInNewer.mockReset()
  openLink.mockReset()
  store.toast = null
})

describe('NewerAskDialog', () => {
  it('нет данных — окна в DOM нет', () => {
    render(NewerAskDialog)
    expect(dialog()).toBeNull()
  })

  it('версии на компьютере нет: «Закрыть» в фокусе и «Скачать» (главная)', async () => {
    await shown()
    expect(screen.getByRole('dialog').getAttribute('aria-label')).toBe('Альбом из новой версии')
    expect(labels()).toEqual(['Закрыть', 'Скачать'])
    expect(document.activeElement?.textContent).toBe('Закрыть')
    expect(screen.getByText('Скачать').classList.contains('primary')).toBe(true)
    expect(modalStack.any()).toBe(true)
  })

  it('версия есть: Закрыть · Скачать · «Открыть в 0.26» (главная)', async () => {
    await shown(info({ have: '0.26' }))
    expect(labels()).toEqual(['Закрыть', 'Скачать', 'Открыть в 0.26'])
    expect(screen.getByText('Открыть в 0.26').classList.contains('primary')).toBe(true)
    expect(screen.getByText('Скачать').classList.contains('primary')).toBe(false)
  })

  it('черновик: заголовок «Черновик из новой версии»', async () => {
    await shown(info({ name: '', draft: true }))
    expect(screen.getByRole('dialog').getAttribute('aria-label')).toBe('Черновик из новой версии')
  })

  it('«Закрыть» закрывает окно, Go не зовётся', async () => {
    await shown(info({ have: '0.26' }))
    await fireEvent.click(screen.getByText('Закрыть'))
    expect(dialog()).toBeNull()
    expect(modalStack.any()).toBe(false)
    expect(openInNewer).not.toHaveBeenCalled()
    expect(openLink).not.toHaveBeenCalled()
  })

  it('Esc закрывает и не доходит до страницы; повторное окно открывается снова', async () => {
    const page = vi.fn()
    window.addEventListener('keydown', page)
    await shown()
    await fireEvent.keyDown(document.activeElement as Element, { key: 'Escape' })
    expect(dialog()).toBeNull()
    expect(page).not.toHaveBeenCalled()
    window.removeEventListener('keydown', page)
    newerAsk.show(info())
    await vi.waitFor(() => expect(dialog()).not.toBeNull())
  })

  it('«Скачать» зовёт OpenLink("download"), окно остаётся', async () => {
    openLink.mockResolvedValue(undefined)
    await shown()
    await fireEvent.click(screen.getByText('Скачать'))
    expect(openLink).toHaveBeenCalledExactlyOnceWith('download')
    expect(dialog()).not.toBeNull()
  })

  it('«Открыть в 0.26» зовёт OpenInNewer один раз, пока идёт вызов', async () => {
    let done: () => void = () => {}
    openInNewer.mockReturnValue(new Promise<void>((r) => (done = r)))
    await shown(info({ have: '0.26' }))
    const btn = screen.getByText('Открыть в 0.26')
    await fireEvent.click(btn)
    await fireEvent.click(btn)
    expect(openInNewer).toHaveBeenCalledTimes(1)
    done()
    await Promise.resolve()
  })

  it('успех OpenInNewer: busy остаётся (Go закрывает программу позже), повторный щелчок не запускает вторую копию', async () => {
    openInNewer.mockResolvedValue(undefined)
    await shown(info({ have: '0.26' }))
    const btn = screen.getByText('Открыть в 0.26')
    await fireEvent.click(btn)
    await vi.waitFor(() => expect(openInNewer).toHaveBeenCalledTimes(1))
    await new Promise((r) => setTimeout(r, 0))
    expect(newerAsk.busy).toBe(true)
    await fireEvent.click(btn)
    expect(openInNewer).toHaveBeenCalledTimes(1)
  })

  it('ошибка OpenInNewer — тост словами с заглавной, окно остаётся', async () => {
    openInNewer.mockRejectedValue('новая версия не найдена')
    await shown(info({ have: '0.26' }))
    await fireEvent.click(screen.getByText('Открыть в 0.26'))
    await vi.waitFor(() => expect(store.toast?.text).toBe('Новая версия не найдена'))
    expect(store.toast?.kind).toBe('error')
    expect(dialog()).not.toBeNull()
    expect(newerAsk.busy).toBe(false)
    // после ошибки кнопка снова рабочая
    openInNewer.mockResolvedValue(undefined)
    await fireEvent.click(screen.getByText('Открыть в 0.26'))
    await vi.waitFor(() => expect(openInNewer).toHaveBeenCalledTimes(2))
  })

  it('стенд QA: «Скачать» — тост «На стенде ссылки не открываются», Go не зовётся, окно остаётся', async () => {
    store.stand = true
    await shown()
    await fireEvent.click(screen.getByText('Скачать'))
    expect(openLink).not.toHaveBeenCalled()
    expect(store.toast?.text).toBe('На стенде ссылки не открываются')
    expect(dialog()).not.toBeNull()
    store.stand = false
  })

  it('ошибка OpenLink — тост', async () => {
    openLink.mockRejectedValue(new Error('Error: не удалось открыть ссылку'))
    await shown()
    await fireEvent.click(screen.getByText('Скачать'))
    await vi.waitFor(() => expect(store.toast?.text).toBe('Не удалось открыть ссылку'))
  })

  // Внешнее открытие (OpenExternal), «Открыть» и «Последние» идут через replaceState: хук wire() ведёт ответ в окно.
  it('ответ OpenExternal {cancelled, newer}: окно показано через wire()', async () => {
    wire()
    render(NewerAskDialog)
    const view = { spreads: [], folders: [], path: '', rev: 0 }
    expect(await store.replaceState(Promise.resolve({ view, photos: [], cancelled: true, newer: info({ have: '0.26' }) }))).toBe(true)
    await vi.waitFor(() => expect(dialog()).not.toBeNull())
    expect(labels()).toEqual(['Закрыть', 'Скачать', 'Открыть в 0.26'])
  })
})
