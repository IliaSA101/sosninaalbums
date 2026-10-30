import { describe, expect, it } from 'vitest'
import { groupAxes } from './groupaxes'
import {
  angleField,
  cornerAnchor,
  cornerKRot,
  frameAngle,
  groupOBB,
  mixedAngles,
  moveField,
  reduce90,
  sideAnchor,
  sideK,
  sizeField,
  type AxesBase,
  type Frame,
  type Turn,
} from './groupframe'
import { groupBox, type GxText } from './groupxf'
import { rotate } from '../geom/textbox'
import type { RotRect } from '../core/types'

const near = (a: number, b: number, eps = 1e-6) => expect(Math.abs(a - b)).toBeLessThan(eps)

// Три места под одним углом: рамка в его осях.
const turned = (deg: number): RotRect[] => {
  const c = (x: number, y: number, w: number, h: number): RotRect => {
    const o = rotate(deg, x, y)
    return { x: 200 + o.x - w / 2, y: 140 + o.y - h / 2, w, h, rot: deg }
  }
  return [c(-120, 0, 100, 60), c(0, 0, 80, 120), c(110, 0, 60, 60)]
}

describe('groupOBB', () => {
  it('при φ = 0 это groupBox дословно', () => {
    const pls: RotRect[] = [
      { x: 10, y: 20, w: 100, h: 50 },
      { x: 150, y: 40, w: 60, h: 60, rot: 30 },
    ]
    const txs: GxText[] = [{ x: 30, y: 120, w: 90, h: 12, rot: -10, size: 12 }]
    expect(groupOBB(pls, txs, 0)).toEqual({ ...groupBox(pls, txs), rot: 0 })
  })
  it('пусто — null', () => expect(groupOBB([], [], 20)).toBeNull())
  it('места под 20° — рамка в осях 20° точно по их краям', () => {
    const f = groupOBB(turned(20), [], 20)!
    near(f.w, 310) // от −170 до +140 вдоль оси
    near(f.h, 120)
    const c = rotate(20, -15, 0) // середина по оси: (−170 + 140) / 2
    near(f.x + f.w / 2, 200 + c.x)
    near(f.y + f.h / 2, 140 + c.y)
    expect(f.rot).toBe(20)
  })
  it('жёсткий поворот: рамка в осях φ+θ — та же, повёрнутая', () => {
    const a = groupOBB(turned(0), [], 0)!
    const b = groupOBB(turned(25), [], 25)!
    near(a.w, b.w, 0.02)
    near(a.h, b.h, 0.02)
  })
})

describe('reduce90', () => {
  it('в (−45, 45]', () => {
    expect(reduce90(90)).toBe(0)
    expect(reduce90(0)).toBe(0)
    expect(reduce90(50)).toBe(-40)
    expect(reduce90(45)).toBe(45)
    expect(reduce90(-45)).toBe(45)
    expect(reduce90(-100)).toBe(-10)
    expect(reduce90(180)).toBe(0)
  })
})

describe('frameAngle', () => {
  const rots = new Map([
    ['a', 30],
    ['b', 30],
  ])
  const turn = (deg: number, r = rots): Turn => ({ key: 'k', deg, rots: new Map(r) })
  it('поворот действует: углы членов те же', () => expect(frameAngle(turn(30), 'k', rots)).toBe(30))
  it('поворот 90°: φ = 90, хотя члены повёрнуты иначе', () => {
    const r = new Map([
      ['a', 90],
      ['b', 90],
    ])
    expect(frameAngle(turn(90, r), 'k', r)).toBe(90)
  })
  it('сбит чужим углом — общий угол членов', () => {
    const now = new Map([
      ['a', 30],
      ['b', 40],
    ])
    expect(frameAngle(turn(30), 'k', now)).toBe(0)
    expect(mixedAngles(now)).toBe(true)
  })
  it('другое выделение — общий угол, иначе 0', () => {
    expect(frameAngle(turn(50), 'other', rots)).toBe(30)
    expect(frameAngle(null, 'k', rots)).toBe(30)
    expect(frameAngle(null, 'k', new Map())).toBe(0)
  })
  it('разные углы без поворота — 0 и «разные»', () => {
    const now = new Map([
      ['a', 0],
      ['b', 15],
    ])
    expect(frameAngle(null, 'k', now)).toBe(0)
    expect(mixedAngles(now)).toBe(true)
    expect(mixedAngles(rots)).toBe(false)
  })
  it('после выпрямления (поворот действует) разные углы остаются «разными»', () => {
    const r = new Map([
      ['a', 0],
      ['b', 15],
    ])
    expect(frameAngle(turn(0, r), 'k', r)).toBe(0)
    expect(mixedAngles(r)).toBe(true)
  })
})

describe('боковые ручки и углы', () => {
  const f: Frame = { x: 100, y: 100, w: 200, h: 100, rot: 0 }
  it('e/w/s/n: множитель и якорь — середина противоположной стороны', () => {
    expect(sideK(f, 'e', 100, 50)).toEqual({ kx: 1.5, ky: 1 })
    expect(sideK(f, 'w', 100, 0)).toEqual({ kx: 0.5, ky: 1 })
    expect(sideK(f, 's', 0, 50)).toEqual({ kx: 1, ky: 1.5 })
    expect(sideK(f, 'n', 0, 50)).toEqual({ kx: 1, ky: 0.5 })
    expect(sideAnchor(f, 'e')).toEqual({ x: 100, y: 150 })
    expect(sideAnchor(f, 'n')).toEqual({ x: 200, y: 200 })
  })
  it('не меньше 0,001', () => expect(sideK(f, 'e', -500, 0).kx).toBe(0.001))
  it('повёрнутая рамка: сдвиг считается в её осях', () => {
    const r: Frame = { ...f, rot: 90 } // ширина идёт вниз по экрану
    const k = sideK(r, 'e', 0, 100)
    near(k.kx, 1.5)
    const a = sideAnchor(r, 'e')
    near(a.x, 200)
    near(a.y, 50) // центр (200, 150), −w/2 вдоль оси 90° = вверх
  })
  it('угол: якорь и k у рамки 0° как в 014, у повёрнутой — от повёрнутого угла', () => {
    expect(cornerAnchor(f, 'se')).toEqual({ x: 100, y: 100 })
    near(cornerKRot(f, 'se', 100, 50), 1.5, 1e-9)
    const r: Frame = { ...f, rot: 90 }
    const a = cornerAnchor(r, 'nw') // противоположный nw — se: центр + R(90)(100, 50) = (200−50, 150+100)
    near(a.x, 150)
    near(a.y, 250)
    near(cornerKRot(r, 'se', 0, 100), 1.4, 1e-9) // экранный сдвиг вниз = вдоль ширины на 100
  })
})

describe('поля карточки → аргументы', () => {
  const f: Frame = { x: 20, y: 10, w: 100, h: 50, rot: 0 }
  const base = (places: RotRect[], texts: GxText[] = []): AxesBase => ({ sw: 562, ph: 284, places, texts })

  it('X и Y — сдвиг без масштаба', () => {
    expect(moveField(f, 'x', 30)).toEqual({ k: 1, ax: 0, ay: 0, dx: 10, dy: 0 })
    expect(moveField(f, 'y', 4)).toEqual({ k: 1, ax: 0, ay: 0, dx: 0, dy: -6 })
  })
  it('холостой ввод — null', () => {
    expect(moveField(f, 'x', NaN)).toBeNull()
    expect(moveField(f, 'x', 20.04)).toBeNull()
    expect(sizeField(f, 'w', 0, base([]))).toBeNull()
    expect(sizeField(f, 'h', -5, base([]))).toBeNull()
    expect(sizeField(f, 'w', 100.04, base([]))).toBeNull()
    expect(sizeField({ ...f, w: 0 }, 'w', 10, base([]))).toBeNull()
    expect(angleField(f, 0, NaN)).toBeNull()
    expect(angleField(f, 20, 20.04)).toBeNull()
  })
  it('Ш соосной группы — ровно v / текущее, от левой стороны', () => {
    const a = sizeField(f, 'w', 150, base([{ x: 20, y: 10, w: 100, h: 50 }]))!
    expect(a).toMatchObject({ kx: 1.5, ky: 1, phi: 0, ax: 20, ay: 35 })
  })
  it('В — по высоте от верхней стороны', () => {
    const a = sizeField(f, 'h', 25, base([{ x: 20, y: 10, w: 100, h: 50 }]))!
    expect(a).toMatchObject({ kx: 1, ky: 0.5, ax: 70, ay: 10 })
  })
  it('Ш несоосной группы: подбор до сотой', () => {
    const pls: RotRect[] = [
      { x: 10, y: 10, w: 100, h: 60 },
      { x: 130, y: 100, w: 100, h: 60, rot: 30 },
    ]
    const fr = groupOBB(pls, [], 0)!
    const v = fr.w * 1.4
    const a = sizeField(fr, 'w', v, base(pls))!
    const o = groupAxes({ ...base(pls), ...a })
    const got = groupOBB(o.places, o.texts, 0)!
    near(got.w, v, 0.05) // шаг сетки мест 0,01
  })
  it('Ш с текстом по высоте-соосной группе тоже сходится', () => {
    const pls: RotRect[] = [{ x: 10, y: 10, w: 100, h: 60 }]
    const txs: GxText[] = [{ x: 10, y: 80, w: 100, h: 12, rot: 0, size: 12, al: 0.5 }]
    const fr = groupOBB(pls, txs, 0)!
    const a = sizeField(fr, 'w', fr.w * 2, base(pls, txs))!
    near(a.kx, 2, 1e-9)
  })
  it('угол: v − φ вокруг центра рамки', () => {
    expect(angleField(f, 0, 45)).toEqual({ deg: 45, cx: 70, cy: 35 })
    expect(angleField(f, 30, 10)).toMatchObject({ deg: -20 })
    expect(angleField(f, 170, -170)).toMatchObject({ deg: 20 })
    expect(angleField(f, 0, 12.34)).toMatchObject({ deg: 12.3 })
  })
})
