// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'

// У кнопки «+» нет текста — имя для скринридера и UIA (проверка Windows-exe, спека 118) даёт aria-label.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: AddColumn } = await import('./AddColumn.svelte')
const { store } = await import('../../base/core/store.svelte')

afterEach(cleanup)

describe('AddColumn: доступное имя кнопки', () => {
  it('у кнопки «+» есть aria-label «Новый разворот»', () => {
    store.view = { ...store.view, spreads: [], settings: {} } as never
    const { container } = render(AddColumn)
    const b = container.querySelector('button.add')
    expect(b?.textContent?.trim()).toBe('+')
    expect(b?.getAttribute('aria-label')).toBe('Новый разворот')
  })
})
