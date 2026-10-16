import { describe, expect, it, vi } from 'vitest'

// Go и store не нужны: чистая логика формы мест в окне «Настроить виньетку» (спека 044).
vi.mock('../../base/core/store.svelte', () => ({ store: {}, Api: {} }))

import type { VigChoice, VignetteView } from '../../base/core/types'
import { SHAPE_KEYS, VIG_ROUND, setupOf, shapeKey, shapePatch, varsFormOf } from './vigdialog'
import { formOf, paramsOf, sameAsSheet, type VigForm } from './vignette'
import { pickedShape } from './vigown'

const form = (o: Partial<VigForm> = {}): VigForm => ({ tpl: 't', teachers: 1, pupils: 20, ratio: '3:4', ttext: '', ...o })
const vig = (o: Partial<VignetteView> = {}): VignetteView => ({
  tpl: 't',
  teachers: 1,
  pupils: 20,
  ratio: '3:4',
  places: 21,
  max: 64,
  group: false,
  ...o,
})
const own = (o: Partial<VigChoice> = {}): VigChoice =>
  ({ id: 'my:a', name: 'Мой', boxes: [], roles: [], pupilW: 30, small: false, over: false, own: true, ...o }) as VigChoice

describe('форма мест в окне виньетки', () => {
  it('три кнопки: прямоугольник, скруглённые, круг', () => {
    expect(SHAPE_KEYS.map((k) => k.label)).toEqual(['Прямоугольник', 'Скруглённые', 'Круг'])
  })
  it('какая кнопка подсвечена: круг, скругление (любое), прямоугольник', () => {
    expect(shapeKey(form())).toBe('rect')
    expect(shapeKey(form({ shape: '', round: 0 }))).toBe('rect')
    expect(shapeKey(form({ shape: '', round: 10 }))).toBe('round')
    expect(shapeKey(form({ shape: '', round: 25 }))).toBe('round')
    expect(shapeKey(form({ shape: 'oval', round: 0 }))).toBe('oval')
  })
  it('«Круг» ставит портрет 1:1, остальные портрет не трогают', () => {
    expect(shapePatch('oval')).toEqual({ shape: 'oval', round: 0, ratio: '1:1' })
    expect(shapePatch('round')).toEqual({ shape: '', round: VIG_ROUND })
    expect(shapePatch('rect')).toEqual({ shape: '', round: 0 })
    expect(VIG_ROUND).toBe(10)
  })
  it('выбор «Круг» меняет форму и пропорцию формы окна', () => {
    const f = form({ ratio: '3:4' })
    Object.assign(f, shapePatch('oval'))
    expect(f).toMatchObject({ shape: 'oval', round: 0, ratio: '1:1' })
    Object.assign(f, shapePatch('rect')) // вернули прямоугольник: портрет остаётся 1:1
    expect(f).toMatchObject({ shape: '', round: 0, ratio: '1:1' })
  })
  it('форма уходит в VigSetup.vig; без формы полей нет', () => {
    const s = setupOf(form({ shape: 'oval', round: 0, ratio: '1:1' }), varsFormOf(undefined))
    expect(s.vig).toMatchObject({ shape: 'oval', ratio: '1:1' })
    expect(setupOf(form({ shape: '', round: 10 }), varsFormOf(undefined)).vig).toMatchObject({ round: 10 })
    const plain = setupOf(form(), varsFormOf(undefined)).vig
    expect('shape' in plain).toBe(false)
    expect('round' in plain).toBe(false)
  })
  it('formOf читает форму с листа; paramsOf без формы не добавляет полей', () => {
    expect(formOf(vig({ shape: 'oval' }))).toMatchObject({ shape: 'oval', round: 0 })
    expect(formOf(vig({ round: 10 }))).toMatchObject({ shape: '', round: 10 })
    expect(formOf(vig())).toMatchObject({ shape: '', round: 0 })
    expect('shape' in paramsOf(form())).toBe(false)
  })
  it('смена только формы — «Применить» открыто', () => {
    expect(sameAsSheet(form(), vig())).toBe(true)
    expect(sameAsSheet(form({ shape: 'oval' }), vig())).toBe(false)
    expect(sameAsSheet(form({ round: 10 }), vig())).toBe(false)
    expect(sameAsSheet(form({ shape: 'oval' }), vig({ shape: 'oval' }))).toBe(true)
  })
})

describe('pickedShape: свой шаблон переключает «Форму» как портрет', () => {
  it('свой шаблон с формой — форма шаблона', () => {
    const list = [own({ shape: 'oval' })]
    expect(pickedShape(list, 'my:a', form())).toEqual({ shape: 'oval', round: 0 })
    expect(pickedShape([own({ round: 10 })], 'my:a', form())).toEqual({ shape: '', round: 10 })
  })
  it('форма та же, шаблон без полей формы или каталожный — менять нечего', () => {
    expect(pickedShape([own({ shape: 'oval' })], 'my:a', form({ shape: 'oval' }))).toBeNull()
    expect(pickedShape([own()], 'my:a', form({ shape: 'oval' }))).toBeNull()
    expect(pickedShape([own({ own: false, shape: 'oval' })], 'my:a', form())).toBeNull()
    expect(pickedShape([], 'my:zzz', form())).toBeNull()
  })
  it('у круга скругления нет', () => {
    expect(pickedShape([own({ shape: 'oval', round: 20 })], 'my:a', form())).toEqual({ shape: 'oval', round: 0 })
  })
})
