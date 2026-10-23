// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Go подменён: браузер не открывается, считаем вызовы OpenLink.
const openLink = vi.hoisted(() => vi.fn(async (_id: string) => {}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ OpenLink: openLink }))

const { default: AboutDialog } = await import('./AboutDialog.svelte')
const { aboutUi } = await import('./about.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')

beforeEach(() => {
  store.version = '0.23.0'
  store.toast = null
})
afterEach(() => {
  cleanup()
  aboutUi.close()
  openLink.mockReset()
  openLink.mockResolvedValue(undefined)
})

function open() {
  aboutUi.show()
  flushSync()
}

describe('AboutDialog: «О программе»', () => {
  it('закрыто — окна в DOM нет', () => {
    render(AboutDialog)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('показывает версию из store, автора, строку 7(b) и адрес репозитория', () => {
    render(AboutDialog)
    open()
    const box = screen.getByRole('dialog')
    expect(box.getAttribute('aria-label')).toBe('О программе')
    const text = box.textContent ?? ''
    expect(text).toContain('Sosnina Albums 0.23.0')
    expect(text).toContain('Автор — Илья Соснин')
    expect(text).toContain('Вместе с фотографом Мариной Сосниной')
    expect(text).toContain('Основано на Sosnina Albums, автор Илья Соснин')
    expect(text).toContain('github.com/IliaSA101/sosninaalbums')
    expect(text).toContain('GNU GPL v3')
  })

  it('семь кнопок зовут OpenLink со своим id', async () => {
    render(AboutDialog)
    open()
    const click = (name: string, group?: string) => {
      const scope = group ? within(screen.getByRole('group', { name: group })) : screen
      return fireEvent.click(scope.getByText(name, { selector: 'button' }))
    }
    await click('Исходный код')
    await click('Текст лицензии')
    await click('Sosnina films')
    await click('Telegram', 'Сообщить о проблеме')
    await click('GitHub', 'Сообщить о проблеме')
    await click('Telegram', 'Предложить идею')
    await click('GitHub', 'Предложить идею')
    expect(openLink.mock.calls.map((c) => c[0])).toEqual(['source', 'license', 'marina', 'telegram', 'bug', 'telegram', 'idea'])
    // Окно после ссылок остаётся: фотограф возвращается к программе.
    expect(screen.queryByRole('dialog')).not.toBeNull()
  })

  it('обратная связь — две строки с подписями и по две кнопки', () => {
    render(AboutDialog)
    open()
    for (const name of ['Сообщить о проблеме', 'Предложить идею']) {
      const g = within(screen.getByRole('group', { name }))
      expect(g.getAllByRole('button').map((b) => b.textContent)).toEqual(['Telegram', 'GitHub'])
    }
  })

  it('«Закрыть» и Esc закрывают; повторное открытие снова показывает окно', async () => {
    render(AboutDialog)
    open()
    expect(document.activeElement?.textContent).toBe('Закрыть')
    await fireEvent.click(screen.getByText('Закрыть'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(modalStack.any()).toBe(false)
    open()
    await fireEvent.keyDown(document.activeElement as Element, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    open()
    expect(screen.queryByRole('dialog')).not.toBeNull()
  })

  it('ошибка OpenLink — тост, окно остаётся', async () => {
    openLink.mockRejectedValueOnce('неизвестная ссылка')
    render(AboutDialog)
    open()
    await fireEvent.click(within(screen.getByRole('group', { name: 'Предложить идею' })).getByText('Telegram'))
    expect(store.toast?.text).toBe('Неизвестная ссылка')
    expect(screen.queryByRole('dialog')).not.toBeNull()
  })
})
