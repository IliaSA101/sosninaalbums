// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// «Форма ▭ ▢ ◯» на общем Seg: щелчок — setShape, подсвечено то, что у всех выделенных, без выделения серое.
const ps = vi.hoisted(() => ({
  selected: [] as unknown[],
  allRect: false,
  allRounded: false,
  allOval: false,
  hasRound: false,
  round: 0,
  setShape: vi.fn(),
}))
vi.mock('../places/placesel.svelte', () => ({ placesel: ps }))

const { default: ShapeToggle } = await import('./ShapeToggle.svelte')

afterEach(() => {
  cleanup()
  Object.assign(ps, { selected: [], allRect: false, allRounded: false, allOval: false, hasRound: false })
  ps.setShape.mockReset()
})

describe('ShapeToggle', () => {
  it('щелчок по значкам зовёт setShape(rect/round/oval); имя пункта — подсказка', async () => {
    ps.selected = [{}]
    render(ShapeToggle)
    await fireEvent.click(screen.getByRole('button', { name: 'Прямоугольник' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Скруглённые углы' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Круг; при разных Ш и В — овал' }))
    expect(ps.setShape.mock.calls).toEqual([['rect'], ['round'], ['oval']])
  })

  it('подсвечен только значок, общий для всех выделенных', () => {
    ps.selected = [{}, {}]
    ps.allRounded = true
    render(ShapeToggle)
    expect(screen.getByRole('button', { name: 'Скруглённые углы' }).getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByRole('button', { name: 'Прямоугольник' }).getAttribute('aria-pressed')).toBe('false')
  })

  it('ничего не выделено: серый (aria-disabled), щелчок молчит', async () => {
    render(ShapeToggle)
    const b = screen.getByRole('button', { name: 'Прямоугольник' })
    expect(b.getAttribute('aria-disabled')).toBe('true')
    await fireEvent.click(b)
    expect(ps.setShape).not.toHaveBeenCalled()
  })
})
