import { describe, expect, it } from 'vitest'
import { windowOf, winStyle } from './border'
import { opacityOf } from '../backdrop/fade'
import { liveImage } from '../tilt/tiltlive'
import { DEFAULT_FRAME, placeImage } from '../../base/geom/frame'
import { outlineOf } from '../../base/geom/shape'
import type { FrameView, Rect } from '../../base/core/types'

const cell: Rect = { x: 10, y: 20, w: 100, h: 150 }
const fv = (o: Partial<FrameView> = {}): FrameView =>
  ({
    s: 1,
    x: 0,
    y: 0,
    r: 0,
    img: cell,
    turn: 0,
    tilt: 0,
    dpi: 0,
    flipH: false,
    flipV: false,
    gray: false,
    bright: 0,
    contrast: 0,
    filter: '',
    cellRot: 0,
    z: 0,
    ...o,
  }) as FrameView

describe('окно ячейки: без рамочки рисунок прежний', () => {
  it('нет win в View (старый мок, «Авто») — окно = место, форма и скругление места', () => {
    const w = windowOf(cell, fv({ shape: 'oval' }), 0, undefined)
    expect(w).toEqual({ win: cell, shape: 'oval', radius: 0, windowed: false })
    expect(winStyle(cell, w)).toBeUndefined()
  })
  it('нет кадра вообще (пустой слот) — то же', () => {
    expect(windowOf(cell, undefined, 4, undefined)).toEqual({ win: cell, shape: undefined, radius: 4, windowed: false })
  })
  it('win = место (Go без рамочки) — не окно: .clip занимает ячейку, форма та же', () => {
    const w = windowOf(cell, fv({ win: { ...cell }, winShape: '', winRadius: 6, shape: '', radius: 6 }), 6, [0, 0, 0, 0])
    expect(w.windowed).toBe(false)
    expect(w.radius).toBe(6)
    expect(winStyle(cell, w)).toBeUndefined()
  })
  it('галерея раскладок (preview): кадров из View нет, окна нет, форма не берётся', () => {
    const w = windowOf(cell, fv({ shape: 'oval', win: { x: 12, y: 22, w: 96, h: 146 } }), 0, [2, 2, 2, 2], true)
    expect(w).toEqual({ win: cell, shape: undefined, radius: 0, windowed: false })
  })
  it('ползунок скругления без рамочки — скругление ползунка, как раньше', () => {
    const w = windowOf(cell, fv({ win: { ...cell }, winShape: '', winRadius: 0, radius: 0 }), 7, [])
    expect(w.radius).toBe(7)
  })
})

describe('окно ячейки: с рамочкой', () => {
  const win: Rect = { x: 12, y: 22, w: 96, h: 146 } // линия 2 мм у 100×150
  const framed = (o: Partial<FrameView> = {}) => fv({ win, winShape: '', winRadius: 0, ...o })

  it('позиция окна в ячейке — проценты от места', () => {
    const w = windowOf(cell, framed(), 0, [2, 2, 2, 2])
    expect(w.windowed).toBe(true)
    expect(winStyle(cell, w)).toEqual({ left: '2%', top: `${(2 / 150) * 100}%`, width: '96%', height: `${(146 / 150) * 100}%` })
  })
  it('полароид: низ шире — окно несимметрично', () => {
    const wnd: Rect = { x: 15, y: 25, w: 90, h: 127.5 }
    const w = windowOf(cell, framed({ win: wnd }), 0, [5, 5, 5, 17.5])
    expect(winStyle(cell, w)).toEqual({ left: '5%', top: `${(5 / 150) * 100}%`, width: '90%', height: '85%' })
  })
  it('круг: окно овальное, радиуса нет', () => {
    const w = windowOf(cell, framed({ shape: 'oval', winShape: 'oval', radius: 0 }), 0, [2, 2, 2, 2])
    expect(w.shape).toBe('oval')
    expect(w.radius).toBe(0)
  })
  it('скругление окна приходит из Go (R − min отступа)', () => {
    const w = windowOf(cell, framed({ radius: 10, winRadius: 7 }), 10, [3, 3, 3, 3])
    expect(w.radius).toBe(7)
  })
  it('скругление окна пропало (R меньше отступа) — прямые углы окна', () => {
    const w = windowOf(cell, framed({ radius: 2 }), 2, [3, 3, 3, 3])
    expect(w.radius).toBe(0)
  })
  it('ползунок скругления на ходу: окно по зеркалу InsetBox, Go не ждём', () => {
    const w = windowOf(cell, framed({ radius: 10, winRadius: 7 }), 15, [3, 3, 3, 3])
    expect(w.radius).toBe(12)
    expect(windowOf(cell, framed({ radius: 10, winRadius: 7 }), 2, [3, 3, 3, 3]).radius).toBe(0)
  })
  it('кадр держится относительно окна: живое фото считается по окну, а не по месту', () => {
    const w = windowOf(cell, framed(), 0, [2, 2, 2, 2])
    const aspect = 1.5
    const inWin = liveImage(w.win, aspect, DEFAULT_FRAME, false, outlineOf(w.shape, w.radius))
    expect(inWin).toEqual(placeImage(win, aspect, DEFAULT_FRAME, undefined))
    expect(inWin).not.toEqual(liveImage(cell, aspect, DEFAULT_FRAME, false))
  })
  it('«Целиком» вписывается в окно', () => {
    const w = windowOf(cell, framed(), 0, [2, 2, 2, 2])
    const img = liveImage(w.win, 1, DEFAULT_FRAME, true)
    expect(img.w).toBeLessThanOrEqual(win.w + 1e-9)
    expect(img.h).toBeLessThanOrEqual(win.h + 1e-9)
    expect(img.x).toBeGreaterThanOrEqual(win.x - 1e-9)
  })
})

describe('прозрачность — только у фото', () => {
  const files = import.meta.glob(['/src/editor/photo/Cell.svelte', '/src/editor/border/BorderOverlay.svelte'], {
    query: '?raw',
    import: 'default',
    eager: true,
  }) as Record<string, string>
  const src = (name: string) => files[Object.keys(files).find((p) => p.endsWith(name)) ?? ''] ?? ''

  it('непрозрачность = 1 − fade; 0 % — атрибута нет', () => {
    expect(opacityOf(0)).toBeUndefined()
    expect(opacityOf(0.3)).toBeCloseTo(0.7, 9)
  })
  it('opacity стоит на .clip, накладка рамочки лежит вне его и без своей opacity', () => {
    const cellSrc = src('Cell.svelte')
    const clip = cellSrc.slice(cellSrc.indexOf('<div\n    class="clip"'), cellSrc.indexOf('{#if missing}'))
    expect(clip).toContain('style:opacity={opacityOf(fade)}')
    expect(cellSrc.match(/opacityOf\(/g)).toHaveLength(1)
    expect(src('BorderOverlay.svelte').replace(/\/\/.*$/gm, '')).not.toMatch(/opacity/)
  })
  it('рамочка идёт после .clip (поверх фото)', () => {
    const cellSrc = src('Cell.svelte')
    expect(cellSrc.indexOf('<BorderOverlay')).toBeGreaterThan(cellSrc.indexOf('class="clip"'))
  })
})

describe('пустое место с рамочкой', () => {
  const files = import.meta.glob('/src/editor/places/PlaceLayer.svelte', { query: '?raw', import: 'default', eager: true }) as Record<
    string,
    string
  >
  const src = Object.values(files)[0] ?? ''

  it('рисует ту же BorderOverlay отдельным слоем над местом, мышь сквозь неё', () => {
    expect(src).toContain('<BorderOverlay border={p.border}')
    expect(src.indexOf('<BorderOverlay')).toBeGreaterThan(src.indexOf('class="place"'))
    expect(src).toMatch(/\.pb \{[^}]*pointer-events: none/)
  })
})
