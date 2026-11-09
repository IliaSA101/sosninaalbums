import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { COMMANDS } from './commands'
import { liveSpreadCmds, menuCtx } from './menuctx'
import { buildMenu, enabledCmds } from './menu'
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { glueAsk } from '../../panels/spreadmenu/menuact.svelte'
import { clearVignette, sortVignette } from '../../panels/vignette/vigact'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import type { SpreadView } from '../../base/core/types'

// Пять команд разворота (141): обработчики зовут те же вызовы, что правый щелчок; на сером состоянии молчат.
vi.mock('../../panels/vignette/vigact', async (orig) => ({
  ...(await orig<typeof import('../../panels/vignette/vigact')>()),
  sortVignette: vi.fn(async () => true),
  clearVignette: vi.fn(async () => true),
}))

const sp = (id: string, o: Partial<SpreadView> = {}): SpreadView =>
  ({
    id,
    photos: [`${id}0`, `${id}1`, `${id}2`],
    rects: [0, 1, 2].map(() => ({ x: 0, y: 0, w: 1, h: 1 })),
    texts: [],
    grid: false,
    kind: '',
    ...o,
  }) as unknown as SpreadView

const was = { view: store.view, current: store.current, onCover: store.onCover, level: prefs.level, custom: prefs.custom }
let add: ReturnType<typeof vi.spyOn>
let split: ReturnType<typeof vi.spyOn>
let merge: ReturnType<typeof vi.spyOn>
let askCut: ReturnType<typeof vi.spyOn>
let askJoin: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  useCatalog()
  add = vi.spyOn(store, 'addEmptySpread').mockResolvedValue(undefined)
  split = vi.spyOn(store, 'split').mockResolvedValue(undefined)
  merge = vi.spyOn(store, 'merge').mockResolvedValue(undefined)
  askCut = vi.spyOn(glueAsk, 'cut').mockImplementation(() => {})
  askJoin = vi.spyOn(glueAsk, 'join').mockImplementation(() => {})
  // 1 — обычный, 2 — ручной, 3 — виньетка
  store.view = {
    ...store.view,
    spreads: [
      sp('a'),
      sp('b', { grid: true, places: [0, 1, 2].map((i) => ({ id: `pl${i}`, photo: `b${i}`, x: 0, y: 0, w: 1, h: 1, fit: '' })) as never }),
      sp('v', { kind: 'vignette', vig: {} as never }),
    ],
  }
  store.current = 0
  store.onCover = false
  prefs.level = 'full'
  vi.mocked(sortVignette).mockClear()
  vi.mocked(clearVignette).mockClear()
})

afterEach(() => {
  vi.restoreAllMocks()
  placesel.refs = []
  store.view = was.view
  store.current = was.current
  store.onCover = was.onCover
  prefs.level = was.level
  prefs.custom = was.custom
  dropCatalog()
})

describe('«Вставить разворот после» (141)', () => {
  it('вставляет за текущим; на последнем — в конец', () => {
    store.current = 1
    void COMMANDS.insertSpread()
    expect(add).toHaveBeenLastCalledWith(2)
    store.current = 2
    void COMMANDS.insertSpread()
    expect(add).toHaveBeenLastCalledWith(3)
  })

  it('обложка — вставка идёт первым разворотом', () => {
    store.onCover = true
    void COMMANDS.insertSpread()
    expect(add).toHaveBeenLastCalledWith(0)
  })

  it('нет разворотов — молчит', () => {
    store.view = { ...store.view, spreads: [] }
    void COMMANDS.insertSpread()
    expect(add).not.toHaveBeenCalled()
  })
})

describe('«Разрезать перед выделенным фото» (141)', () => {
  it('обычный разворот: store.split без вопроса', () => {
    placesel.refs = ['a1']
    void COMMANDS.splitSpread()
    expect(split).toHaveBeenCalledWith('a', 1)
    expect(askCut).not.toHaveBeenCalled()
  })

  it('ручной разворот: вопрос glueAsk.cut, модель не трогаем', () => {
    store.current = 1
    placesel.refs = ['pl2']
    void COMMANDS.splitSpread()
    expect(askCut).toHaveBeenCalledWith('b', 2)
    expect(split).not.toHaveBeenCalled()
  })

  it('молчит: ничего не выделено, два фото, первое, виньетка', () => {
    void COMMANDS.splitSpread()
    placesel.refs = ['a1', 'a2']
    void COMMANDS.splitSpread()
    placesel.refs = ['a0']
    void COMMANDS.splitSpread()
    store.current = 2
    placesel.refs = ['v1']
    void COMMANDS.splitSpread()
    expect(split).not.toHaveBeenCalled()
    expect(askCut).not.toHaveBeenCalled()
  })

  it('молчит на обложке, даже с выделенным фото', () => {
    placesel.refs = ['a1']
    store.onCover = true
    void COMMANDS.splitSpread()
    expect(split).not.toHaveBeenCalled()
    expect(askCut).not.toHaveBeenCalled()
  })

  it('молчит без функции «Разрез и склейка» на уровне «Свой»', () => {
    placesel.refs = ['a1']
    prefs.level = 'custom'
    prefs.custom = { base: 'full', on: [], off: ['timeline'] }
    expect(prefs.can('timeline')).toBe(false)
    void COMMANDS.splitSpread()
    expect(split).not.toHaveBeenCalled()
    expect(askCut).not.toHaveBeenCalled()
  })
})

describe('«Склеить со следующим» (141)', () => {
  it('обычная пара — store.merge', () => {
    store.view = { ...store.view, spreads: [sp('a'), sp('c')] }
    void COMMANDS.joinSpread()
    expect(merge).toHaveBeenCalledWith('a')
    expect(askJoin).not.toHaveBeenCalled()
  })

  it('ручной в паре — вопрос glueAsk.join', () => {
    void COMMANDS.joinSpread() // a + b(ручной)
    expect(askJoin).toHaveBeenCalledWith('a')
    expect(merge).not.toHaveBeenCalled()
  })

  it('молчит: виньетка в паре, последний разворот', () => {
    store.current = 1 // b + v(виньетка)
    void COMMANDS.joinSpread()
    store.current = 2
    void COMMANDS.joinSpread()
    expect(merge).not.toHaveBeenCalled()
    expect(askJoin).not.toHaveBeenCalled()
  })

  it('молчит на обложке', () => {
    store.onCover = true
    void COMMANDS.joinSpread()
    expect(merge).not.toHaveBeenCalled()
    expect(askJoin).not.toHaveBeenCalled()
  })

  it('молчит без функции «Разрез и склейка» на уровне «Свой»', () => {
    prefs.level = 'custom'
    prefs.custom = { base: 'full', on: [], off: ['timeline'] }
    void COMMANDS.joinSpread()
    expect(merge).not.toHaveBeenCalled()
    expect(askJoin).not.toHaveBeenCalled()
  })
})

describe('«Сортировать по фамилии» и «Обычный разворот» (141)', () => {
  it('на виньетке зовут действия карточки', () => {
    store.current = 2
    void COMMANDS.sortVignette()
    void COMMANDS.clearVignette()
    expect(sortVignette).toHaveBeenCalledWith('v')
    expect(clearVignette).toHaveBeenCalledWith('v')
  })

  it('на обычном листе и на «Простом» молчат', () => {
    void COMMANDS.sortVignette()
    void COMMANDS.clearVignette()
    store.current = 2
    prefs.level = 'simple'
    void COMMANDS.sortVignette()
    void COMMANDS.clearVignette()
    expect(sortVignette).not.toHaveBeenCalled()
    expect(clearVignette).not.toHaveBeenCalled()
  })
})

describe('меню и команды согласованы (141, 074-8)', () => {
  it('пункт доступен ровно тогда, когда обработчик что-то делает', () => {
    const on = () => enabledCmds(buildMenu(menuCtx()))
    expect(on().has('insertSpread')).toBe(true)
    expect(on().has('splitSpread')).toBe(false) // ничего не выделено
    expect(on().has('sortVignette')).toBe(false)
    placesel.refs = ['a2']
    expect(on().has('splitSpread')).toBe(true)
    expect(on().has('joinSpread')).toBe(true)
    store.current = 2
    placesel.refs = []
    expect(on().has('sortVignette')).toBe(true)
    expect(on().has('clearVignette')).toBe(true)
    expect(on().has('joinSpread')).toBe(false) // последний
  })

  it('liveSpreadCmds берёт выделенные фото текущего листа', () => {
    placesel.refs = ['a1']
    expect(liveSpreadCmds().cut).toMatchObject({ ok: true, id: 'a', at: 1 })
    store.current = 1
    expect(liveSpreadCmds().cut.ok).toBe(false) // выделено фото другого листа
  })
})
