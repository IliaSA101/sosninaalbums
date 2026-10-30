import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Тост правки агента (149-30): «Клиент · что сделано», «Вернуть» только при той же ревизии, переход к go,
// одна правка — один тост. Go подменён: Undo отвечает видом на ревизию меньше.
const ev = vi.hoisted(() => ({ on: new Map<string, (d: unknown) => void>() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, cb: (d: unknown) => void) => {
    ev.on.set(name, cb)
    return () => ev.on.delete(name)
  },
}))
const api = vi.hoisted(() => ({ Undo: vi.fn(), AgentsInfo: vi.fn(), AgentOpenPrev: vi.fn() }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.stubGlobal('window', globalThis)

// Фокус окна (тесты идут в node): document.hasFocus и слушатели focus на window подменены.
let focused = true
const focusFns = new Set<() => void>()
vi.stubGlobal('document', { hasFocus: () => focused })
vi.stubGlobal('addEventListener', (n: string, cb: () => void) => {
  if (n === 'focus') focusFns.add(cb)
})
vi.stubGlobal('removeEventListener', (n: string, cb: () => void) => {
  if (n === 'focus') focusFns.delete(cb)
})
const gainFocus = () => {
  focused = true
  for (const f of [...focusFns]) f()
}

const { agentEdited, agentDenied, resetEdited } = await import('./agentedited')
const { agents } = await import('./agents.svelte')
const { store } = await import('../../base/core/store.svelte')
type Toast = NonNullable<typeof store.toast>

const view = (rev: number, ids = ['s1', 's2', 's3'], cover = false) => ({
  rev,
  folders: [],
  spreads: ids.map((id) => ({ id, photos: [] as string[] })),
  cover: cover ? { sheet: { id: 'cover', photos: [] as string[] }, spine: 14 } : null,
})
const edited = (
  o: Partial<{ client: string; text: string; rev: number; go: string; prev: boolean; quiet: boolean; path: string }> = {},
) => ({
  client: 'Claude Code',
  text: 'Собрано разворотов: 24',
  rev: 5,
  go: '',
  ...o,
})
const shownRev = (rev: number) => {
  store.apply(view(rev) as never)
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date(2026, 9, 29, 12, 0, 0))
  api.Undo.mockReset()
  api.Undo.mockResolvedValue(view(4))
  api.AgentsInfo.mockReset()
  api.AgentsInfo.mockResolvedValue({})
  api.AgentOpenPrev.mockReset()
  api.AgentOpenPrev.mockResolvedValue({ view: view(6) })
  focused = true
  focusFns.clear()
  resetEdited()
  store.toast = null
  store.onCover = false
  shownRev(5)
  store.goId('s1')
})
afterEach(() => {
  vi.useRealTimers()
  store.toast = null
})

describe('тост правки агента', () => {
  it('«Клиент · текст Go» с кнопкой «Вернуть»; тост привязан к ревизии правки', () => {
    agentEdited(edited())
    expect(store.toast?.text).toBe('Claude Code · Собрано разворотов: 24')
    expect(store.toast?.action?.label).toBe('Вернуть')
    expect(store.toast?.rev).toBe(5)
    expect(store.toast?.kind).toBe('info')
  })

  it('«Вернуть» при той же ревизии → Undo', async () => {
    agentEdited(edited())
    await store.toast?.action?.run()
    expect(api.Undo).toHaveBeenCalledTimes(1)
    expect(store.view.rev).toBe(4)
  })

  it('«Вернуть» после чужой правки: Undo не зовём, тост подсказывает «Отменить»', async () => {
    agentEdited(edited())
    const act = store.toast?.action
    store.view = { ...store.view, rev: 6 }
    await act?.run()
    expect(api.Undo).not.toHaveBeenCalled()
    expect(store.toast?.text).toMatch(/Уже были другие изменения — используйте «Отменить»/)
    expect(store.toast?.action).toBeUndefined()
  })

  it('любая другая смена ревизии гасит тост сама', () => {
    agentEdited(edited())
    shownRev(6)
    expect(store.toast).toBeNull()
  })

  it('вид той же ревизии (событие view догоняет agents:edited) тост не гасит', () => {
    shownRev(4)
    agentEdited(edited({ rev: 5 }))
    shownRev(5)
    expect(store.toast?.text).toBe('Claude Code · Собрано разворотов: 24')
  })

  it('ревизия уже дальше правки агента — тост без «Вернуть»: откатило бы не её', () => {
    shownRev(7)
    agentEdited(edited({ rev: 5 }))
    expect(store.toast?.text).toBe('Claude Code · Собрано разворотов: 24')
    expect(store.toast?.action).toBeUndefined()
  })

  it('«Вернуть» ждёт очередь удаления', async () => {
    let free: () => void = () => {}
    const gate = new Promise<void>((r) => (free = r))
    const q = vi.spyOn(store, 'delQueue', 'get').mockReturnValue(gate)
    agentEdited(edited())
    const run = store.toast?.action?.run()
    await Promise.resolve()
    expect(api.Undo).not.toHaveBeenCalled()
    free()
    await run
    expect(api.Undo).toHaveBeenCalledTimes(1)
    q.mockRestore()
  })
})

describe('переход к go', () => {
  it('go — разворот альбома: холст переходит на него', () => {
    agentEdited(edited({ go: 's3' }))
    expect(store.currentId).toBe('s3')
  })

  it('go пуст — остаёмся, где были', () => {
    store.goId('s2')
    agentEdited(edited({ go: '' }))
    expect(store.currentId).toBe('s2')
  })

  it('разворот ещё не дошёл событием view: переход, как только он появился', () => {
    agentEdited(edited({ go: 's4' }))
    expect(store.currentId).toBe('s1')
    store.apply(view(5, ['s1', 's2', 's3', 's4']) as never)
    vi.advanceTimersByTime(200)
    expect(store.currentId).toBe('s4')
  })

  it('go — cover: холст переходит к обложке, как к развороту', () => {
    store.apply(view(5, ['s1', 's2', 's3'], true) as never)
    store.goId('s2')
    agentEdited(edited({ go: 'cover' }))
    expect(store.onCover).toBe(true)
    expect(store.spread?.id).toBe('cover')
  })

  it('обложка ещё не дошла событием view: переход, как только она появилась', () => {
    agentEdited(edited({ go: 'cover' }))
    expect(store.onCover).toBe(false)
    store.apply(view(5, ['s1', 's2', 's3'], true) as never)
    vi.advanceTimersByTime(200)
    expect(store.onCover).toBe(true)
  })

  it('обложки в альбоме так и нет — после ожидания молча остаёмся', () => {
    agentEdited(edited({ go: 'cover' }))
    vi.advanceTimersByTime(5000)
    expect(store.onCover).toBe(false)
    expect(store.currentId).toBe('s1')
  })

  it('со страницы обложки go — разворот: холст уходит с обложки', () => {
    store.apply(view(5, ['s1', 's2', 's3'], true) as never)
    store.goCover()
    agentEdited(edited({ go: 's3' }))
    expect(store.onCover).toBe(false)
    expect(store.currentId).toBe('s3')
  })

  it('разворота так и нет — после ожидания молча остаёмся', () => {
    agentEdited(edited({ go: 'нет-такого' }))
    vi.advanceTimersByTime(5000)
    expect(store.currentId).toBe('s1')
  })
})

// Настоящий порядок Go: view с новой ревизией приходит раньше agents:edited и гасит прежний тост агента.
describe('правка агента заменяет прежний тост агента', () => {
  it('две правки подряд: один тост с текстом последней, без дописывания', () => {
    agentEdited(edited())
    store.apply(view(6) as never)
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('Claude Code · Раскладки изменены: 3')
    expect(store.toast?.rev).toBe(6)
  })

  it('прежний тост ещё на экране (view не успел) — тоже заменяется', () => {
    agentEdited(edited())
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('Claude Code · Раскладки изменены: 3')
  })

  it('та же правка дважды («Отметки: 1 фото») — без дубля', () => {
    agentEdited(edited({ text: 'Отметки: 1 фото' }))
    store.apply(view(6) as never)
    agentEdited(edited({ text: 'Отметки: 1 фото', rev: 6 }))
    expect(store.toast?.text).toBe('Claude Code · Отметки: 1 фото')
  })

  it('«Вернуть» — один шаг Undo (последняя правка)', async () => {
    agentEdited(edited())
    store.apply(view(6) as never)
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 6 }))
    await store.toast?.action?.run()
    expect(api.Undo).toHaveBeenCalledTimes(1)
  })

  it('много правок за долгое время — тост не копит строки', () => {
    for (let i = 1; i <= 5; i++) {
      store.apply(view(4 + i) as never)
      agentEdited(edited({ text: `Правка ${i}`, rev: 4 + i }))
      vi.advanceTimersByTime(1000)
    }
    expect(store.toast?.text).toBe('Claude Code · Правка 5')
  })

  it('тост гаснет по сроку обычного тоста: 8 с с кнопкой «Вернуть»', () => {
    agentEdited(edited())
    vi.advanceTimersByTime(7999)
    expect(store.toast).not.toBeNull()
    vi.advanceTimersByTime(1)
    expect(store.toast).toBeNull()
  })

  it('другой клиент — свой тост', () => {
    agentEdited(edited())
    store.apply(view(6) as never)
    agentEdited(edited({ client: 'OpenCode', text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('OpenCode · Раскладки изменены: 3')
  })

  it('между ними был другой тост — не склеивается', () => {
    agentEdited(edited())
    store.apply(view(6) as never)
    store.notify('Автосохранено')
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('Claude Code · Раскладки изменены: 3')
  })
})

describe('событие agents:edited от Go', () => {
  it('значок «ИИ» держит подписку: событие даёт тост и переход без открытого окна', () => {
    const off = agents.watch()
    ev.on.get('agents:edited')?.(edited({ go: 's2' }))
    expect(store.toast?.text).toBe('Claude Code · Собрано разворотов: 24')
    expect(store.currentId).toBe('s2')
    off()
    expect(ev.on.has('agents:edited')).toBe(false)
  })
})

const PATH = 'D:\\Фото\\2026\\Петровы'
const quiet = (o: Partial<Parameters<typeof agentEdited>[0]> = {}) =>
  edited({ text: 'Новый альбом из', quiet: true, path: PATH, rev: 5, ...o })
const HOLD_TAIL = 8000
const TOAST_FULL = `Claude Code · Новый альбом из ${PATH}`

// 150: правка без вопроса и первый отказ «Запретить» — тост «до фокуса» (Toasts.hold, design §7.3–7.4).
describe('тост «до фокуса» (150)', () => {
  it('тихая папка: полный путь в тексте и в paths (для <bdi>), «Вернуть» при той же ревизии, тост помечен hold', () => {
    agentEdited(quiet({ text: 'Добавлена папка' }))
    expect(store.toast?.text).toBe(`Claude Code · Добавлена папка ${PATH}`)
    expect(store.toast?.paths).toEqual([PATH])
    expect(store.toast?.hold).toBe(true)
    expect(store.toast?.action?.label).toBe('Вернуть')
    expect(store.toast?.rev).toBe(5)
  })

  it('окно без фокуса: тост висит сколько угодно; фокус включает отсчёт, ещё 8 с — и тост уходит', () => {
    focused = false
    agentEdited(quiet())
    vi.advanceTimersByTime(10 * 60_000)
    expect(store.toast?.text).toBe(TOAST_FULL)
    gainFocus()
    vi.advanceTimersByTime(HOLD_TAIL - 1)
    expect(store.toast).not.toBeNull()
    vi.advanceTimersByTime(1)
    expect(store.toast).toBeNull()
    expect(focusFns.size).toBe(0) // слушатель снят
  })

  it('окно уже в фокусе: тост уходит через 8 с', () => {
    agentEdited(quiet())
    vi.advanceTimersByTime(HOLD_TAIL - 1)
    expect(store.toast).not.toBeNull()
    vi.advanceTimersByTime(1)
    expect(store.toast).toBeNull()
  })

  it('prev: действие «Открыть прежний» зовёт AgentOpenPrev; «Вернуть» нет', async () => {
    agentEdited(quiet({ prev: true }))
    expect(store.toast?.action?.label).toBe('Открыть прежний')
    expect(store.toast?.rev).toBeUndefined()
    store.toast?.action?.run()
    await vi.advanceTimersByTimeAsync(50)
    expect(api.AgentOpenPrev).toHaveBeenCalledTimes(1)
    expect(api.Undo).not.toHaveBeenCalled()
  })

  // open_album (152): Go шлёт Rev -1 (открытие не правка текущего альбома и не отменяется), Quiet и Prev — как у тихого нового альбома.
  const OPENED = 'Открыт альбом «Свадьба Ивановы»'
  it('открытие альбома (Rev -1, prev): тост агента «Клиент · Открыт альбом…» с «Открыть прежний», без «Вернуть»', async () => {
    agentEdited(edited({ text: OPENED, rev: -1, quiet: true, prev: true }))
    expect(store.toast?.text).toBe(`Claude Code · ${OPENED}`)
    expect(store.toast?.action?.label).toBe('Открыть прежний')
    expect(store.toast?.rev).toBeUndefined()
    expect(store.toast?.hold).toBeTruthy()
    store.toast?.action?.run()
    await vi.advanceTimersByTimeAsync(50)
    expect(api.AgentOpenPrev).toHaveBeenCalledTimes(1)
    expect(api.Undo).not.toHaveBeenCalled()
  })

  it('открытие без прежнего файла (prev=false): тост без действий, «Вернуть» нет, текущая ревизия не затронута', () => {
    agentEdited(edited({ text: OPENED, rev: -1, quiet: true }))
    expect(store.toast?.text).toBe(`Claude Code · ${OPENED}`)
    expect(store.toast?.action).toBeUndefined()
    expect(store.toast?.rev).toBeUndefined()
    expect(store.view.rev).toBe(5)
  })

  it('«Открыть прежний» не вышло — тост с ошибкой словами Go', async () => {
    api.AgentOpenPrev.mockRejectedValue(new Error('файл не найден'))
    agentEdited(quiet({ prev: true }))
    store.toast?.action?.run()
    await vi.advanceTimersByTimeAsync(50)
    expect(store.toast?.text).toBe('Файл не найден')
    expect(store.toast?.kind).toBe('error')
  })

  it('тихий альбом → 30 с → автосборка того же клиента: один тост с путём, «Собрано…» и «Открыть прежний», живой до фокуса', () => {
    focused = false
    agentEdited(quiet({ prev: true }))
    vi.advanceTimersByTime(30_000) // сканирование папки: дольше GLUE_MS
    store.apply(view(6) as never) // view с новой ревизией: у hold-тоста гаснет только «Вернуть»
    agentEdited(edited({ text: 'Собрано разворотов: 24', rev: 6 }))
    expect(store.toast?.text).toBe(`${TOAST_FULL}. Собрано разворотов: 24`)
    expect(store.toast?.paths).toEqual([PATH])
    expect(store.toast?.action?.label).toBe('Открыть прежний')
    vi.advanceTimersByTime(5 * 60_000)
    expect(store.toast).not.toBeNull()
    gainFocus()
    vi.advanceTimersByTime(HOLD_TAIL)
    expect(store.toast).toBeNull()
  })

  it('склейка продлевает: каждая новая правка клиента заново ждёт фокуса и даёт ещё 8 с', () => {
    agentEdited(quiet({ prev: true }))
    vi.advanceTimersByTime(HOLD_TAIL - 1000)
    agentEdited(edited({ text: 'Собрано разворотов: 24', rev: 5 }))
    vi.advanceTimersByTime(HOLD_TAIL - 1)
    expect(store.toast?.text).toBe(`${TOAST_FULL}. Собрано разворотов: 24`)
    vi.advanceTimersByTime(1)
    expect(store.toast).toBeNull()
  })

  it('тихая папка → смена ревизии: текст и путь живы, «Вернуть» и rev сняты', () => {
    focused = false
    agentEdited(quiet({ text: 'Добавлена папка' }))
    shownRev(6)
    expect(store.toast?.text).toBe(`Claude Code · Добавлена папка ${PATH}`)
    expect(store.toast?.action).toBeUndefined()
    expect(store.toast?.rev).toBeUndefined()
    expect(store.toast?.hold).toBe(true)
  })

  it('«Открыть прежний» переживает смену ревизии (без rev)', () => {
    focused = false
    agentEdited(quiet({ prev: true }))
    shownRev(6)
    expect(store.toast?.action?.label).toBe('Открыть прежний')
  })

  it('без hold смена ревизии гасит тост, как раньше', () => {
    agentEdited(edited())
    expect(store.toast?.hold).toBeUndefined()
    shownRev(6)
    expect(store.toast).toBeNull()
  })

  it('первая часть закреплена: GLUE_MAX её не вытесняет — в склейке первая и две последних', () => {
    focused = false
    agentEdited(quiet({ prev: true }))
    for (let i = 1; i <= 5; i++) {
      store.apply(view(5 + i) as never)
      agentEdited(edited({ text: `Правка ${i}`, rev: 5 + i }))
    }
    expect(store.toast?.text).toBe(`${TOAST_FULL}. Правка 4. Правка 5`)
  })

  it('две тихие папки подряд: обе в одном тосте, пути обе в paths, «Вернуть» последней', () => {
    focused = false
    agentEdited(quiet({ text: 'Добавлена папка' }))
    store.apply(view(6) as never)
    agentEdited(quiet({ text: 'Добавлена папка', path: 'D:\\Фото\\Камера 2', rev: 6 }))
    expect(store.toast?.text).toBe(`Claude Code · Добавлена папка ${PATH}. Добавлена папка D:\\Фото\\Камера 2`)
    expect(store.toast?.paths).toEqual([PATH, 'D:\\Фото\\Камера 2'])
    expect(store.toast?.rev).toBe(6)
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('«Вернуть» тихой папки при той же ревизии → Undo; после чужой правки — подсказка «Отменить»', async () => {
    agentEdited(quiet({ text: 'Добавлена папка' }))
    const act = store.toast?.action
    await act?.run()
    expect(api.Undo).toHaveBeenCalledTimes(1)
    agentEdited(quiet({ text: 'Добавлена папка' }))
    const late = store.toast?.action
    store.view = { ...store.view, rev: 9 }
    api.Undo.mockClear()
    await late?.run()
    expect(api.Undo).not.toHaveBeenCalled()
  })

  it('ревизия уже дальше тихой правки — тост без «Вернуть», но с путём и hold', () => {
    shownRev(7)
    agentEdited(quiet({ text: 'Добавлена папка', rev: 5 }))
    expect(store.toast?.action).toBeUndefined()
    expect(store.toast?.hold).toBe(true)
  })

  it('чужой тост заменяет тост «до фокуса»; правка агента после него — новый тост, не склейка', () => {
    focused = false
    agentEdited(quiet({ prev: true }))
    store.notify('Автосохранено')
    expect(focusFns.size).toBe(0) // ожидание фокуса снято вместе с тостом
    store.apply(view(6) as never)
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('Claude Code · Раскладки изменены: 3')
    expect(store.toast?.hold).toBeUndefined()
  })

  it('другой клиент в тост «до фокуса» не склеивается', () => {
    focused = false
    agentEdited(quiet({ prev: true }))
    store.apply(view(6) as never)
    agentEdited(edited({ client: 'OpenCode', text: 'Раскладки изменены: 3', rev: 6 }))
    expect(store.toast?.text).toBe('OpenCode · Раскладки изменены: 3')
  })

  it('тост закрыт вручную: правка того же клиента — новый тост (без склейки)', () => {
    focused = false
    agentEdited(quiet({ prev: true }))
    store.toast = null
    agentEdited(edited({ text: 'Собрано разворотов: 24', rev: 5 }))
    expect((store.toast as Toast | null)?.text).toBe('Claude Code · Собрано разворотов: 24')
    gainFocus() // слушатель закрытого тоста не оживляет новый
    expect((store.toast as Toast | null)?.hold).toBeUndefined()
  })
})

describe('первый отказ «Запретить» (agents:denied)', () => {
  const denied = (o: Partial<{ client: string; tool: string; title: string }> = {}) => ({
    client: 'Claude Code',
    tool: 'create_album',
    title: 'Создать альбом',
    ...o,
  })
  const setup = vi.fn()

  beforeEach(() => setup.mockReset())

  it('тост «Клиент: «Инструмент» запрещено» с «Настроить»; «Настроить» открывает окно на строке инструмента', () => {
    agentDenied(denied(), setup)
    expect(store.toast?.text).toBe('Claude Code: «Создать альбом» запрещено')
    expect(store.toast?.hold).toBe(true)
    expect(store.toast?.action?.label).toBe('Настроить')
    store.toast?.action?.run()
    expect(setup).toHaveBeenCalledExactlyOnceWith('create_album')
  })

  it('отказ → 30 с → правка того же клиента: правка не дописывается к отказу, тост обычный', () => {
    focused = false
    agentDenied(denied(), setup)
    vi.advanceTimersByTime(30_000)
    agentEdited(edited({ text: 'Раскладки изменены: 3', rev: 5 }))
    expect(store.toast?.text).toBe('Claude Code · Раскладки изменены: 3')
    expect(store.toast?.action?.label).toBe('Вернуть')
    expect(store.toast?.hold).toBeUndefined()
    vi.advanceTimersByTime(8000)
    expect(store.toast).toBeNull()
  })

  it('правка, потом отказ: отказ — новый тост без правки', () => {
    agentEdited(edited())
    agentDenied(denied(), setup)
    expect(store.toast?.text).toBe('Claude Code: «Создать альбом» запрещено')
  })

  it('второй отказ другого инструмента подряд — в тот же тост одной строкой; «Настроить» ведёт к последнему', () => {
    focused = false
    agentDenied(denied(), setup)
    agentDenied(denied({ tool: 'autobuild', title: 'Автосборка' }), setup)
    expect(store.toast?.text).toBe('Claude Code: «Создать альбом», «Автосборка» запрещено')
    store.toast?.action?.run()
    expect(setup).toHaveBeenCalledExactlyOnceWith('autobuild')
  })

  it('пять отказов разных инструментов — в тосте все пять названий', () => {
    focused = false
    const titles = ['Отметки фото', 'Фото на обложку', 'Фон на развороты', 'Стикеры на развороты', 'Формат типографии']
    titles.forEach((title, i) => agentDenied(denied({ tool: `t${i}`, title }), setup))
    expect(store.toast?.text).toBe(`Claude Code: ${titles.map((t) => `«${t}»`).join(', ')} запрещено`)
  })

  it('повтор отказа по тому же инструменту — без дубля', () => {
    focused = false
    agentDenied(denied(), setup)
    agentDenied(denied(), setup)
    expect(store.toast?.text).toBe('Claude Code: «Создать альбом» запрещено')
  })

  it('отказ гаснет по сроку: фокус окна и ещё 8 с', () => {
    focused = false
    agentDenied(denied(), setup)
    vi.advanceTimersByTime(60_000)
    expect(store.toast).not.toBeNull()
    gainFocus()
    vi.advanceTimersByTime(HOLD_TAIL)
    expect(store.toast).toBeNull()
  })

  it('смена ревизии «Настроить» не снимает; без hold отказ ничего не гасит', () => {
    focused = false
    agentDenied(denied(), setup)
    shownRev(6)
    expect(store.toast?.action?.label).toBe('Настроить')
  })

  it('«Открыть прежний» старше «Настроить»: тихий альбом после отказа оставляет «Открыть прежний»', () => {
    focused = false
    agentDenied(denied(), setup)
    agentEdited(quiet({ prev: true }))
    expect(store.toast?.action?.label).toBe('Открыть прежний')
  })

  it('agents:denied от Go открывает окно на строке инструмента (подписка значка «ИИ»)', () => {
    const show = vi.spyOn(agents, 'show').mockResolvedValue()
    const off = agents.watch()
    ev.on.get('agents:denied')?.(denied())
    store.toast?.action?.run()
    expect(show).toHaveBeenCalledExactlyOnceWith('tool:create_album')
    off()
    show.mockRestore()
  })
})
