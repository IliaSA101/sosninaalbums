import { describe, expect, it } from 'vitest'
import { fillKeys } from '../../base/keys/keytext'
import {
  carry,
  CLICK_PX,
  FIT,
  fitScale,
  hintLine,
  isClick,
  origMax,
  origWhy,
  pan,
  place,
  sameShape,
  toggle,
  zoomAt,
  zoomLabel,
  type Loupe,
  type Pt,
} from './loupe'

// Лупа окна крупного просмотра (128-4): вписано ↔ 1:1 в точке, зум у курсора, сдвиг, перенос при листании.
const nat = { w: 2048, h: 1365 }
const box = { w: 1200, h: 700 }
const fit = fitScale(nat, box)

// Доля кадра под точкой окна: по ней проверяем, что точка осталась на месте.
function under(l: Loupe, n: { w: number; h: number }, pt: Pt) {
  const p = place(l, n, box)
  return { x: (pt.x - p.left) / p.w, y: (pt.y - p.top) / p.h }
}

describe('вписано', () => {
  it('fitScale: кадр целиком; размеры неизвестны — 0', () => {
    expect(fit).toBeCloseTo(700 / 1365, 6)
    expect(fitScale({ w: 0, h: 0 }, box)).toBe(0)
    expect(fitScale(nat, { w: 0, h: 700 })).toBe(0)
  })
  it('fitScale: мелкая картинка не растягивается сверх 1:1; значок «1:1»', () => {
    const small = { w: 400, h: 300 }
    expect(fitScale(small, box)).toBe(1)
    expect(place(FIT, small, box)).toEqual({ left: 400, top: 200, w: 400, h: 300 })
    expect(zoomLabel(FIT, small, box)).toBe('1:1')
    expect(toggle(FIT, small, box, null, 1)).toBe(FIT)
  })
  it('place: вписанный кадр по центру окна', () => {
    const p = place(FIT, nat, box)
    expect(p.w).toBeCloseTo(nat.w * fit, 6)
    expect(p.h).toBeCloseTo(700, 6)
    expect(p.left).toBeCloseTo((1200 - p.w) / 2, 6)
    expect(p.top).toBeCloseTo(0, 6)
  })
})

describe('1:1 в точке', () => {
  const at = { x: 650, y: 350 }
  it('точка под курсором остаётся на месте, масштаб ровно 1:1', () => {
    const before = under(FIT, nat, at)
    const l = toggle(FIT, nat, box, at, 1)
    expect(l.s).toBe(1)
    expect(place(l, nat, box).w).toBe(nat.w)
    const after = under(l, nat, at)
    expect(after.x).toBeCloseTo(before.x, 6)
    expect(after.y).toBeCloseTo(before.y, 6)
  })
  it('у края кадра центр зажат: край кадра не отходит от края окна', () => {
    const l = toggle(FIT, nat, box, { x: 0, y: 0 }, 1)
    const p = place(l, nat, box)
    expect(p.left).toBeLessThanOrEqual(0)
    expect(p.left + p.w).toBeGreaterThanOrEqual(box.w)
    expect(p.top).toBeLessThanOrEqual(0)
    expect(p.top + p.h).toBeGreaterThanOrEqual(box.h)
    const far = place(toggle(FIT, nat, box, { x: 1200, y: 700 }, 1), nat, box)
    expect(far.left + far.w).toBeCloseTo(box.w, 6)
    expect(far.top + far.h).toBeCloseTo(box.h, 6)
  })
  it('второй щелчок — обратно во «вписано»', () => {
    expect(toggle(toggle(FIT, nat, box, at, 1), nat, box, at, 1)).toEqual(FIT)
  })
  it('без курсора (клавиша без движения мыши) — в центре окна', () => {
    const before = under(FIT, nat, { x: 600, y: 350 })
    const l = toggle(FIT, nat, box, null, 1)
    const after = under(l, nat, { x: 600, y: 350 })
    expect(after.x).toBeCloseTo(before.x, 6)
    expect(after.y).toBeCloseTo(before.y, 6)
  })
  it('фото меньше окна: щелчок без изменений', () => {
    const small = { w: 400, h: 300 }
    expect(toggle(FIT, small, box, at, 1)).toBe(FIT)
    expect(zoomAt(FIT, 2, small, box, at, 1)).toEqual(FIT)
  })
  it('вертикальное превью (EXIF 6, 1365×2048): точка 1:1 = точка вписанного', () => {
    const v = { w: 1365, h: 2048 }
    for (const pt of [
      { x: 600, y: 300 },
      { x: 560, y: 150 },
      { x: 640, y: 600 },
    ]) {
      const before = under(FIT, v, pt)
      const l = toggle(FIT, v, box, pt, 1)
      expect(place(l, v, box).w).toBe(v.w)
      const after = under(l, v, pt)
      expect(after.x).toBeCloseTo(before.x, 6)
      expect(after.y).toBeCloseTo(before.y, 6)
    }
  })
  it('неизвестные размеры (картинка не загружена): ничего не меняется', () => {
    const l = { s: 0.8, cx: 0.4, cy: 0.6 }
    expect(toggle(l, { w: 0, h: 0 }, box, null, 1)).toBe(l)
    expect(zoomAt(l, 2, { w: 0, h: 0 }, box, null, 1)).toBe(l)
  })
})

describe('зум у курсора', () => {
  const at = { x: 600, y: 300 }
  it('точка под курсором остаётся на месте', () => {
    const before = under(FIT, nat, at)
    const l = zoomAt(FIT, 1.5, nat, box, at, 1)
    expect(l.s).toBeCloseTo(fit * 1.5, 6)
    const after = under(l, nat, at)
    expect(after.x).toBeCloseTo(before.x, 6)
    expect(after.y).toBeCloseTo(before.y, 6)
  })
  it('масштаб держится в [вписано, max]', () => {
    expect(zoomAt(FIT, 100, nat, box, at, 1).s).toBe(1)
    expect(zoomAt({ s: 1, cx: 0.5, cy: 0.5 }, 0.001, nat, box, at, 1)).toEqual(FIT)
    expect(zoomAt(FIT, 0.5, nat, box, at, 1)).toEqual(FIT)
    // ступень 2: предел выше 1:1
    expect(zoomAt(FIT, 100, nat, box, at, 2).s).toBe(2)
  })
  it('шаг Mod+= и Mod+− возвращает на то же место', () => {
    const up = zoomAt(FIT, 1.25, nat, box, at, 1)
    const back = zoomAt(up, 1 / 1.25, nat, box, at, 1)
    expect(back).toEqual(FIT)
  })
  it('мусорный множитель не меняет кадр', () => {
    const l = { s: 0.8, cx: 0.4, cy: 0.6 }
    for (const k of [0, -1, NaN, Infinity]) expect(zoomAt(l, k, nat, box, at, 1)).toBe(l)
  })
})

describe('сдвиг', () => {
  it('кадр едет за курсором, у края упирается', () => {
    const start = toggle(FIT, nat, box, { x: 600, y: 350 }, 1)
    const a = place(start, nat, box)
    const moved = place(pan(start, 100, 0, nat, box), nat, box)
    expect(moved.left).toBeCloseTo(a.left + 100, 6)
    const edge = place(pan(start, 99999, 99999, nat, box), nat, box)
    expect(edge.left).toBe(0)
    expect(edge.top).toBe(0)
  })
  it('вписанный кадр не двигается', () => {
    expect(pan(FIT, 50, 50, nat, box)).toBe(FIT)
  })
})

describe('перенос при листании', () => {
  const l = { s: 1, cx: 0.3, cy: 0.7 }
  it('те же пропорции (±1 %) — тот же масштаб и точка', () => {
    expect(carry(l, nat, { w: 2048, h: 1365 })).toBe(l)
    expect(carry(l, nat, { w: 1500, h: 1000 })).toBe(l) // 1,5 против 1,5003
    expect(carry(l, nat, { w: 2048, h: 1352 })).toBe(l) // ≈ 0,95 %
  })
  it('другие пропорции (вертикаль после горизонтали, квадрат) — вписано', () => {
    expect(carry(l, nat, { w: 1365, h: 2048 })).toEqual(FIT)
    expect(carry(l, nat, { w: 1500, h: 1500 })).toEqual(FIT)
    expect(carry(l, nat, { w: 2048, h: 1330 })).toEqual(FIT) // ≈ 2,6 %
  })
  it('вписанное остаётся вписанным; размеры неизвестны — вписано', () => {
    expect(carry(FIT, nat, { w: 1365, h: 2048 })).toBe(FIT)
    expect(carry(l, { w: 0, h: 0 }, nat)).toEqual(FIT)
  })
})

describe('щелчок и значок', () => {
  it('порог сдвига — CLICK_PX', () => {
    expect(CLICK_PX).toBe(5)
    expect(isClick(0, 0)).toBe(true)
    expect(isClick(3, 3.9)).toBe(true)
    expect(isClick(5, 0)).toBe(false)
    expect(isClick(0, -6)).toBe(false)
  })
  it('значок: «Вписано · N %», «1:1», «N %», пусто без размера', () => {
    expect(zoomLabel(FIT, nat, box)).toBe('Вписано · 51 %')
    expect(zoomLabel({ s: 1, cx: 0.5, cy: 0.5 }, nat, box)).toBe('1:1')
    expect(zoomLabel({ s: 0.8, cx: 0.5, cy: 0.5 }, nat, box)).toBe('80 %')
    expect(zoomLabel(FIT, { w: 0, h: 0 }, box)).toBe('')
  })
})

describe('строка клавиш', () => {
  const hints: Record<string, string> = { photoPreview: 'Z', fitZoom: '0' }
  const fill = (lib: boolean, mod = 'Ctrl', h = hints, marks = true) => fillKeys(hintLine(lib, mod, marks), (id) => h[id] ?? '')
  it('из ленты: листание и отметки есть, клавиши из реестра', () => {
    expect(fill(true)).toBe(
      '← → — листать · щелчок или Z — 1:1 · Ctrl+колесо — зум · 0 — вписать · отметки — клавишами ленты · Esc — закрыть',
    )
  })
  it('отметки выключены уровнем — куска про них нет', () => {
    expect(fill(true, 'Ctrl', hints, false)).toBe('← → — листать · щелчок или Z — 1:1 · Ctrl+колесо — зум · 0 — вписать · Esc — закрыть')
  })
  it('с холста: без «листать» и «отметки»; на Mac ⌘', () => {
    expect(fill(false, '⌘')).toBe('щелчок или Z — 1:1 · ⌘+колесо — зум · 0 — вписать · Esc — закрыть')
  })
  it('клавиши сняты — куски опускаются, строка не врёт', () => {
    expect(fill(false, 'Ctrl', {})).toBe('щелчок — 1:1 · Ctrl+колесо — зум · Esc — закрыть')
  })
})

// Ступень 2 (128-14): предел зума выше 1:1 по превью, пока есть оригинал.
describe('предел выше 1:1 (оригинал)', () => {
  const at = { x: 600, y: 300 }
  it('origMax: ширина оригинала к ширине превью; не меньше 1', () => {
    expect(origMax({ w: 4096, h: 2730 }, nat)).toBe(2)
    expect(origMax({ w: 4096, h: 2730, color: 'srgb' }, nat)).toBe(2)
    expect(origMax({ w: 1500, h: 1000 }, nat)).toBe(1) // оригинал меньше превью — не уменьшать
  })
  it('origMax: профиль, > 100 Мп, неизвестные размеры — 1', () => {
    expect(origMax({ w: 4096, h: 2730, color: 'adobe' }, nat)).toBe(1)
    expect(origMax({ w: 12000, h: 9000 }, nat)).toBe(1)
    expect(origMax({ w: 0, h: 0 }, nat)).toBe(1) // у фото проекта размеров нет
    expect(origMax({ w: 4096, h: 2730 }, { w: 0, h: 0 })).toBe(1)
    expect(origMax(undefined, nat)).toBe(1)
  })
  it('origWhy: причина отказа; ровно 100 Мп ещё можно', () => {
    expect(origWhy({ w: 4096, h: 2730 })).toBe('')
    expect(origWhy({ w: 4096, h: 2730, color: 'srgb' })).toBe('')
    expect(origWhy({ w: 4096, h: 2730, color: 'adobe' })).toBe('adobe')
    expect(origWhy({ w: 4096, h: 2730, color: 'p3' })).toBe('p3')
    expect(origWhy({ w: 4096, h: 2730, color: 'icc' })).toBe('icc')
    expect(origWhy({ w: 4096, h: 2730, color: 'other' })).toBe('other')
    expect(origWhy({ w: 10000, h: 10000 })).toBe('')
    expect(origWhy({ w: 10001, h: 10000 })).toBe('big')
    expect(origWhy(undefined)).toBe('')
  })
  it('zoomAt при max > 1 идёт выше 1:1 и упирается в max; точка под курсором на месте', () => {
    const before = under(FIT, nat, at)
    const l = zoomAt({ s: 1, cx: 0.5, cy: 0.5 }, 1.5, nat, box, at, 2)
    expect(l.s).toBeCloseTo(1.5, 6)
    expect(zoomAt(l, 100, nat, box, at, 2).s).toBe(2)
    expect(zoomAt(l, 100, nat, box, at, 1).s).toBe(1) // без оригинала предел прежний
    const up = zoomAt(FIT, 1 / fit, nat, box, at, 2)
    const after = under(up, nat, at)
    expect(after.x).toBeCloseTo(before.x, 6)
    expect(after.y).toBeCloseTo(before.y, 6)
  })
  it('значок выше 1:1 — проценты от превью; щелчок из зума выше 1:1 возвращает во вписано', () => {
    const l = { s: 1.5, cx: 0.5, cy: 0.5 }
    expect(zoomLabel(l, nat, box)).toBe('150 %')
    expect(toggle(l, nat, box, at, 1)).toEqual(FIT)
  })
  it('sameShape: ±1 %', () => {
    expect(sameShape(nat, { w: 4096, h: 2730 })).toBe(true)
    expect(sameShape(nat, { w: 4096, h: 2736 })).toBe(true)
    expect(sameShape(nat, { w: 1365, h: 2048 })).toBe(false)
    expect(sameShape(nat, { w: 0, h: 0 })).toBe(false)
  })
})
