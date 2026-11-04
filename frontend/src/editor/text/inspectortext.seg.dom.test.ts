// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// Выравнивание и Ж/К карточки текста на общем Seg: щелчок — одна SetTextStyle, фокус редактора не отнимается, серое по шрифту.
const setStyle = vi.hoisted(() => vi.fn(async () => undefined))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    MyColors: vi.fn(async () => []),
    SetTextStyle: setStyle,
  }
})

const { default: InspectorText } = await import('./InspectorText.svelte')
const { edit } = await import('./edit.svelte')
const { store } = await import('../../base/core/store.svelte')

afterEach(() => {
  cleanup()
  setStyle.mockClear()
  edit.sel = null
})

function show(style: object, faces: { bold: boolean; italic: boolean }[]) {
  const t = { id: 't1', x: 0, y: 0, w: 50, h: 10, style, lines: [], z: 0, rot: 0 }
  store.view = {
    ...store.view,
    rev: 6,
    settings: { pageW: 300, pageH: 300 },
    spreads: [{ id: 's1', grid: false, photos: [], places: [], frames: [], rects: [], texts: [t], captions: [] }],
  } as never
  store.fonts = [{ id: 'f', name: 'F', faces, css: 'F' }] as never
  store.go(0)
  edit.sel = { kind: 'text', spread: 's1', id: 't1' }
  render(InspectorText)
}
const base = { font: 'f', size: 12, color: '#000000', align: 'left' }

describe('InspectorText: Seg', () => {
  it('выравнивание: нажат текущий, щелчок по другому — SetTextStyle(align)', async () => {
    show(base, [{ bold: true, italic: true }])
    expect(screen.getByRole('button', { name: 'По левому краю' }).getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(screen.getByRole('button', { name: 'По центру' }))
    expect(setStyle).toHaveBeenCalledExactlyOnceWith('t1', expect.objectContaining({ align: 'center' }))
    expect(screen.getByRole('button', { name: 'По правому краю' }).getAttribute('title')).toBe('По правому краю')
  })

  it('Ж и К переключают свои флаги; нажаты включённые', async () => {
    show({ ...base, bold: true }, [{ bold: true, italic: true }])
    const b = screen.getByRole('button', { name: 'Жирный' })
    const i = screen.getByRole('button', { name: 'Курсив' })
    expect(b.getAttribute('aria-pressed')).toBe('true')
    expect(i.getAttribute('aria-pressed')).toBe('false')
    await fireEvent.click(i)
    expect(setStyle).toHaveBeenLastCalledWith('t1', expect.objectContaining({ bold: true, italic: true }))
    await fireEvent.click(b)
    expect(setStyle).toHaveBeenLastCalledWith('t1', expect.objectContaining({ bold: false }))
  })

  it('у шрифта нет начертания: пункт серый с подсказкой и не нажимается', async () => {
    show(base, [{ bold: false, italic: false }])
    const b = screen.getByRole('button', { name: 'Жирный' })
    expect(b.getAttribute('aria-disabled')).toBe('true')
    expect(b.getAttribute('title')).toBe('У этого шрифта нет жирного начертания')
    await fireEvent.click(b)
    expect(setStyle).not.toHaveBeenCalled()
  })

  it('mousedown по пунктам не отнимает фокус у редактора', async () => {
    show(base, [{ bold: true, italic: true }])
    expect(await fireEvent.mouseDown(screen.getByRole('button', { name: 'По центру' }))).toBe(false)
    expect(await fireEvent.mouseDown(screen.getByRole('button', { name: 'Жирный' }))).toBe(false)
  })
})
