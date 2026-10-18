import { describe, expect, it, vi } from 'vitest'
import { COMMANDS } from './commands'
import { canvas } from '../../editor/canvas/canvas.svelte'
import { ownMarks } from '../../panels/marks/ownmarks.svelte'
import { HOTKEYS, KEY_ACTIONS } from './hotkeys'
import { buildMenu, type MenuCtx, type MenuItem } from './menu'
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { modes } from '../../base/prefs/view.svelte'
import { spreadsel } from '../../panels/timeline/spreadsel.svelte'
import { makeVignette } from '../../panels/vignette/vigact'
import { vigDialog } from '../../panels/vignette/vigsession.svelte'
import { decor } from '../../panels/decor/decor.svelte'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import type { SpreadView } from '../../base/core/types'

// Go не зовём: «Сделать виньеткой» проверяем по вызову действия vigact.
vi.mock('../../panels/vignette/vigact', async (orig) => ({
  ...(await orig<typeof import('../../panels/vignette/vigact')>()),
  makeVignette: vi.fn(async () => true),
}))

const full: MenuCtx = {
  spreads: 1,
  hasSpread: true,
  canUndo: true,
  canRedo: true,
  zone: 'canvas',
  libraryPhotos: 1,
  anySelection: true,
  deletable: true,
  flippable: true,
  missing: 1,
  big: false,
  details: false,
  safe: false,
  rulers: true,
  snap: true,
  recent: [{ path: 'a.salbum', name: 'a', exists: true, modified: 1 }],
}
const flat = (items: MenuItem[]): MenuItem[] => items.flatMap((i) => [i, ...(i.sub ? flat(i.sub) : [])])

describe('команды', () => {
  it('у каждого пункта меню есть команда', () => {
    // Пункты обеих зон: «Выделить всё» меняет команду.
    const items = [...buildMenu(full), ...buildMenu({ ...full, zone: 'library' })].flatMap((m) => flat(m.items))
    expect(items.length).toBeGreaterThan(20)
    for (const it of items) {
      if (it.sub) expect(it.cmd, it.id).toBeUndefined()
      else {
        expect(it.cmd, it.id).toBeDefined()
        expect(typeof COMMANDS[it.cmd!], it.id).toBe('function')
      }
    }
  })

  it('у каждого действия таблицы клавиш есть команда', () => {
    for (const a of KEY_ACTIONS) expect(typeof COMMANDS[a], a).toBe('function')
    for (const h of HOTKEYS) if (h.action) expect(COMMANDS[h.action], h.show).toBeDefined()
  })

  it('Ctrl+R ничего не делает, но команда есть (окно не перезагружается)', () => {
    expect(COMMANDS.reload()).toBeUndefined()
  })

  it('клавиши отметок собирают MarkPatch из e.key', () => {
    const set = vi.spyOn(ownMarks, 'setMarks').mockResolvedValue(true)
    const key = (k: string) => ({ key: k }) as KeyboardEvent
    COMMANDS.markRating(key('4'))
    COMMANDS.markRating(key('0'))
    COMMANDS.markStep(key(']'))
    COMMANDS.markStep(key('х'))
    COMMANDS.markStep(key('ъ'))
    COMMANDS.markLabel(key('7'))
    COMMANDS.markPick(key('p'))
    COMMANDS.markReject(key('x'))
    COMMANDS.markUnflag(key('u'))
    expect(set.mock.calls.map((c) => c[1])).toEqual([
      { rating: 4 },
      { rating: 0 },
      { step: 1 },
      { step: -1 },
      { step: 1 },
      { label: 'yellow', toggle: true },
      { flag: 1 },
      { flag: -1 },
      { flag: 0 },
    ])
    // Без события (меню) и с чужой клавишей — ничего.
    set.mockClear()
    COMMANDS.markRating()
    COMMANDS.markLabel(key('1'))
    expect(set).not.toHaveBeenCalled()
    set.mockRestore()
  })

  it('«Настроить виньетку…»: на виньетке открывает окно этого листа; на обычном листе и на «Простом» — молчит', () => {
    const was = { view: store.view, current: store.current, level: prefs.level }
    const vig = { id: 'v', kind: 'vignette', vig: {}, photos: [] } as unknown as SpreadView
    try {
      store.view = { ...store.view, spreads: [{ id: 'a', photos: [] } as unknown as SpreadView, vig] }
      store.current = 0
      prefs.level = 'standard'
      COMMANDS.vigSetup()
      expect(vigDialog.id).toBeNull()
      store.current = 1
      prefs.level = 'simple'
      COMMANDS.vigSetup()
      expect(vigDialog.id).toBeNull()
      prefs.level = 'standard'
      COMMANDS.vigSetup()
      expect(vigDialog.id).toBe('v')
    } finally {
      vigDialog.close()
      store.view = was.view
      store.current = was.current
      prefs.level = was.level
    }
  })

  it('«Галерея фонов…» (048): открывает окно текущего листа на весь лист; на «Простом» и без листа — молчит', () => {
    const was = { view: store.view, current: store.current, level: prefs.level }
    const show = vi.spyOn(decor, 'show').mockImplementation(() => {})
    try {
      store.view = { ...store.view, spreads: [{ id: 'a', photos: [] } as unknown as SpreadView] }
      store.current = 0
      prefs.level = 'simple'
      COMMANDS.bgGallery()
      expect(show).not.toHaveBeenCalled()
      prefs.level = 'standard'
      COMMANDS.bgGallery()
      expect(show).toHaveBeenCalledWith('bg', 'a', 'spread')
    } finally {
      show.mockRestore()
      store.view = was.view
      store.current = was.current
      prefs.level = was.level
    }
  })

  it('«Копировать оформление…» (049 Ф2): открывает окно режима decor для текущего листа; обложка, «Простой» и без листа — молчит', () => {
    const was = { view: store.view, current: store.current, level: prefs.level, onCover: store.onCover }
    try {
      store.view = { ...store.view, spreads: [{ id: 'a', photos: [] } as unknown as SpreadView] }
      store.current = 0
      prefs.level = 'simple'
      COMMANDS.copyDecor()
      expect(gridAct.copyGrid).toBeNull()
      prefs.level = 'standard'
      COMMANDS.copyDecor()
      expect(gridAct.copyGrid).toEqual({ spread: 'a', from: 'spread', mode: 'decor' })
      // повторный вызов — то же окно, без второго
      COMMANDS.copyDecor()
      expect(gridAct.copyGrid).toEqual({ spread: 'a', from: 'spread', mode: 'decor' })
      gridAct.copyGrid = null
      store.view = { ...store.view, spreads: [] }
      COMMANDS.copyDecor()
      expect(gridAct.copyGrid).toBeNull()
      store.view = {
        ...store.view,
        spreads: [{ id: 'a', photos: [] } as unknown as SpreadView],
        cover: { sheet: { id: 'cover', photos: [] } } as never,
      }
      store.onCover = true
      COMMANDS.copyDecor()
      expect(gridAct.copyGrid).toBeNull()
    } finally {
      gridAct.copyGrid = null
      store.view = was.view
      store.current = was.current
      store.onCover = was.onCover
      prefs.level = was.level
    }
  })

  it('«Добавить стикер…» (048): открывает окно стикеров текущего листа; на «Простом» и без листа — молчит', () => {
    const was = { view: store.view, current: store.current, level: prefs.level }
    const show = vi.spyOn(decor, 'show').mockImplementation(() => {})
    try {
      store.view = { ...store.view, spreads: [{ id: 'a', photos: [] } as unknown as SpreadView] }
      store.current = 0
      prefs.level = 'simple'
      COMMANDS.addSticker()
      expect(show).not.toHaveBeenCalled()
      prefs.level = 'standard'
      COMMANDS.addSticker()
      expect(show).toHaveBeenCalledWith('sticker', 'a', 'spread')
      show.mockClear()
      store.view = { ...store.view, spreads: [] }
      COMMANDS.addSticker()
      expect(show).not.toHaveBeenCalled()
    } finally {
      show.mockRestore()
      store.view = was.view
      store.current = was.current
      prefs.level = was.level
    }
  })

  it('«Готовые развороты…» (067): окно разворотов текущего листа на весь лист; обложка, «Простой» и без листа — молчит', () => {
    const was = { view: store.view, current: store.current, level: prefs.level, onCover: store.onCover }
    const show = vi.spyOn(decor, 'show').mockImplementation(() => {})
    try {
      store.view = { ...store.view, spreads: [{ id: 'a', photos: [] } as unknown as SpreadView] }
      store.current = 0
      prefs.level = 'simple'
      COMMANDS.spreadTpl()
      expect(show).not.toHaveBeenCalled()
      prefs.level = 'standard'
      COMMANDS.spreadTpl()
      expect(show).toHaveBeenCalledWith('spread', 'a', 'spread')
      show.mockClear()
      store.view = { ...store.view, spreads: [] }
      COMMANDS.spreadTpl()
      expect(show).not.toHaveBeenCalled()
      store.view = {
        ...store.view,
        spreads: [{ id: 'a', photos: [] } as unknown as SpreadView],
        cover: { sheet: { id: 'cover', photos: [] } } as never,
      }
      store.onCover = true
      COMMANDS.spreadTpl()
      expect(show).not.toHaveBeenCalled()
    } finally {
      show.mockRestore()
      store.view = was.view
      store.current = was.current
      store.onCover = was.onCover
      prefs.level = was.level
    }
  })

  it('«Разнообразить раскладки» (041): выделенные в ленте, иначе текущий; уровень из окна автосборки; на «Простом» тоже', async () => {
    const was = { view: store.view, current: store.current, level: prefs.level, variety: modes.autoVariety }
    const vary = vi.spyOn(store, 'varyLayouts').mockResolvedValue(true)
    const sp = (id: string, o: Partial<SpreadView> = {}) => ({ id, photos: ['p'], grid: false, kind: '', ...o }) as unknown as SpreadView
    try {
      store.view = { ...store.view, spreads: [sp('a'), sp('b'), sp('c')] }
      store.current = 1
      prefs.level = 'simple'
      await COMMANDS.varyLayouts()
      expect(vary).toHaveBeenLastCalledWith(['b'], 1, [])
      spreadsel.toggle('c')
      spreadsel.toggle('a')
      modes.autoVariety = 0 // «Меньше» считается «Обычным»
      await COMMANDS.varyLayouts()
      expect(vary).toHaveBeenLastCalledWith(['a', 'c'], 1, [])
      modes.autoVariety = 2
      await COMMANDS.varyLayouts()
      expect(vary).toHaveBeenLastCalledWith(['a', 'c'], 2, [])
      modes.autoFamilies = ['dense', 'hero'] // 065: «Разнообразить» берёт семейства окна автосборки
      await COMMANDS.varyLayouts()
      expect(vary).toHaveBeenLastCalledWith(['a', 'c'], 2, ['dense', 'hero'])
      // Все цели — сетка или виньетка: молча, Go не зовём (пункт меню серый, тоста нет)
      vary.mockClear()
      store.view = { ...store.view, spreads: [sp('a', { grid: true }), sp('b'), sp('c', { kind: 'vignette' })] }
      await COMMANDS.varyLayouts()
      expect(vary).not.toHaveBeenCalled()
    } finally {
      spreadsel.clear()
      vary.mockRestore()
      store.view = was.view
      store.current = was.current
      prefs.level = was.level
      modes.autoVariety = was.variety
      modes.autoFamilies = []
    }
  })

  it('«Сделать разворот виньеткой»: обычный текущий разворот на «Стандартном» — makeVignette; иначе ничего', async () => {
    const was = { view: store.view, current: store.current, level: prefs.level }
    const sp = { id: 'a', photos: ['1'] } as unknown as SpreadView
    try {
      store.view = { ...store.view, spreads: [sp] }
      store.current = 0
      prefs.level = 'simple'
      await COMMANDS.makeVignette()
      expect(makeVignette).not.toHaveBeenCalled()
      prefs.level = 'standard'
      await COMMANDS.makeVignette()
      expect(makeVignette).toHaveBeenCalledWith(store.view.spreads[0])
      vi.mocked(makeVignette).mockClear()
      store.view = { ...store.view, spreads: [{ ...sp, kind: 'vignette', vig: {} } as unknown as SpreadView] }
      await COMMANDS.makeVignette()
      expect(makeVignette).not.toHaveBeenCalled()
    } finally {
      store.view = was.view
      store.current = was.current
      prefs.level = was.level
    }
  })

  // Цель keydown бывает document/window (синтетическое событие, фокус на документе): у неё нет closest.
  // Тесты идут без DOM — Element подменяем заглушкой, у которой closest есть.
  describe('Пробел', () => {
    class FakeEl {
      blurred = false
      closest = () => ({ blur: () => (this.blurred = true) })
    }
    const press = (target: unknown, shiftKey = false) => ({ key: ' ', shiftKey, target }) as unknown as KeyboardEvent

    it('с целью-документом не бросает и листает как обычно', () => {
      vi.stubGlobal('Element', FakeEl)
      try {
        for (const target of [{}, { closest: undefined }, null]) {
          expect(() => COMMANDS.spaceDown(press(target))).not.toThrow()
          expect(canvas.space).toBe(true)
          expect(canvas.spaceUp()).toBe(1)
        }
      } finally {
        vi.unstubAllGlobals()
      }
    })

    it('снимает фокус с кнопки', () => {
      vi.stubGlobal('Element', FakeEl)
      try {
        const el = new FakeEl()
        COMMANDS.spaceDown(press(el))
        expect(el.blurred).toBe(true)
        canvas.spaceUp()
      } finally {
        vi.unstubAllGlobals()
      }
    })
  })
})
