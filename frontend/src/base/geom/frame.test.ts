import { describe, expect, it } from 'vitest'
import vectors from '../../../../testdata/parity/frames.json'
import containVectors from '../../../../testdata/parity/contain.json'
import tiltVectors from '../../../../testdata/parity/tilt.json'
import {
  clampFrame,
  containImage,
  effectiveDPI,
  fitTilt,
  flipCss,
  hasSlack,
  isDefaultFrame,
  panFrame,
  photoCss,
  placeImage,
  rotateFrame,
  thumbBox,
  tiltFrame,
  zoomFrame,
} from './frame'
import type { Frame, Rect } from '../core/types'
import type { Outline } from './shape'

interface Vector {
  cell: Rect
  aspect: number
  frame: Frame
  px: number
  outline?: Outline // форма места (044); нет — прямоугольник
  img: Rect
  dpi: number
}

describe('общие векторы с Go (frames.json)', () => {
  const list = vectors as Vector[]
  it('векторы прочитаны', () => expect(list.length).toBeGreaterThan(0))
  it('есть векторы с поворотом', () => expect(list.some((v) => v.frame.r)).toBe(true))
  it('есть векторы с формой: овал и скругление', () => {
    expect(list.some((v) => v.outline?.oval)).toBe(true)
    expect(list.some((v) => (v.outline?.radius ?? 0) > 0)).toBe(true)
  })
  list.forEach((v, i) => {
    it(`вектор ${i}: img и dpi`, () => {
      const img = placeImage(v.cell, v.aspect, v.frame, v.outline)
      expect(img.x).toBeCloseTo(v.img.x, 6)
      expect(img.y).toBeCloseTo(v.img.y, 6)
      expect(img.w).toBeCloseTo(v.img.w, 6)
      expect(img.h).toBeCloseTo(v.img.h, 6)
      expect(effectiveDPI(v.px, img)).toBe(v.dpi)
    })
  })
})

describe('effectiveDPI', () => {
  it('px = 0 — 0', () => expect(effectiveDPI(0, { x: 0, y: 0, w: 100, h: 100 })).toBe(0))
  it('нулевая ширина — 0', () => expect(effectiveDPI(6000, { x: 0, y: 0, w: 0, h: 0 })).toBe(0))
})

describe('hasSlack', () => {
  const cell = { x: 0, y: 0, w: 100, h: 100 }
  it('фото ровно в ячейке — хода нет', () => expect(hasSlack(cell, cell)).toBe(false))
  it('в пределах 0.5 мм — нет', () => expect(hasSlack(cell, { x: 0, y: 0, w: 100.4, h: 100 })).toBe(false))
  it('по одной оси — есть', () => expect(hasSlack(cell, { x: 0, y: 0, w: 150, h: 100 })).toBe(true))
})

describe('panFrame', () => {
  const cell = { x: 0, y: 0, w: 100, h: 100 }
  const f: Frame = { s: 2, x: 0, y: 0 }
  const img = placeImage(cell, 1, f) // 200×200, ход 100 по каждой оси
  it('фото едет за курсором: сдвиг вправо уменьшает x', () => {
    const r = panFrame(f, cell, img, 25, 0)
    expect(r.x).toBeCloseTo(-0.5)
    expect(r.y).toBe(0)
    // на экране картинка сдвинулась ровно на dx
    expect(placeImage(cell, 1, r).x - img.x).toBeCloseTo(25)
  })
  it('зажимается в −1..1', () => {
    expect(panFrame(f, cell, img, 1000, -1000)).toEqual({ s: 2, x: -1, y: 1 })
  })
  it('нулевой ход по оси — значение не меняется', () => {
    const wide = placeImage(cell, 2, { s: 1, x: 0.3, y: 0.4 }) // 200×100: ход только по x
    const r = panFrame({ s: 1, x: 0.3, y: 0.4 }, cell, wide, 10, 10)
    expect(r.y).toBe(0.4)
    expect(r.x).not.toBe(0.3)
  })
})

describe('zoomFrame и clampFrame', () => {
  it('сохраняет сдвиг и зажимает масштаб', () => {
    expect(zoomFrame({ s: 1, x: 0.5, y: -0.5 }, 2.5)).toEqual({ s: 2.5, x: 0.5, y: -0.5 })
    expect(zoomFrame({ s: 1, x: 0, y: 0 }, 9).s).toBe(4)
    expect(zoomFrame({ s: 2, x: 0, y: 0 }, 0.2).s).toBe(1)
    expect(zoomFrame({ s: 2, x: 0, y: 0 }, NaN).s).toBe(2)
  })
  it('clampFrame: NaN и выход за пределы', () => {
    expect(clampFrame({ s: NaN, x: NaN, y: 5 })).toEqual({ s: 1, x: 0, y: 1 })
    expect(clampFrame({ s: 10, x: -3, y: 0.2 })).toEqual({ s: 4, x: -1, y: 0.2 })
  })
  it('isDefaultFrame', () => {
    expect(isDefaultFrame({ s: 1, x: 0, y: 0 })).toBe(true)
    expect(isDefaultFrame({ s: 1.01, x: 0, y: 0 })).toBe(false)
  })
})

describe('flipCss', () => {
  it('без зеркала — ничего', () => {
    expect(flipCss()).toBeUndefined()
    expect(flipCss({ flipH: false, flipV: false })).toBeUndefined()
  })
  it('по осям', () => {
    expect(flipCss({ flipH: true })).toBe('scale(-1, 1)')
    expect(flipCss({ flipV: true })).toBe('scale(1, -1)')
    expect(flipCss({ flipH: true, flipV: true })).toBe('scale(-1, -1)')
  })
})

describe('поворот фото (r)', () => {
  const cell = { x: 10, y: 20, w: 100, h: 60 }
  const center = (r: Rect) => [r.x + r.w / 2, r.y + r.h / 2]
  it('r не теряется в clampFrame, zoomFrame, panFrame, rotateFrame', () => {
    const f: Frame = { s: 1.5, x: 0.2, y: -0.2, r: 12.5 }
    expect(clampFrame(f).r).toBe(12.5)
    expect(zoomFrame(f, 2).r).toBe(12.5)
    expect(rotateFrame(f, 30).r).toBe(30)
    expect(rotateFrame(f, NaN).r).toBe(12.5)
    expect(panFrame(f, cell, placeImage(cell, 1.5, f), 3, 2).r).toBe(12.5)
  })
  it('без поворота поля r в кадре нет', () => {
    expect(zoomFrame({ s: 1, x: 0, y: 0 }, 2)).toEqual({ s: 2, x: 0, y: 0 })
    expect('r' in panFrame({ s: 2, x: 0, y: 0 }, cell, placeImage(cell, 1.5, { s: 2, x: 0, y: 0 }), 1, 1)).toBe(false)
    expect(rotateFrame({ s: 1, x: 0, y: 0, r: 5 }, 0)).toEqual({ s: 1, x: 0, y: 0 })
  })
  it('центр фото стоит на центре рамки при x=y=0', () => {
    const img = placeImage(cell, 1.5, { s: 1, x: 0, y: 0, r: 20 })
    expect(center(img)[0]).toBeCloseTo(60, 6)
    expect(center(img)[1]).toBeCloseTo(50, 6)
  })
  it('фото закрывает все углы повёрнутой рамки', () => {
    const [sn, cs] = [Math.sin((33 * Math.PI) / 180), Math.cos((33 * Math.PI) / 180)]
    const f: Frame = { s: 1.3, x: 0.7, y: -0.4, r: 33 }
    const img = placeImage(cell, 1.5, f)
    const [cx, cy] = center(img)
    for (const [px, py] of [
      [10, 20],
      [110, 20],
      [10, 80],
      [110, 80],
    ]) {
      // угол рамки — в оси фото: поворот на −r вокруг центра фото
      const dx = px - cx
      const dy = py - cy
      expect(Math.abs(dx * cs + dy * sn)).toBeLessThanOrEqual(img.w / 2 + 1e-9)
      expect(Math.abs(-dx * sn + dy * cs)).toBeLessThanOrEqual(img.h / 2 + 1e-9)
    }
  })
  it('panFrame в осях: при r=90 сдвиг вправо двигает y, фото едет за курсором', () => {
    const f: Frame = { s: 2, x: 0, y: 0, r: 90 }
    const img = placeImage(cell, 1.5, f)
    const p = panFrame(f, cell, img, 10, 0)
    expect(p.x).toBe(0)
    expect(p.y).not.toBe(0)
    const moved = placeImage(cell, 1.5, p)
    expect(center(moved)[0] - center(img)[0]).toBeCloseTo(10, 6)
    expect(center(moved)[1] - center(img)[1]).toBeCloseTo(0, 6)
  })
  it('panFrame под произвольным углом: центр фото сдвигается ровно на курсор', () => {
    const f: Frame = { s: 2, x: 0.1, y: 0.1, r: 27 }
    const img = placeImage(cell, 1.5, f)
    const p = panFrame(f, cell, img, 4, -3)
    const moved = placeImage(cell, 1.5, p)
    expect(center(moved)[0] - center(img)[0]).toBeCloseTo(4, 6)
    expect(center(moved)[1] - center(img)[1]).toBeCloseTo(-3, 6)
  })
  it('hasSlack считает от рамки в осях фото', () => {
    const f: Frame = { s: 1, x: 0, y: 0, r: 90 }
    // фото 3:2 в рамке 100×60 при r=90: рамка в осях фото 60×100, ход есть по ширине
    expect(hasSlack(cell, placeImage(cell, 1.5, f), 90)).toBe(true)
    expect(hasSlack({ x: 0, y: 0, w: 100, h: 100 }, placeImage({ x: 0, y: 0, w: 100, h: 100 }, 1, { s: 1, x: 0, y: 0, r: 90 }), 90)).toBe(
      false,
    )
  })
})

describe('photoCss, thumbBox, fitTilt', () => {
  it('поворот после зеркала', () => {
    expect(photoCss()).toBeUndefined()
    expect(photoCss({ flipH: true })).toBe('scale(-1, 1)')
    expect(photoCss({ r: 90 })).toBe('rotate(90deg)')
    expect(photoCss({ r: -12.5, flipV: true })).toBe('rotate(-12.5deg) scale(1, -1)')
  })
  it('thumbBox: при 90° фото 3:2 в квадрате — вертикальное, по центру', () => {
    const box = thumbBox({ x: 0, y: 0, w: 100, h: 100 }, 1.5, 90)
    // в осях фото 150×100 → ширина 150 %, высота 100 %, центр по ячейке
    expect(box).toBe('left:-25%;top:0%;width:150%;height:100%')
  })
  it('fitTilt', () => {
    expect(fitTilt(0, 1.5)).toBe(1)
    expect(fitTilt(10, 1.5)).toBeLessThan(1)
    expect(fitTilt(-10, 1.5)).toBeCloseTo(fitTilt(10, 1.5), 12)
  })
})

interface ContainVector {
  cell: Rect
  aspect: number
  r: number
  outline?: Outline
  img: Rect
}

describe('contain parity', () => {
  const list = containVectors as ContainVector[]
  it('векторы прочитаны: все формы и углы', () => {
    expect(list.length).toBeGreaterThanOrEqual(12)
    expect(list.some((v) => v.outline?.oval)).toBe(true)
    expect(list.some((v) => (v.outline?.radius ?? 0) > 0)).toBe(true)
    for (const r of [0, 30, -44.9, 90]) expect(list.some((v) => v.r === r)).toBe(true)
  })
  list.forEach((v, i) => {
    it(`вектор ${i}: img`, () => {
      const img = containImage(v.cell, v.aspect, v.r, v.outline)
      for (const k of ['x', 'y', 'w', 'h'] as const) expect(img[k]).toBeCloseTo(v.img[k], 9)
    })
  })
  it('нулевой и бесконечный aspect — как 1', () => {
    const cell = { x: 0, y: 0, w: 100, h: 60 }
    expect(containImage(cell, 0, 0)).toEqual(containImage(cell, 1, 0))
    expect(containImage(cell, Infinity, 30)).toEqual(containImage(cell, 1, 30))
  })
})

interface TiltVector {
  cell: Rect
  aspect: number
  outline?: Outline
  base: Frame
  r: number
  want: Frame
}

describe('tilt parity', () => {
  const list = tiltVectors as TiltVector[]
  it('векторы прочитаны: формы, четверти, отрицательный наклон, пределы S', () => {
    expect(list.length).toBeGreaterThanOrEqual(12)
    expect(list.some((v) => v.outline?.oval)).toBe(true)
    expect(list.some((v) => (v.outline?.radius ?? 0) > 0)).toBe(true)
    expect(list.some((v) => v.r < 0)).toBe(true)
    expect(list.some((v) => v.r > 90 && v.r < 100)).toBe(true)
    expect(list.some((v) => v.want.s === 1 && v.base.s > 1)).toBe(true)
  })
  list.forEach((v, i) => {
    it(`вектор ${i}: кадр`, () => {
      const got = tiltFrame(v.cell, v.aspect, v.base, v.r, v.outline)
      for (const k of ['s', 'x', 'y'] as const) expect(got[k]).toBeCloseTo(v.want[k], 9)
      expect(got.r ?? 0).toBe(v.want.r ?? 0)
    })
  })
  const cell = { x: 0, y: 0, w: 150, h: 100 }
  it('тот же угол — кадр как есть; без приближения S остаётся 1', () => {
    const f = { s: 1.5, x: 0.2, y: -0.1, r: 3 }
    expect(tiltFrame(cell, 1.5, f, 3)).toBe(f)
    expect(tiltFrame(cell, 1.5, { s: 1, x: 0, y: 0, r: 3 }, 0).s).toBe(1)
  })
  it('нулевой и бесконечный aspect — как 1', () => {
    const f = { s: 2, x: 0.3, y: 0.3 }
    expect(tiltFrame(cell, 0, f, 5)).toEqual(tiltFrame(cell, 1, f, 5))
    expect(tiltFrame(cell, Infinity, f, 5)).toEqual(tiltFrame(cell, 1, f, 5))
  })
})
