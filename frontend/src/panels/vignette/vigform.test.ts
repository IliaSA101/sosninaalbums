import { describe, expect, it, vi } from 'vitest'

// Go и store не нужны: чистая логика формы мест (044, 056).
vi.mock('../../base/core/store.svelte', () => ({ store: {}, Api: {} }))

import type { VigChoice, VignetteView } from '../../base/core/types'
import { setupOf, varsFormOf } from './vigdialog'
import {
  FORMS,
  FORM_FILTERS,
  FORM_FILTERS_SHORT,
  VIG_ROUND,
  curKey,
  isCurrent,
  pickTile,
  shapeKey,
  stepTile,
  tileFormOf,
  tileShape,
  vigTiles,
  type Tile,
} from './vigform'
import { formOf, paramsOf, sameAsSheet, type VigForm } from './vignette'

const form = (o: Partial<VigForm> = {}): VigForm => ({ tpl: 'a', teachers: 1, pupils: 20, ratio: '3:4', ttext: '', ...o })
const vig = (o: Partial<VignetteView> = {}): VignetteView => ({
  tpl: 'a',
  teachers: 1,
  pupils: 20,
  ratio: '3:4',
  places: 21,
  max: 64,
  group: false,
  ...o,
})
const ch = (id: string, o: Partial<VigChoice> = {}): VigChoice =>
  ({ id, name: id, boxes: [], roles: [], pupilW: 30, small: false, over: false, family: 'side', ...o }) as VigChoice
const own = (id: string, o: Partial<VigChoice> = {}): VigChoice => ch(id, { own: true, family: '', ratio: '4:5', ...o })
const keys = (t: Tile[]) => t.map((x) => x.key)
const cur = (o: Partial<VigForm> = {}) => form(o)

describe('форма мест: ключи и значения', () => {
  it('shapeKey: круг, любое скругление (в том числе из своего шаблона), прямоугольник', () => {
    expect(shapeKey(form())).toBe('rect')
    expect(shapeKey(form({ shape: '', round: 0 }))).toBe('rect')
    expect(shapeKey(form({ shape: '', round: 10 }))).toBe('round')
    expect(shapeKey(form({ shape: '', round: 25 }))).toBe('round')
    expect(shapeKey(form({ shape: 'oval', round: 0 }))).toBe('oval')
  })
  it('tileShape: прямоугольник, скруглённые 10 %, круг; портрет не входит', () => {
    expect(tileShape('rect')).toEqual({ shape: '', round: 0 })
    expect(tileShape('round')).toEqual({ shape: '', round: VIG_ROUND })
    expect(tileShape('oval')).toEqual({ shape: 'oval', round: 0 })
    expect(VIG_ROUND).toBe(10)
  })
  it('фильтр формы: Все · ▭ Прямоугольные · ▢ Скруглённые · ◯ Круги, подсказки, короткий вид для ▦', () => {
    expect(FORM_FILTERS.map((f) => f.id)).toEqual(['all', 'rect', 'round', 'oval'])
    expect(FORM_FILTERS.map((f) => f.label)).toEqual(['Все', '▭ Прямоугольные', '▢ Скруглённые', '◯ Круги'])
    expect(FORM_FILTERS[1].title).toBe('Места прямоугольные')
    expect(FORM_FILTERS[2].title).toBe('Углы мест скруглены (10 %)')
    expect(FORM_FILTERS[3].title).toBe('Круглые места; при портрете не 1:1 — овалы')
    expect(FORM_FILTERS_SHORT.map((f) => f.label)).toEqual(['Все', '▭', '▢', '◯'])
    expect(FORMS.oval.name).toBe('круги')
  })
  it('форма уходит в VigSetup.vig; без формы полей нет', () => {
    expect(setupOf(form({ shape: 'oval', round: 0, ratio: '1:1' }), varsFormOf(undefined)).vig).toMatchObject({
      shape: 'oval',
      ratio: '1:1',
    })
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

describe('vigTiles', () => {
  const items = [ch('a'), ch('b', { over: true }), ch('c', { family: 'big' })]

  it('tiles: all forms grouped — N шаблонов → 3N плиток группами rect → round → oval, заголовки на первой плитке группы', () => {
    const t = vigTiles(items, 'all', 'all', cur())
    expect(keys(t)).toEqual(['a|rect', 'b|rect', 'c|rect', 'a|round', 'b|round', 'c|round', 'a|oval', 'b|oval', 'c|oval'])
    expect(t.filter((x) => x.sep).map((x) => [x.key, x.sep])).toEqual([
      ['a|rect', 'Прямоугольные'],
      ['a|round', 'Скруглённые'],
      ['a|oval', 'Круги'],
    ])
  })
  it('tiles: own once in own form — свой со скруглением 7 % одна плитка в «Скруглённых», под «Круги» не виден', () => {
    const list = [...items, own('my:r', { round: 7 })]
    const all = vigTiles(list, 'all', 'all', cur())
    expect(all.filter((x) => x.c.id === 'my:r').map((x) => x.key)).toEqual(['my:r|round'])
    expect(all.at(-1)?.sep).toBe('Мои шаблоны') // в конце, под заголовком
    expect(keys(vigTiles(list, 'all', 'oval', cur())).includes('my:r|round')).toBe(false)
    expect(keys(vigTiles(list, 'all', 'round', cur())).at(-1)).toBe('my:r|round')
    expect(keys(vigTiles(list, 'all', 'rect', cur())).includes('my:r|round')).toBe(false)
  })
  it('tiles: own — круг и прямоугольник по своим полям; вид шаблонов свои не отбирает', () => {
    const list = [...items, own('my:o', { shape: 'oval', round: 20 }), own('my:p')]
    const t = vigTiles(list, 'big', 'all', cur())
    expect(t.filter((x) => x.c.own).map((x) => x.key)).toEqual(['my:o|oval', 'my:p|rect'])
  })
  it('tiles: filter keeps current — «Круги»: только круги + выбранная плитка другой формы на своём месте', () => {
    const t = vigTiles(items, 'all', 'oval', cur({ tpl: 'a' })) // лист: a, прямоугольный
    expect(keys(t)).toEqual(['a|rect', 'a|oval', 'b|oval', 'c|oval'])
    expect(vigTiles(items, 'all', 'oval', cur({ tpl: 'a', shape: 'oval' })).map((x) => x.key)).toEqual(['a|oval', 'b|oval', 'c|oval'])
  })
  it('фильтр формы: одна группа — без заголовка; две (с текущей) — с заголовками', () => {
    const one = vigTiles(items, 'all', 'round', cur({ tpl: 'a', round: 10 }))
    expect(one.some((x) => x.sep)).toBe(false)
    const two = vigTiles(items, 'all', 'oval', cur({ tpl: 'a' }))
    expect(two.filter((x) => x.sep).map((x) => x.sep)).toEqual(['Прямоугольные', 'Круги'])
  })
  it('фильтр вида: пересечение с формой; текущая плитка видна вне вида', () => {
    const t = vigTiles(items, 'big', 'round', cur({ tpl: 'a' }))
    expect(keys(t)).toEqual(['a|rect', 'c|round']) // a|rect — текущая вне вида и формы
  })
  it('нет данных: пусто; нет каталога — только свои', () => {
    expect(vigTiles([], 'all', 'all', cur())).toEqual([])
    expect(keys(vigTiles([own('my:x')], 'all', 'oval', cur({ tpl: 'zzz' })))).toEqual([])
    expect(keys(vigTiles([own('my:x')], 'all', 'all', cur({ tpl: 'zzz' })))).toEqual(['my:x|rect'])
  })
  it('«не поместится» остаётся плиткой во всех формах (серой)', () => {
    const t = vigTiles(items, 'all', 'all', cur())
    expect(t.filter((x) => x.c.over).map((x) => x.key)).toEqual(['b|rect', 'b|round', 'b|oval'])
  })
  it('старый лист «россыпью» с кругами 1:1: подсвечена плитка «Россыпью» в группе «Круги»', () => {
    const list = [ch('round', { family: 'round' }), ch('a')]
    const c = cur({ tpl: 'round', shape: 'oval', round: 0, ratio: '1:1' })
    const t = vigTiles(list, 'all', 'all', c)
    expect(curKey(t, c)).toBe('round|oval')
    expect(t.filter((x) => isCurrent(x, c)).length).toBe(1)
  })
  it('curKey: плитки нет (шаблон не пришёл) — пусто; свой шаблон ищется по id', () => {
    expect(curKey(vigTiles(items, 'all', 'all', cur()), cur({ tpl: 'нет' }))).toBe('')
    const list = [own('my:r', { round: 7 })]
    // лист получил форму раньше, чем она записана в шаблоне: плитка своя одна — подсвечена
    expect(curKey(vigTiles(list, 'all', 'all', cur({ tpl: 'my:r' })), cur({ tpl: 'my:r', shape: 'oval' }))).toBe('my:r|round')
  })
})

describe('pickTile', () => {
  const t = (c: VigChoice, k: Tile['k']): Tile => ({ c, k, key: `${c.id}|${k}` })

  it("pick: exactly shown — «Россыпью» прямоугольная при форме окна «круги» и портрете 3:4: форма '', round 0, портрет не трогается", () => {
    const r = pickTile(t(ch('round'), 'rect'), '3:4')
    expect(r).toEqual({ tpl: 'round', shape: '', round: 0 })
    expect('ratio' in r).toBe(false)
  })
  it('скруглённые — 10 %, круг — oval без скругления', () => {
    expect(pickTile(t(ch('a'), 'round'), '3:4')).toEqual({ tpl: 'a', shape: '', round: 10 })
    expect(pickTile(t(ch('a'), 'oval'), '3:4')).toEqual({ tpl: 'a', shape: 'oval', round: 0 })
  })
  it('pick: ratio only from own — каталожная портрет не меняет (даже у кругов), своя ставит свой, если он другой', () => {
    expect('ratio' in pickTile(t(ch('a', { ratio: '1:1' }), 'oval'), '3:4')).toBe(false)
    expect(pickTile(t(own('my:x', { ratio: '4:5' }), 'rect'), '3:4')).toEqual({ tpl: 'my:x', shape: '', round: 0, ratio: '4:5' })
    expect('ratio' in pickTile(t(own('my:x', { ratio: '4:5' }), 'rect'), '4:5')).toBe(false)
  })
  it('свой шаблон ставит форму как сохранена; плитка рисуется в ней же', () => {
    const o = t(own('my:r', { round: 7 }), 'round')
    expect(pickTile(o, '4:5')).toMatchObject({ shape: '', round: 7 })
    expect(tileFormOf(o)).toEqual({ shape: '', round: 7 })
    const c = t(own('my:o', { shape: 'oval', round: 9 }), 'oval')
    expect(tileFormOf(c)).toEqual({ shape: 'oval', round: 0 })
    expect(tileFormOf(t(ch('a'), 'round'))).toEqual({ shape: '', round: 10 })
  })
  it('применение к форме окна не трогает числа и текст', () => {
    const f = form({ teachers: 2, pupils: 30, ttext: 'T', shape: 'oval' })
    Object.assign(f, pickTile(t(ch('b'), 'rect'), f.ratio))
    expect(f).toMatchObject({ tpl: 'b', teachers: 2, pupils: 30, ttext: 'T', shape: '', round: 0, ratio: '3:4' })
  })
})

describe('stepTile', () => {
  const items = [ch('a'), ch('b', { over: true }), ch('c')]

  it('следующая видимая плитка — шаблон или форма; «не поместится» пропускается', () => {
    const c = cur({ tpl: 'a' })
    const tiles = vigTiles(items, 'all', 'all', c)
    expect(stepTile(tiles, c, 1)?.key).toBe('c|rect') // b|rect серая
    expect(stepTile(tiles, cur({ tpl: 'c' }), 1)?.key).toBe('a|round') // конец группы → следующая форма
    expect(stepTile(tiles, cur({ tpl: 'a' }), -1)?.key).toBe('c|oval') // назад по кругу
  })
  it('идёт по видимым: фильтр формы оставляет только свою группу', () => {
    const c = cur({ tpl: 'c', shape: 'oval' })
    const tiles = vigTiles(items, 'all', 'oval', c)
    expect(stepTile(tiles, c, 1)?.key).toBe('a|oval')
  })
  it('видна одна плитка (текущая) — null', () => {
    const c = cur({ tpl: 'a' })
    expect(stepTile(vigTiles([ch('a')], 'all', 'rect', c), c, 1)).toBeNull()
    expect(stepTile([], c, 1)).toBeNull()
  })
  it('текущей нет среди плиток — с края, откуда идёт шаг', () => {
    const c = cur({ tpl: 'нет' })
    const tiles = vigTiles(items, 'all', 'all', c)
    expect(stepTile(tiles, c, 1)?.key).toBe('a|rect')
    expect(stepTile(tiles, c, -1)?.key).toBe('c|oval')
  })
})
