// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// Ползунок масштаба на ячейке (130): на ходу виден силуэт (fitting), на отпускании гаснет сразу, до ответа Go;
// вернули к исходному значению — change не придёт, поэтому гасим на input.
const api = vi.hoisted(() => ({ SetFrame: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})

const { default: FrameControls } = await import('./FrameControls.svelte')
const { framelive } = await import('./framelive.svelte')

const frame = { s: 1.234, x: 0, y: 0, r: 0 }
const props = () => ({
  id: 'a',
  spread: { id: 's1' } as never,
  r: { x: 0, y: 0, w: 40, h: 60 },
  fv: undefined,
  frame,
  cur: frame,
  movable: false,
  zoomable: true,
})
const slider = () => screen.getByRole('slider', { name: 'Масштаб кадра' }) as HTMLInputElement

beforeEach(() => {
  framelive.clear()
  api.SetFrame.mockReset()
  api.SetFrame.mockResolvedValue(undefined)
})
afterEach(() => {
  cleanup()
  framelive.clear()
})

describe('FrameControls: ползунок масштаба', () => {
  it('input — подгонка (силуэт виден), change — гаснет сразу, до ответа Go, кадр держится', async () => {
    let done!: () => void
    api.SetFrame.mockImplementationOnce(() => new Promise((res) => (done = () => res(undefined))))
    render(FrameControls, props())
    await fireEvent.input(slider(), { target: { value: '200' } })
    expect(framelive.fitting('a')).toBe(true)
    expect(api.SetFrame).not.toHaveBeenCalled()
    void fireEvent.change(slider(), { target: { value: '200' } })
    await Promise.resolve()
    expect(api.SetFrame).toHaveBeenCalledTimes(1)
    expect(framelive.fitting('a')).toBe(false)
    expect(framelive.of('a')?.s).toBe(2)
    done()
    await vi.waitFor(() => expect(framelive.of('a')).toBeNull())
  })

  it('потянули и вернули к исходному (123 %): change не приходит, просмотр и силуэт гаснут на input', async () => {
    render(FrameControls, props())
    await fireEvent.input(slider(), { target: { value: '180' } })
    expect(framelive.fitting('a')).toBe(true)
    await fireEvent.input(slider(), { target: { value: '123' } })
    expect(framelive.fitting('a')).toBe(false)
    expect(framelive.of('a')).toBeNull()
    expect(api.SetFrame).not.toHaveBeenCalled()
  })
})
