// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Форма «Настройки альбома» для агентов (149-16): «занят» — непринятая правка черновика или фокус внутри раздела.
// Само раскрытие раздела ни правкой, ни занятостью не считается. Go подменён.
const api = vi.hoisted(() => ({ MarginNote: vi.fn(), PagePresets: vi.fn(), SetSettings: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: SettingsForm } = await import('./SettingsForm.svelte')
const { settingsDraft, settingsBusy } = await import('./settingsbusy.svelte')
const { store } = await import('../../base/core/store.svelte')

const settings = { pageW: 300, pageH: 300, dpi: 300, margin: 10, gap: 3, safe: 5, bleed: 3 }
const input = (label: string) =>
  Array.from(document.querySelectorAll('label'))
    .find((l) => l.textContent?.includes(label))!
    .querySelector('input')!

// Тело раздела, как его собирает Section.svelte (id section-settings); кнопка заголовка — снаружи тела.
function host() {
  const head = document.createElement('button')
  head.className = 'head'
  const body = document.createElement('div')
  body.id = 'section-settings'
  document.body.append(head, body)
  return render(SettingsForm, { target: body })
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.PagePresets.mockResolvedValue([])
  api.MarginNote.mockResolvedValue('')
  store.view = { rev: 1, folders: [], spreads: [], settings } as never
})
afterEach(() => {
  cleanup()
  document.body.replaceChildren()
})

describe('SettingsForm → settingsDraft', () => {
  it('форма открыта, правок и фокуса нет — не занят', () => {
    host()
    flushSync()
    expect(settingsBusy()).toBe(false)
  })

  it('правка поля — занят; вернули прежнее значение — снова нет', async () => {
    host()
    await fireEvent.input(input('Поля, мм'), { target: { value: '12' } })
    flushSync()
    expect(settingsDraft.dirty).toBe(true)
    expect(settingsBusy()).toBe(true)
    await fireEvent.input(input('Поля, мм'), { target: { value: '10' } })
    flushSync()
    expect(settingsBusy()).toBe(false)
  })

  it('фокус в поле раздела — занят, уход фокуса за раздел — нет', async () => {
    host()
    const f = input('Поля, мм')
    await fireEvent.focusIn(f)
    expect(settingsDraft.focus).toBe(true)
    expect(settingsBusy()).toBe(true)
    await fireEvent.focusOut(f, { relatedTarget: document.body })
    expect(settingsBusy()).toBe(false)
  })

  it('фокус на кнопке заголовка раздела формой не считается', async () => {
    host()
    await fireEvent.focusIn(document.querySelector('button.head') as Element)
    expect(settingsBusy()).toBe(false)
  })

  it('форма убрана вместе с неприменённой правкой и фокусом — флаги сняты', async () => {
    const v = host()
    await fireEvent.input(input('Поля, мм'), { target: { value: '12' } })
    await fireEvent.focusIn(input('Поля, мм'))
    flushSync()
    expect(settingsBusy()).toBe(true)
    v.unmount()
    expect(settingsBusy()).toBe(false)
  })
})
