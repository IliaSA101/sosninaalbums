import { describe, expect, it } from 'vitest'
import { clampPad, padOf, sampleCaption, startCaps, templateOf } from './sheetcaps'
import type { CaptionView, SpreadView, TextStyle } from '../../base/core/types'

const style: TextStyle = { font: 'montserrat', size: 10, color: '#555555', align: 'center' }
const cap = (over: Partial<CaptionView> = {}) => ({ photo: 'p1', style, over: false, pad: 1, ...over }) as CaptionView

describe('clampPad', () => {
  it('зажимает 0…20 и округляет до 0,1', () => {
    expect(clampPad(-3)).toBe(0)
    expect(clampPad(99)).toBe(20)
    expect(clampPad(4.04)).toBe(4)
    expect(clampPad(0.55)).toBeCloseTo(0.6, 5)
  })
  it('не число — прежнее значение', () => {
    expect(clampPad(NaN, 3)).toBe(3)
    expect(clampPad(Infinity, 2)).toBe(2)
  })
})

describe('padOf', () => {
  it('нет поля у Go — 1 мм', () => {
    expect(padOf(undefined)).toBe(1)
    expect(padOf({})).toBe(1)
    expect(padOf({ pad: 4 })).toBe(4)
    expect(padOf({ pad: 0 })).toBe(0)
  })
})

describe('startCaps', () => {
  it('берёт стиль образца, ужимать включено, шаблона нет', () => {
    const c = startCaps(cap({ over: true, pad: 4 }))
    expect(c).toEqual({ style, pad: 4, over: true, wrap: false, text: '' })
    expect(c.style).not.toBe(style) // копия: правка листа не трогает вид подписи
  })
})

describe('sampleCaption', () => {
  const sp = { captions: [cap({ photo: 'a' }), cap({ photo: 'b' })] } as unknown as SpreadView
  it('выделенная главнее первой', () => {
    expect(sampleCaption(sp, sp.captions[1])?.photo).toBe('b')
    expect(sampleCaption(sp)?.photo).toBe('a')
    expect(sampleCaption({ captions: [] } as unknown as SpreadView)).toBeUndefined()
  })
})

describe('templateOf', () => {
  it('пустое и из пробелов — пусто, остальное как есть', () => {
    expect(templateOf('  \n ')).toBe('')
    expect(templateOf('{Фамилия}\n{Имя}')).toBe('{Фамилия}\n{Имя}')
  })
})
