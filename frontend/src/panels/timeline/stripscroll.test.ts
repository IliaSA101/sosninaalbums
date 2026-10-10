import { describe, expect, it } from 'vitest'
import { shouldScroll } from './stripscroll'

const base = { posChanged: false, movingStarted: false, scrollLeft: 500, written: 500 }

describe('shouldScroll', () => {
  it('ручная прокрутка + новый rev: не трогаем', () => {
    expect(shouldScroll({ ...base, scrollLeft: 1200 })).toBe(false)
  })
  it('новая позиция ряда: едем, даже после ручной прокрутки', () => {
    expect(shouldScroll({ ...base, scrollLeft: 1200, posChanged: true })).toBe(true)
  })
  it('начался ход ряда: едем', () => {
    expect(shouldScroll({ ...base, scrollLeft: 1200, movingStarted: true })).toBe(true)
  })
  it('прокрутки не было, новый rev/bw: едем', () => {
    expect(shouldScroll(base)).toBe(true)
    expect(shouldScroll({ ...base, scrollLeft: 500.4 })).toBe(true)
  })
  it('первая запись: едем', () => {
    expect(shouldScroll({ ...base, written: null, scrollLeft: 0 })).toBe(true)
  })
})
