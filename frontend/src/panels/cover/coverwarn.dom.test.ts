// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/svelte'

// Строка «Типография не найдена: <id>» (123, design §4) в карточке обложки: предупреждение того же вида, что «Не по шаблону…».
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    CoverPresets: vi.fn(async () => []),
  }
})

const { default: InspectorCover } = await import('./InspectorCover.svelte')
const { store } = await import('../../base/core/store.svelte')

const cover = (o: Record<string, unknown> = {}) => ({
  sheet: { id: 'cover', w: 680, h: 240 },
  spec: { preset: 'custom', wrap: 17, steps: [] },
  preset: 'custom',
  spine: 26,
  n: 40,
  over: false,
  layout: 'both',
  spineBg: '',
  spineText: '',
  zones: [],
  drop: [],
  scopes: [],
  safe: [],
  markup: null,
  pxW: 7000,
  pxH: 2000,
  fix: '',
  missing: '',
  ...o,
})
const show = (c: unknown) => {
  store.view = { ...store.view, rev: 2, settings: { pageW: 300, pageH: 300, dpi: 150 }, spreads: [], cover: c } as never
}

beforeEach(() => show(cover()))
afterEach(() => cleanup())

describe('предупреждение «Типография не найдена» в карточке обложки', () => {
  it('missing пуст — строки нет', () => {
    render(InspectorCover)
    expect(screen.queryByText(/Типография не найдена/)).toBeNull()
  })
  it('missing задан — строка-предупреждение (класс warn) без кнопки «Исправить»', () => {
    show(cover({ missing: 'Типография не найдена: old-shop' }))
    render(InspectorCover)
    const el = screen.getByText('Типография не найдена: old-shop')
    expect(el.classList.contains('warn')).toBe(true)
    expect(screen.queryByText('Исправить')).toBeNull()
  })
  it('вместе с «не по шаблону» показываются обе строки, «Исправить» остаётся', () => {
    show(cover({ missing: 'Типография не найдена: x', fix: 'Не по шаблону Фотосферы 20×20' }))
    render(InspectorCover)
    expect(screen.getByText('Типография не найдена: x')).toBeTruthy()
    expect(screen.getByText('Не по шаблону Фотосферы 20×20')).toBeTruthy()
    expect(screen.getByText('Исправить')).toBeTruthy()
  })
})
