import { beforeEach, describe, expect, it, vi } from 'vitest'

// Окно «Фото уже в альбоме» (098): тексты, «Перенести» только при одном месте, ID для Go по ответу, ссылки, «Отмена» без Go.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  WhereUsed: vi.fn(),
  AddPhotos: vi.fn(),
  NewSpread: vi.fn(),
  PlacePhotos: vi.fn(),
  DropPhotos: vi.fn(),
  ForgetFileNames: vi.fn(),
}))
vi.stubGlobal('window', globalThis)

const Api = await import('../../../wailsjs/go/app/App')
const { store } = await import('../core/store.svelte')
const { noHooks } = await import('../core/hooks')
const { askCopy, answerCopy, copyAsk, gateCopy, showWhere } = await import('./copyask.svelte')
const { copyAskText, sentIds, whereOf, WHERE_MAX } = await import('./copyask')

const WhereUsed = vi.mocked(Api.WhereUsed)
const use = (sheet: number, label: string, ref?: string) => ({ sheet, label, ...(ref ? { ref } : {}) })
const at = (id: string, uses: ReturnType<typeof use>[], places: number, move = '') => ({ id, uses, places, move })
const sp = (id: string, ...photos: string[]) => ({ id, photos, grid: false })
const view = (rev: number, ...spreads: object[]) => ({ rev, marksRev: 0, folders: [], spreads, settings: { sheet: false } }) as never
const flush = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  vi.clearAllMocks()
  copyAsk.cur = null
  store.selected.clear()
  store.plug({ ...noHooks, gate: gateCopy, select: vi.fn() })
  store.apply(view(1, sp('s1', 'f1'), sp('s2', 'f2')))
})

describe('тексты окна', () => {
  it('одно фото: «Это фото уже стоит:» и строка места', () => {
    const t = copyAskText(['f1'], [at('f1', [use(0, 'Разворот 1 — место')], 1, 'f1')])
    expect(t).toMatchObject({ title: 'Фото уже в альбоме', lead: 'Это фото уже стоит:', rest: '', move: true, hint: '' })
    expect(t.where).toEqual([{ sheet: 0, link: 'Разворот 1', kind: 'место', ref: 'f1' }])
  })

  it('несколько фото, часть свободна: «2 из 5», остальные встанут', () => {
    const used = [
      at('a', [use(0, 'Разворот 1 — место')], 1, 'a'),
      at('b', [use(1, 'Разворот 2 — место')], 1, 'b'),
      at('c', [], 0),
      at('d', [], 0),
      at('e', [], 0),
    ]
    const t = copyAskText(['a', 'b', 'c', 'd', 'e'], used)
    expect(t.lead).toBe('2 из 5 фото уже стоят в альбоме')
    expect(t.rest).toBe('Остальные фото встанут в любом случае.')
    expect(t.move).toBe(true)
  })

  it('одно из пяти стоит — «стоит»', () => {
    const t = copyAskText(['a', 'b'], [at('a', [use(0, 'Разворот 1 — место')], 1, 'a'), at('b', [], 0)])
    expect(t.lead).toBe('1 из 2 фото уже стоит в альбоме')
  })

  it('у фото два места — «Перенести» нет, подсказка про холст', () => {
    const t = copyAskText(['f1'], [at('f1', [use(0, 'Обложка — место'), use(1, 'Разворот 1 — место', 'f1-2')], 2)])
    expect(t.move).toBe(false)
    expect(t.hint).toBe('Перенести одно из мест — перетащите его на холсте.')
  })

  it('одно из двух стоящих фото с двумя местами — «Перенести» нет для всех', () => {
    const t = copyAskText(
      ['a', 'b'],
      [at('a', [use(0, 'Разворот 1 — место')], 1, 'a'), at('b', [use(0, 'Разворот 1 — место'), use(1, 'Разворот 2 — место')], 2)],
    )
    expect(t.move).toBe(false)
  })

  it('места: фон, стикер, рамочка и фон альбома — не «стоит»; больше пяти — «и ещё N»', () => {
    const uses = [
      use(-1, 'Фон альбома'),
      use(0, 'Разворот 1 — фон'),
      use(0, 'Разворот 1 — стикер'),
      use(0, 'Разворот 1 — рамочка'),
      ...Array.from({ length: 7 }, (_, i) => use(i, `Разворот ${i + 1} — место`)),
    ]
    const u = [at('f1', uses, 7)]
    expect(whereOf(u)).toHaveLength(7)
    const t = copyAskText(['f1'], u)
    expect(t.where).toHaveLength(WHERE_MAX)
    expect(t.more).toBe(2)
  })

  it('ссылка на копию подсвечивает запись копии (ref), иначе файл', () => {
    const w = whereOf([at('f1', [use(0, 'Разворот 1 — место', 'f1-2'), use(1, 'Разворот 2 — место')], 2)])
    expect(w.map((x) => x.ref)).toEqual(['f1-2', 'f1'])
  })
})

describe('ID для Go по ответу', () => {
  const used = [at('a', [use(0, 'Разворот 1 — место')], 1, 'a-2'), at('b', [], 0)]
  it('копия: стоящее — запрос «<файл>-new», свободное — как есть', () => {
    expect(sentIds(['a', 'b'], used, 'copy')).toEqual(['a-new', 'b'])
  })
  it('перенос: стоящее — запись единственного места', () => {
    expect(sentIds(['a', 'b'], used, 'move')).toEqual(['a-2', 'b'])
  })
  it('отмена — null', () => {
    expect(sentIds(['a'], used, 'cancel')).toBeNull()
  })
  it('ID записи на входе (вставка 099) сводится к файлу', () => {
    expect(sentIds(['a-3'], used, 'copy')).toEqual(['a-new'])
  })
})

describe('askCopy и ворота', () => {
  it('askCopy: окно открыто, кнопка отдаёт ответ и закрывает окно', async () => {
    WhereUsed.mockResolvedValueOnce([at('f1', [use(0, 'Разворот 1 — место')], 1, 'f1')] as never)
    const p = askCopy(['f1'])
    await flush()
    expect(WhereUsed).toHaveBeenCalledExactlyOnceWith(['f1'])
    expect(copyAsk.cur?.ids).toEqual(['f1'])
    answerCopy('move')
    expect(await p).toBe('move')
    expect(copyAsk.cur).toBeNull()
  })

  it('ошибка Go — тост и «Отмена»', async () => {
    WhereUsed.mockRejectedValueOnce(new Error('сбой'))
    expect(await askCopy(['f1'])).toBe('cancel')
    expect(store.toast?.kind).toBe('error')
    store.toast = null
  })

  it('ворота: ничего не стоит — ids как есть, Go не зовём и окна нет', async () => {
    expect(await gateCopy(['x', 'y'])).toEqual(['x', 'y'])
    expect(WhereUsed).not.toHaveBeenCalled()
    expect(copyAsk.cur).toBeNull()
  })

  it('ворота: «Поставить копию» — запрос копии; «Отмена» — null', async () => {
    WhereUsed.mockResolvedValue([at('f1', [use(0, 'Разворот 1 — место')], 1, 'f1')] as never)
    let p = gateCopy(['f1', 'x'])
    await flush()
    answerCopy('copy')
    expect(await p).toEqual(['f1-new', 'x'])
    p = gateCopy(['f1'])
    await flush()
    answerCopy('cancel')
    expect(await p).toBeNull()
  })

  it('ворота: фото стоит только копией («f1-2») — спрашиваем по файлу', async () => {
    store.apply(view(2, sp('s1', 'f1-2')))
    WhereUsed.mockResolvedValue([at('f1', [use(0, 'Разворот 1 — место', 'f1-2')], 1, 'f1-2')] as never)
    const p = gateCopy(['f1'])
    await flush()
    answerCopy('move')
    expect(await p).toEqual(['f1-2'])
  })

  it('второй вопрос, пока открыт первый, — «Отмена»; первый жив', async () => {
    WhereUsed.mockResolvedValue([at('f1', [use(0, 'Разворот 1 — место')], 1, 'f1')] as never)
    const p = askCopy(['f1'])
    await flush()
    expect(await askCopy(['f1'])).toBe('cancel')
    answerCopy('copy')
    expect(await p).toBe('copy')
  })

  it('правка из другого места (rev сменился) отменяет вопрос', async () => {
    WhereUsed.mockResolvedValue([at('f1', [use(0, 'Разворот 1 — место')], 1, 'f1')] as never)
    const p = askCopy(['f1'])
    await flush()
    copyAsk.prune(view(5))
    expect(await p).toBe('cancel')
    expect(copyAsk.cur).toBeNull()
  })

  it('ссылка «где»: вопрос отменён, лист открыт, место выделено', async () => {
    WhereUsed.mockResolvedValue([at('f1', [use(1, 'Разворот 2 — место', 'f1-2')], 1, 'f1-2')] as never)
    const select = vi.fn()
    store.plug({ ...noHooks, gate: gateCopy, select })
    const p = askCopy(['f1'])
    await flush()
    showWhere(whereOf(copyAsk.cur!.used)[0])
    expect(await p).toBe('cancel')
    expect(store.current).toBe(1)
    expect(select).toHaveBeenCalledWith(['f1-2'])
  })
})
