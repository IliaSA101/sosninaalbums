// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Плашка корешка и окно цвета (109): при показе плашка видна, даже если цвета ещё нет.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))

const { default: CoverPlate } = await import('./CoverPlate.svelte')
const { colorLive } = await import('../../base/color/colorlive.svelte')

const cover = (spineBg: string) =>
  ({
    sheet: { w: 600, h: 300 },
    spineBg,
    zones: [{ kind: 'spine', rect: { x: 290, y: 0, w: 20, h: 300 } }],
  }) as never

afterEach(() => {
  cleanup()
  colorLive.clear()
})

describe('CoverPlate: показ цвета плашки', () => {
  it('нет цвета — плашки нет; наведение показывает её, уход убирает', () => {
    const r = render(CoverPlate, { cover: cover('') })
    expect(r.container.querySelector('.plate')).toBeNull()
    colorLive.set(['spine'], '#D8A9A1')
    flushSync()
    expect((r.container.querySelector('.plate') as HTMLElement).style.background).toContain('#d8a9a1')
    colorLive.clear()
    flushSync()
    expect(r.container.querySelector('.plate')).toBeNull()
  })

  it('есть цвет — показ другого цвета перекрывает его, чужой ключ не мешает', () => {
    const r = render(CoverPlate, { cover: cover('#112233') })
    colorLive.set(['bg:cover:spread'], '#ffffff')
    flushSync()
    expect((r.container.querySelector('.plate') as HTMLElement).style.background).toContain('#112233')
    colorLive.set(['spine'], '#ffffff')
    flushSync()
    expect((r.container.querySelector('.plate') as HTMLElement).style.background).toContain('#ffffff')
  })
})
