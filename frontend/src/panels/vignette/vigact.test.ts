import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go и store подменены: проверяем, что уходит в Go и что делается с ответом.
vi.mock('../../base/core/store.svelte', () => ({
  store: {
    view: { rev: 5, spreads: [{ id: 'a' }, { id: 'b' }] },
    apply: vi.fn(),
    notify: vi.fn(),
    undoToast: vi.fn(),
    run: vi.fn(async () => true),
    go: vi.fn(),
  },
  Api: {
    SetVignette: vi.fn(),
    AddVignette: vi.fn(async () => ({})),
    ClearVignette: vi.fn(async () => ({})),
    SortVignette: vi.fn(async () => ({})),
    VignetteChoices: vi.fn(),
    PreviewVignette: vi.fn(),
    SetupVignette: vi.fn(),
  },
}))

import { Api, store } from '../../base/core/store.svelte'
import { addVignette, applySetup, applyVignette, clearVignette, makeVignette, previewVignette, sortVignette, stepVignette } from './vigact'
import { startForm } from './vignette'

beforeEach(() => {
  vi.clearAllMocks()
  store.view.rev = 5
  store.view.spreads = [{ id: 'a' }, { id: 'b' }] as never
  vi.mocked(store.run).mockResolvedValue(true)
})

describe('applyVignette', () => {
  it('параметры уходят в Go, вид применяется; left > 0 — тост «Убрано в ленту» с «Вернуть»', async () => {
    const view = { rev: 6 }
    vi.mocked(Api.SetVignette).mockResolvedValue({ view, left: 5 } as never)
    expect(await applyVignette('a', { tpl: 't', teachers: 2, pupils: 20, ratio: '4:5', ttext: '' })).toBe(true)
    expect(Api.SetVignette).toHaveBeenCalledWith('a', { tpl: 't', teachers: 2, pupils: 20, ratio: '4:5', ttext: '', head: '' })
    expect(store.apply).toHaveBeenCalledWith(view)
    expect(store.undoToast).toHaveBeenCalledWith('Убрано в ленту: 5')
  })

  it('left = 0 — тоста нет', async () => {
    vi.mocked(Api.SetVignette).mockResolvedValue({ view: {}, left: 0 } as never)
    await applyVignette('a', startForm(0))
    expect(store.undoToast).not.toHaveBeenCalled()
  })

  it('ошибка Go (больше 64 мест) — тост-ошибка, вид не меняется', async () => {
    vi.mocked(Api.SetVignette).mockRejectedValue('не больше 64 мест на развороте')
    expect(await applyVignette('a', startForm(0))).toBe(false)
    expect(store.apply).not.toHaveBeenCalled()
    expect(store.notify).toHaveBeenCalledWith('не больше 64 мест на развороте', 'error')
  })

  it('makeVignette: учеников по числу фото', async () => {
    vi.mocked(Api.SetVignette).mockResolvedValue({ view: {}, left: 0 } as never)
    await makeVignette({ id: 'a', photos: ['1', '2', '3'] } as never)
    expect(Api.SetVignette).toHaveBeenCalledWith('a', expect.objectContaining({ pupils: 3, teachers: 1 }))
  })

  it('makeVignette: тост с «Вернуть» всегда, «Вернуть» ведёт на этот разворот', async () => {
    vi.mocked(Api.SetVignette).mockResolvedValue({ view: {}, left: 0 } as never)
    expect(await makeVignette({ id: 'b', photos: [] } as never)).toBe(true)
    expect(store.undoToast).toHaveBeenCalledTimes(1)
    expect(store.undoToast).toHaveBeenCalledWith('Разворот стал виньеткой', 'b')
  })

  it('makeVignette: не вставшие фото — в том же тосте; у проекта-листа — «Лист»', async () => {
    vi.mocked(Api.SetVignette).mockResolvedValue({ view: {}, left: 2 } as never)
    await makeVignette({ id: 'a', photos: [] } as never)
    expect(store.undoToast).toHaveBeenCalledWith('Разворот стал виньеткой. Убрано в ленту: 2', 'a')
    vi.mocked(store.undoToast).mockClear()
    ;(store.view as { settings?: unknown }).settings = { sheet: true }
    try {
      await makeVignette({ id: 'a', photos: [] } as never)
      expect(store.undoToast).toHaveBeenCalledWith('Лист стал виньеткой. Убрано в ленту: 2', 'a')
    } finally {
      delete (store.view as { settings?: unknown }).settings
    }
  })

  it('makeVignette: Go отказал — тоста «стал виньеткой» нет', async () => {
    vi.mocked(Api.SetVignette).mockRejectedValue('не больше 64 мест на развороте')
    expect(await makeVignette({ id: 'a', photos: [] } as never)).toBe(false)
    expect(store.undoToast).not.toHaveBeenCalled()
  })
})

describe('addVignette', () => {
  it('новый лист открывается', async () => {
    vi.mocked(store.run).mockImplementation(async () => {
      store.view.spreads = [{ id: 'a' }, { id: 'new' }, { id: 'b' }] as never
      return true
    })
    await addVignette(1)
    expect(Api.AddVignette).toHaveBeenCalledWith(1)
    expect(store.go).toHaveBeenCalledWith(1)
  })

  it('Go отказал — на новый не переходим', async () => {
    vi.mocked(store.run).mockResolvedValue(false)
    await addVignette(2)
    expect(store.go).not.toHaveBeenCalled()
  })
})

describe('clearVignette и sortVignette', () => {
  it('«Обычный разворот» зовёт ClearVignette', async () => {
    await clearVignette('a')
    expect(Api.ClearVignette).toHaveBeenCalledWith('a')
  })

  it('сортировка: порядок не изменился (rev тот же) — объясняем тостом', async () => {
    await sortVignette('a')
    expect(Api.SortVignette).toHaveBeenCalledWith('a')
    expect(store.notify).toHaveBeenCalledWith('Ученики уже по фамилии: менять нечего')
  })

  it('сортировка изменила проект (rev вырос) — без тоста', async () => {
    vi.mocked(store.run).mockImplementation(async () => {
      store.view.rev = 6
      return true
    })
    await sortVignette('a')
    expect(store.notify).not.toHaveBeenCalled()
  })
})

describe('stepVignette', () => {
  const sp = (tpl = 'b') =>
    ({
      id: 'a',
      kind: 'vignette',
      vig: { tpl, teachers: 2, pupils: 20, ratio: '4:5', ttext: 'T', places: 22, max: 64, group: false },
    }) as never
  const cards = [
    { id: 'a', over: false },
    { id: 'b', over: false },
    { id: 'c', over: true },
    { id: 'd', over: false },
  ]
  beforeEach(() => {
    vi.mocked(Api.VignetteChoices).mockResolvedValue(cards as never)
    vi.mocked(Api.SetVignette).mockResolvedValue({ view: {}, left: 0 } as never)
  })

  it('следующий шаблон с числами листа, остальное не трогает', async () => {
    expect(await stepVignette(sp(), 1)).toBe(true)
    expect(Api.VignetteChoices).toHaveBeenCalledWith('a', 2, 20, '4:5')
    expect(Api.SetVignette).toHaveBeenCalledWith('a', { tpl: 'd', teachers: 2, pupils: 20, ratio: '4:5', ttext: 'T', head: '' })
  })

  it('свой шаблон с другим портретом ставит и свой портрет (как окно), каталожный — портрет листа', async () => {
    vi.mocked(Api.VignetteChoices).mockResolvedValue([
      { id: 'b', over: false },
      { id: 'my:x', over: false, own: true, ratio: '2:3' },
      { id: 'd', over: false },
    ] as never)
    await stepVignette(sp(), 1)
    expect(Api.SetVignette).toHaveBeenLastCalledWith('a', expect.objectContaining({ tpl: 'my:x', ratio: '2:3', pupils: 20 }))
    await stepVignette(sp('my:x'), 1)
    expect(Api.SetVignette).toHaveBeenLastCalledWith('a', expect.objectContaining({ tpl: 'd', ratio: '4:5' }))
  })

  it('фильтр видов ▦: Пробел идёт по видимым, скрытые вид пропускает', async () => {
    vi.mocked(Api.VignetteChoices).mockResolvedValue([
      { id: 'a', over: false, family: 'side' },
      { id: 'b', over: false, family: 'side' },
      { id: 'c', over: false, family: 'big' },
      { id: 'd', over: false, family: 'side' },
    ] as never)
    await stepVignette(sp('b'), 1, 'side')
    expect(Api.SetVignette).toHaveBeenLastCalledWith('a', expect.objectContaining({ tpl: 'd' }))
    await stepVignette(sp('d'), 1, 'side')
    expect(Api.SetVignette).toHaveBeenLastCalledWith('a', expect.objectContaining({ tpl: 'a' }))
    await stepVignette(sp('b'), 1, 'all')
    expect(Api.SetVignette).toHaveBeenLastCalledWith('a', expect.objectContaining({ tpl: 'c' }))
  })

  it('назад по кругу; фото, не вставшие в места, — тост «Убрано в ленту»', async () => {
    vi.mocked(Api.SetVignette).mockResolvedValue({ view: {}, left: 2 } as never)
    await stepVignette(sp('a'), -1)
    expect(Api.SetVignette).toHaveBeenCalledWith('a', expect.objectContaining({ tpl: 'd' }))
    expect(store.undoToast).toHaveBeenCalledWith('Убрано в ленту: 2')
  })

  it('других шаблонов нет — тост, Go не зовётся', async () => {
    vi.mocked(Api.VignetteChoices).mockResolvedValue([{ id: 'b', over: false }] as never)
    expect(await stepVignette(sp(), 1)).toBe(false)
    expect(Api.SetVignette).not.toHaveBeenCalled()
    expect(store.notify).toHaveBeenCalledWith('Других шаблонов для этих чисел нет')
  })

  it('не виньетка — ничего', async () => {
    expect(await stepVignette({ id: 'a' } as never, 1)).toBe(false)
    expect(Api.VignetteChoices).not.toHaveBeenCalled()
  })

  it('ошибка Go — тост-ошибка', async () => {
    vi.mocked(Api.VignetteChoices).mockRejectedValue('сбой')
    expect(await stepVignette(sp(), 1)).toBe(false)
    expect(store.notify).toHaveBeenCalledWith('сбой', 'error')
  })
})

describe('окно «Настроить виньетку»: вызовы Go', () => {
  const setup = { vig: { tpl: 't', teachers: 1, pupils: 20, ratio: '3:4', ttext: '', head: '' }, vars: { title: 'Гимназия' } }

  it('previewVignette: id листа и форма уходят в Go, ответ возвращается как есть', async () => {
    const r = { spread: { id: 'a' }, left: 2 }
    vi.mocked(Api.PreviewVignette).mockResolvedValue(r as never)
    expect(await previewVignette('a', setup)).toBe(r)
    expect(Api.PreviewVignette).toHaveBeenCalledWith('a', setup)
  })

  it('applySetup: вид применяется, left > 0 — тост «Убрано в ленту» с «Вернуть»', async () => {
    const view = { rev: 7 }
    vi.mocked(Api.SetupVignette).mockResolvedValue({ view, left: 3 } as never)
    expect(await applySetup('a', setup)).toBe(true)
    expect(Api.SetupVignette).toHaveBeenCalledWith('a', setup)
    expect(store.apply).toHaveBeenCalledWith(view)
    expect(store.undoToast).toHaveBeenCalledWith('Убрано в ленту: 3')
  })

  it('applySetup: ошибка Go — тост-ошибка, false, вид не меняется', async () => {
    vi.mocked(Api.SetupVignette).mockRejectedValue('не больше 64 мест на развороте')
    expect(await applySetup('a', setup)).toBe(false)
    expect(store.apply).not.toHaveBeenCalled()
    expect(store.notify).toHaveBeenCalledWith('не больше 64 мест на развороте', 'error')
  })
})
