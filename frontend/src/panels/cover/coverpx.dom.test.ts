// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// Строка «Файл обложки, px: Ш × В» в карточке обложки (123, design §1.3): ввод → App.SetCoverPx (300 dpi), один вызов — один шаг отмены,
// ошибка Go — тостом, как у других правок. Go подменён.
const api = vi.hoisted(() => ({ SetCoverPx: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    CoverPresets: vi.fn(async () => []),
    ...api,
  }
})

const { default: InspectorCover } = await import('./InspectorCover.svelte')
const { store } = await import('../../base/core/store.svelte')

const tick = () => new Promise((r) => setTimeout(r, 0))
// Фабрика, корешок 26 мм при 40 разворотах: лист 8031 × 2835 px при 300 dpi.
// Лист обложки как от Go: с фоном, узорами и подписями — карточка рисует и группу «Фон»
const SHEET = {
  id: 'cover',
  w: 680,
  h: 240,
  bg: ['#ffffff', '#ffffff'],
  bgSet: ['', ''],
  backdrops: [],
  patterns: [],
  captions: [],
  photos: [],
}
const cover = (o: Record<string, unknown> = {}) => ({
  sheet: SHEET,
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
  ...o,
})
const show = (c: unknown, rev = 2) => {
  store.view = { ...store.view, rev, settings: { pageW: 300, pageH: 300, dpi: 150 }, spreads: [], cover: c } as never
}
const w = () => screen.getByLabelText('Ширина файла обложки, px') as HTMLInputElement
const h = () => screen.getByLabelText('Высота файла обложки, px') as HTMLInputElement
const type = async (el: HTMLInputElement, v: string) => {
  await fireEvent.input(el, { target: { value: v } })
  await fireEvent.change(el)
}

beforeEach(() => {
  api.SetCoverPx.mockReset()
  store.toast = null
  show(cover())
})
afterEach(() => cleanup())

describe('строка «Файл обложки, px»', () => {
  it('показывает лист при 300 dpi (не dpi альбома) и подпись про разворотов и корешок', () => {
    render(InspectorCover)
    expect(screen.getByText('Файл обложки, px: Ш × В')).toBeTruthy()
    expect(w().value).toBe('8031')
    expect(h().value).toBe('2835')
    expect(screen.getByText('при 40 разворотах (корешок 26 мм)')).toBeTruthy()
  })
  it('ввод зовёт SetCoverPx(w, h) один раз и после ответа показывает новый лист', async () => {
    api.SetCoverPx.mockImplementation(async () => {
      const v = { ...store.view, rev: 3, cover: cover({ sheet: { ...SHEET, w: (8031 * 25.4) / 300, h: (2836 * 25.4) / 300 } }) }
      return v
    })
    render(InspectorCover)
    await fireEvent.input(w(), { target: { value: '8031' } })
    await type(h(), '2836')
    await tick()
    expect(api.SetCoverPx).toHaveBeenCalledTimes(1)
    expect(api.SetCoverPx).toHaveBeenCalledWith(8031, 2836)
    expect(w().value).toBe('8031')
    expect(h().value).toBe('2836')
    expect(store.cover?.sheet.h).toBeCloseTo(240.12, 1) // вид принят от Go
  })
  it('отказ Go — тост об ошибке, поля возвращаются к листу обложки', async () => {
    api.SetCoverPx.mockRejectedValue('сторонка 20 мм меньше 50')
    render(InspectorCover)
    await type(w(), '100')
    await tick()
    expect(store.toast?.kind).toBe('error')
    expect(store.toast?.text).toContain('торонка 20 мм меньше 50')
    expect(w().value).toBe('8031')
  })
  it('мусор, ноль, дробь, пусто и то же число: Go не зовётся, поле возвращает прежнее', async () => {
    render(InspectorCover)
    for (const v of ['', '0', '-3', '2.5', '8031']) {
      await type(w(), v)
      expect(w().value, `«${v}»`).toBe('8031')
    }
    expect(api.SetCoverPx).not.toHaveBeenCalled()
  })
  it('Esc возвращает прежнее без вызова', async () => {
    render(InspectorCover)
    await fireEvent.input(w(), { target: { value: '9000' } })
    await fireEvent.keyDown(w(), { key: 'Escape' })
    expect(w().value).toBe('8031')
    expect(api.SetCoverPx).not.toHaveBeenCalled()
  })
  it('один разворот: подпись в единственном числе падежа', () => {
    show(cover({ n: 1, spine: 6 }), 3)
    render(InspectorCover)
    expect(screen.getByText('при 1 развороте (корешок 6 мм)')).toBeTruthy()
  })
})
