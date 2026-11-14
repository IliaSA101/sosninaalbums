// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Крестик сброса цвета страниц альбома (135): всегда виден, гаснет при исходном цвете, щелчок — в черновик формы,
// «Применить» сохраняет одним SetSettings; наведение показывает исходный цвет.
const setSettings = vi.hoisted(() => vi.fn(async () => undefined))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    PagePresets: vi.fn(async () => []),
    MarginNote: vi.fn(async () => ''),
    MyColors: vi.fn(async () => []),
    SetSettings: setSettings,
  }
})

const { default: SettingsForm } = await import('./SettingsForm.svelte')
const { store } = await import('../../base/core/store.svelte')
const { colorLive } = await import('../../base/color/colorlive.svelte')

const base = { pageW: 300, pageH: 300, dpi: 300, margin: 10, gap: 3, safe: 5, bleed: 3 }
const TIP = 'Исходный цвет (как у нового альбома)'
const cross = () => screen.getByRole('button', { name: 'Цвет страниц альбома: исходный' }) as HTMLButtonElement
const apply = () => screen.getByRole('button', { name: 'Применить' }) as HTMLButtonElement
const tick = () => new Promise((r) => setTimeout(r, 0))

function open(bg: string, extra: object = {}) {
  store.view = { rev: 1, folders: [], spreads: [], settings: { ...base, bg, ...extra } } as never
  render(SettingsForm)
}
beforeEach(() => {
  store.view = { rev: 1, folders: [], spreads: [], settings: base } as never
})
afterEach(() => {
  cleanup()
  colorLive.clear()
  setSettings.mockClear()
})

describe('SettingsForm: крестик сброса цвета страниц', () => {
  it('цвет исходный (bg пуст): крестик виден, но неактивен, с подсказкой', () => {
    open('')
    expect(cross().disabled).toBe(true)
    expect(cross().title).toBe(TIP)
  })

  it('свой цвет: щелчок — цвет в черновике исходный, Go не зван; «Применить» — один SetSettings с bg ""', async () => {
    open('#4a4643')
    expect(cross().disabled).toBe(false)
    await fireEvent.click(cross())
    flushSync()
    expect(setSettings).not.toHaveBeenCalled()
    expect(cross().disabled).toBe(true)
    expect(screen.getByRole('button', { name: 'Цвет страниц альбома' }).style.getPropertyValue('--c')).toBe('#ffffff')
    await fireEvent.click(apply())
    await tick()
    expect(setSettings).toHaveBeenCalledTimes(1)
    expect((setSettings.mock.calls[0] as unknown[])[0]).toMatchObject({ bg: '' })
  })

  it('наведение показывает исходный цвет на страницах без своего; уход и щелчок снимают показ', async () => {
    open('#4a4643')
    await fireEvent.pointerEnter(cross())
    expect(colorLive.cur?.keys).toEqual(['album'])
    expect(colorLive.cur?.hex).toBe('#ffffff')
    await fireEvent.pointerLeave(cross())
    expect(colorLive.cur).toBeNull()
    await fireEvent.pointerEnter(cross())
    await fireEvent.click(cross())
    expect(colorLive.cur).toBeNull()
  })

  it('режим листов: подпись «Цвет листов», крестик тот же', () => {
    open('#4a4643', { sheet: true })
    const b = screen.getByRole('button', { name: 'Цвет листов: исходный' }) as HTMLButtonElement
    expect(b.disabled).toBe(false)
    expect(b.title).toBe(TIP)
  })

  it('«Сбросить» формы возвращает прежний цвет и гасит «Применить»', async () => {
    open('#4a4643')
    await fireEvent.click(cross())
    flushSync()
    expect(apply().disabled).toBe(false)
    await fireEvent.click(screen.getByRole('button', { name: 'Сбросить' }))
    flushSync()
    expect(cross().disabled).toBe(false)
    expect(apply().disabled).toBe(true)
  })
})
