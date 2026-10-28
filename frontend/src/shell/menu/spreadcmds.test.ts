import { describe, expect, it } from 'vitest'
import type { SpreadView } from '../../base/core/types'
import { COVER_ID, unitOf, type Unit } from '../../base/geom/sheet'
import { VIG_CUT_WHY, VIG_WHY } from '../../panels/timeline/timeline'
import { CUT_FIRST, CUT_PICK, gateHint, NO_COVER_CUT, NO_COVER_JOIN, spreadCmds } from './spreadcmds'

// Пять команд разворота из меню (141): серое и причины. Условия разреза и склейки — из ленты (cutState/joinState).
const sp = (id: string, n: number, o: Partial<SpreadView> = {}): SpreadView =>
  ({ id, photos: Array.from({ length: n }, (_, i) => `${id}p${i}`), rects: [], grid: false, kind: '', ...o }) as unknown as SpreadView
const U = unitOf(undefined)
const SHEET = unitOf({ sheet: true } as never)
const run = (spreads: SpreadView[], cur: string | null, sel: string[] = [], o: { cover?: boolean; max?: number; u?: Unit } = {}) =>
  spreadCmds(spreads, cur, !!o.cover, sel, o.max ?? 64, o.u ?? U)

describe('spreadCmds: вставка', () => {
  const list = [sp('a', 2), sp('b', 2), sp('c', 2)]
  it('после текущего листа; на обложке — первым после неё; нет листа — серое', () => {
    expect(run(list, 'a').insertAt).toBe(1)
    expect(run(list, 'c').insertAt).toBe(3)
    expect(run(list, COVER_ID, [], { cover: true }).insertAt).toBe(0)
    expect(run(list, null).insertAt).toBeNull()
    expect(run([], null).insertAt).toBeNull()
    expect(run(list, 'нет такого').insertAt).toBeNull()
  })
  it('единственный разворот: вставка после него', () => {
    expect(run([sp('a', 1)], 'a').insertAt).toBe(1)
  })
})

describe('spreadCmds: разрез перед выделенным фото', () => {
  const list = [sp('a', 3), sp('b', 3, { grid: true }), sp('v', 3, { kind: 'vignette' })]
  it('одно выделенное не первое фото — можно, at — его номер в sp.photos', () => {
    expect(run(list, 'a', ['ap2']).cut).toEqual({ ok: true, why: '', id: 'a', at: 2 })
  })
  it('ручной разворот — можно, но с вопросом', () => {
    const c = run(list, 'b', ['bp1']).cut
    expect(c).toMatchObject({ ok: true, id: 'b', at: 1 })
    expect(c.ask).toBe('У разворота ручная раскладка')
  })
  it('причины серого: виньетка, не выделено, несколько, чужое фото, первое, обложка, нет листа', () => {
    expect(run(list, 'v', ['vp1']).cut).toMatchObject({ ok: false, why: VIG_CUT_WHY })
    expect(run(list, 'a', []).cut).toMatchObject({ ok: false, why: CUT_PICK })
    expect(run(list, 'a', ['ap1', 'ap2']).cut).toMatchObject({ ok: false, why: CUT_PICK })
    expect(run(list, 'a', ['bp1']).cut).toMatchObject({ ok: false, why: CUT_PICK })
    expect(run(list, 'a', ['ap0']).cut).toMatchObject({ ok: false, why: CUT_FIRST })
    expect(run(list, COVER_ID, ['x'], { cover: true }).cut).toMatchObject({ ok: false, why: NO_COVER_CUT })
    expect(run(list, null).cut).toMatchObject({ ok: false, why: '' })
  })
  it('в одном фото резать нечего: единственное — первое', () => {
    expect(run([sp('a', 1)], 'a', ['ap0']).cut).toMatchObject({ ok: false, why: CUT_FIRST })
  })
  it('в проекте «виньетка-лист» слова со словом «лист»', () => {
    expect(run([sp('a', 3), sp('b', 3)], 'b', [], { u: SHEET }).join.why).toBe('Склеивать не с чем: это последний лист')
    expect(run([sp('a', 3, { grid: true })], 'a', ['ap1'], { u: SHEET }).cut.ask).toBe('У листа ручная раскладка')
  })
})

describe('spreadCmds: склейка со следующим', () => {
  it('обычная пара — можно', () => {
    expect(run([sp('a', 2), sp('b', 2)], 'a').join).toEqual({ ok: true, why: '', id: 'a' })
  })
  it('ручной в паре — можно с вопросом: номера ручных (с 1)', () => {
    expect(run([sp('a', 2), sp('b', 2, { grid: true })], 'a').join).toMatchObject({
      ok: true,
      id: 'a',
      ask: 'У разворота 2 ручная раскладка',
    })
    expect(run([sp('a', 2, { grid: true }), sp('b', 2, { grid: true })], 'a').join.ask).toBe('У разворотов 1 и 2 ручная раскладка')
  })
  it('виньетка в паре, предел фото, последний разворот, обложка, нет листа — серое с причиной', () => {
    const list = [sp('a', 2), sp('v', 2, { kind: 'vignette' }), sp('b', 40), sp('c', 30)]
    expect(run(list, 'a').join).toMatchObject({ ok: false, why: VIG_WHY })
    expect(run(list, 'v').join).toMatchObject({ ok: false, why: VIG_WHY })
    expect(run(list, 'b').join).toMatchObject({ ok: false, why: 'Нельзя склеить: вместе 70 фото, максимум 64' })
    expect(run(list, 'c').join).toMatchObject({ ok: false, why: 'Склеивать не с чем: это последний разворот' })
    expect(run(list, COVER_ID, [], { cover: true }).join).toMatchObject({ ok: false, why: NO_COVER_JOIN })
    expect(run(list, null).join).toMatchObject({ ok: false, why: '' })
  })
  it('единственный разворот — склеивать не с чем', () => {
    expect(run([sp('a', 2)], 'a').join.why).toContain('не с чем')
  })
})

describe('gateHint', () => {
  it('серое — причина; доступное с ручным — что будет; обычное и пустая причина — без подсказки', () => {
    expect(gateHint({ ok: false, why: 'Обложку не склеить' })).toBe('Обложку не склеить')
    expect(gateHint({ ok: false, why: '' })).toBeUndefined()
    expect(gateHint({ ok: true, why: '', ask: 'У разворота 2 ручная раскладка' })).toBe(
      'У разворота 2 ручная раскладка: места вернутся к «Авто», сначала спросим',
    )
    expect(gateHint({ ok: true, why: '' })).toBeUndefined()
    expect(gateHint(undefined)).toBeUndefined()
  })
})
