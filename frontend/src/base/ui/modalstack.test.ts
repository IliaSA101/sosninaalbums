import { describe, expect, it } from 'vitest'
import { modalStack } from './modalstack.svelte'

describe('modalStack', () => {
  it('верхним считается последнее открытое окно', () => {
    const a = modalStack.push()
    expect(modalStack.isTop(a)).toBe(true)
    const b = modalStack.push()
    expect(modalStack.isTop(b)).toBe(true)
    expect(modalStack.isTop(a)).toBe(false)
    modalStack.remove(b)
    // Нижнее окно снова верхнее: ему вернутся Esc и фокус.
    expect(modalStack.isTop(a)).toBe(true)
    modalStack.remove(a)
    expect(modalStack.isTop(a)).toBe(false)
  })

  it('закрытие нижнего окна не отнимает верх у верхнего', () => {
    const a = modalStack.push()
    const b = modalStack.push()
    modalStack.remove(a)
    expect(modalStack.isTop(b)).toBe(true)
    modalStack.remove(b)
  })

  it('удаление неизвестного id безвредно', () => {
    const a = modalStack.push()
    modalStack.remove(-1)
    expect(modalStack.isTop(a)).toBe(true)
    modalStack.remove(a)
  })
})
