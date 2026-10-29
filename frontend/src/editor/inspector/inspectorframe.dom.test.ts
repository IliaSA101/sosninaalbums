// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// Строка «Свой кадр» карточки фото (098): подпись пропорций, ⊗ зовёт ResetFrame.
const resetFrame = vi.hoisted(() => vi.fn(async () => ({ rev: 3, folders: [], spreads: [], settings: {} })))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
// Карточка тянет много вложенных компонентов с разными вызовами Go: остальные методы — пустые заглушки.
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ResetFrame: resetFrame }
})

const { default: InspectorFrame } = await import('./InspectorFrame.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

const frame = (extra: object) => ({ s: 1, x: 0, y: 0, img: { x: 0, y: 0, w: 100, h: 100 }, dpi: 300, ...extra })

function show(grid: boolean, extra: object, place: object = {}) {
  store.view = {
    ...store.view,
    spreads: [{ id: 's1', grid, photos: ['a'], places: [{ id: 'p', photo: 'a', ...place }], frames: [frame(extra)], rects: [] }],
  } as never
  vi.spyOn(store, 'selectedCell', 'get').mockReturnValue('a')
  // Только кадр: остальные строки карточки (подписи, имена) в этом тесте не нужны
  vi.spyOn(prefs, 'can').mockImplementation((f) => f === 'frame')
  render(InspectorFrame)
}

afterEach(() => {
  cleanup()
  resetFrame.mockClear()
  vi.restoreAllMocks()
})

describe('InspectorFrame: «Свой кадр»', () => {
  it('на «Авто» со своим кадром — строка с пропорциями; ⊗ зовёт ResetFrame даже при масштабе 100 %', async () => {
    show(false, { own: 16 / 9 })
    expect(screen.getByTestId('own-crop').textContent).toContain('Свой кадр 16 : 9')
    const resets = screen.getAllByRole('button', { name: 'Сбросить кадр' })
    expect(resets).toHaveLength(2) // у «Масштаба» и в строке
    expect((resets[0] as HTMLButtonElement).disabled).toBe(false)
    await fireEvent.click(resets[1])
    expect(resetFrame).toHaveBeenCalledExactlyOnceWith('a')
  })

  it('без своего кадра строки нет, ⊗ у «Масштаба» серый', () => {
    show(false, {})
    expect(screen.queryByTestId('own-crop')).toBeNull()
    expect((screen.getByRole('button', { name: 'Сбросить кадр' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('на ручном месте свой кадр не действует — строки нет', () => {
    show(true, { own: 1.5 })
    expect(screen.queryByTestId('own-crop')).toBeNull()
  })

  it('«Вписано целиком» — строки нет', () => {
    show(true, { own: 1.5 }, { fit: 'contain' })
    expect(screen.queryByTestId('own-crop')).toBeNull()
  })
})
