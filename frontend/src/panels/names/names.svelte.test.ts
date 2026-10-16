import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Go и store подменены: проверяем, что окно просит у Go, и что делает с ответом.
const view = { rev: 8, spreads: [{ id: 's1', photos: ['a', 'b'] }] }
const live = $state({ rev: 7, spreads: [{ id: 's1', photos: ['a', 'b'] }] }) // реактивный вид: тест эффекта окна
vi.mock('../../base/core/store.svelte', () => ({
  store: {
    get view() {
      return live
    },
    spread: { id: 's1' },
    apply: vi.fn(),
    notify: vi.fn(),
  },
  Api: {
    PasteNames: vi.fn(),
    PreviewNames: vi.fn(),
    PickNamesFile: vi.fn(),
  },
}))
vi.mock('../../editor/text/edit.svelte', () => ({ edit: { idle: vi.fn(async () => {}) } }))

import { Api, store } from '../../base/core/store.svelte'
import { namesUi, PREVIEW_DELAY } from './names.svelte'

const plan = (over: object = {}) => ({
  rows: [{ photo: 'a', tag: '', no: 1, line: 1, who: { last: 'Иванов', first: 'Пётр' } }],
  left: [],
  extra: 0,
  seats: 2,
  named: 1,
  missing: 1,
  cols: null,
  ignored: [],
  byFile: false,
  order: true,
  teachers: 0,
  pupils: 1,
  needCaption: false,
  ...over,
})

beforeEach(() => {
  vi.useFakeTimers()
  vi.clearAllMocks()
  vi.mocked(Api.PreviewNames).mockResolvedValue(plan() as never)
  namesUi.close()
  namesUi.firstFirst = false
})

afterEach(() => {
  vi.useRealTimers()
})

describe('namesUi: открытие', () => {
  it('show открывает окно на листе в режиме «Список» с флажком подписи', async () => {
    await namesUi.show()
    expect(namesUi.open).toBe(true)
    expect(namesUi.spreadId).toBe('s1')
    expect(namesUi.from).toBe('list')
    expect(namesUi.caption).toBe(true)
  })

  it('нет такого листа — окно не открывается', async () => {
    await namesUi.show('нет')
    expect(namesUi.open).toBe(false)
  })

  it('первый предпросмотр после открытия — без паузы', async () => {
    await namesUi.show()
    namesUi.refresh()
    await vi.advanceTimersByTimeAsync(0)
    expect(Api.PreviewNames).toHaveBeenCalledTimes(1)
    expect(namesUi.plan?.seats).toBe(2)
  })
})

describe('namesUi: смена rev', () => {
  // Сам $effect в vitest (серверная сборка Svelte) не бежит: проверяем, что refresh читает rev и после правки листа
  // заказывает новый запрос с теми же входами; подписку на rev держит `void store.view.rev` в refresh.
  it('правка проекта (например, «Настроить виньетку…») — refresh перезапрашивает предпросмотр', async () => {
    await namesUi.show()
    namesUi.refresh()
    await vi.advanceTimersByTimeAsync(0)
    vi.mocked(Api.PreviewNames).mockResolvedValue(plan({ seats: 5 }) as never)
    live.rev = 8
    namesUi.refresh()
    await vi.advanceTimersByTimeAsync(PREVIEW_DELAY)
    expect(Api.PreviewNames).toHaveBeenCalledTimes(2)
    expect(namesUi.plan?.seats).toBe(5)
    live.rev = 7
  })
})

describe('namesUi: файл', () => {
  const file = (over: object = {}) => ({
    text: 'Фамилия\tИмя\nИванов\tПётр',
    file: 'Класс.xlsx',
    sheet: 'Класс',
    sheets: 2,
    rows: 2,
    cancelled: false,
    ...over,
  })

  it('файл кладёт текст в поле и запоминает сведения', async () => {
    vi.mocked(Api.PickNamesFile).mockResolvedValue(file() as never)
    await namesUi.show()
    expect(await namesUi.loadFile()).toBe(true)
    expect(namesUi.text).toBe('Фамилия\tИмя\nИванов\tПётр')
    expect(namesUi.file?.file).toBe('Класс.xlsx')
  })

  it('отказ от диалога не трогает текст', async () => {
    vi.mocked(Api.PickNamesFile).mockResolvedValue(file({ text: '', cancelled: true }) as never)
    await namesUi.show()
    namesUi.text = 'мой текст'
    expect(await namesUi.loadFile()).toBe(false)
    expect(namesUi.text).toBe('мой текст')
    expect(namesUi.file).toBeNull()
    expect(namesUi.error).toBe('')
  })

  it('ошибка файла — в окне, текст прежний', async () => {
    vi.mocked(Api.PickNamesFile).mockRejectedValue('это .xls или файл с паролем')
    await namesUi.show()
    namesUi.text = 'мой текст'
    expect(await namesUi.loadFile()).toBe(false)
    expect(namesUi.error).toBe('Это .xls или файл с паролем')
    expect(namesUi.text).toBe('мой текст')
  })
})

describe('namesUi: предпросмотр', () => {
  it('зовёт Go не чаще раза в 150 мс', async () => {
    await namesUi.show()
    namesUi.refresh()
    await vi.advanceTimersByTimeAsync(0)
    vi.mocked(Api.PreviewNames).mockClear()
    for (const t of ['И', 'Ив', 'Ива']) {
      namesUi.text = t
      namesUi.refresh()
      await vi.advanceTimersByTimeAsync(PREVIEW_DELAY - 50)
    }
    expect(Api.PreviewNames).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(50)
    expect(Api.PreviewNames).toHaveBeenCalledTimes(1)
    expect(Api.PreviewNames).toHaveBeenCalledWith('s1', { from: 'list', text: 'Ива', firstFirst: false, caption: true })
  })

  it('ответ на устаревший ввод отбрасывается', async () => {
    await namesUi.show()
    let slow!: (p: unknown) => void
    vi.mocked(Api.PreviewNames)
      .mockImplementationOnce(() => new Promise((r) => (slow = r)) as never)
      .mockResolvedValueOnce(plan({ seats: 9 }) as never)
    namesUi.refresh()
    await vi.advanceTimersByTimeAsync(0) // первый запрос ушёл и висит
    namesUi.text = 'новый'
    namesUi.refresh()
    await vi.advanceTimersByTimeAsync(PREVIEW_DELAY) // второй ответил
    expect(namesUi.plan?.seats).toBe(9)
    slow(plan({ seats: 1 })) // запоздавший ответ на старый ввод
    await vi.advanceTimersByTimeAsync(0)
    expect(namesUi.plan?.seats).toBe(9)
  })

  it('ошибка Go — в окне, вставить нельзя', async () => {
    vi.mocked(Api.PreviewNames).mockRejectedValue('список имён слишком длинный')
    await namesUi.show()
    namesUi.refresh()
    await vi.advanceTimersByTimeAsync(0)
    expect(namesUi.planError).toBe('Список имён слишком длинный')
    expect(namesUi.plan).toBeNull()
    expect(namesUi.canPaste).toBe(false)
  })

  it('«Имена файлов» не шлёт текст поля', async () => {
    await namesUi.show()
    namesUi.text = 'Иванов Пётр'
    namesUi.from = 'files'
    expect(namesUi.opts()).toEqual({ from: 'files', text: '', firstFirst: false, caption: true })
  })

  it('«Имя первым» серое, когда порядок задают столбцы', async () => {
    await namesUi.show()
    namesUi.refresh()
    await vi.advanceTimersByTimeAsync(0)
    expect(namesUi.orderOff).toBe(false)
    vi.mocked(Api.PreviewNames).mockResolvedValue(plan({ order: false, cols: ['Фамилия', 'Имя'] }) as never)
    namesUi.text = 'x'
    namesUi.refresh()
    await vi.advanceTimersByTimeAsync(PREVIEW_DELAY)
    expect(namesUi.orderOff).toBe(true)
    namesUi.from = 'files'
    expect(namesUi.orderOff).toBe(false) // в режиме файлов порядок действует всегда
  })

  it('вставить можно, только если хоть одно место получит имя', async () => {
    vi.mocked(Api.PreviewNames).mockResolvedValue(plan({ rows: [{ photo: 'a', tag: '', no: 1, line: 0, who: null }] }) as never)
    await namesUi.show()
    namesUi.refresh()
    await vi.advanceTimersByTimeAsync(0)
    expect(namesUi.canPaste).toBe(false)
  })
})

describe('namesUi: вставка', () => {
  it('apply отдаёт Go NamesOpts, берёт вид, закрывает окно и сообщает итог', async () => {
    vi.mocked(Api.PasteNames).mockResolvedValue({ view, extra: 1, missing: 2, named: 23, seats: 25 } as never)
    await namesUi.show()
    namesUi.text = 'Иванов Пётр'
    namesUi.firstFirst = true
    namesUi.caption = false
    expect(await namesUi.apply()).toBe(true)
    expect(Api.PasteNames).toHaveBeenCalledWith('s1', { from: 'list', text: 'Иванов Пётр', firstFirst: true, caption: false })
    expect(store.apply).toHaveBeenCalledWith(view)
    expect(store.notify).toHaveBeenCalledWith('Имена вставлены. Подписано: 23 из 25, лишних строк: 1')
    expect(namesUi.open).toBe(false)
  })

  it('те же имена (rev не изменился) — тост «Без изменений», а не «вставлены»', async () => {
    vi.mocked(Api.PasteNames).mockResolvedValue({ view: { ...view, rev: 7 }, extra: 0, missing: 0, named: 2, seats: 2 } as never)
    await namesUi.show()
    expect(await namesUi.apply()).toBe(true)
    expect(store.notify).toHaveBeenCalledTimes(1)
    expect(store.notify).toHaveBeenCalledWith('Без изменений')
    expect(namesUi.open).toBe(false)
  })

  it('режим «Имена файлов» вставляет без текста', async () => {
    vi.mocked(Api.PasteNames).mockResolvedValue({ view, extra: 0, missing: 0, named: 2, seats: 2 } as never)
    await namesUi.show()
    namesUi.text = 'забытый текст'
    namesUi.from = 'files'
    await namesUi.apply()
    expect(Api.PasteNames).toHaveBeenCalledWith('s1', { from: 'files', text: '', firstFirst: false, caption: true })
  })

  it('ошибка Go остаётся в окне, правки нет', async () => {
    vi.mocked(Api.PasteNames).mockRejectedValue('список имён слишком длинный')
    await namesUi.show()
    expect(await namesUi.apply()).toBe(false)
    expect(namesUi.open).toBe(true)
    expect(namesUi.error).toBe('Список имён слишком длинный')
    expect(store.apply).not.toHaveBeenCalled()
  })
})
