import { describe, expect, it } from 'vitest'
import { fitScale, FIT, place, ORIG_MAX_PX, ORIG_WHY_TEXT, type Pt, type Size } from '../../editor/photo/loupe'
import {
  BUDGET_TEXT,
  budgetText,
  carrySync,
  FIT_SYNC,
  isFit,
  NO_OFF,
  origBudget,
  panOne,
  panSync,
  tileLoupe,
  tileMax,
  toggleSync,
  zoomSync,
  type Off,
  type Sync,
  type Tile,
} from './synczoom'

// Общий зум окна сравнения (138-7): относительный зум, точка под курсором, общий и свой сдвиг, бюджет оригиналов.
const box = { w: 600, h: 500 }
const landscape = { w: 2048, h: 1365 }
const portrait = { w: 1365, h: 2048 }
const wide = { w: 3000, h: 1000 }
const tile = (nat: Size, over: Partial<Tile> = {}): Tile => ({ nat, box, max: 1, off: NO_OFF, ...over })
const A = tile(landscape)
const fitA = fitScale(landscape, box)
const at: Pt = { x: 400, y: 200 }

// Доля кадра под точкой окна.
function under(s: Sync, t: Tile, pt: Pt) {
  const p = place(tileLoupe(s, t.off, t.nat, t.box, t.max), t.nat, t.box)
  return { x: (pt.x - p.left) / p.w, y: (pt.y - p.top) / p.h }
}

describe('общий относительный зум', () => {
  it('вписано (z = 0) — все в FIT, свой сдвиг не виден', () => {
    for (const nat of [landscape, portrait, wide]) expect(tileLoupe(FIT_SYNC, NO_OFF, nat, box, 1)).toBe(FIT)
    expect(tileLoupe(FIT_SYNC, { dx: 0.3, dy: 0.1 }, landscape, box, 1)).toBe(FIT)
    expect(isFit(FIT_SYNC)).toBe(true)
  })
  it('масштаб ячейки = z × её «вписано»: у кадров разных пропорций разный абсолют, та же доля', () => {
    const s = { z: 1.5, cx: 0.5, cy: 0.5 }
    for (const nat of [landscape, portrait, wide]) {
      const l = tileLoupe(s, NO_OFF, nat, box, 4)
      expect(l.s).toBeCloseTo(1.5 * fitScale(nat, box), 9)
      const p = place(l, nat, box)
      // Видимая доля кадра по ширине одна и та же: окно / (z·вписанный по ширине)
      expect(p.w / (nat.w * fitScale(nat, box))).toBeCloseTo(1.5, 9)
    }
  })
  it('предел по кадру: больше max не растёт, меньше «вписано» не падает', () => {
    const big = { z: 100, cx: 0.5, cy: 0.5 }
    expect(tileLoupe(big, NO_OFF, landscape, box, 1).s).toBe(1)
    expect(tileLoupe(big, NO_OFF, landscape, box, 3).s).toBe(3)
    // Кадр с большим пределом (оригинал) и с малым (1:1 по превью) в одном состоянии
    expect(tileLoupe(big, NO_OFF, portrait, box, 2).s).toBe(2)
    // z < 1 меньше «вписано» не делает
    expect(tileLoupe({ z: 0.5, cx: 0.5, cy: 0.5 }, NO_OFF, landscape, box, 1)).toBe(FIT)
  })
  it('мелкое фото (вписано = 1:1) не зумится, остальные — да', () => {
    const small = { w: 300, h: 200 }
    expect(tileLoupe({ z: 3, cx: 0.5, cy: 0.5 }, NO_OFF, small, box, 1)).toBe(FIT)
    expect(tileLoupe({ z: 3, cx: 0.5, cy: 0.5 }, NO_OFF, landscape, box, 1).s).toBeGreaterThan(fitA)
  })
  it('размеры не загружены — FIT и без падения', () => {
    expect(tileLoupe({ z: 2, cx: 0.5, cy: 0.5 }, NO_OFF, { w: 0, h: 0 }, box, 1)).toBe(FIT)
    expect(zoomSync(FIT_SYNC, 2, tile({ w: 0, h: 0 }), at)).toBe(FIT_SYNC)
    expect(toggleSync(FIT_SYNC, tile({ w: 0, h: 0 }), at)).toBe(FIT_SYNC)
  })
})

describe('зум у курсора', () => {
  it('точка под курсором активной ячейки остаётся на месте', () => {
    const before = under(FIT_SYNC, A, at)
    const s = zoomSync(FIT_SYNC, 2, A, at)
    expect(s.z).toBeCloseTo(2, 6)
    const after = under(s, A, at)
    expect(after.x).toBeCloseTo(before.x, 6)
    expect(after.y).toBeCloseTo(before.y, 6)
  })
  it('у других ячеек та же общая точка: центр окна показывает ту же долю кадра', () => {
    const s = zoomSync(FIT_SYNC, 2, A, at)
    const lp = tileLoupe(s, NO_OFF, portrait, box, 1)
    expect(lp.cx).toBeCloseTo(s.cx, 9)
    expect(lp.cy).toBeCloseTo(s.cy, 9)
    expect(lp.s).toBeCloseTo(2 * fitScale(portrait, box), 9)
  })
  it('предел активной ячейки: колесо дальше 1:1 при max = 1 не идёт; с оригиналом — до его max', () => {
    expect(zoomSync(FIT_SYNC, 100, A, at).z).toBeCloseTo(1 / fitA, 6)
    expect(zoomSync(FIT_SYNC, 100, tile(landscape, { max: 2 }), at).z).toBeCloseTo(2 / fitA, 6)
  })
  it('зум назад до «вписано» — z = 0; шаг туда-обратно возвращает на то же место', () => {
    const up = zoomSync(FIT_SYNC, 1.25, A, at)
    expect(zoomSync(up, 1 / 1.25, A, at)).toEqual(FIT_SYNC)
    expect(zoomSync(FIT_SYNC, 0.5, A, at)).toEqual(FIT_SYNC)
  })
  it('без курсора — в центре окна; мусорный множитель ничего не меняет', () => {
    const s = zoomSync(FIT_SYNC, 2, A, null)
    expect(s.cx).toBeCloseTo(0.5, 6)
    expect(s.cy).toBeCloseTo(0.5, 6)
    const z = { z: 2, cx: 0.4, cy: 0.6 }
    for (const k of [0, -1, NaN, Infinity]) expect(zoomSync(z, k, A, at)).toEqual(z)
  })
  it('активная ячейка со своим сдвигом: точка под курсором тоже стоит; общая точка без сдвига', () => {
    const off: Off = { dx: 0.1, dy: -0.05 }
    const T = tile(landscape, { off })
    const start = { z: 2, cx: 0.5, cy: 0.5 }
    const before = under(start, T, at)
    const s = zoomSync(start, 1.5, T, at)
    const after = under(s, T, at)
    expect(after.x).toBeCloseTo(before.x, 6)
    expect(after.y).toBeCloseTo(before.y, 6)
  })
})

describe('1:1 активного кадра', () => {
  it('вписано → 1:1 в точке (z = 1/вписано), точка на месте; повтор — вписано', () => {
    const before = under(FIT_SYNC, A, at)
    const s = toggleSync(FIT_SYNC, A, at)
    expect(s.z).toBeCloseTo(1 / fitA, 9)
    expect(tileLoupe(s, NO_OFF, landscape, box, 1).s).toBe(1)
    const after = under(s, A, at)
    expect(after.x).toBeCloseTo(before.x, 6)
    expect(after.y).toBeCloseTo(before.y, 6)
    expect(toggleSync(s, A, at)).toEqual(FIT_SYNC)
  })
  it('у других кадров тот же z: портрет и панорама крупнее своего «вписано» в столько же раз', () => {
    const s = toggleSync(FIT_SYNC, A, at)
    for (const nat of [portrait, wide]) {
      const f = fitScale(nat, box)
      expect(tileLoupe(s, NO_OFF, nat, box, 1).s).toBeCloseTo(Math.min(1, s.z * f), 9)
    }
  })
  it('мелкий активный кадр: 1:1 и так вписано — состояние то же', () => {
    const small = tile({ w: 300, h: 200 })
    expect(toggleSync(FIT_SYNC, small, at)).toEqual(FIT_SYNC)
  })
  it('активный кадр с оригиналом: 1:1 по превью, а не по оригиналу', () => {
    const s = toggleSync(FIT_SYNC, tile(landscape, { max: 3 }), at)
    expect(s.z).toBeCloseTo(1 / fitA, 9)
  })
})

describe('общий сдвиг', () => {
  const start = zoomSync(FIT_SYNC, 3, A, { x: 300, y: 250 })
  it('перетаскивание активной двигает общую точку: кадр едет за курсором', () => {
    const s = panSync(start, 40, -30, A)
    expect(s.z).toBe(start.z)
    expect(s.cx).toBeLessThan(start.cx)
    expect(s.cy).toBeGreaterThan(start.cy)
    // Другая ячейка получает тот же сдвиг в долях кадра
    const l0 = tileLoupe(start, NO_OFF, portrait, box, 1)
    const l1 = tileLoupe(s, NO_OFF, portrait, box, 1)
    expect(l1.cx - l0.cx).toBeCloseTo(s.cx - start.cx, 9)
    expect(l1.cy - l0.cy).toBeCloseTo(s.cy - start.cy, 9)
  })
  it('вписано — сдвига нет; нулевой сдвиг не меняет значения', () => {
    expect(panSync(FIT_SYNC, 50, 50, A)).toBe(FIT_SYNC)
    const s = panSync(start, 0, 0, A)
    expect(s.cx).toBeCloseTo(start.cx, 9)
    expect(s.cy).toBeCloseTo(start.cy, 9)
  })
  it('у края кадр не уходит: край кадра не отходит от края окна', () => {
    const s = panSync(start, 100000, 100000, A)
    const p = place(tileLoupe(s, NO_OFF, landscape, box, 1), landscape, box)
    expect(p.left).toBeLessThanOrEqual(1e-6)
    expect(p.top).toBeLessThanOrEqual(1e-6)
  })
})

describe('свой сдвиг ячейки (Shift)', () => {
  const sync = zoomSync(FIT_SYNC, 3, A, { x: 300, y: 250 })
  it('двигает только эту ячейку: общая точка та же', () => {
    const off = panOne(sync, 30, 20, A)
    expect(off.dx).not.toBe(0)
    expect(off.dy).not.toBe(0)
    const own = tileLoupe(sync, off, landscape, box, 1)
    const other = tileLoupe(sync, NO_OFF, landscape, box, 1)
    expect(own.cx).not.toBeCloseTo(other.cx, 4)
    expect(tileLoupe(sync, NO_OFF, landscape, box, 1)).toEqual(other)
  })
  it('сдвиг держится при общем зуме и общем сдвиге', () => {
    const off = panOne(sync, 30, 20, A)
    const T = tile(landscape, { off })
    const zoomed = zoomSync(sync, 1.2, T, null)
    expect(tileLoupe(zoomed, off, landscape, box, 1).cx - zoomed.cx).toBeCloseTo(off.dx, 6)
    const moved = panSync(sync, 10, 10, A)
    expect(tileLoupe(moved, off, landscape, box, 1).cx - moved.cx).toBeCloseTo(off.dx, 6)
  })
  it('вписать снимает: при z = 0 сдвиг не виден, кадр по центру', () => {
    const off = panOne(sync, 30, 20, A)
    const fitted = zoomSync(sync, 0.001, tile(landscape, { off }), null)
    expect(fitted).toEqual(FIT_SYNC)
    expect(place(tileLoupe(fitted, off, landscape, box, 1), landscape, box)).toEqual(place(FIT, landscape, box))
  })
  it('продолжение жеста от нажатия: тот же итог от того же начала', () => {
    const one = panOne(sync, 30, 20, A)
    const T = tile(landscape, { off: one })
    // Следующее перетаскивание начинается от прежнего сдвига и добавляет к нему
    const two = panOne(sync, 10, 0, T)
    expect(two.dx).toBeLessThan(one.dx)
  })
  it('вписано — сдвигать нечего; сдвиг зажат в пределах половины кадра', () => {
    expect(panOne(FIT_SYNC, 50, 50, A)).toBe(NO_OFF)
    const off = panOne(sync, -1e6, -1e6, A)
    expect(Math.abs(off.dx)).toBeLessThanOrEqual(0.5)
    expect(Math.abs(off.dy)).toBeLessThanOrEqual(0.5)
  })
})

describe('листание ячейки', () => {
  it('те же пропорции (±1 %) — зум и точка остаются; другие — вписаны все', () => {
    const s = { z: 2, cx: 0.3, cy: 0.7 }
    expect(carrySync(s, landscape, { w: 2048, h: 1370 })).toBe(s)
    expect(carrySync(s, landscape, portrait)).toBe(FIT_SYNC)
    expect(carrySync(s, landscape, { w: 0, h: 0 })).toBe(FIT_SYNC)
    expect(carrySync(FIT_SYNC, landscape, portrait)).toBe(FIT_SYNC)
  })
})

describe('бюджет оригиналов', () => {
  const mp = (n: number, color?: string) => ({ w: 1000, h: n * 1000, color })
  it('2–4 кадра в пределах 100 Мп — можно всем', () => {
    expect(origBudget([mp(24), mp(24)])).toBe('')
    expect(origBudget([mp(24), mp(24), mp(24), mp(24)])).toBe('')
    expect(origBudget([])).toBe('')
  })
  it('ровно 100 Мп — можно, на пиксель больше — big', () => {
    expect(origBudget([mp(50), mp(50)])).toBe('')
    expect(origBudget([{ w: ORIG_MAX_PX, h: 1 }])).toBe('')
    expect(
      origBudget([
        { w: ORIG_MAX_PX, h: 1 },
        { w: 1, h: 1 },
      ]),
    ).toBe('big')
    expect(origBudget([mp(50), mp(51)])).toBe('big')
  })
  it('четыре кадра по 26 Мп вместе больше — big; один кадр больше 100 Мп — big', () => {
    expect(origBudget([mp(26), mp(26), mp(26), mp(26)])).toBe('big')
    expect(origBudget([mp(101), mp(1)])).toBe('big')
  })
  it('не sRGB у любого кадра — отказ всем с причиной профиля', () => {
    expect(origBudget([mp(10), mp(10, 'adobe')])).toBe('adobe')
    expect(origBudget([mp(10, 'p3'), mp(10)])).toBe('p3')
    expect(origBudget([mp(10, 'icc'), mp(10)])).toBe('icc')
    expect(origBudget([mp(10, 'other'), mp(10)])).toBe('other')
    // Профиль важнее размера
    expect(origBudget([mp(90, 'adobe'), mp(90)])).toBe('adobe')
    // sRGB без профиля — не причина
    expect(origBudget([mp(10, 'srgb'), mp(10, '')])).toBe('')
  })
  it('нет сведений о кадре (не загружен, нулевой размер) — не мешает', () => {
    expect(origBudget([undefined, mp(10)])).toBe('')
    expect(origBudget([{ w: 0, h: 0 }, mp(10)])).toBe('')
  })
  it('подсказка значка: big — про кадры вместе, остальное — слова окна 128, можно — пусто', () => {
    expect(budgetText('')).toBe('')
    expect(budgetText('big')).toBe(BUDGET_TEXT)
    expect(budgetText('adobe')).toBe(ORIG_WHY_TEXT.adobe)
    expect(budgetText('p3')).toBe(ORIG_WHY_TEXT.p3)
  })
  it('предел ячейки: бюджет позволяет — по оригиналу (не меньше 1:1), иначе 1:1 по превью', () => {
    const photo = { w: 6000, h: 4000 }
    const nat = { w: 2000, h: 1333 }
    expect(tileMax('', photo, nat)).toBe(3)
    expect(tileMax('big', photo, nat)).toBe(1)
    expect(tileMax('adobe', photo, nat)).toBe(1)
    expect(tileMax('', undefined, nat)).toBe(1)
    expect(tileMax('', { w: 1000, h: 700 }, nat)).toBe(1)
  })
})
