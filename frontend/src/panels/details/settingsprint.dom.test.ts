// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// Список «Типография» в «Настройках альбома» (123 Б3): у своей — «(своя)», последним пунктом — «Своя типография…» (открывает окно
// на основе выбранной, список остаётся на ней). Go подменён.
const api = vi.hoisted(() => ({ PrintProfileEdit: vi.fn(), PrintProfiles: vi.fn(), PagePresets: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    MarginNote: vi.fn(async () => ''),
    ...api,
  }
})

import type { PrintSource } from '../../base/core/types'
const { default: SettingsForm } = await import('./SettingsForm.svelte')
const { store } = await import('../../base/core/store.svelte')
const { printList } = await import('../../shell/printprof/printlist.svelte')
const { printEdit } = await import('../../shell/printprof/printedit.svelte')
const { NEW_VALUE } = await import('../../shell/printprof/printwords')

const info = (id: string, name: string, source: PrintSource) => ({ id, name, source, version: 1, hasExport: true, formats: [] })
const tick = () => new Promise((r) => setTimeout(r, 0))
const list = () => screen.getByLabelText('Типография') as HTMLSelectElement

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.PagePresets.mockResolvedValue([
    { label: '20×20', group: '', w: 200, h: 200 },
    { label: '30×20', group: 'Фабрика Фотокниги', w: 304, h: 203 },
    { label: '20×20', group: 'Типография Альфа', w: 200, h: 200 },
    { label: '25×25', group: 'Нетпринт', w: 250, h: 250 },
  ])
  api.PrintProfileEdit.mockResolvedValue({
    name: 'x',
    base: '',
    dpi: 300,
    layflat: false,
    safe: 10,
    bleed: 3,
    formats: [],
    spreads: 1,
    prefix: '',
    quality: 100,
    url: '',
  })
  api.PrintProfiles.mockResolvedValue([])
  printList.infos = [info('fabrika', 'Фабрика Фотокниги', 'builtin'), info('own-1', 'Типография Альфа', 'own')]
  printList.rev = 0
  printEdit.close()
  store.view = {
    rev: 1,
    folders: [],
    spreads: [],
    settings: { pageW: 304, pageH: 203, dpi: 300, margin: 10, gap: 3, safe: 5, bleed: 3, bg: '' },
  } as never
})
afterEach(() => cleanup())

describe('список «Типография»', () => {
  it('своя — «(своя)», встроенная и «Общие» как есть, «Своя типография…» последним', async () => {
    render(SettingsForm)
    await tick()
    const texts = Array.from(list().options).map((o) => o.textContent)
    expect(texts).toEqual(['Общие', 'Фабрика Фотокниги', 'Типография Альфа (своя)', 'Нетпринт', 'Своя типография…'])
    expect(list().options[list().options.length - 1].value).toBe(NEW_VALUE)
  })

  it('«Своя типография…» на выбранной типографии: окно на её основе, список не уходит с неё', async () => {
    render(SettingsForm)
    await tick()
    list().value = 'Типография Альфа'
    await fireEvent.change(list())
    list().value = NEW_VALUE
    await fireEvent.change(list())
    await tick()
    expect(api.PrintProfileEdit).toHaveBeenCalledWith('own-1')
    expect(printEdit.open).toBe(true)
    expect(list().value).toBe('Типография Альфа')
  })

  it('на «Общие» — пустая основа', async () => {
    render(SettingsForm)
    await tick()
    list().value = ''
    await fireEvent.change(list())
    list().value = NEW_VALUE
    await fireEvent.change(list())
    await tick()
    expect(api.PrintProfileEdit).toHaveBeenCalledWith('')
  })

  it('print:changed (rev) — пресеты читаются заново', async () => {
    render(SettingsForm)
    await tick()
    api.PagePresets.mockClear()
    printList.rev++
    await tick()
    expect(api.PagePresets).toHaveBeenCalledTimes(1)
  })

  it('print:changed два раза подряд: поздний ответ раннего запроса не затирает свежий', async () => {
    render(SettingsForm)
    await tick()
    const late: Array<(l: unknown) => void> = []
    api.PagePresets.mockReset()
    api.PagePresets.mockImplementation(() => new Promise((r) => late.push(r)))
    printList.rev++
    await tick()
    printList.rev++
    await tick()
    expect(late).toHaveLength(2)
    late[1]([{ label: '30×20', group: 'Новая', w: 304, h: 203 }]) // свежий ответ пришёл первым
    await tick()
    late[0]([{ label: '30×20', group: 'Старая', w: 304, h: 203 }])
    await tick()
    const texts = Array.from(list().options).map((o) => o.textContent)
    expect(texts).toContain('Новая')
    expect(texts).not.toContain('Старая')
  })
})

describe('список «Типография» следует за видом Go', () => {
  const settings = (pageW: number, pageH: number) => ({ pageW, pageH, dpi: 300, margin: 10, gap: 3, safe: 5, bleed: 3, bg: '' })
  const setView = (pageW: number, pageH: number) => {
    store.view = { ...store.view, rev: store.view.rev + 1, settings: settings(pageW, pageH) } as never
  }

  it('смена настроек не из формы (Undo, Redo, открытие альбома) — список на типографии альбома, ручной выбор сбрасывается', async () => {
    render(SettingsForm)
    await tick()
    expect(list().value).toBe('Фабрика Фотокниги')
    list().value = 'Типография Альфа' // выбор вручную, без применения
    await fireEvent.change(list())
    expect(list().value).toBe('Типография Альфа')
    setView(250, 250) // Undo вернул формат Нетпринта
    await tick()
    expect(list().value).toBe('Нетпринт')
    setView(304, 203) // Redo
    await tick()
    expect(list().value).toBe('Фабрика Фотокниги')
    setView(120, 130) // формат без пресета: по прежнему правилу — последняя выбранная вручную, а не «Общие»
    await tick()
    expect(list().value).toBe('Типография Альфа')
  })

  it('есть неприменённые правки формы — смена вида ручной выбор не трогает', async () => {
    render(SettingsForm)
    await tick()
    const w = screen.getByLabelText('Ширина страницы, мм') as HTMLInputElement
    await fireEvent.input(w, { target: { value: '310' } })
    list().value = 'Типография Альфа'
    await fireEvent.change(list())
    setView(250, 250)
    await tick()
    expect(list().value).toBe('Типография Альфа')
  })
})
