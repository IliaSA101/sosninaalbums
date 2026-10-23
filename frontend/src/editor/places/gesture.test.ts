import { describe, expect, it } from 'vitest'
import { bodyAction } from './gesture'
import { setMacForTest, toggleKey } from '../../base/core/platform'

describe('bodyAction', () => {
  it('«Авто»: тело всегда переносит фото, в группе тоже', () => {
    expect(bodyAction({ grid: false, selected: false, inGroup: false, swap: false })).toBe('drag')
    expect(bodyAction({ grid: false, selected: true, inGroup: false, swap: false })).toBe('drag')
    expect(bodyAction({ grid: false, selected: true, inGroup: true, swap: false })).toBe('drag')
  })
  it('«Ручная»: тело переносит место — и невыделенное, и выделенное', () => {
    expect(bodyAction({ grid: true, selected: false, inGroup: false, swap: false })).toBe('move')
    expect(bodyAction({ grid: true, selected: true, inGroup: false, swap: false })).toBe('move')
  })
  it('«Ручная»: член группы переносит всю группу', () => {
    expect(bodyAction({ grid: true, selected: true, inGroup: true, swap: false })).toBe('moveGroup')
  })
})

describe('bodyAction: Ctrl/⌘ — обмен фото (спека 100 п. 6)', () => {
  it('«Ручная» с swap: переносится фото, а не место', () => {
    expect(bodyAction({ grid: true, selected: false, inGroup: false, swap: true })).toBe('drag')
    expect(bodyAction({ grid: true, selected: true, inGroup: false, swap: true })).toBe('drag')
  })
  it('«Ручная», член группы с swap: только это фото, группа не двигается', () => {
    expect(bodyAction({ grid: true, selected: true, inGroup: true, swap: true })).toBe('drag')
  })
  it('«Авто»: swap ничего не меняет', () => {
    expect(bodyAction({ grid: false, selected: true, inGroup: true, swap: true })).toBe('drag')
  })
  it('swap — это toggleKey: Ctrl на Windows, ⌘ на Mac; Shift и Alt его не включают', () => {
    const ev = (o: object) => ({ ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, ...o })
    const pick = (e: ReturnType<typeof ev>) => bodyAction({ grid: true, selected: false, inGroup: false, swap: toggleKey(e) })
    setMacForTest(false)
    expect(pick(ev({ ctrlKey: true }))).toBe('drag')
    expect(pick(ev({ shiftKey: true }))).toBe('move')
    expect(pick(ev({ altKey: true }))).toBe('move')
    expect(pick(ev({}))).toBe('move')
    setMacForTest(true)
    expect(pick(ev({ metaKey: true }))).toBe('drag')
    expect(pick(ev({ ctrlKey: true }))).toBe('move') // на Mac Ctrl — правая кнопка, не обмен
    setMacForTest(null)
  })
})
