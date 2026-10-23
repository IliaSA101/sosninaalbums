// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'

// Строка предупреждения о полях у реза под сводкой «Разворот = …» (0.23-q7 а). Go подменён.
const api = vi.hoisted(() => ({ MarginNote: vi.fn(), PagePresets: vi.fn(), DecorCatalog: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: SettingsForm } = await import('./SettingsForm.svelte')
const { store } = await import('../../base/core/store.svelte')

const WARN = 'Поля равны обрезу: край фото ляжет на линию реза.'
const settings = { pageW: 300, pageH: 300, dpi: 300, margin: 10, gap: 3, safe: 5, bleed: 3 }
const inputByLabel = (label: string) =>
  Array.from(document.querySelectorAll('label'))
    .find((l) => l.textContent?.includes(label))!
    .querySelector('input')!
const noteTexts = () => Array.from(document.querySelectorAll('p.note')).map((p) => p.textContent?.trim())
const tick = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.PagePresets.mockResolvedValue([])
  api.MarginNote.mockResolvedValue('')
  store.view = { rev: 1, folders: [], spreads: [], settings } as never
})
afterEach(() => cleanup())

describe('предупреждение о полях у линии реза', () => {
  it('Go вернул пусто — строки нет; вызов идёт с Полями и Обрезом черновика', async () => {
    render(SettingsForm)
    await tick()
    expect(api.MarginNote).toHaveBeenCalledWith(10, 3)
    expect(noteTexts()).not.toContain(WARN)
  })
  it('смена Полей и Обреза зовёт Go заново; непустой ответ — строка, пустой — гаснет', async () => {
    render(SettingsForm)
    await tick()
    api.MarginNote.mockResolvedValue(WARN)
    await fireEvent.input(inputByLabel('Поля, мм'), { target: { value: '3' } })
    await tick()
    expect(api.MarginNote).toHaveBeenLastCalledWith(3, 3)
    expect(noteTexts()).toContain(WARN)
    api.MarginNote.mockResolvedValue('')
    await fireEvent.input(inputByLabel('Обрез, мм'), { target: { value: '0' } })
    await tick()
    expect(api.MarginNote).toHaveBeenLastCalledWith(3, 0)
    expect(noteTexts()).not.toContain(WARN)
  })
  it('строка стоит сразу после сводки «Разворот = …»', async () => {
    api.MarginNote.mockResolvedValue(WARN)
    render(SettingsForm)
    await tick()
    const t = noteTexts()
    const i = t.findIndex((x) => x?.startsWith('Разворот'))
    expect(i).toBeGreaterThanOrEqual(0)
    expect(t[i + 1]).toBe(WARN)
  })
})
