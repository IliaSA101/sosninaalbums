import { describe, expect, it } from 'vitest'
import { COL_W, MINI_H, ROT_KEEP } from './minipanel'
import { panelOutside } from './miniview'
import { arcWhereOf, colHeight, colHeightOf, colPlace, placeMini } from './minicol'

// Холст 1000×450, разворот 600×305 мм вписан (1,37 px/мм): серое поле по бокам 89 px, 16 px сверху и снизу (прототип 108).
const PPM = 1.37
const SX = 89
const SY = 16
const view = { left: -SX, top: -SY, right: 1000 - SX, bottom: 450 - SY }
const H = colHeight(9, 5) // 269: самый высокий столбец («Подогнать» есть)
const mm = (x: number, y: number, w: number, h: number) => ({ x, y, w, h })
const px = (r: { x: number; y: number; w: number; h: number }) => ({
  left: r.x * PPM,
  right: (r.x + r.w) * PPM,
  top: r.y * PPM,
  bottom: (r.y + r.h) * PPM,
})

describe('colHeight', () => {
  it('colHeight: Г одно фото 245, с «Подогнать» 269, группа 188, «Простой» 98', () => {
    expect(colHeight(8, 5)).toBe(245)
    expect(colHeight(9, 5)).toBe(269)
    expect(colHeight(6, 4)).toBe(188)
    expect(colHeight(3, 2)).toBe(98)
  })
  it('по списку значков: линия перед значком считается; пусто — 0', () => {
    expect(colHeightOf([])).toBe(0)
    expect(colHeightOf([{}, {}, { sep: true }, { sep: true }])).toBe(colHeight(4, 2))
  })
})

describe('colPlace', () => {
  it('справа от фото, верх вровень с верхом фото (владелец 2026-10-23)', () => {
    const b = mm(14, 14, 180, 270)
    const p = colPlace(b, PPM, H, view)
    expect(p).toEqual({ left: px(b).right + 8, top: px(b).top, where: 'right' })
  })
  it('справа нет места — слева (владелец 2026-10-23)', () => {
    // фото у правого края видимой области: справа 20 px, слева — свободно
    const b = mm(380, 20, 100, 150)
    const p = colPlace(b, PPM, H, { ...view, right: px(b).right + 20 })
    expect(p?.where).toBe('left')
    expect(p?.left).toBe(px(b).left - 8 - COL_W)
  })
  it('ни справа, ни слева — внутри у правого края, зазор 14, по середине видимой части (владелец 2026-10-23)', () => {
    const b = mm(-200, -300, 1200, 800) // приближено: больше экрана
    const v = { left: 0, top: 0, right: 1000, bottom: 450 }
    const p = colPlace(b, 1, H, v)
    expect(p).toEqual({ left: 1000 - 14 - COL_W, top: 450 / 2 - H / 2, where: 'inright' })
  })
  it('соседнее фото справа сторону не меняет (владелец 2026-10-23)', () => {
    const a = mm(14, 14, 100, 150)
    const neighbour = mm(120, 14, 100, 150) // вплотную, столбец ляжет на него
    const p = colPlace(a, PPM, H, view)
    expect(p?.where).toBe('right')
    expect(p?.left).toBeLessThan(px(neighbour).right)
    // от соседей ничего не зависит: в аргументах их нет
    expect(colPlace(a, PPM, H, view, undefined, [])).toEqual(p)
  })
  it('фото выше или ниже видимой области — столбца нет; частично видно — есть', () => {
    const v = { left: 0, top: 1000, right: 1000, bottom: 1600 }
    expect(colPlace(mm(100, 200, 200, 200), 1, H, v)).toBeNull() // целиком выше окна
    expect(colPlace(mm(100, 1600, 200, 200), 1, H, v)).toBeNull() // целиком ниже окна
    expect(colPlace(mm(100, 1700, 200, 200), 1, H, v)).toBeNull()
    expect(colPlace(mm(100, 900, 200, 200), 1, H, v)?.where).toBe('right') // низ виден (y1 = 1100)
    expect(colPlace(mm(100, 1500, 200, 200), 1, H, v)?.where).toBe('right') // верх виден
  })
  it('placeMini: фото вне окна по вертикали — панель спрятана (panelOutside), как ряд в 0.23', () => {
    const v = { left: 0, top: 1000, right: 1000, bottom: 1600 }
    for (const b of [mm(100, 200, 200, 200), mm(100, 2000, 200, 200)]) {
      const p = placeMini(b, 1, 3000, 3000, 300, H, undefined, 0, v, [])
      expect(['right', 'left', 'inright']).not.toContain(p.where)
      expect(panelOutside(p, MINI_H, false, v)).toBe(true)
    }
  })
  it('у низа видимой области столбец поднимается и не выходит за неё', () => {
    const b = mm(300, 250, 60, 50) // мелкое у низа
    const p = colPlace(b, PPM, H, view)!
    expect(p.where).toBe('right')
    expect(p.top).toBe(view.bottom - H - 4)
    expect(p.top).toBeGreaterThanOrEqual(view.top + 4)
  })
  it('низкий холст: заход на ленту, если низ фото виден; низ за краем — без захода', () => {
    const low = { left: -SX, top: -SY, right: 1000 - SX, bottom: 200 - SY } // 216 px высоты: 269 не помещается
    expect(colPlace(mm(14, 20, 100, 100), PPM, H, low)).toBeNull() // ленты под холстом нет
    const withStrip = { ...low, spill: 200 - SY + 110 }
    const p = colPlace(mm(14, 20, 100, 100), PPM, H, withStrip)!
    expect(p.top + H).toBeGreaterThan(low.bottom) // зашёл на ленту
    expect(p.top + H).toBeLessThanOrEqual(withStrip.spill - 4)
    // низ фото за краем холста — ленту не используем
    expect(colPlace(mm(14, 20, 100, 300), PPM, H, withStrip)).toBeNull()
  })
  it('не помещается и с лентой — null (запасной ряд по правилам 2026-10-17)', () => {
    const tiny = { left: -SX, top: -SY, right: 1000 - SX, bottom: 100, spill: 150 }
    expect(colPlace(mm(14, 14, 60, 40), PPM, H, tiny)).toBeNull()
    expect(colPlace(mm(14, 14, 60, 40), PPM, 0, view)).toBeNull() // нет значков
  })
  it('внутри: видимая часть уже трёх столбцов или ниже столбца + 28 — null', () => {
    const v = { left: 0, top: 0, right: 1000, bottom: 450 }
    // видимая ширина 3·34 − 1 (область узкая: сбоку от фото места тоже нет)
    const wide = mm(-500, -100, 1600, 800)
    expect(colPlace(wide, 1, H, { ...v, right: 3 * COL_W - 1 })).toBeNull()
    expect(colPlace(wide, 1, H, { ...v, right: 3 * COL_W })?.where).toBe('inright')
    // видимая высота h + 27
    expect(colPlace(mm(-500, 450 - (H + 27), 1600, 800), 1, H, v)).toBeNull()
    expect(colPlace(mm(-500, 450 - (H + 28), 1600, 800), 1, H, v)?.where).toBe('inright')
  })
  it('внутри: подпись у низа — столбец над ней; места нет — null', () => {
    const b = mm(-200, -300, 1200, 800)
    const v = { left: 0, top: 0, right: 1000, bottom: 450 }
    const cap = mm(900, 330, 80, 30) // под столбцом (середина 90…360), ниже середины
    const p = colPlace(b, 1, H, v, undefined, [cap])!
    expect(p.where).toBe('inright')
    expect(p.top + H).toBe(330 - 8)
    // подпись слишком высоко: столбец поднимается выше допустимого — null
    expect(colPlace(b, 1, H, v, undefined, [mm(900, 150, 80, 30)])).toBeNull()
    // подпись в стороне от столбца по x — не мешает
    expect(colPlace(b, 1, H, v, undefined, [mm(100, 400, 80, 30)])?.top).toBe(450 / 2 - H / 2)
  })
  it('⟳ повёрнутого места на пути столбца — сторона пропускается', () => {
    const b = mm(14, 14, 180, 270)
    const r = px(b)
    // ⟳ стоит там, где лёг бы столбец справа
    const av = { x: r.right + 8 + COL_W / 2, y: r.top + 40 }
    expect(colPlace(b, PPM, H, view, av)?.where).toBe('left')
    // ⟳ далеко — сторона прежняя
    expect(colPlace(b, PPM, H, view, { x: r.right + 8 + COL_W + ROT_KEEP + 1, y: r.top })?.where).toBe('right')
  })
  it('карточка справа срезает видимую область — столбец слева', () => {
    const b = mm(200, 14, 100, 150)
    expect(colPlace(b, PPM, H, view)?.where).toBe('right')
    const cut = colPlace(b, PPM, H, { ...view, right: px(b).right + 30 }) // карточка поверх холста срезала справа
    expect(cut?.where).toBe('left')
  })
})

describe('placeMini и arcWhereOf', () => {
  const b = mm(14, 14, 180, 270)
  it('столбец, если помещается; иначе прежний ряд', () => {
    expect(placeMini(b, PPM, 600 * PPM, 305 * PPM, 200, H, undefined, 0, view, []).where).toBe('right')
    const low = { left: -SX, top: -SY, right: 1000 - SX, bottom: 250 } // столбец не помещается, ряд под фото — да
    const row = placeMini(mm(14, 14, 60, 40), PPM, 600 * PPM, 305 * PPM, 200, H, undefined, 0, low, [])
    expect(['below', 'above', 'inside', 'top']).toContain(row.where)
    expect(row.top + MINI_H).toBeGreaterThan(0)
  })
  it('место дуги считается тем же путём, что панель: сначала столбец, потом ряд', () => {
    expect(arcWhereOf(b, PPM, 600 * PPM, 305 * PPM, 200, H, 0, view, [])).toBe('right')
    const low = { left: -SX, top: -SY, right: 1000 - SX, bottom: 250 } // столбец не помещается, ряд под фото — да
    expect(arcWhereOf(mm(14, 14, 60, 40), PPM, 600 * PPM, 305 * PPM, 200, H, 0, low, [])).toBe('below')
  })
})
