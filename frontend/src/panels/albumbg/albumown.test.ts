import { describe, expect, it } from 'vitest'
import type { BackdropView, PatternView, SpreadView } from '../../base/core/types'
import { unitOf } from '../../base/geom/sheet'
import { ownAskText, ownBgOf, ownColorNums, ownDoneText, ownNumsText } from './albumown'

const spread = unitOf({ sheet: false })
const sheet = unitOf({ sheet: true })
const sp = (o: Partial<SpreadView> = {}) => ({ id: 's', bgSet: ['', ''], backdrops: null, patterns: null, ...o }) as unknown as SpreadView
const bd = (o: Partial<BackdropView> = {}) => ({ scope: 'spread', photo: 'p', ...o }) as unknown as BackdropView
const pat = (o: Partial<PatternView> = {}) => ({ scope: 'spread', ...o }) as unknown as PatternView
const many = (n: number) => Array.from({ length: n }, (_, i) => i + 1)

describe('ownColorNums', () => {
  it('номера с 1 по развороту; обложки в view.spreads нет, не считается', () => {
    const spreads = [sp(), sp({ bgSet: ['#4f7a5a', ''] }), sp(), sp({ bgSet: ['', '#111111'] }), sp({ bgSet: ['#1', '#2'] })]
    expect(ownColorNums({ spreads })).toEqual([2, 4, 5])
  })
  it('свой цвет только на левой странице уже считается', () => {
    expect(ownColorNums({ spreads: [sp({ bgSet: ['#4f7a5a', ''] })] })).toEqual([1])
  })
  it('пусто: нет разворотов и нет своих', () => {
    expect(ownColorNums({ spreads: [] })).toEqual([])
    expect(ownColorNums({ spreads: [sp(), sp()] })).toEqual([])
  })
})

describe('ownBgOf', () => {
  it('своё фото или узор любой области; слои альбома не в счёт', () => {
    const spreads = [
      sp({ backdrops: [bd({ album: true })], patterns: [pat({ album: true })] }),
      sp({ backdrops: [bd()] }),
      sp({ patterns: [pat({ scope: 'left' })] }),
      sp(),
    ]
    expect(ownBgOf({ spreads }).nums).toEqual([2, 3])
  })
  it('photos — только свои фото фона, без album и без узоров', () => {
    const spreads = [
      sp({ backdrops: [bd(), bd({ scope: 'left' }), bd({ album: true })] }),
      sp({ backdrops: [bd()], patterns: [pat()] }),
      sp({ patterns: [pat()] }),
    ]
    expect(ownBgOf({ spreads })).toEqual({ nums: [1, 2, 3], photos: 3 })
  })
  it('своих нет (в том числе «скрыт» без своего) — пусто', () => {
    expect(ownBgOf({ spreads: [sp(), sp({ noAlbumBg: true })] })).toEqual({ nums: [], photos: 0 })
  })
})

describe('номера', () => {
  it('склеены диапазонами', () => {
    expect(ownNumsText([3, 5, 6, 7, 8])).toBe('3, 5-8')
  })
  it('12 номеров — все', () => {
    expect(ownNumsText(many(12).map((n) => n * 2))).not.toContain('ещё')
  })
  it('13 номеров — первые 12 и «и ещё 1»', () => {
    const t = ownNumsText(many(13).map((n) => n * 2))
    expect(t.endsWith(', и ещё 1')).toBe(true)
    expect(t).toContain('24')
    expect(t).not.toContain('26')
  })
  it('20 номеров — «и ещё 8»', () => {
    expect(ownNumsText(many(20).map((n) => n * 2))).toMatch(/, и ещё 8$/)
  })
})

describe('тексты вопросов (А1, Б1)', () => {
  it('цвет: дословно из спеки', () => {
    expect(ownAskText('color', { nums: [3, 7, 18], photos: 0 }, spread)).toEqual({
      title: 'Развороты со своим цветом',
      text: 'У 3 разворотов свой цвет: 3, 7, 18. Новый цвет альбома их не коснётся. Сделать их как в альбоме?',
      note: '',
      keep: 'Оставить свой',
      all: 'Как в альбоме',
    })
  })
  it('фон: дословно, без фото — строки note нет', () => {
    const a = ownAskText('bg', { nums: [3, 7, 18, 21], photos: 0 }, spread)
    expect(a.title).toBe('Развороты со своим фоном')
    expect(a.text).toBe('У 4 разворотов свой фон: 3, 7, 18, 21. Фон альбома на них не виден. Сделать их как в альбоме?')
    expect(a.note).toBe('')
  })
  it('фон: есть фото фона — «Фото фона с них уйдут: 2»', () => {
    expect(ownAskText('bg', { nums: [3, 7], photos: 2 }, spread).note).toBe('Фото фона с них уйдут: 2')
  })
  it('падежи: 1 / 2 / 5 / 21', () => {
    const t = (n: number) => ownAskText('color', { nums: many(n), photos: 0 }, spread).text.split(' свой')[0]
    expect(t(1)).toBe('У 1 разворота')
    expect(t(2)).toBe('У 2 разворотов')
    expect(t(5)).toBe('У 5 разворотов')
    expect(t(21)).toBe('У 21 разворота')
  })
  it('один разворот — «его» и «нём», а не «их»', () => {
    expect(ownAskText('color', { nums: [4], photos: 0 }, spread).text).toBe(
      'У 1 разворота свой цвет: 4. Новый цвет альбома его не коснётся. Сделать его как в альбоме?',
    )
    const b = ownAskText('bg', { nums: [4], photos: 1 }, spread)
    expect(b.text).toBe('У 1 разворота свой фон: 4. Фон альбома на нём не виден. Сделать его как в альбоме?')
    expect(b.note).toBe('Фото фона с него уйдут: 1')
  })
  it('проект из листов (031) — «листов», «Листы»', () => {
    const a = ownAskText('color', { nums: [2, 3], photos: 0 }, sheet)
    expect(a.title).toBe('Листы со своим цветом')
    expect(a.text).toBe('У 2 листов свой цвет: 2, 3. Новый цвет альбома их не коснётся. Сделать их как в альбоме?')
    expect(ownAskText('bg', { nums: many(21), photos: 0 }, sheet).text).toContain('У 21 листа свой фон')
  })
})

describe('тост после «Как в альбоме»', () => {
  it('цвет и фон, падежи', () => {
    expect(ownDoneText('color', 3, spread)).toBe('Цвет альбома — теперь и на 3 разворотах')
    expect(ownDoneText('bg', 4, spread)).toBe('Фон альбома — теперь и на 4 разворотах')
    expect(ownDoneText('bg', 1, spread)).toBe('Фон альбома — теперь и на 1 развороте')
    expect(ownDoneText('color', 2, sheet)).toBe('Цвет альбома — теперь и на 2 листах')
    expect(ownDoneText('color', 1, sheet)).toBe('Цвет альбома — теперь и на 1 листе')
  })

  // QA 0.25 (стенд b): слышалось «теперь и у 3 разворотов»; в коде всегда предложный «на», как у фона — падежи 1, 2, 5, 11, 21.
  it('цвет: «на», не «у»; падеж по числу 1, 2, 5, 11, 21', () => {
    const t = (n: number) => ownDoneText('color', n, spread)
    expect(t(1)).toBe('Цвет альбома — теперь и на 1 развороте')
    expect(t(2)).toBe('Цвет альбома — теперь и на 2 разворотах')
    expect(t(5)).toBe('Цвет альбома — теперь и на 5 разворотах')
    expect(t(11)).toBe('Цвет альбома — теперь и на 11 разворотах')
    expect(t(21)).toBe('Цвет альбома — теперь и на 21 развороте')
    for (const n of [1, 2, 3, 5, 21]) expect(t(n)).not.toContain(' у ')
  })
})
