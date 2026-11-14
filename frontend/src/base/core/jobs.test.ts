import { beforeEach, describe, expect, it, vi } from 'vitest'

// Обработчики событий Go, которые store.init() кладёт в EventsOn.
const handlers: Record<string, (...a: unknown[]) => void> = {}
let resolveState: (st: unknown) => void = () => {}
let resolveCollect: (started: boolean) => void = () => {}

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, fn: (...a: unknown[]) => void) => {
    handlers[name] = fn
    return () => {}
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => ({
  GetState: () => new Promise((r) => (resolveState = r)),
  MissingNames: async () => ({}),
  Collect: () => new Promise<boolean>((r) => (resolveCollect = r)),
}))

vi.stubGlobal('window', globalThis) // notify ставит таймер на window
const { store } = await import('./store.svelte')
const { jobs, exportShut } = await import('./jobs.svelte')

const view = { spreads: [], folders: [] }
const sp = (id: string) => ({ id, photos: [`ph-${id}`] })
const load = (o: { spreads?: string[]; cover?: boolean } = {}) => {
  store.onCover = false
  store.apply({
    rev: 1,
    folders: [],
    settings: { sheet: false },
    spreads: (o.spreads ?? ['a', 'b', 'c']).map(sp),
    cover: o.cover ? { sheet: sp('cover'), spine: 14 } : null,
  } as never)
  store.go(0)
}

beforeEach(() => {
  for (const k of Object.keys(handlers)) delete handlers[k]
  store.toast = null
  jobs.exporting = null
  jobs.exportOpen = false
})

describe('jobs.exportAll', () => {
  it('одна обложка без разворотов выгружается', () => {
    load({ spreads: [], cover: true })
    jobs.exportAll()
    expect(jobs.exportOpen).toBe(true)
  })
  it('пустой альбом — окно не открывается', () => {
    load({ spreads: [] })
    jobs.exportAll()
    expect(jobs.exportOpen).toBe(false)
    expect(store.toast?.text).toBe('Выгружать нечего: в альбоме нет разворотов')
  })
  it('экспорт идёт, а листов не осталось — окно хода всё равно открывается («Остановить» достать можно)', () => {
    load({ spreads: [] })
    jobs.exporting = { done: 1, total: 4 }
    jobs.exportAll()
    expect(jobs.exportOpen).toBe(true)
  })
  it('exportShut: закрыт только при нуле листов и без выгрузки', () => {
    expect(exportShut(0, false)).toBe(true)
    expect(exportShut(0, true)).toBe(false)
    expect(exportShut(1, false)).toBe(false)
    expect(exportShut(5, true)).toBe(false)
  })
  it('экспорт уже идёт — окно открывается и показывает ход (073)', () => {
    load()
    jobs.exporting = { done: 1, total: 4 }
    jobs.exportAll()
    expect(jobs.exportOpen).toBe(true)
  })
})

describe('export:done (047)', () => {
  // Регистрирует обработчики; init дозавершаем, чтобы он не висел.
  async function ready() {
    const done = store.init()
    resolveState({ view, photos: [], missing: [] })
    await done
    store.toast = null
    jobs.exportMemo = null
    jobs.exportNote = ''
  }
  const memo = ['Фотокнига 30×30', 'Папка: Иванов — 01…20.jpg']

  it('с памяткой — окно «Готово для Фотосферы», тоста нет; сводка цвета остаётся', async () => {
    await ready()
    handlers['export:done']('C:/out/Иванов', '', 'Пересчитано в sRGB: 2 фото (Adobe RGB — 2).', memo)
    expect(jobs.exportMemo).toEqual(memo)
    expect(jobs.exportNote).toBe('Пересчитано в sRGB: 2 фото (Adobe RGB — 2).')
    expect(store.toast).toBeNull()
  })

  it('без памятки (и с пустой) — прежний тост', async () => {
    await ready()
    handlers['export:done']('C:/out', '', '', null)
    expect(jobs.exportMemo).toBeNull()
    expect(store.toast).toMatchObject({ kind: 'info', text: 'Выгрузка готова' })
    store.toast = null
    handlers['export:done']('C:/out', '', '', [])
    expect(jobs.exportMemo).toBeNull()
    expect((store.toast as { text: string } | null)?.text).toBe('Выгрузка готова')
  })

  it('ошибка — красный тост, окна нет, даже если памятка пришла', async () => {
    await ready()
    handlers['export:done']('C:/out', 'диск полон', '', memo)
    expect(jobs.exportMemo).toBeNull()
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Экспорт не удался: диск полон' })
  })

  it('export:done stopped — красный тост из note, без «Открыть папку», окно хода закрыто (073)', async () => {
    await ready()
    jobs.exportOpen = true
    jobs.exporting = { done: 5, total: 12 }
    const note = 'Экспорт остановлен: готово 5 из 12. Уже выгруженные файлы остались в папке.'
    handlers['export:done']('C:/out', '', note, null, true)
    expect(jobs.exporting).toBeNull()
    expect(jobs.exportOpen).toBe(false)
    expect(jobs.exportMemo).toBeNull()
    expect(store.toast).toMatchObject({ kind: 'error', text: note })
    expect((store.toast as { action?: unknown }).action).toBeUndefined()
  })

  it('export:done stopped без note — запасной текст', async () => {
    await ready()
    handlers['export:done']('C:/out', '', '', null, true)
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Экспорт остановлен' })
  })

  it('export:progress ведёт счётчик, export:done его снимает', async () => {
    await ready()
    handlers['export:progress'](2, 10)
    expect(jobs.exporting).toEqual({ done: 2, total: 10 })
    handlers['export:done']('C:/out', '', '', null)
    expect(jobs.exporting).toBeNull()
  })
})

describe('jobs.runCollect (069)', () => {
  async function ready() {
    const done = store.init()
    resolveState({ view, photos: [], missing: [] })
    await done
    jobs.collecting = null
  }
  it('collect:progress до ответа Collect — заглушка 0/0 его не затирает', async () => {
    await ready()
    const run = jobs.runCollect(false)
    handlers['collect:progress'](3, 10, 300, 1000)
    resolveCollect(true)
    await run
    expect(jobs.collecting).toEqual({ doneFiles: 3, totalFiles: 10, doneBytes: 300, totalBytes: 1000 })
    expect(jobs.collectOpen).toBe(true)
  })
  it('прогресса ещё не было — полоса стартует с нулей', async () => {
    await ready()
    const run = jobs.runCollect(true)
    resolveCollect(true)
    await run
    expect(jobs.collecting).toEqual({ doneFiles: 0, totalFiles: 0, doneBytes: 0, totalBytes: 0 })
  })
})
