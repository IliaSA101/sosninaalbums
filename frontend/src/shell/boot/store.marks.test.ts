import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go подменён: Marks отвечает по очереди (можно задержать ответ), SetMarks/ResetMarks записывают вызов.
const calls: { name: string; ids: string[]; patch?: unknown }[] = []
let marksCalls = 0
let buildArgs: unknown = null
let marksAnswer: () => Promise<unknown> = async () => ({})
const view = (rev: number, marksRev: number, extra: object = {}) => ({ rev, marksRev, folders: [], spreads: [], ...extra })

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  Marks: () => {
    marksCalls++
    return marksAnswer()
  },
  SetMarks: async (ids: string[], patch: unknown) => {
    calls.push({ name: 'set', ids, patch })
    if ((patch as { rating?: number }).rating === 99) throw 'слишком много своих отметок (больше 100 000)'
    return view(5, 2)
  },
  AutoBuild: async (ids: string[], at: number, o: { spreads: number; maxPer: number; variety: number }) => {
    buildArgs = { at, spreads: o.spreads, maxPer: o.maxPer, variety: o.variety }
    calls.push({ name: 'build', ids })
    return view(7, 1, {
      spreads: [
        { id: 's', photos: ['a'] },
        { id: 'n', photos: ids },
      ],
    })
  },
  FillPlaces: async (ids: string[]) => {
    calls.push({ name: 'fill', ids })
    return { view: view(8, 1, { spreads: [{ id: 's', photos: ['a'] }] }), left: 0 }
  },
  ResetMarks: async (ids: string[]) => {
    calls.push({ name: 'reset', ids })
    return view(6, 3)
  },
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('./wire')
const { libfilter } = await import('../../panels/marks/libfilter.svelte')
const { ownMarks } = await import('../../panels/marks/ownmarks.svelte')

const ph = (id: string, xmp?: object) => ({ id, path: id, name: id, w: 100, h: 100, xmp })
wire()

const tick = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  calls.length = 0
  marksCalls = 0
  marksAnswer = async () => ({})
  libfilter.reset()
  store.selected.clear()
  store.toast = null
  store.filter = 'all'
  store.photos = [ph('a', { rating: 5 }), ph('b', { rating: 2 }), ph('c')] as never
  ownMarks.invalidate()
  store.apply(view(1, 1) as never)
})

describe('своя отметка: загрузка по marksRev', () => {
  it('apply читает отметки при смене marksRev, не чаще', async () => {
    await tick()
    expect(marksCalls).toBe(1) // из beforeEach
    marksCalls = 0
    marksAnswer = async () => ({ c: { rating: 4 } })
    store.apply(view(2, 5) as never)
    await tick()
    expect(marksCalls).toBe(1)
    expect(store.marks).toEqual({ c: { rating: 4 } })
    store.apply(view(3, 5) as never) // marksRev тот же
    expect(marksCalls).toBe(1)
    store.apply(view(4, 6) as never)
    expect(marksCalls).toBe(2)
  })

  it('ответ null — пустая карта', async () => {
    marksAnswer = async () => null
    ownMarks.invalidate()
    store.apply(view(2, 1) as never)
    await tick()
    expect(store.marks).toEqual({})
  })

  it('устаревший ответ отбрасывается', async () => {
    let first!: (v: unknown) => void
    marksAnswer = () => new Promise((r) => (first = r))
    store.apply(view(2, 2) as never) // запрос 1 висит
    marksAnswer = async () => ({ new: { rating: 1 } })
    store.apply(view(3, 3) as never) // запрос 2 отвечает сразу
    await tick()
    first({ old: { rating: 5 } })
    await tick()
    expect(store.marks).toEqual({ new: { rating: 1 } })
  })

  it('ошибка Api.Marks не роняет apply, следующий apply пробует снова', async () => {
    marksAnswer = async () => {
      throw new Error('нет')
    }
    store.apply(view(2, 2) as never)
    await tick()
    marksAnswer = async () => ({ a: { flag: 1 } })
    store.apply(view(3, 2) as never)
    await tick()
    expect(store.marks).toEqual({ a: { flag: 1 } })
  })
})

describe('visible и фильтр по отметкам', () => {
  it('без условий — вся лента; с условиями — по действующей отметке', async () => {
    marksAnswer = async () => ({ c: { rating: 4 } })
    ownMarks.invalidate()
    store.apply(view(2, 1) as never)
    await tick()
    expect(store.visible.map((p) => p.id)).toEqual(['a', 'b', 'c'])
    libfilter.minRating = 4
    expect(store.visible.map((p) => p.id)).toEqual(['a', 'c'])
    libfilter.reset()
    expect(store.visible).toHaveLength(3)
  })

  it('фото, ушедшее из ленты по фильтру, не входит в selectedIds', () => {
    store.selected.add('a')
    store.selected.add('b')
    libfilter.minRating = 3
    expect(store.selectedIds).toEqual(['a'])
  })

  it('перекрывается с «Неиспользованными»', () => {
    store.apply(view(2, 1, { spreads: [{ id: 's', photos: ['a'] }] }) as never)
    store.filter = 'unused'
    libfilter.minRating = 1
    expect(store.visible.map((p) => p.id)).toEqual(['b'])
  })
})

describe('setMarks / resetMarks', () => {
  it('отправляют выделенные видимые фото; view приходит из ответа', async () => {
    store.selected.add('a')
    store.selected.add('c')
    libfilter.minRating = 0
    expect(await ownMarks.setMarks(store, { rating: 3 })).toBe(true)
    expect(calls).toEqual([{ name: 'set', ids: ['a', 'c'], patch: { rating: 3 } }])
    expect(store.view.rev).toBe(5)
    expect(await ownMarks.resetMarks(store)).toBe(true)
    expect(calls[1]).toEqual({ name: 'reset', ids: ['a', 'c'] })
  })

  it('невидимые по фильтру выделенные не отправляются', async () => {
    store.selected.add('a')
    store.selected.add('c')
    libfilter.minRating = 3
    await ownMarks.setMarks(store, { label: 'red' })
    expect(calls[0].ids).toEqual(['a'])
  })

  it('ошибка Go («слишком много») — тост с заглавной', async () => {
    store.selected.add('a')
    expect(await ownMarks.setMarks(store, { rating: 99 })).toBe(false)
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Слишком много своих отметок (больше 100 000)' })
  })

  it('без выделения — тост и без вызова Go', async () => {
    expect(await ownMarks.setMarks(store, { rating: 1 })).toBe(false)
    expect(calls).toEqual([])
    expect(store.toast?.text).toBe('Выделите фото в ленте, чтобы поставить отметку')
  })
})

describe('автосборка и заполнение мест: что уходит в Go', () => {
  // a стоит в альбоме, b отклонён своей отметкой, c и d свободны.
  async function setup(opts: { rejected?: string[] } = {}) {
    store.photos = ['a', 'b', 'c', 'd'].map((id) => ph(id)) as never
    marksAnswer = async () => Object.fromEntries((opts.rejected ?? ['b']).map((id) => [id, { flag: -1 }]))
    ownMarks.invalidate()
    store.apply(view(2, 1, { spreads: [{ id: 's', photos: ['a'] }] }) as never)
    await tick()
  }

  it('автосборка с выделением — выделение как есть, и уже стоящие фото тоже', async () => {
    await setup()
    store.selected.add('a')
    store.selected.add('b')
    store.openAuto()
    await store.autoBuild(1, 64)
    expect(calls).toEqual([{ name: 'build', ids: ['a', 'b'] }])
    // Контракт 041: (ids, at, AutoOpts{spreads, maxPer, variety}), at — конец альбома; по умолчанию «Обычное»
    expect(buildArgs).toEqual({ at: 1, spreads: 1, maxPer: 64, variety: 1 })
    expect(store.toast?.text).toBe('Собрано разворотов: 1')
  })

  it('автосборка без выделения — свободные без отклонённых, тост «Отклонённые не взяты»', async () => {
    await setup()
    store.openAuto()
    await store.autoBuild(1, 64)
    expect(calls).toEqual([{ name: 'build', ids: ['c', 'd'] }])
    expect(store.toast?.text).toBe('Собрано разворотов: 1. Отклонённые не взяты: 1')
  })

  it('заполнение мест с выделением — без уже стоящих в альбоме', async () => {
    await setup()
    store.selected.add('a')
    store.selected.add('b')
    await store.fillPlaces()
    expect(calls).toEqual([{ name: 'fill', ids: ['b'] }])
  })

  it('заполнение мест без выделения — без отклонённых, тост о них', async () => {
    await setup()
    await store.fillPlaces()
    expect(calls).toEqual([{ name: 'fill', ids: ['c', 'd'] }])
    expect(store.toast?.text).toBe('Отклонённые не взяты: 1')
  })

  it('нечего брать: все свободные отклонены — Go не зовём, тост с причиной', async () => {
    await setup({ rejected: ['b', 'c', 'd'] })
    store.openAuto()
    await store.autoBuild(1, 64)
    expect(calls).toEqual([])
    expect(store.toast?.text).toBe('Отклонённые не взяты: 3')
  })

  it('заполнение мест: выделены только стоящие в альбоме — «уже в альбоме»', async () => {
    await setup()
    store.selected.add('a')
    await store.fillPlaces()
    expect(calls).toEqual([])
    expect(store.toast?.text).toBe('Выделенные фото уже в альбоме')
  })
})

describe('ownMarks.invalidate', () => {
  it('сбрасывает карту и заставляет перечитать при том же marksRev', async () => {
    marksAnswer = async () => ({ a: { rating: 3 } })
    ownMarks.invalidate()
    store.apply(view(2, 1) as never)
    await tick()
    expect(store.marks).toEqual({ a: { rating: 3 } })
    marksCalls = 0
    marksAnswer = async () => {
      throw new Error('нет')
    }
    ownMarks.invalidate()
    expect(store.marks).toEqual({}) // отметки прошлого альбома не висят
    store.apply(view(3, 1) as never)
    await tick()
    expect(marksCalls).toBe(1)
  })
})
