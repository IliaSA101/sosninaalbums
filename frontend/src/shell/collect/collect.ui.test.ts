import { beforeEach, describe, expect, it, vi } from 'vitest'

// События Go, которые jobs.listen() кладёт в EventsOn.
const handlers: Record<string, (...a: unknown[]) => void> = {}
const calls: string[] = []
let collectImpl: (o: { all: boolean }) => Promise<boolean> = async () => true
let infoImpl: (all: boolean) => Promise<unknown> = async (all) => info(all)

const info = (all: boolean) => ({
  photos: 312,
  extra: all ? 928 : 0,
  own: 3,
  guides: 2,
  missing: 4,
  missingGuides: 0,
  bytes: all ? 24 : 6,
  scanning: false,
  folderName: 'Иванов_Петрова',
})

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, fn: (...a: unknown[]) => void) => {
    handlers[name] = fn
    return () => {}
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => ({
  CollectInfo: (all: boolean) => infoImpl(all),
  Collect: (o: { all: boolean }) => (calls.push('Collect'), collectImpl(o)),
  CancelCollect: async () => void calls.push('CancelCollect'),
  ShowCollected: async () => void calls.push('ShowCollected'),
  MissingNames: async () => ({}),
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)
Object.defineProperty(globalThis, 'localStorage', {
  value: { getItem: () => null, setItem: () => {} },
  configurable: true,
  writable: true,
})

const { store } = await import('../../base/core/store.svelte')
const { jobs } = await import('../../base/core/jobs.svelte')
const { collectUi } = await import('./collect.svelte')
jobs.listen()

const idle = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  calls.length = 0
  store.toast = null
  jobs.collecting = null
  jobs.collectOpen = false
  collectImpl = async () => true
  infoImpl = async (all) => info(all)
  collectUi.reset()
})

describe('открытие окна', () => {
  it('пункт меню открывает окно настроек', () => {
    jobs.collectShow()
    expect(jobs.collectOpen).toBe(true)
  })
  it('пока сбор идёт, окно повторно не открывается', () => {
    jobs.collecting = { doneFiles: 1, totalFiles: 4, doneBytes: 1, totalBytes: 4 }
    jobs.collectShow()
    expect(jobs.collectOpen).toBe(false)
  })
  it('повторный вызов при открытом окне ничего не ломает', () => {
    jobs.collectShow()
    jobs.collectShow()
    expect(jobs.collectOpen).toBe(true)
  })
})

describe('сводка', () => {
  it('обе сводки приходят сразу; текущая следует за выбором', async () => {
    await collectUi.refresh()
    expect(collectUi.album?.extra).toBe(0)
    expect(collectUi.full?.extra).toBe(928)
    expect(collectUi.info).toBe(collectUi.album)
    collectUi.setAll(true)
    expect(collectUi.info).toBe(collectUi.full)
    collectUi.setAll(false)
    expect(collectUi.info).toBe(collectUi.album)
  })
  it('запоздавший старый ответ отбрасывается', async () => {
    const waiting: ((v: unknown) => void)[] = [] // по одному на каждый вариант (с фото альбома / все)
    infoImpl = () => new Promise((r) => waiting.push(r))
    const slow = collectUi.refresh() // ответ ещё в пути
    infoImpl = async (all) => ({ ...info(all), photos: 7 })
    await collectUi.refresh() // свежий
    waiting.forEach((r) => r({ ...info(false), photos: 999 }))
    await slow
    expect(collectUi.album?.photos).toBe(7)
  })
  it('ошибка Go — окно без чисел, но работает', async () => {
    infoImpl = async () => {
      throw 'нет'
    }
    await collectUi.refresh()
    expect(collectUi.album).toBeNull()
    expect(collectUi.error).toBe('')
  })
  it('новое открытие стирает прежнее', async () => {
    await collectUi.refresh()
    collectUi.error = 'старое'
    collectUi.reset()
    expect(collectUi.album).toBeNull()
    expect(collectUi.full).toBeNull()
    expect(collectUi.error).toBe('')
  })
})

describe('имя папки в подсказке', () => {
  it('до ответа Go — заглушка, после — folderName из ответа (не сырое имя проекта)', async () => {
    expect(collectUi.folder).toBe('Альбом') // ответа ещё нет, альбом не сохранён
    infoImpl = async (all) => ({ ...info(all), folderName: 'Иванов_Петрова' })
    await collectUi.refresh()
    expect(collectUi.folder).toBe('Иванов_Петрова')
    collectUi.setAll(true)
    expect(collectUi.folder).toBe('Иванов_Петрова')
  })
  it('ошибка Go — остаётся заглушка', async () => {
    infoImpl = async () => {
      throw 'нет'
    }
    await collectUi.refresh()
    expect(collectUi.folder).toBe('Альбом')
  })
})

describe('«Собрать…»', () => {
  it('Go принял: идёт копирование, окно открыто', async () => {
    jobs.collectOpen = true
    await collectUi.go()
    expect(jobs.collecting).toEqual({ doneFiles: 0, totalFiles: 0, doneBytes: 0, totalBytes: 0 })
    expect(jobs.collectOpen).toBe(true)
    expect(collectUi.busy).toBe(false)
  })
  it('отказ пользователя (вопрос «Нет», диалог папки закрыт): окно как было, без ошибки', async () => {
    jobs.collectOpen = true
    collectImpl = async () => false
    await collectUi.go()
    expect(jobs.collecting).toBeNull()
    expect(jobs.collectOpen).toBe(true)
    expect(collectUi.error).toBe('')
  })
  it('ошибка до копирования показывается в окне, не тостом', async () => {
    jobs.collectOpen = true
    collectImpl = async () => {
      throw 'не хватает места на диске E:: нужно 6,3 ГБ, свободно 4,1 ГБ'
    }
    await collectUi.go()
    expect(collectUi.error).toBe('Не хватает места на диске E:: нужно 6,3 ГБ, свободно 4,1 ГБ')
    expect(store.toast).toBeNull()
    expect(jobs.collecting).toBeNull()
    expect(collectUi.busy).toBe(false)
  })
  it('вариант уходит в Go как есть', async () => {
    const seen: boolean[] = []
    collectImpl = async (o) => (seen.push(o.all), false)
    collectUi.setAll(true)
    await collectUi.go()
    collectUi.setAll(false)
    await collectUi.go()
    expect(seen).toEqual([true, false])
  })
  it('второй щелчок, пока открыты вопросы Go, не запускает второй сбор', async () => {
    let release: (v: boolean) => void = () => {}
    collectImpl = () => new Promise((r) => (release = r))
    const a = collectUi.go()
    await idle()
    expect(collectUi.busy).toBe(true)
    await collectUi.go()
    release(false)
    await a
    expect(calls.filter((c) => c === 'Collect')).toHaveLength(1)
  })
  it('пока идёт копирование, «Собрать…» молчит', async () => {
    jobs.collecting = { doneFiles: 1, totalFiles: 4, doneBytes: 1, totalBytes: 4 }
    await collectUi.go()
    expect(calls).not.toContain('Collect')
  })
  it('сбор закончился раньше ответа Collect (пустой альбом): полоса не зависает', async () => {
    collectImpl = async () => {
      handlers['collect:done']('E:\\Иванов', '', 'Проект собран: 1 файл', false)
      return true
    }
    await collectUi.go()
    expect(jobs.collecting).toBeNull()
    expect(jobs.collectOpen).toBe(false)
  })
})

describe('закрытие и «Остановить»', () => {
  it('в настройках Esc/✕/Отмена закрывают', () => {
    jobs.collectOpen = true
    collectUi.close()
    expect(jobs.collectOpen).toBe(false)
  })
  it('во время копирования окно не закрывается', () => {
    jobs.collectOpen = true
    jobs.collecting = { doneFiles: 1, totalFiles: 4, doneBytes: 1, totalBytes: 4 }
    collectUi.close()
    expect(jobs.collectOpen).toBe(true)
  })
  it('«Остановить» зовёт CancelCollect, окно закрывает только collect:done', async () => {
    jobs.collectOpen = true
    jobs.collecting = { doneFiles: 1, totalFiles: 4, doneBytes: 1, totalBytes: 4 }
    collectUi.stop()
    await idle()
    expect(calls).toEqual(['CancelCollect'])
    expect(jobs.collectOpen).toBe(true)
  })
})

describe('события Go', () => {
  it('collect:progress обновляет ход', () => {
    handlers['collect:progress'](120, 312, 2400, 6200)
    expect(jobs.collecting).toEqual({ doneFiles: 120, totalFiles: 312, doneBytes: 2400, totalBytes: 6200 })
  })
  it('готово: окно закрыто, тост с «Открыть папку» открывает папку последнего сбора', async () => {
    jobs.collectOpen = true
    jobs.collecting = { doneFiles: 312, totalFiles: 312, doneBytes: 1, totalBytes: 1 }
    handlers['collect:done']('E:\\Иванов', '', 'Проект собран: 312 файлов, 6,2 ГБ', false)
    expect(jobs.collecting).toBeNull()
    expect(jobs.collectOpen).toBe(false)
    expect(store.toast?.text).toBe('Проект собран: 312 файлов, 6,2 ГБ')
    expect(store.toast?.kind).toBe('info')
    expect(store.toast?.action?.label).toBe('Открыть папку')
    store.toast?.action?.run()
    await idle()
    expect(calls).toEqual(['ShowCollected'])
  })
  it('остановлено: красный тост без кнопки, окно закрыто', () => {
    jobs.collectOpen = true
    jobs.collecting = { doneFiles: 120, totalFiles: 312, doneBytes: 1, totalBytes: 1 }
    handlers['collect:done']('E:\\Иванов', '', 'Сбор остановлен: скопировано 120 из 312.', true)
    expect(jobs.collectOpen).toBe(false)
    expect(store.toast?.kind).toBe('error')
    expect(store.toast?.action).toBeUndefined()
  })
  it('ошибка копирования: красный тост с фразой Go', () => {
    jobs.collectOpen = true
    jobs.collecting = { doneFiles: 120, totalFiles: 312, doneBytes: 1, totalBytes: 1 }
    handlers['collect:done']('E:\\Иванов', 'Диск заполнен: скопировано 120 из 312. Альбом в папку не записан.', '', false)
    expect(jobs.collecting).toBeNull()
    expect(store.toast?.kind).toBe('error')
    expect(store.toast?.text).toContain('Диск заполнен')
  })
  it('после итога сбор можно сразу повторить', async () => {
    handlers['collect:done']('E:\\Иванов', '', 'ok', false)
    jobs.collectShow()
    expect(jobs.collectOpen).toBe(true)
    jobs.collectOpen = true
    await collectUi.go()
    expect(jobs.collecting).not.toBeNull()
  })
})
