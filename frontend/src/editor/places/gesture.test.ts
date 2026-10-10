import { describe, expect, it } from 'vitest'
import { bodyAction } from './gesture'

describe('bodyAction', () => {
  it('«Авто»: тело всегда переносит фото, в группе тоже', () => {
    expect(bodyAction({ grid: false, selected: false, inGroup: false })).toBe('drag')
    expect(bodyAction({ grid: false, selected: true, inGroup: false })).toBe('drag')
    expect(bodyAction({ grid: false, selected: true, inGroup: true })).toBe('drag')
  })
  it('«Ручная»: тело переносит место — и невыделенное, и выделенное', () => {
    expect(bodyAction({ grid: true, selected: false, inGroup: false })).toBe('move')
    expect(bodyAction({ grid: true, selected: true, inGroup: false })).toBe('move')
  })
  it('«Ручная»: член группы переносит всю группу', () => {
    expect(bodyAction({ grid: true, selected: true, inGroup: true })).toBe('moveGroup')
  })
})
