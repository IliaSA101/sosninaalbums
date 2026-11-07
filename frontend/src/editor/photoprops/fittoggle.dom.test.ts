// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// «Целиком / С обрезкой» на общем Seg: выбор пункта, подсветка по всем выделенным, серый без выделения.
const ps = vi.hoisted(() => ({ selected: [] as unknown[], allContain: false, allCover: false, setFit: vi.fn() }))
vi.mock('../places/placesel.svelte', () => ({ placesel: ps }))

const { default: FitToggle } = await import('./FitToggle.svelte')

afterEach(() => {
  cleanup()
  ps.selected = []
  ps.allContain = false
  ps.allCover = false
  ps.setFit.mockReset()
})

describe('FitToggle', () => {
  it('щелчок по пунктам зовёт setFit(true/false)', async () => {
    ps.selected = [{}]
    render(FitToggle)
    await fireEvent.click(screen.getByRole('button', { name: 'Целиком' }))
    await fireEvent.click(screen.getByRole('button', { name: 'С обрезкой' }))
    expect(ps.setFit.mock.calls).toEqual([[true], [false]])
  })

  it('подсвечен только пункт, общий для всех выделенных', () => {
    ps.selected = [{}, {}]
    ps.allCover = true
    render(FitToggle)
    expect(screen.getByRole('button', { name: 'С обрезкой' }).getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByRole('button', { name: 'Целиком' }).getAttribute('aria-pressed')).toBe('false')
  })

  it('смешанное выделение (ни все «Целиком», ни все «С обрезкой»): оба пункта не нажаты', () => {
    ps.selected = [{}, {}]
    render(FitToggle)
    for (const n of ['Целиком', 'С обрезкой']) {
      expect(screen.getByRole('button', { name: n }).getAttribute('aria-pressed')).toBe('false')
    }
  })

  it('ничего не выделено: сегмент серый (aria-disabled) и не нажимается', async () => {
    render(FitToggle)
    const b = screen.getByRole('button', { name: 'Целиком' })
    expect(b.getAttribute('aria-disabled')).toBe('true')
    await fireEvent.click(b)
    expect(ps.setFit).not.toHaveBeenCalled()
  })
})
