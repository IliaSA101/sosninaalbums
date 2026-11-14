import { describe, expect, it } from 'vitest'
import type { GuideView } from '../../base/core/types'
import { canClear, clampOpacity, hasImage, linePositions, MAX_LINES, markupUrl, nextRev, OPACITY_DEF, OPACITY_MIN } from './markup'
import { markupUi } from './markupui.svelte'

const g = (o: Partial<GuideView> = {}): GuideView => ({ name: 'spread_30x30.psd', key: 'k1', ok: true, image: true, ...o })

describe('адрес картинки', () => {
  it('по виду и номеру правки', () => {
    expect(markupUrl('cover', 7)).toBe('/media/cover-guide?r=7')
    expect(markupUrl('spreads', 7)).toBe('/media/spread-guide?r=7')
    expect(markupUrl('spreads', NaN)).toBe('/media/spread-guide?r=0')
  })
  it('картинка только при ok и image', () => {
    expect(hasImage(g())).toBe(true)
    expect(hasImage(g({ ok: false }))).toBe(false)
    expect(hasImage(g({ image: false }))).toBe(false) // PSD без сведённого — одни линии
    expect(hasImage(null)).toBe(false)
    expect(hasImage(undefined)).toBe(false)
  })
})

describe('линии направляющих → проценты листа', () => {
  it('вертикальная считается от ширины, горизонтальная от высоты', () => {
    const l = linePositions(
      g({
        lines: [
          { v: true, at: 150 },
          { v: false, at: 30 },
        ],
      }),
      600,
      300,
    )
    expect(l).toEqual([
      { v: true, pct: 25 },
      { v: false, pct: 10 },
    ])
  })
  it('края листа допустимы, вне листа и мусор отброшены', () => {
    const lines = [
      { v: true, at: 0 },
      { v: true, at: 600 },
      { v: true, at: 600.5 },
      { v: false, at: -1 },
      { v: false, at: NaN },
      { v: false, at: 301 },
    ]
    expect(linePositions(g({ lines }), 600, 300).map((x) => x.pct)).toEqual([0, 100])
  })
  it('нет файла, нет линий или нет размера — пусто', () => {
    const lines = [{ v: true, at: 10 }]
    expect(linePositions(null, 600, 300)).toEqual([])
    expect(linePositions(g({ ok: false, lines }), 600, 300)).toEqual([])
    expect(linePositions(g(), 600, 300)).toEqual([])
    expect(linePositions(g({ lines }), 0, 300)).toEqual([])
    expect(linePositions(g({ lines }), 600, NaN)).toEqual([])
  })
  it('линий не больше предела, даже если Go прислал больше', () => {
    const lines = Array.from({ length: MAX_LINES + 50 }, (_, i) => ({ v: true, at: i % 100 }))
    expect(linePositions(g({ lines }), 600, 300)).toHaveLength(MAX_LINES)
  })
})

describe('прозрачность', () => {
  it('зажимается, мусор — по умолчанию', () => {
    expect(clampOpacity(0.7)).toBe(0.7)
    expect(clampOpacity(0)).toBe(OPACITY_MIN)
    expect(clampOpacity(5)).toBe(1)
    expect(clampOpacity(NaN)).toBe(OPACITY_DEF)
  })
  it('у обложки и разворотов своя, не сохраняется', () => {
    markupUi.setOpacity('cover', 0.2)
    expect(markupUi.opacity).toEqual({ cover: 0.2, spreads: OPACITY_DEF })
    markupUi.setOpacity('cover', OPACITY_DEF)
  })
})

describe('номер в адресе (кэш картинки)', () => {
  it('растёт строго, даже при том же номере правки', () => {
    expect(nextRev(0, 5)).toBe(5)
    expect(nextRev(5, 5)).toBe(6)
    expect(nextRev(9, 5)).toBe(10)
    expect(nextRev(3, NaN)).toBe(4)
  })
  it('track: тот же key — номер прежний, другой key — новый, повторный вызов соседа не сбивает', () => {
    const r0 = markupUi.rev.spreads
    markupUi.track('spreads', 'aaa', 100)
    const r1 = markupUi.rev.spreads
    expect(r1).toBeGreaterThan(r0)
    markupUi.track('spreads', 'aaa', 101) // второй экземпляр Spread на тот же файл
    expect(markupUi.rev.spreads).toBe(r1)
    markupUi.track('spreads', 'bbb', 102)
    const r2 = markupUi.rev.spreads
    expect(r2).toBeGreaterThan(r1)
    markupUi.track('spreads', '', 103) // убрали файл
    expect(markupUi.rev.spreads).toBeGreaterThan(r2)
  })
  it('track: одноимённые файлы из разных папок (Undo/Redo) — разный key, адрес новый', () => {
    // Имя в track не участвует: PSD с одним именем, но другим путём/размером/временем даёт другой key.
    markupUi.track('spreads', 'dir1-same.psd', 200)
    const r1 = markupUi.rev.spreads
    markupUi.track('spreads', 'dir2-same.psd', 201)
    expect(markupUi.rev.spreads).toBeGreaterThan(r1)
    const r2 = markupUi.rev.spreads
    markupUi.track('spreads', 'dir1-same.psd', 202) // Undo назад
    expect(markupUi.rev.spreads).toBeGreaterThan(r2)
  })
  it('«Убрать» доступна, пока разметка выбрана, в том числе пропавшая с диска', () => {
    expect(canClear(g())).toBe(true)
    expect(canClear(g({ ok: false }))).toBe(true)
    expect(canClear(null)).toBe(false)
    expect(canClear(undefined)).toBe(false)
  })
  it('виды независимы', () => {
    markupUi.track('cover', 'c.psd', 1)
    const spreads = markupUi.rev.spreads
    markupUi.track('cover', 'd.psd', 2)
    expect(markupUi.rev.spreads).toBe(spreads)
  })
})
