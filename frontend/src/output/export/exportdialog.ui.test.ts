import { beforeEach, describe, expect, it, vi } from 'vitest'

// Окно «Экспорт» (073): состояние хода живёт в jobs — окно только рисует его; тест гоняет сценарий через jobs.
const handlers: Record<string, (...a: unknown[]) => void> = {}
const calls: string[] = []
let exportImpl: () => Promise<boolean> = async () => true
let resolveState: (st: unknown) => void = () => {}

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, fn: (...a: unknown[]) => void) => {
    handlers[name] = fn
    return () => {}
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => ({
  GetState: () => new Promise((r) => (resolveState = r)),
  MissingNames: async () => ({}),
  Export: () => (calls.push('Export'), exportImpl()),
  CancelExport: async () => void calls.push('CancelExport'),
  ShowInExplorer: async () => void calls.push('ShowInExplorer'),
}))

vi.stubGlobal('window', globalThis)
const { store } = await import('../../base/core/store.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')

const opt = { range: '', quality: 95, dpi: 0, pages: false, cover: false, coverOnly: false, print: '' }
const sp = (id: string) => ({ id, photos: [`ph-${id}`] })
const album = (ids: string[]) => ({ rev: 1, folders: [], settings: { sheet: false }, spreads: ids.map(sp), cover: null }) as never

beforeEach(async () => {
  for (const k of Object.keys(handlers)) delete handlers[k]
  calls.length = 0
  exportImpl = async () => true
  const ready = store.init()
  resolveState({ view: { spreads: [], folders: [] }, photos: [], missing: [] })
  await ready
  store.apply(album(['a', 'b']))
  store.toast = null
  jobs.exporting = null
  jobs.exportOpen = false
  jobs.exportStopping = false
  jobs.exportMemo = null
  jobs.exportPrint = null
  jobs.exportPending = null
})

describe('ход экспорта в окне', () => {
  it('«Выгрузить»: окно остаётся, ход появляется со total 0 («Готовим…»)', async () => {
    jobs.exportAll()
    expect(await jobs.runExport(opt as never)).toBe(true)
    expect(jobs.exportOpen).toBe(true)
    expect(jobs.exporting).toEqual({ done: 0, total: 0 })
  })
  it('отказ Go (диалог папки закрыт) — хода нет, окно с настройками', async () => {
    exportImpl = async () => false
    jobs.exportAll()
    expect(await jobs.runExport(opt as never)).toBe(false)
    expect(jobs.exporting).toBeNull()
    expect(jobs.exportOpen).toBe(true)
  })
  it('export:progress до ответа Export — заглушка 0/0 его не затирает', async () => {
    let answer: (v: boolean) => void = () => {}
    exportImpl = () => new Promise((r) => (answer = r))
    const run = jobs.runExport(opt as never)
    handlers['export:progress'](2, 10)
    answer(true)
    await run
    expect(jobs.exporting).toEqual({ done: 2, total: 10 })
  })
  it('мгновенная выгрузка: export:done раньше ответа — ход не зависает', async () => {
    let answer: (v: boolean) => void = () => {}
    exportImpl = () => new Promise((r) => (answer = r))
    const run = jobs.runExport(opt as never)
    handlers['export:done']('C:/out', '', '', null, false)
    answer(true)
    await run
    expect(jobs.exporting).toBeNull()
  })
})

describe('крестик, шапка и меню', () => {
  it('крестик прячет окно, выгрузка идёт; «Экспорт…» открывает его снова', () => {
    jobs.exportAll()
    jobs.exporting = { done: 3, total: 12 }
    jobs.exportOpen = false // крестик/Esc (ExportDialog.close)
    expect(jobs.exporting).toEqual({ done: 3, total: 12 })
    jobs.exportAll() // кнопка в шапке или «Файл ▸ Экспорт…»
    expect(jobs.exportOpen).toBe(true)
  })
  it('повторный вызов при открытом окне хода ничего не ломает', () => {
    jobs.exporting = { done: 1, total: 4 }
    jobs.exportAll()
    jobs.exportAll()
    expect(jobs.exportOpen).toBe(true)
    expect(jobs.exporting).toEqual({ done: 1, total: 4 })
  })
  it('пустой альбом — окно не открывается', () => {
    store.apply(album([]))
    jobs.exportAll()
    expect(jobs.exportOpen).toBe(false)
  })
  it('export:done закрывает открытое окно хода; закрытое остаётся закрытым', () => {
    jobs.exportOpen = true
    jobs.exporting = { done: 12, total: 12 }
    handlers['export:done']('C:/out', '', '', null, false)
    expect(jobs.exportOpen).toBe(false)
    handlers['export:done']('C:/out', '', '', null, false)
    expect(jobs.exportOpen).toBe(false)
  })
})

describe('«Остановить»', () => {
  it('зовёт CancelExport один раз, кнопка переходит в «Останавливаем…»', () => {
    jobs.exporting = { done: 3, total: 12 }
    jobs.stopExport()
    jobs.stopExport()
    expect(calls).toEqual(['CancelExport'])
    expect(jobs.exportStopping).toBe(true)
  })
  it('состояние «Останавливаем…» переживает закрытие и повторное открытие окна', () => {
    jobs.exporting = { done: 3, total: 12 }
    jobs.exportOpen = true
    jobs.stopExport()
    jobs.exportOpen = false
    jobs.exportAll()
    expect(jobs.exportStopping).toBe(true)
  })
  it('выгрузки нет — ничего не зовёт', () => {
    jobs.stopExport()
    expect(calls).toEqual([])
    expect(jobs.exportStopping).toBe(false)
  })
  it('export:done stopped: хода нет, флаг снят, красный тост; новая выгрузка идёт', async () => {
    jobs.exportOpen = true
    jobs.exporting = { done: 5, total: 12 }
    jobs.stopExport()
    handlers['export:done']('C:/out', '', 'Экспорт остановлен: готово 5 из 12. Уже выгруженные файлы остались в папке.', null, true)
    expect(jobs.exporting).toBeNull()
    expect(jobs.exportStopping).toBe(false)
    expect(jobs.exportOpen).toBe(false)
    expect(store.toast).toMatchObject({ kind: 'error' })
    expect(await jobs.runExport(opt as never)).toBe(true)
    expect(jobs.exporting).toEqual({ done: 0, total: 0 })
  })
})
