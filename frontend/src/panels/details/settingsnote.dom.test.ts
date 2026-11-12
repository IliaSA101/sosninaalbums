// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// Строка предупреждения о полях у реза под сводкой «Разворот = …» (0.23-q7 а). Go подменён.
const api = vi.hoisted(() => ({ MarginNote: vi.fn(), PagePresets: vi.fn(), DecorCatalog: vi.fn(), SetSettings: vi.fn() }))
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

// Подрезка, чтобы закрыть лист (104): блок в том же виде, что «Фото через корешок»; предел лежит в черновике до «Применить».
describe('блок «Подрезать фото, чтобы закрыть лист»', () => {
  const block = () => document.querySelector('[data-guide="crop-mode"]')
  const seg = () => Array.from(block()!.querySelectorAll('.seg button')) as HTMLButtonElement[]
  const pressed = () => seg().find((b) => b.getAttribute('aria-pressed') === 'true')?.textContent
  const notes = () => Array.from(block()!.querySelectorAll('p.note')).map((p) => p.textContent?.trim())

  it('альбом: блок стоит сразу под «Фото через корешок», четыре пункта, по умолчанию «Нет»', async () => {
    render(SettingsForm)
    await tick()
    expect(block()).not.toBeNull()
    expect(block()!.previousElementSibling?.getAttribute('data-guide')).toBe('fold-mode')
    expect(seg().map((b) => b.textContent)).toEqual(['Нет', 'до 5 %', 'до 10 %', 'до 15 %'])
    expect(pressed()).toBe('Нет')
    expect(notes()).toHaveLength(1)
  })
  it('сохранённое значение показано выбранным; чужое число — «Нет»', async () => {
    store.view = { rev: 1, folders: [], spreads: [], settings: { ...settings, crop: 10 } } as never
    render(SettingsForm)
    await tick()
    expect(pressed()).toBe('до 10 %')
    cleanup()
    store.view = { rev: 2, folders: [], spreads: [], settings: { ...settings, crop: 7 } } as never
    render(SettingsForm)
    await tick()
    expect(pressed()).toBe('Нет')
  })
  it('выбор меняет пояснение и добавляет вторую строку; возврат к сохранённому её гасит; повторный щелчок ничего не ломает', async () => {
    render(SettingsForm)
    await tick()
    await fireEvent.click(seg()[1])
    expect(pressed()).toBe('до 5 %')
    expect(notes()[0]).toContain('не больше 5 % по стороне')
    expect(notes()).toHaveLength(2)
    await fireEvent.click(seg()[1])
    expect(pressed()).toBe('до 5 %')
    expect(notes()).toHaveLength(2)
    await fireEvent.click(seg()[0])
    expect(notes()).toHaveLength(1)
  })
  it('«Применить» отправляет подрезку в Go одним вызовом; без правки кнопка серая', async () => {
    const apply = () => screen.getByRole('button', { name: 'Применить' }) as HTMLButtonElement
    render(SettingsForm)
    await tick()
    expect(apply().disabled).toBe(true)
    await fireEvent.click(seg()[3])
    expect(apply().disabled).toBe(false)
    await fireEvent.click(apply())
    await tick()
    expect(api.SetSettings).toHaveBeenCalledTimes(1)
    expect(api.SetSettings.mock.calls[0][0]).toMatchObject({ crop: 15 })
  })
  it('лист виньетки: блока нет', async () => {
    store.view = { rev: 1, folders: [], spreads: [], settings: { ...settings, sheet: true } } as never
    render(SettingsForm)
    await tick()
    expect(block()).toBeNull()
    expect(document.querySelector('[data-guide="fold-mode"]')).toBeNull()
  })
})
