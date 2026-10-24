import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { can, type Level } from '../../base/prefs/levels'
import { features, fromBase } from '../../base/prefs/levelcustom'
import type { SheetInfo } from '../guide/show'
import { ringBox } from '../guide/spot'
import { TOUR } from './steps'
import {
  activeSteps,
  cardPlace,
  enterSteps,
  hasPhotoOnSpread,
  isLive,
  resolve,
  shouldAutostart,
  withMissing,
  type AutoState,
  type Box,
  type TourCtx,
} from './tour'

const sheet = (o: Partial<SheetInfo> = {}): SheetInfo => ({ kind: '', grid: false, photos: 2, texts: 0, ...o })
const step = (id: string) => TOUR.find((s) => s.id === id)!

// Полный уровень, один обычный разворот, ничего не выделено.
const ctx = (o: Partial<TourCtx> = {}): TourCtx => ({
  level: 'full',
  custom: null,
  can: (f) => can(f, 'full'),
  sheet: false,
  spreads: [sheet()],
  index: 0,
  cover: null,
  onCover: false,
  selPhoto: false,
  selText: false,
  selAny: false,
  libPhotos: 3,
  libSel: false,
  cardBody: true,
  big: false,
  bgMode: false,
  galleryOpen: false,
  menusOpen: false,
  zoomed: false,
  sources: true,
  ...o,
})
const empty = (o: Partial<TourCtx> = {}) => ctx({ spreads: [], index: -1, libPhotos: 0, sources: false, ...o })
const kinds = (c: TourCtx, id: string, first = false) => enterSteps(step(id), c, first).map((s) => s.do)

beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

describe('вход', () => {
  it('шагов level и custom нет никогда', () => {
    for (const s of TOUR)
      for (const first of [true, false]) expect(kinds(ctx({ onCover: true, big: true }), s.id, first)).not.toContain('level')
    for (const s of TOUR) expect(kinds(ctx(), s.id)).not.toContain('custom')
  })

  it('шаги 1–4 и 7 вид не двигают; на первом шаге прогона сбрасывается только включённое', () => {
    for (const id of ['folders', 'library', 'autobuild', 'timeline', 'export']) expect(kinds(ctx(), id)).toEqual([])
    expect(kinds(ctx({ big: true, bgMode: true, galleryOpen: true, menusOpen: true }), 'folders', true)).toEqual([
      'unbig',
      'bgExit',
      'closeGallery',
      'closeMenus',
    ])
    expect(kinds(ctx(), 'folders', true)).toEqual([])
  })

  it('шаг 5 с обложки, «Ручной» и виньетки — go к первому «Авто»; на «Авто» — остаётся', () => {
    const list = [sheet({ grid: true }), sheet({ kind: 'vignette' }), sheet()]
    expect(enterSteps(step('layouts'), ctx({ spreads: list, index: 0 }), false)).toEqual([{ do: 'go', index: 2 }])
    expect(enterSteps(step('layouts'), ctx({ spreads: list, index: 1 }), false)).toEqual([{ do: 'go', index: 2 }])
    expect(enterSteps(step('layouts'), ctx({ spreads: list, index: 0, onCover: true }), false)).toEqual([{ do: 'go', index: 2 }])
    expect(enterSteps(step('layouts'), ctx({ spreads: list, index: 2 }), false)).toEqual([])
  })

  it('шаг 5: выделено на листе — clearSel после go; без выделения и без «Авто» его нет', () => {
    const list = [sheet({ grid: true }), sheet()]
    expect(kinds(ctx({ spreads: [sheet()], selAny: true }), 'layouts')).toEqual(['clearSel'])
    expect(kinds(ctx({ spreads: list, index: 0, selAny: true }), 'layouts')).toEqual(['go', 'clearSel'])
    expect(kinds(ctx({ spreads: list, index: 1 }), 'layouts')).toEqual([])
    expect(kinds(ctx({ spreads: [sheet({ grid: true })], selAny: true }), 'layouts')).toEqual([])
    // остальные шаги выделение не снимают
    for (const id of ['folders', 'library', 'autobuild', 'timeline', 'photo', 'export'])
      expect(kinds(ctx({ selAny: true }), id)).not.toContain('clearSel')
  })

  it('шаг 5: «Авто» нет — лист не трогаем; зум вписывается; открытая галерея закрывается', () => {
    expect(kinds(ctx({ spreads: [sheet({ grid: true })] }), 'layouts')).toEqual([])
    expect(kinds(ctx({ zoomed: true }), 'layouts')).toEqual(['fit'])
    expect(kinds(ctx({ galleryOpen: true }), 'layouts')).toEqual(['closeGallery'])
  })

  it('шаг 6: go к развороту с фото и selectPhoto; на листе с фото — только selectPhoto', () => {
    const list = [sheet({ photos: 0 }), sheet({ photos: 3 })]
    expect(enterSteps(step('photo'), ctx({ spreads: list, index: 0 }), false)).toEqual([{ do: 'go', index: 1 }, { do: 'selectPhoto' }])
    expect(enterSteps(step('photo'), ctx({ spreads: list, index: 1 }), false)).toEqual([{ do: 'selectPhoto' }])
    expect(enterSteps(step('photo'), ctx({ spreads: list, index: 1, onCover: true }), false)).toEqual([
      { do: 'go', index: 1 },
      { do: 'selectPhoto' },
    ])
  })

  it('шаг 6: уже выделенное фото не трогаем; фото нигде нет — ничего; зум вписывается', () => {
    expect(kinds(ctx({ selPhoto: true }), 'photo')).toEqual([])
    expect(kinds(ctx({ spreads: [sheet({ photos: 0 })] }), 'photo')).toEqual([])
    expect(kinds(empty(), 'photo')).toEqual([])
    expect(kinds(ctx({ zoomed: true, selPhoto: true }), 'photo')).toEqual(['fit'])
  })

  it('«Свой» без «Вариантов раскладки» — шаг 5 пропущен, «из 6»', () => {
    const off = features('custom', { ...fromBase('full'), off: ['variants'] })
    const steps = activeSteps((f) => off.has(f))
    expect(steps.map((s) => s.id)).not.toContain('layouts')
    expect(steps).toHaveLength(6)
    expect(activeSteps((f) => can(f, 'full'))).toHaveLength(7)
  })

  it('продолжение показывает только шаги 5–7', () => {
    expect(activeSteps((f) => can(f, 'full'), 'more').map((s) => s.id)).toEqual(['layouts', 'photo', 'export'])
    expect(activeSteps((f) => can(f, 'simple'), 'auto').length).toBeGreaterThanOrEqual(5)
  })
})

describe('живое место', () => {
  it('без папок шаг 1 «нет», с папкой — «есть»; шаги вида не нужны', () => {
    expect(resolve(step('folders'), empty()).live).toBe(false)
    expect(resolve(step('folders'), empty()).text).toContain('+ Папка с фото')
    const c = empty({ sources: true })
    expect(resolve(step('folders'), c).live).toBe(true)
    expect(resolve(step('folders'), c).text).toContain('Фото уже в ленте')
    expect(enterSteps(step('folders'), c, false)).toEqual([])
  })

  it('лента, автосборка, развороты зависят от фото и разворотов', () => {
    expect(isLive('library', empty())).toBe(false)
    expect(isLive('autobuild', empty())).toBe(false)
    expect(resolve(step('autobuild'), empty()).text).toContain('Кнопка заработает')
    expect(resolve(step('autobuild'), ctx()).text).not.toContain('Кнопка заработает')
    expect(isLive('timeline', empty())).toBe(false)
    expect(isLive('timeline', ctx())).toBe(true)
    expect(resolve(step('timeline'), empty()).spot).toBe('ui:timeline')
  })

  it('шаг 5 живой на «Авто»; шаг 6 без выделения — холст', () => {
    expect(resolve(step('layouts'), ctx())).toMatchObject({ live: true, spot: 'ui:layout-all' })
    expect(resolve(step('layouts'), ctx({ spreads: [sheet({ grid: true })] }))).toMatchObject({ live: false, spot: 'ui:canvas' })
    expect(resolve(step('photo'), ctx({ selPhoto: true }))).toMatchObject({ live: true, spot: 'ui:mini-panel' })
    const none = resolve(step('photo'), ctx())
    expect(none).toMatchObject({ live: false, spot: 'ui:canvas' })
    expect(none.text).toContain('Щёлкните по фото')
  })

  it('экспорт всегда живой, клавиша подставлена', () => {
    const r = resolve(step('export'), empty())
    expect(r).toMatchObject({ live: true, spot: 'ui:topbar-export' })
    expect(r.text).toContain('(Ctrl+E)')
  })

  it('текст шага 6 зависит от уровня', () => {
    const at = (lv: Level) => resolve(step('photo'), ctx({ selPhoto: true, can: (f) => can(f, lv) })).text
    expect(at('simple').length).toBeLessThan(at('full').length)
  })

  it('withMissing добавляет пояснение', () => {
    expect(withMissing('Текст.', true)).toBe('Текст. Сейчас этого места на экране нет.')
    expect(withMissing('Текст.', false)).toBe('Текст.')
  })

  it('hasPhotoOnSpread', () => {
    expect(hasPhotoOnSpread({ spreads: [sheet({ photos: 0 })] })).toBe(false)
    expect(hasPhotoOnSpread({ spreads: [sheet({ photos: 0 }), sheet()] })).toBe(true)
  })
})

describe('карточка', () => {
  const card = { w: 320, h: 200 }
  const inside = (p: { left: number; top: number }, win: { w: number; h: number }) =>
    p.left >= 8 && p.top >= 8 && p.left + card.w <= win.w - 8 && p.top + card.h <= win.h - 8
  const overlaps = (p: { left: number; top: number }, r: Box) =>
    p.left < r.left + r.width && p.left + card.w > r.left && p.top < r.top + r.height && p.top + card.h > r.top

  // Кольца шести видов мест в окне W×H: шапка, холст-кнопка ⊞, столбец, лента разворотов, лента фото, кнопка папки.
  const wins = [
    { w: 1200, h: 640 },
    { w: 1024, h: 700 },
  ]
  const rings = (win: { w: number; h: number }): Record<string, Box> => ({
    export: { left: win.w - 100, top: 4, width: 90, height: 40 },
    layoutAll: { left: win.w / 2 - 14, top: 52, width: 28, height: 28 },
    miniPanel: { left: win.w / 2 + 100, top: 150, width: 40, height: 200 },
    timeline: { left: 4, top: win.h - 210, width: win.w - 8, height: 78 },
    library: { left: 4, top: win.h - 130, width: win.w - 8, height: 126 },
    folders: { left: 8, top: win.h - 172, width: 140, height: 32 },
  })

  it('12 положений цели: в окне и кольцо не перекрыто', () => {
    for (const win of wins)
      for (const [name, ring] of Object.entries(rings(win))) {
        const p = cardPlace(ring, card, win)
        expect(inside(p, win), `${win.w}×${win.h} ${name}: в окне`).toBe(true)
        expect(overlaps(p, ring), `${win.w}×${win.h} ${name}: не перекрывает`).toBe(false)
      }
  })

  // QA 0.25: шаг 2 «Лента фото» в 1024×700 — элемент у левого и нижнего края, кольцо вылезало за окно (низ 704 при 700)
  // и карточка встала вплотную/внахлёст. Берём прямоугольник элемента, как его отдаёт getBoundingClientRect.
  it('шаг 2 в 1024×700: кольцо в окне, карточка в окне и кольцо не перекрывает', () => {
    const win = { w: 1024, h: 700 }
    const ring = ringBox({ left: 0, top: 468, width: 624, height: 232 }, win)
    expect(ring.left).toBeGreaterThanOrEqual(0)
    expect(ring.top + ring.height).toBeLessThanOrEqual(win.h)
    for (const h of [160, 180, 181, 240]) {
      const p = cardPlace(ring, { w: 320, h }, win)
      const c = { w: 320, h }
      expect(p.left >= 8 && p.top >= 8 && p.left + c.w <= win.w - 8 && p.top + c.h <= win.h - 8, `h=${h}: в окне`).toBe(true)
      expect(p.top + h + 12, `h=${h}: от кольца ≥ GAP`).toBeLessThanOrEqual(ring.top)
    }
  })

  it('цель больше 60 % окна (холст) — внутри у правого нижнего угла', () => {
    for (const win of wins) {
      const canvas: Box = { left: 4, top: 40, width: win.w - 8, height: win.h - 160 }
      const p = cardPlace(canvas, card, win)
      expect(inside(p, win)).toBe(true)
      expect(p.left + card.w).toBeGreaterThan(canvas.left + canvas.width - 40)
      expect(p.top + card.h).toBeGreaterThan(canvas.top + canvas.height - 40)
    }
  })

  it('без кольца — по центру', () => {
    expect(cardPlace(null, card, wins[0])).toEqual({ left: 440, top: 220 })
  })

  it('кольцо в углу: карточка рядом снизу, не выходит за край', () => {
    const win = wins[0]
    const p = cardPlace({ left: win.w - 60, top: 4, width: 56, height: 36 }, card, win)
    expect(p.left + card.w).toBeLessThanOrEqual(win.w - 8)
    expect(p.top).toBeGreaterThanOrEqual(40)
  })
})

describe('автозапуск', () => {
  const ok: AutoState = { due: true, startOpen: false, modal: false, loaded: true, opening: false, running: false }
  it('идёт, когда все условия выполнены', () => {
    expect(shouldAutostart(ok)).toBe(true)
  })
  it('ждёт стартовое окно, любое окно, загрузку, «Открываем альбом»; уже идущий — не повторяет', () => {
    expect(shouldAutostart({ ...ok, startOpen: true })).toBe(false)
    expect(shouldAutostart({ ...ok, modal: true })).toBe(false)
    expect(shouldAutostart({ ...ok, loaded: false })).toBe(false)
    expect(shouldAutostart({ ...ok, opening: true })).toBe(false)
    expect(shouldAutostart({ ...ok, running: true })).toBe(false)
    expect(shouldAutostart({ ...ok, due: false })).toBe(false)
  })
})
