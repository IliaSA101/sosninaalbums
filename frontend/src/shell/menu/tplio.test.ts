import { beforeEach, describe, expect, it, vi } from 'vitest'

// «Мои шаблоны» в файл и из файла (086): Go подменён; итог — тост с текстом Go как есть.
const api = vi.hoisted(() => ({
  ExportTemplates: vi.fn(),
  ImportTemplates: vi.fn(),
  ShowDataFolder: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.stubGlobal('window', globalThis)

const { tplio } = await import('./tplio.svelte')
const { COMMANDS } = await import('./commands')
const { store } = await import('../../base/core/store.svelte')
const { vigOwn } = await import('../../panels/vignette/vigown.svelte')

const toast = () => store.toast as { text: string; kind?: string } | null

beforeEach(() => {
  vi.clearAllMocks()
  store.toast = null
  tplio.seq = 0
  vigOwn.rev = 0
})

describe('tplio.save', () => {
  it('итог Go — тост как есть, счётчики не трогает', async () => {
    api.ExportTemplates.mockResolvedValue({ canceled: false, saved: 14, text: 'Сохранено шаблонов: 14' })
    await tplio.save()
    expect(toast()?.text).toBe('Сохранено шаблонов: 14')
    expect(toast()?.kind).not.toBe('error')
    expect(tplio.seq).toBe(0)
  })

  it('отказ от диалога — без тоста', async () => {
    api.ExportTemplates.mockResolvedValue({ canceled: true, saved: 0, text: '' })
    await tplio.save()
    expect(toast()).toBeNull()
  })

  it('пусто и другие отказы Go — тост-ошибка с заглавной буквы', async () => {
    api.ExportTemplates.mockRejectedValue('своих шаблонов пока нет — сохранять нечего')
    await tplio.save()
    expect(toast()).toMatchObject({ text: 'Своих шаблонов пока нет — сохранять нечего', kind: 'error' })
  })
})

describe('tplio.load', () => {
  it('успех: тост с текстом Go, tplSeq и vigOwn.rev растут — галерея и полоса виньеток перезапросят список', async () => {
    api.ImportTemplates.mockResolvedValue({ canceled: false, added: 12, same: 3, over: 0, text: 'Добавлено шаблонов: 12 · уже были: 3' })
    await tplio.load()
    expect(toast()?.text).toBe('Добавлено шаблонов: 12 · уже были: 3')
    expect(tplio.seq).toBe(1)
    expect(vigOwn.rev).toBe(1)
    await tplio.load()
    expect(tplio.seq).toBe(2)
  })

  it('отказ от диалога — без тоста и без перезапроса', async () => {
    api.ImportTemplates.mockResolvedValue({ canceled: true, added: 0, same: 0, over: 0, text: '' })
    await tplio.load()
    expect(toast()).toBeNull()
    expect(tplio.seq).toBe(0)
    expect(vigOwn.rev).toBe(0)
  })

  it('отказ Go («Все шаблоны из файла уже есть») — тост-ошибка, список не перезапрашивается', async () => {
    api.ImportTemplates.mockRejectedValue(new Error('все шаблоны из файла уже есть'))
    await tplio.load()
    expect(toast()).toMatchObject({ text: 'Все шаблоны из файла уже есть', kind: 'error' })
    expect(tplio.seq).toBe(0)
  })
})

describe('команды меню', () => {
  it('tplExport и tplImport зовут свои методы Go', async () => {
    api.ExportTemplates.mockResolvedValue({ canceled: true, saved: 0, text: '' })
    api.ImportTemplates.mockResolvedValue({ canceled: true, added: 0, same: 0, over: 0, text: '' })
    await COMMANDS.tplExport()
    await COMMANDS.tplImport()
    expect(api.ExportTemplates).toHaveBeenCalledTimes(1)
    expect(api.ImportTemplates).toHaveBeenCalledTimes(1)
  })

  it('showData: Go без аргументов, тоста нет; ошибка — тост с заглавной буквы', async () => {
    api.ShowDataFolder.mockResolvedValue(undefined)
    await COMMANDS.showData()
    expect(api.ShowDataFolder).toHaveBeenCalledWith()
    expect(toast()).toBeNull()
    api.ShowDataFolder.mockRejectedValue(new Error('не удалось открыть папку данных'))
    await COMMANDS.showData()
    expect(toast()).toMatchObject({ text: 'Не удалось открыть папку данных', kind: 'error' })
  })

  it('showData на стенде: тост «На стенде папки не открываются», Go не зовётся', async () => {
    store.stand = true
    try {
      await COMMANDS.showData()
      expect(api.ShowDataFolder).not.toHaveBeenCalled()
      expect(toast()).toMatchObject({ text: 'На стенде папки не открываются' })
    } finally {
      store.stand = false
    }
  })
})
