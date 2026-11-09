import { describe, expect, it, vi } from 'vitest'
import { escCloses } from './modalesc'

describe('escCloses (Modal.onescape)', () => {
  it('без onescape Esc закрывает окно, как прежде', () => {
    expect(escCloses()).toBe(true)
    expect(escCloses(undefined)).toBe(true)
  })
  it('onescape ответил false — окно закрывается; ответил true — остаётся', () => {
    expect(escCloses(() => false)).toBe(true)
    expect(escCloses(() => true)).toBe(false)
  })
  it('onescape зовётся один раз на нажатие', () => {
    const f = vi.fn(() => true)
    escCloses(f)
    expect(f).toHaveBeenCalledTimes(1)
  })
})
