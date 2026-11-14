import { describe, expect, it } from 'vitest'
import { FEATURES, LEVELS, type FeatureId } from '../../base/prefs/levels'
import { features } from '../../base/prefs/levelcustom'
import { GROUP_IDS } from '../../base/prefs/cardgroups.svelte'
import {
  GROUP_NEEDS,
  aboutSum,
  bgKind,
  bgSum,
  capsKind,
  capsSum,
  dropOpen,
  frameSum,
  groupShown,
  nameSum,
  num,
  placeSum,
  shapeSum,
  sizeSum,
  spineSum,
  toneSum,
} from './groupsum'

const ALL_FEATURES = new Set<FeatureId>(FEATURES.flatMap((f) => [f.id, ...(f.also ?? [])]))

describe('GROUP_NEEDS: видимость групп', () => {
  it('каждая группа описана, а функции — настоящие id уровней', () => {
    expect(Object.keys(GROUP_NEEDS).sort()).toEqual([...GROUP_IDS].sort())
    for (const needs of Object.values(GROUP_NEEDS)) for (const f of needs) expect(ALL_FEATURES.has(f)).toBe(true)
  })

  it('«Простой»: нет кадра/тона/формы/места/фона, но есть «Кадр» (поворот), имя, «О снимке», виньетка', () => {
    const can = (f: FeatureId) => features('simple', null).has(f)
    expect(groupShown('photo.frame', can)).toBe(true) // turn входит в «Простой»
    expect(groupShown('photo.tone', can)).toBe(false)
    expect(groupShown('photo.shape', can)).toBe(false)
    expect(groupShown('photo.place', can)).toBe(false)
    expect(groupShown('photo.name', can)).toBe(true)
    expect(groupShown('photo.about', can)).toBe(true)
    expect(groupShown('spread.vig', can)).toBe(true)
    expect(groupShown('spread.bg', can)).toBe(true) // bgColor
  })

  it('«Полный»: видны все группы', () => {
    const can = (f: FeatureId) => features('full', null).has(f)
    for (const id of GROUP_IDS) expect(groupShown(id, can), id).toBe(true)
  })

  it('«Свой» без функций: группа с условиями скрыта, остальные — нет; на каждом уровне видимая группа имеет функцию', () => {
    const none = () => false
    for (const id of GROUP_IDS) expect(groupShown(id, none), id).toBe(GROUP_NEEDS[id].length === 0)
    for (const level of LEVELS) {
      const can = (f: FeatureId) => features(level, null).has(f)
      for (const id of GROUP_IDS) {
        const needs = GROUP_NEEDS[id]
        expect(groupShown(id, can), `${level} ${id}`).toBe(needs.length === 0 || needs.some(can))
      }
    }
  })
})

describe('группы обложки (124 U6)', () => {
  it('«Фон» и «Подписи» обложки держатся на тех же функциях, что у листа; остальные есть на любом уровне', () => {
    expect(GROUP_NEEDS['cover.bg']).toEqual(GROUP_NEEDS['spread.bg'])
    expect(GROUP_NEEDS['cover.caps']).toEqual(GROUP_NEEDS['spread.caps'])
    const none = () => false
    for (const id of ['cover.layout', 'cover.spine', 'cover.size'] as const) expect(groupShown(id, none), id).toBe(true)
  })
})

describe('«Свой»: набор без функций группы', () => {
  it('сняты все функции «Кадра» — группы нет; «Форма» держится на border, пока он включён', () => {
    const can = (f: FeatureId) => features('custom', { base: 'full', on: [], off: ['frame', 'turn', 'tilt', 'flip'] }).has(f)
    expect(groupShown('photo.frame', can)).toBe(false)
    expect(groupShown('photo.shape', can)).toBe(true) // border
    const noBorder = (f: FeatureId) => can(f) && f !== 'border'
    expect(groupShown('photo.shape', noBorder)).toBe(false)
  })
})

describe('dropOpen', () => {
  it('раскрыта, только пока тянут фото', () => {
    expect(dropOpen({ active: true, payload: { kind: 'photos' } })).toBe(true)
    expect(dropOpen({ active: true, payload: { kind: 'spread' } })).toBe(false)
    expect(dropOpen({ active: false, payload: { kind: 'photos' } })).toBe(false)
    expect(dropOpen({ active: true, payload: null })).toBe(false)
  })
})

describe('значения свёрнутых групп', () => {
  it('num: запятая, один знак, настоящий минус', () => {
    expect(num(2.5)).toBe('2,5')
    expect(num(-2.5)).toBe('−2,5')
    expect(num(299)).toBe('299')
    expect(num(195.66)).toBe('195,7')
    expect(num(-0.04)).toBe('0')
  })

  it('Кадр', () => {
    expect(frameSum({ pct: 100, contain: false, tilt: 0 })).toBe('100 %')
    expect(frameSum({ pct: 120, contain: false, tilt: 2.5 })).toBe('120 % · с обрезкой · 2,5°')
    expect(frameSum({ pct: 120, contain: false, tilt: -3 })).toBe('120 % · с обрезкой · −3°')
    expect(frameSum({ pct: 100, contain: true, tilt: 0 })).toBe('целиком')
  })

  it('Цвет и тон', () => {
    expect(toneSum({ gray: false, bright: 0, contrast: 0, fade: 0 })).toBe('цвет')
    expect(toneSum({ gray: true, bright: 10, contrast: -5, fade: 20 })).toBe('ч/б · +10 · −5 · 20 %')
    expect(toneSum({ gray: false, bright: 0, contrast: 7, fade: 0 })).toBe('+7')
  })

  it('Форма и рамочка', () => {
    expect(shapeSum({ shape: 'oval', border: 'Паспарту' })).toBe('круг · Паспарту')
    expect(shapeSum({ shape: 'rect', border: '' })).toBe('прямоугольник · без рамочки')
    expect(shapeSum({ shape: 'round', border: '' })).toBe('скруглённый · без рамочки')
  })

  it('Имя, место, снимок', () => {
    expect(nameSum({ last: 'Иванов', first: 'Иван', patr: '' })).toBe('Иванов Иван')
    expect(nameSum({ last: ' ', first: '', patr: '' })).toBe('нет')
    expect(placeSum({ w: 195.7, h: 299 })).toBe('195,7 × 299 мм')
    expect(aboutSum({ w: 2667, h: 4000 })).toBe('2667×4000 px')
    expect(aboutSum({ w: 0, h: 0 })).toBe('')
  })

  it('Фон, подписи, корешок, размеры', () => {
    expect([bgSum('album'), bgSum('color'), bgSum('photo'), bgSum('pattern')]).toEqual([
      'как в альбоме',
      'свой цвет',
      '+ фото фона',
      'узор',
    ])
    expect([capsSum('each'), capsSum('own'), capsSum('none')]).toEqual(['к каждому фото', 'свои', 'нет'])
    expect(bgKind({ photo: false, pattern: false, color: false })).toBe('album')
    expect(bgKind({ photo: false, pattern: false, color: true })).toBe('color')
    expect(bgKind({ photo: false, pattern: true, color: true })).toBe('pattern')
    expect(bgKind({ photo: true, pattern: true, color: true })).toBe('photo')
    expect(capsKind({ styled: true, template: true, own: 3 })).toBe('each')
    expect(capsKind({ styled: true, template: false, own: 0 })).toBe('own')
    expect(capsKind({ styled: false, template: false, own: 2 })).toBe('own')
    expect(capsKind({ styled: false, template: false, own: 0 })).toBe('none')
    expect(spineSum({ text: true, plate: true })).toBe('надпись · плашка')
    expect(spineSum({ text: false, plate: false })).toBe('без плашки')
    expect(spineSum({ text: true, plate: false })).toBe('надпись · без плашки')
    expect(sizeSum('A4', 'корешок 6 мм')).toBe('A4 · корешок 6 мм')
    expect(sizeSum('A4', '')).toBe('A4')
  })
})
