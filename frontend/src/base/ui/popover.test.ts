import { describe, expect, it } from 'vitest'
import { closesOnPeer, placeMenu, placePopover, placeSubmenu, placeSubmenuFit, type Box } from './popover'

const view = { width: 1280, height: 760 }
const size = { width: 240, height: 300 }
const anchor = (left: number, top: number): Box => ({ left, top, width: 28, height: 28 })

// Окно целиком внутри экрана.
function inside(p: { left: number; top: number }) {
  expect(p.left).toBeGreaterThanOrEqual(0)
  expect(p.top).toBeGreaterThanOrEqual(0)
  expect(p.left + size.width).toBeLessThanOrEqual(view.width)
  expect(p.top + size.height).toBeLessThanOrEqual(view.height)
}

describe('placePopover', () => {
  it('в середине: под якорем по левому краю', () => {
    expect(placePopover(anchor(400, 200), size, view)).toEqual({ left: 400, top: 234, above: false })
  })
  it('у правого края не выходит за экран', () => {
    const p = placePopover(anchor(1250, 200), size, view)
    inside(p)
    expect(p.left).toBe(view.width - size.width - 8)
  })
  it('у нижнего края разворачивается вверх', () => {
    const p = placePopover(anchor(400, 720), size, view)
    inside(p)
    expect(p.above).toBe(true)
    expect(p.top + size.height).toBeLessThanOrEqual(720)
  })
  it('правый нижний угол', () => {
    inside(placePopover(anchor(1260, 740), size, view))
  })
  it('у левого и верхнего края — отступ', () => {
    const p = placePopover(anchor(0, 0), size, view)
    inside(p)
    expect(p.left).toBeGreaterThanOrEqual(8)
  })
  it('мало места и сверху, и снизу: прижато к окну', () => {
    const tall = { width: 240, height: 500 }
    const v = { width: 1280, height: 600 }
    const p = placePopover(anchor(400, 300), tall, v)
    expect(p.top).toBeGreaterThanOrEqual(8)
    expect(p.top + tall.height).toBeLessThanOrEqual(v.height)
  })
  it('выросло после показа (ошибка HEX): влезало вниз — теперь переворот вверх', () => {
    const a = anchor(400, 450)
    expect(placePopover(a, { width: 188, height: 250 }, view).above).toBe(false)
    const grown = { width: 188, height: 290 }
    const p = placePopover(a, grown, view)
    expect(p.above).toBe(true)
    expect(p.top).toBeGreaterThanOrEqual(8)
    expect(p.top + grown.height).toBeLessThanOrEqual(a.top)
  })
  it('выросло и не влезает ни вниз, ни вверх: поднято, низ в окне', () => {
    const grown = { width: 188, height: 330 }
    const v = { width: 1280, height: 600 }
    const p = placePopover(anchor(400, 300), grown, v)
    expect(p.top).toBeGreaterThanOrEqual(8)
    expect(p.top + grown.height).toBeLessThanOrEqual(v.height - 8)
  })
  it('окно шире экрана не уходит левее отступа', () => {
    expect(placePopover(anchor(50, 50), { width: 400, height: 100 }, { width: 300, height: 400 }).left).toBe(8)
  })
})

describe('placeSubmenu', () => {
  const item: Box = { left: 100, top: 200, width: 220, height: 28 }
  const sub = { width: 180, height: 60 }
  it('справа от пункта, по его высоте', () => {
    expect(placeSubmenu(item, sub, view)).toEqual({ left: 320, top: 200, flipped: false })
  })
  it('у правого края окна — слева от пункта', () => {
    const edge: Box = { ...item, left: 1100 }
    expect(placeSubmenu(edge, sub, view)).toEqual({ left: 920, top: 200, flipped: true })
  })
  it('не выходит за нижний край', () => {
    const low: Box = { ...item, top: 740 }
    expect(placeSubmenu(low, sub, view).top).toBe(view.height - sub.height - 8)
  })
  it('с обеих сторон тесно — остаётся справа', () => {
    expect(placeSubmenu({ left: 10, top: 0, width: 220, height: 28 }, { width: 1200, height: 60 }, view).flipped).toBe(false)
  })
})

// Меню «Правка»: 25 пунктов по 28 px (+ рамка, отступы, разделители) ≈ 735 px; строка меню кончается на 32.
describe('placeMenu (выпадающий список строки меню)', () => {
  const bar: Box = { left: 100, top: 4, width: 60, height: 28 }
  const barBottom = bar.top + bar.height
  const list = { width: 240, height: 25 * 28 + 35 }

  it.each([760, 700, 640])('окно %i px: не выше строки меню, низ в окне, нужна прокрутка', (h) => {
    const p = placeMenu(bar, list, { width: 1280, height: h })
    expect(p.top).toBeGreaterThanOrEqual(barBottom)
    expect(p.maxHeight).not.toBeNull()
    expect(p.top + p.maxHeight!).toBeLessThanOrEqual(h - 8)
    expect(p.maxHeight!).toBeLessThan(list.height)
  })
  it('1080 px: влезает, прокрутки нет', () => {
    const p = placeMenu(bar, list, { width: 1280, height: 1080 })
    expect(p.maxHeight).toBeNull()
    expect(p.top).toBe(barBottom)
  })
  it('короткий список в низком окне прокрутки не просит', () => {
    const p = placeMenu(bar, { width: 240, height: 200 }, { width: 1280, height: 640 })
    expect(p).toEqual({ left: 100, top: barBottom, maxHeight: null })
  })
  it('у правого края окна сдвигается влево', () => {
    expect(placeMenu({ ...bar, left: 1250 }, list, { width: 1280, height: 1080 }).left).toBe(1280 - 240 - 8)
  })
})

describe('placeSubmenuFit (подменю строки меню)', () => {
  const minTop = 32
  const view640 = { width: 1280, height: 640 }
  const sub = { width: 180, height: 6 * 28 + 8 }
  it('стоит у своего пункта, в том числе у прокрученного родителя', () => {
    const item: Box = { left: 100, top: 300, width: 240, height: 28 }
    expect(placeSubmenuFit(item, sub, view640, minTop)).toEqual({ left: 340, top: 300, flipped: false, maxHeight: null })
  })
  it('пункт у низа окна: подменю поднято, низ в окне', () => {
    const item: Box = { left: 100, top: 610, width: 240, height: 28 }
    const p = placeSubmenuFit(item, sub, view640, minTop)
    expect(p.top + sub.height).toBeLessThanOrEqual(640 - 8)
    expect(p.top).toBeGreaterThanOrEqual(minTop)
    expect(p.maxHeight).toBeNull()
  })
  it('пункт прокручен выше строки меню: подменю не заходит на неё', () => {
    const p = placeSubmenuFit({ left: 100, top: 10, width: 240, height: 28 }, sub, view640, minTop)
    expect(p.top).toBe(minTop)
  })
  it('подменю выше окна — ограничено и прокручивается', () => {
    const p = placeSubmenuFit({ left: 100, top: 300, width: 240, height: 28 }, { width: 180, height: 900 }, view640, minTop)
    expect(p.top).toBe(minTop)
    expect(p.maxHeight).toBe(640 - 8 - minTop)
  })
  it('у правого края — слева от пункта', () => {
    const p = placeSubmenuFit({ left: 1100, top: 300, width: 240, height: 28 }, sub, view640, minTop)
    expect(p.flipped).toBe(true)
  })
})

describe('closesOnPeer (открылось другое окно панели)', () => {
  const a = {}
  const other = {}
  const inner = {}
  const pop = { contains: (n: unknown) => n === inner }
  it('чужой якорь вне окна — закрыться (Папки закрываются, когда клавиша B открыла «Подборки»)', () => {
    expect(closesOnPeer(other, { anchor: a, pop })).toBe(true)
  })
  it('своё открытие не закрывает себя', () => {
    expect(closesOnPeer(a, { anchor: a, pop })).toBe(false)
  })
  it('вложенное окно (якорь внутри этого) не закрывает родителя', () => {
    expect(closesOnPeer(inner, { anchor: a, pop })).toBe(false)
  })
  it('нет якоря в событии или окна ещё нет — закрыться по чужому якорю нечем/можно', () => {
    expect(closesOnPeer(undefined, { anchor: a, pop })).toBe(false)
    expect(closesOnPeer(other, { anchor: a, pop: null })).toBe(true)
  })
})
