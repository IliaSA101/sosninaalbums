// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])) }
})

const { default: PlaceAddMenu } = await import('./PlaceAddMenu.svelte')
const { default: MirrorMenu } = await import('../../panels/layouts/MirrorMenu.svelte')

afterEach(cleanup)

// Кнопки меню гасят pointerdown (stop — нужен холсту): закрытие чужого меню обязано идти в фазе захвата.
describe('лента правки: два списка не открыты одновременно', () => {
  const mount = () => {
    render(PlaceAddMenu, { title: 'Место', onpick: () => {} })
    render(MirrorMenu, { side: 'right', onpick: () => {} })
  }
  const place = () => screen.getByRole('button', { name: 'Место' })
  const mirror = () => screen.getByRole('button', { name: 'Отразить раскладку' })
  const click = async (b: HTMLElement) => {
    await fireEvent.pointerDown(b)
    await fireEvent.click(b)
  }

  it('«Отразить» открыт, щелчок по «Место» закрывает его', async () => {
    mount()
    await click(mirror())
    expect(mirror().getAttribute('aria-expanded')).toBe('true')
    await click(place())
    expect(mirror().getAttribute('aria-expanded')).toBe('false')
    expect(place().getAttribute('aria-expanded')).toBe('true')
  })
  it('«Место» открыт, щелчок по «Отразить» закрывает его', async () => {
    mount()
    await click(place())
    await click(mirror())
    expect(place().getAttribute('aria-expanded')).toBe('false')
    expect(mirror().getAttribute('aria-expanded')).toBe('true')
  })
  it('повторный щелчок по своей кнопке закрывает меню', async () => {
    mount()
    await click(place())
    await click(place())
    expect(place().getAttribute('aria-expanded')).toBe('false')
  })
})
