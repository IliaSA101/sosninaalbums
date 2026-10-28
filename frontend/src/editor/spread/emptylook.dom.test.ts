// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'

// 131: на пустом холсте — ссылка «или начните с макета альбома» рядом с «пустой разворот» и «новая виньетка».
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: SpreadSlots } = await import('./SpreadSlots.svelte')
const { store } = await import('../../base/core/store.svelte')
const { looks } = await import('../../shell/looks/looks.svelte')

const fit = { left: 100, top: 50, width: 400, height: 200 }
const props = {
  settings: {},
  k: 1,
  base: { w: 400, h: 200, mid: 200 },
  centers: [],
  spreadW: 400,
  w: 1400,
  fit,
  place: fit,
  zoomed: false,
  onhover: () => {},
}
const link = () => Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'или начните с макета альбома')

let show: ReturnType<typeof vi.spyOn>
beforeEach(() => {
  store.loaded = true
  store.view = { ...store.view, spreads: [], settings: {}, cover: undefined } as never
  show = vi.spyOn(looks, 'show').mockResolvedValue()
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('пустой холст: «или начните с макета альбома»', () => {
  it('ссылка есть и открывает «Макеты альбома»', async () => {
    render(SpreadSlots, props as never)
    expect(link()).toBeTruthy()
    await fireEvent.click(link() as HTMLElement)
    expect(show).toHaveBeenCalledTimes(1)
  })

  it('проект-лист 031: макетов альбома нет — ссылки тоже', () => {
    render(SpreadSlots, { ...props, settings: { sheet: true } } as never)
    expect(link()).toBeUndefined()
    expect(document.body.textContent).toContain('Альбом пуст')
  })

  it('в альбоме с разворотами пустого холста нет — и ссылки тоже', () => {
    store.view = { ...store.view, spreads: [{ id: 's1', name: '', photos: [], rects: [], w: 400, h: 200, kind: '' }] } as never
    render(SpreadSlots, props as never)
    expect(link()).toBeUndefined()
  })
})
