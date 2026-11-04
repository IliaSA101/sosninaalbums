import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { View } from '../../base/core/types'

// Меню правой кнопки (099): надпись выделяется, пустой лист открывает «Вставить» в мм точки щелчка.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ ClipState: vi.fn(), Undo: vi.fn() }))
vi.mock('../text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { edit } = await import('../text/edit.svelte')
const { clip } = await import('../clip/clip.svelte')
const { NO_CLIP } = await import('../clip/clip')
const { photoui, pointOnSheet } = await import('./photoui.svelte')

const sheet = (id: string) => ({
  id,
  w: 600,
  h: 300,
  mid: 300,
  grid: true,
  photos: [],
  rects: [],
  frames: [],
  texts: [{ id: 't1' }, { id: 't2' }, { id: 'd1', draft: true }],
  captions: [],
  backdrops: [],
  stickers: [],
  places: [],
})
const view = (): View =>
  ({
    rev: 1,
    marksRev: 0,
    folders: [],
    spreads: [sheet('s1')],
    settings: { pageW: 300, pageH: 300 },
    backdropUsed: [],
    safe: [],
  }) as unknown as View

// Лист на экране: левый верхний угол (100, 50), 300×150 px — в два раза меньше мм.
const root = (id = 's1', w = 300, h = 150) => ({
  getAttribute: () => id,
  getBoundingClientRect: () => ({ left: 100, top: 50, width: w, height: h }),
})
function click(x = 250, y = 125, el: unknown = root()) {
  const e = {
    clientX: x,
    clientY: y,
    target: { closest: () => el },
    preventDefault: vi.fn(),
    stopPropagation: vi.fn(),
  }
  return e as unknown as MouseEvent & { preventDefault: ReturnType<typeof vi.fn>; stopPropagation: ReturnType<typeof vi.fn> }
}

beforeEach(() => {
  store.apply(view())
  store.go(0)
  placesel.clear()
  edit.clear()
  prefs.level = 'standard'
  clip.info = { ...NO_CLIP, photos: 1 }
  photoui.close()
})

describe('pointOnSheet', () => {
  it('переводит пиксели в мм листа', () => {
    expect(pointOnSheet(click(250, 125))).toEqual({ x: 300, y: 150 })
    expect(pointOnSheet(click(100, 50))).toEqual({ x: 0, y: 0 })
  })
  it('другой лист, нулевой размер, нет листа вокруг — точки нет', () => {
    expect(pointOnSheet(click(250, 125, root('s2')))).toBeUndefined()
    expect(pointOnSheet(click(250, 125, root('s1', 0, 0)))).toBeUndefined()
    expect(pointOnSheet(click(250, 125, null))).toBeUndefined()
  })
})

describe('openSheet', () => {
  it('буфер не пуст — меню «листа» с точкой щелчка', () => {
    const e = click()
    photoui.openSheet(e)
    expect(photoui.menu).toEqual({ x: 250, y: 125, kind: 'sheet', at: { x: 300, y: 150 } })
    expect(e.preventDefault).toHaveBeenCalled()
  })
  it('буфер пуст — меню нет и событие не гасится', () => {
    clip.info = NO_CLIP
    const e = click()
    photoui.openSheet(e)
    expect(photoui.menu).toBeNull()
    expect(e.preventDefault).not.toHaveBeenCalled()
  })
  it('идёт ввод надписи — родное меню текста', () => {
    edit.select({ kind: 'text', spread: 's1', id: 't1' })
    edit.target = { kind: 'text', spread: 's1', id: 't1' }
    photoui.openSheet(click())
    expect(photoui.menu).toBeNull()
  })
  it('«Простой» уровень — меню нет', () => {
    prefs.level = 'simple'
    photoui.openSheet(click())
    expect(photoui.menu).toBeNull()
  })
  it('щелчок вне текущего листа — меню нет', () => {
    photoui.openSheet(click(250, 125, root('s2')))
    expect(photoui.menu).toBeNull()
  })
})

describe('openText', () => {
  it('невыделенная надпись выделяется, меню «надписи» с точкой', () => {
    const e = click()
    photoui.openText(e, 't1')
    expect(edit.sel).toMatchObject({ kind: 'text', id: 't1' })
    expect(photoui.menu).toMatchObject({ kind: 'text', at: { x: 300, y: 150 } })
    expect(e.stopPropagation).toHaveBeenCalled() // меню листа под ней не открывается
  })
  it('надпись в группе — выделение группы остаётся', () => {
    placesel.pickHit({ refs: [], texts: ['t1', 't2'], stickers: [] }, false)
    photoui.openText(click(), 't2')
    expect(placesel.texts).toEqual(['t1', 't2'])
    expect(photoui.menu?.kind).toBe('text')
  })
  it('уже выделенная одиночная надпись — выделение не пересоздаётся', () => {
    edit.select({ kind: 'text', spread: 's1', id: 't1' })
    const sel = edit.sel
    photoui.openText(click(), 't1')
    expect(edit.sel).toBe(sel)
  })
  it('«Простой» уровень — ни меню, ни выделения', () => {
    prefs.level = 'simple'
    photoui.openText(click(), 't1')
    expect(photoui.menu).toBeNull()
    expect(edit.sel).toBeNull()
  })
  // 131: заготовка макета правится на любом уровне (вопрос 20 а) — меню с «Сделать обычной надписью» есть и на «Простом»
  it('заготовка на «Простом» — выделяется и открывает меню', () => {
    prefs.level = 'simple'
    photoui.openText(click(), 'd1')
    expect(edit.sel).toMatchObject({ kind: 'text', id: 'd1' })
    expect(photoui.menu).toMatchObject({ kind: 'text' })
  })
})

describe('open / openSticker: точка для «Вставить»', () => {
  it('меню фото запоминает точку щелчка', () => {
    photoui.open(click(), 'p1')
    expect(photoui.menu).toMatchObject({ kind: 'photo', at: { x: 300, y: 150 } })
  })
  it('меню стикера — тоже', () => {
    photoui.openSticker(click(), 'k1')
    expect(photoui.menu).toMatchObject({ kind: 'photo', at: { x: 300, y: 150 } })
  })
})
