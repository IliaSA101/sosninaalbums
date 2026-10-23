import { describe, expect, it } from 'vitest'
import {
  B,
  GAP,
  H,
  PAD,
  cellW,
  centerScroll,
  chipW,
  cutState,
  cutTip,
  glueText,
  groupW,
  isBlank,
  joinState,
  joinTip,
  mergeCount,
  tipLeft,
  VIG_CUT_WHY,
  VIG_WHY,
} from './timeline'
import type { SpreadView } from '../../base/core/types'

const sp = (n: number, w = 2, h = 1): SpreadView =>
  ({
    id: 's' + n,
    photos: Array.from({ length: n }, (_, i) => 'p' + i),
    rects: Array.from({ length: n }, () => ({ x: 0, y: 0, w, h })),
  }) as SpreadView

describe('chipW', () => {
  it('ширина по аспекту плюс рамка', () => {
    expect(chipW(1)).toBe(H)
    expect(chipW(2)).toBe(2 * (H - 2 * B) + 2 * B)
  })
  it('мусорный аспект — как квадрат', () => {
    for (const a of [0, -1, NaN, Infinity]) expect(chipW(a)).toBe(H)
  })
  it('cellW без прямоугольника — квадрат', () => {
    expect(cellW(undefined)).toBe(H)
  })
})

describe('groupW', () => {
  it('сумма ячеек, зазоры и отступы', () => {
    const s = sp(3)
    expect(groupW(s)).toBe(3 * cellW(s.rects[0]) + 2 * GAP + 2 * PAD)
  })
  it('пустые места сетки добавляют ячейки и зазоры', () => {
    const s = sp(1)
    const pl = { id: 'p1', x: 0, y: 0, w: 100, h: 100, photo: '', fit: '' as const, spine: false, rot: 0, z: 1 }
    s.places = [{ ...pl, id: 'p0', photo: 'a' }, pl]
    expect(groupW(s)).toBe(cellW(s.rects[0]) + cellW(pl) + GAP + 2 * PAD)
  })
  it('пустой разворот — одна заглушка', () => {
    expect(groupW(sp(0))).toBe(chipW(1) + 2 * PAD)
  })
  it('isBlank: только без фото и без пустых мест', () => {
    expect(isBlank(sp(0))).toBe(true)
    expect(isBlank(sp(1))).toBe(false)
    const pl = { id: 'p', x: 0, y: 0, w: 1, h: 1, photo: '', fit: '' as const, spine: false, rot: 0, z: 1 }
    expect(isBlank({ ...sp(0), places: [pl] })).toBe(false)
  })
})

describe('mergeCount', () => {
  it('сумма двух соседних, у последнего — только он сам', () => {
    const list = [sp(2), sp(3), sp(4)]
    expect(mergeCount(list, 0)).toBe(5)
    expect(mergeCount(list, 2)).toBe(4)
  })
})

describe('tipLeft и centerScroll', () => {
  it('подсказка не вылезает за окно', () => {
    expect(tipLeft(500, 100, 1000)).toBe(450)
    expect(tipLeft(10, 100, 1000)).toBe(8)
    expect(tipLeft(990, 100, 1000)).toBe(892)
  })
  it('группа по центру', () => {
    expect(centerScroll(500, 400, 100)).toBe(350)
  })
})

describe('joinState и cutState (094, 103)', () => {
  const grid = (s: SpreadView): SpreadView => ({ ...s, grid: true }) as SpreadView
  const vig = (s: SpreadView): SpreadView => ({ ...s, grid: true, kind: 'vignette' }) as SpreadView
  const sheet = { one: 'лист', of: 'листа', many: 'листы', plural: ['лист', 'листа', 'листов'] } as never

  it('joinState: ручной — ask, виньетка — причина, сверх максимума — причина, «Авто» — просто можно', () => {
    expect(joinState([grid(sp(1)), sp(1)], 0, 64)).toEqual({ ok: true, why: '', ask: 'У разворота 1 ручная раскладка' })
    expect(joinState([sp(1), sp(1), grid(sp(1))], 1, 64).ask).toBe('У разворота 3 ручная раскладка')
    expect(joinState([sp(1), grid(sp(1)), grid(sp(1))], 1, 64).ask).toBe('У разворотов 2 и 3 ручная раскладка')
    expect(joinState([sp(1), grid(sp(1))], 0, 64, sheet).ask).toBe('У листа 2 ручная раскладка')
    expect(joinState([vig(sp(1)), sp(1)], 0, 64)).toEqual({ ok: false, why: VIG_WHY })
    expect(joinState([sp(1), vig(sp(1))], 0, 64).why).toBe(VIG_WHY)
    expect(joinState([sp(3), grid(sp(4))], 0, 6)).toEqual({ ok: false, why: 'Нельзя склеить: вместе 7 фото, максимум 6' })
    expect(joinState([sp(3), sp(3)], 0, 6)).toEqual({ ok: true, why: '' })
  })

  it('joinTip и cutTip: Ctrl / ⌘, у ручного — что будет с местами', () => {
    const ok = joinState([sp(3), sp(3)], 0, 6)
    expect(joinTip(ok, false)).toBe('Ctrl+щелчок — склеить')
    expect(joinTip(ok, true)).toBe('⌘+щелчок — склеить')
    const man = joinState([sp(1), grid(sp(1)), grid(sp(1))], 1, 64)
    expect(joinTip(man, false)).toBe('Ctrl+щелчок — склеить. У разворотов 2 и 3 ручная раскладка: места вернутся к «Авто», сначала спросим')
    expect(joinTip(man, true).startsWith('⌘+щелчок — склеить. ')).toBe(true)
    expect(joinTip({ ok: false, why: VIG_WHY }, false)).toBe(VIG_WHY)
    expect(cutTip(false)).toBe('Ctrl+щелчок — разрезать')
    expect(cutTip(true)).toBe('⌘+щелчок — разрезать')
    expect(cutTip(false, 'У разворота ручная раскладка')).toBe(
      'Ctrl+щелчок — разрезать. У разворота ручная раскладка: места вернутся к «Авто», сначала спросим',
    )
  })

  it('cutState: первое фото — пункта нет, ручной — ask, виньетка — серый с причиной', () => {
    expect(cutState(sp(3), 0).show).toBe(false)
    expect(cutState(sp(3), -1).show).toBe(false)
    expect(cutState(sp(3), 3).show).toBe(false) // за последним фото резать нечего
    expect(cutState(sp(3), 2)).toEqual({ show: true, ok: true, why: '' })
    expect(cutState(grid(sp(3)), 1)).toEqual({ show: true, ok: true, why: '', ask: 'У разворота ручная раскладка' })
    expect(cutState(grid(sp(3)), 1, sheet).ask).toBe('У листа ручная раскладка')
    expect(cutState(vig(sp(3)), 1)).toEqual({ show: true, ok: false, why: VIG_CUT_WHY })
  })
})

describe('glueText (103)', () => {
  const sheet = { one: 'лист', of: 'листа', many: 'листы', plural: ['лист', 'листа', 'листов'] } as never

  it('развороты: склейка и разрез', () => {
    expect(glueText('join', [2], 2)).toEqual({
      title: 'Склеить развороты?',
      ask: 'У разворота 2 ручная раскладка. Ваши правки мест сбросятся. Склеить развороты 2 и 3?',
      ok: 'Склеить',
    })
    expect(glueText('join', [2, 3], 2).ask).toBe(
      'У разворотов 2 и 3 ручная раскладка. Ваши правки мест сбросятся. Склеить развороты 2 и 3?',
    )
    expect(glueText('cut', [2], 2, undefined, 4)).toEqual({
      title: 'Разрезать разворот?',
      ask: 'У разворота 2 ручная раскладка. Ваши правки мест сбросятся. Разрезать разворот 2 перед фото 4?',
      ok: 'Разрезать',
    })
  })

  it('альбом листов: «лист» и «листы»', () => {
    expect(glueText('join', [1], 1, sheet).title).toBe('Склеить листы?')
    expect(glueText('join', [1, 2], 1, sheet).ask).toBe('У листов 1 и 2 ручная раскладка. Ваши правки мест сбросятся. Склеить листы 1 и 2?')
    expect(glueText('cut', [3], 3, sheet, 2).ask).toBe(
      'У листа 3 ручная раскладка. Ваши правки мест сбросятся. Разрезать лист 3 перед фото 2?',
    )
  })
})
