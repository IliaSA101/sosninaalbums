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

  it('any: есть ли открытое окно', () => {
    expect(modalStack.any()).toBe(false)
    const a = modalStack.push()
    expect(modalStack.any()).toBe(true)
    modalStack.remove(a)
    expect(modalStack.any()).toBe(false)
  })

  it('depth: место окна в стеке, не выше 9', () => {
    const a = modalStack.push()
    const b = modalStack.push()
    expect(modalStack.depth(a)).toBe(0)
    expect(modalStack.depth(b)).toBe(1)
    modalStack.remove(a)
    // Верхнее окно осело ниже, порядок сохранился.
    expect(modalStack.depth(b)).toBe(0)
    expect(modalStack.depth(-1)).toBe(-1)
    modalStack.remove(b)
    const many = Array.from({ length: 12 }, () => modalStack.push())
    expect(modalStack.depth(many[11])).toBe(9)
    many.forEach((id) => modalStack.remove(id))
  })
})
