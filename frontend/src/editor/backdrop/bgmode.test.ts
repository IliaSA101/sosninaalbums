import { describe, expect, it } from 'vitest'
import { backdropAt, bdSession, inBox, keepMode, liveImg, modeClick } from './bgmode'
import type { BackdropView, SpreadView } from '../../base/core/types'

const bd = (o: Partial<BackdropView>): BackdropView => ({
  scope: 'spread',
  photo: 'p1',
  area: { x: 0, y: 0, w: 600, h: 300 },
  box: { x: 0, y: 0, w: 600, h: 300, rot: 0 },
  img: { x: 0, y: -50, w: 600, h: 400 },
  frame: { s: 1, x: 0, y: 0 },
  flipH: false,
  flipV: false,
  dpi: 300,
  fade: 0,
  ...o,
})

describe('inBox', () => {
  it('прямая рамка', () => {
    const b = { x: 10, y: 10, w: 100, h: 50, rot: 0 }
    expect(inBox(b, { x: 60, y: 30 })).toBe(true)
    expect(inBox(b, { x: 5, y: 30 })).toBe(false)
  })
  it('повёрнутая на 90° вокруг левого верхнего угла: рамка уходит влево от (x, y)', () => {
    const b = { x: 100, y: 100, w: 100, h: 50, rot: 90 }
    // Ось u смотрит вниз, ось v — влево: точка (u=50, v=25) = (75, 150)
    expect(inBox(b, { x: 75, y: 150 })).toBe(true)
    expect(inBox(b, { x: 150, y: 120 })).toBe(false) // там, где рамка была бы без поворота
  })
  it('45°: угол описанного прямоугольника — вне рамки', () => {
    const b = { x: 0, y: 0, w: 100, h: 100, rot: 45 }
    expect(inBox(b, { x: 0, y: 70 })).toBe(true) // центр (0, ≈70.7)
    expect(inBox(b, { x: 60, y: 5 })).toBe(false)
  })
})

describe('backdropAt', () => {
  const left = bd({ scope: 'left', area: { x: 0, y: 0, w: 300, h: 300 }, box: { x: 0, y: 0, w: 300, h: 300, rot: 0 } })
  const right = bd({ scope: 'right', area: { x: 300, y: 0, w: 300, h: 300 }, box: { x: 350, y: 50, w: 100, h: 100, rot: 30 } })
  it('страницы: своя область и своя рамка', () => {
    expect(backdropAt([left, right], { x: 100, y: 100 })).toBe('left')
    expect(backdropAt([left, right], { x: 380, y: 100 })).toBe('right')
    expect(backdropAt([left, right], { x: 550, y: 250 })).toBeNull() // правая страница мимо рамки
  })
  it('рамка за краем области: снаружи области фото не видно — не попадание', () => {
    const r = bd({ scope: 'right', area: { x: 300, y: 0, w: 300, h: 300 }, box: { x: 200, y: 0, w: 300, h: 300, rot: 0 } })
    expect(backdropAt([r], { x: 250, y: 100 })).toBeNull()
    expect(backdropAt([r], { x: 350, y: 100 })).toBe('right')
  })
  it('весь разворот; пусто — null', () => {
    expect(backdropAt([bd({})], { x: 450, y: 10 })).toBe('spread')
    expect(backdropAt([], { x: 1, y: 1 })).toBeNull()
    expect(backdropAt(undefined, { x: 1, y: 1 })).toBeNull()
  })
})

describe('modeClick', () => {
  it('мимо фона — выход, по чужому — переход, по своему — ничего', () => {
    expect(modeClick(null, 'left')).toBe('exit')
    expect(modeClick('right', 'left')).toBe('switch')
    expect(modeClick('left', 'left')).toBe('none')
  })
})

describe('keepMode', () => {
  const sp = (id: string, bs: BackdropView[]) => ({ id, backdrops: bs }) as unknown as SpreadView
  it('режим жив, пока разворот текущий и фон области есть', () => {
    expect(keepMode({ spread: 's1', scope: 'spread' }, sp('s1', [bd({})]))).toBe(true)
    expect(keepMode({ spread: 's1', scope: 'left' }, sp('s1', [bd({})]))).toBe(false) // фон снят или стал другой формы
    expect(keepMode({ spread: 's1', scope: 'spread' }, sp('s2', [bd({})]))).toBe(false) // другой разворот
    expect(keepMode({ spread: 's1', scope: 'spread' }, null)).toBe(false)
    expect(keepMode(null, sp('s1', [bd({})]))).toBe(false)
  })
  it('появилось выделение — режим закрывается', () => {
    expect(keepMode({ spread: 's1', scope: 'spread' }, sp('s1', [bd({})]), true)).toBe(false)
    expect(keepMode({ spread: 's1', scope: 'spread' }, sp('s1', [bd({})]), false)).toBe(true)
  })
})

describe('liveImg', () => {
  it('рамка без изменений — то же, что дал Go', () => {
    const b = bd({})
    expect(liveImg(b, { box: { ...b.box }, frame: b.frame })).toEqual(b.img)
  })
  it('рамка уже — фото по высоте, пропорции фото сохраняются', () => {
    const b = bd({})
    const r = liveImg(b, { box: { x: 0, y: 0, w: 150, h: 300, rot: 0 }, frame: { s: 1, x: 0, y: 0 } })
    expect(r.h).toBeCloseTo(300)
    expect(r.w).toBeCloseTo(450)
    expect(r.x).toBeCloseTo(-150)
  })
})

it('сессия стрелок подходит Go', () => expect(bdSession(1700000000000)).toMatch(/^[A-Za-z0-9-]{1,32}$/))
