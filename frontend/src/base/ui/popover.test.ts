import { describe, expect, it } from 'vitest'
import { closesOnPeer, placePopover, placeSubmenu, type Box } from './popover'

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
