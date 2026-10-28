import { describe, expect, it } from 'vitest'
import { FACE_REF_TIP, menuContext, photoMenu, type MenuCtx, type MenuItem } from './photomenu'
import type { SelBox } from '../../base/geom/gridedit'
import type { FeatureId } from '../../base/prefs/levels'
import type { SpreadView } from '../../base/core/types'

// «Остальные — как это лицо» в «Кадр ▸» (130 U2): виден при одном выделенном фото на листе-виньетке.
const box = (id: string, photo = 'a.jpg'): SelBox => ({ ref: id, id, photo, fit: '', rot: 0, x: 0, y: 0, w: 10, h: 10 })
const vig = { kind: 'vignette', vig: { tpl: 'head-t-left' }, photos: ['a.jpg', 'b.jpg'], frames: [] } as unknown as SpreadView
const plain = { kind: 'spread', photos: ['a.jpg', 'b.jpg'], frames: [] } as unknown as SpreadView

const ctx = (s: SpreadView, sel: SelBox[]): MenuCtx => menuContext(s, sel, [])
const frameSub = (c: MenuCtx, can: (f: FeatureId) => boolean = () => true): MenuItem[] =>
  photoMenu(c, can).find((i) => i.id === 'frame')?.sub ?? []
const hasRef = (c: MenuCtx, can?: (f: FeatureId) => boolean) => frameSub(c, can).some((i) => i.id === 'faceRef')

describe('menuContext.faceRef', () => {
  it('виньетка и одно фото — да', () => expect(ctx(vig, [box('p1')]).faceRef).toBe(true))
  it('обычный разворот — нет', () => expect(ctx(plain, [box('p1')]).faceRef).toBe(false))
  it('два фото — нет', () => expect(ctx(vig, [box('p1'), box('p2', 'b.jpg')]).faceRef).toBe(false))
  it('пустое место (без фото) — нет', () => expect(ctx(vig, [box('p1', '')]).faceRef).toBe(false))
  it('фото и пустое место вместе — нет', () => expect(ctx(vig, [box('p1'), box('p2', '')]).faceRef).toBe(false))
  it('лист не получен — нет', () => expect(ctx(undefined as unknown as SpreadView, [box('p1')]).faceRef).toBe(false))
})

describe('photoMenu: «Кадр ▸ Остальные — как это лицо»', () => {
  it('на виньетке: последним в «Кадр ▸», после «Сбросить кадр», под чертой, со словом и подсказкой владельца', () => {
    const sub = frameSub({ ...ctx(vig, [box('p1')]), framed: true, differ: true })
    expect(sub.map((i) => i.id)).toEqual(['contain', 'cover', 'fitPhoto', 'resetFrame', 'faceRef'])
    const ref = sub[sub.length - 1]
    expect(ref).toMatchObject({
      label: 'Остальные — как это лицо',
      sep: true,
      title: 'Как у этого фото — размер лица и линия глаз у всех на листе',
    })
    expect(ref.title).toBe(FACE_REF_TIP)
    expect(ref.icon).toBeTruthy()
  })
  it('кадр не тронут — пункт всё равно есть (норма берётся с места, а не с правки)', () => {
    expect(frameSub(ctx(vig, [box('p1')])).map((i) => i.id)).toEqual(['contain', 'cover', 'faceRef'])
  })
  it('обычный разворот и 2+ выделенных — пункта нет', () => {
    expect(hasRef(ctx(plain, [box('p1')]))).toBe(false)
    expect(hasRef(ctx(vig, [box('p1'), box('p2', 'b.jpg')]))).toBe(false)
  })
  it('уровень без функции «Виньетка» — пункта нет', () => {
    expect(hasRef(ctx(vig, [box('p1')]), (f) => f !== 'vignette')).toBe(false)
  })
  it('«Своё» без «Кадра», но с виньеткой: в «Кадр ▸» остаётся один этот пункт и без черты', () => {
    expect(frameSub(ctx(vig, [box('p1')]), (f) => f !== 'frame')).toMatchObject([{ id: 'faceRef' }])
    expect(frameSub(ctx(vig, [box('p1')]), (f) => f !== 'frame')[0].sep).toBeFalsy()
  })
})
