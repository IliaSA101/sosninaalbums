// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// ⊗ у остальных строк TiltRow (124, 0.26-124-36): Скругление, Толщина и Размер рамочки, Наклон фона —
// как у ползунков карточки «Фото»: сброс в значение по умолчанию одним вызовом Go, в значении по умолчанию серая.
const api = vi.hoisted(() => ({ SetPlaceRound: vi.fn(), SetBorder: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})

const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { default: ShapeToggle } = await import('./ShapeToggle.svelte')
const { default: InspectorBorder } = await import('../border/InspectorBorder.svelte')
const { default: BackdropFields } = await import('../backdrop/BackdropFields.svelte')
const { bgmode } = await import('../backdrop/bgmode.svelte')
const { borderAct } = await import('../border/borderact.svelte')
const { backdropTilt } = await import('../tilt/tiltlive')

const rst = (n: string) => screen.getByRole('button', { name: n }) as HTMLButtonElement
const ok = async () => ({ ...store.view, rev: store.view.rev + 1 })

const border = (over: object = {}) => ({
  kind: 'line',
  photo: '',
  lib: '',
  preset: 'sa:fr-white-line',
  name: 'Белая линия',
  inks: ['#ffffff'],
  width: 3,
  defWidth: 1.5,
  scale: 0,
  ins: [3, 3, 3, 3],
  url: '/media/gen/frame?k=line',
  maskUrl: '',
  pic: false,
  open: false,
  known: true,
  ...over,
})
function setup(place: object = {}) {
  store.view = {
    ...store.view,
    rev: 5,
    spreads: [
      {
        id: 's1',
        grid: true,
        photos: ['a'],
        rects: [],
        w: 600,
        h: 300,
        mid: 300,
        frames: [],
        texts: [],
        captions: [],
        places: [{ id: 'p', photo: 'a', x: 10, y: 10, w: 100, h: 100, shape: 'rect', round: 20, ...place }],
      },
    ],
  } as never
  store.go(0)
  placesel.select(['p'])
}

beforeEach(() => {
  vi.spyOn(prefs, 'can').mockImplementation(() => true)
  api.SetPlaceRound.mockImplementation(ok)
  api.SetBorder.mockImplementation(ok)
})
afterEach(() => {
  cleanup()
  placesel.clear()
  borderAct.end()
  backdropTilt.end()
  vi.clearAllMocks()
  vi.restoreAllMocks()
})

describe('Скругление: ⊗', () => {
  it('сбрасывает в 0 % одним SetPlaceRound; при 0 % — серая', async () => {
    setup()
    render(ShapeToggle)
    expect(rst('Сбросить: скругление').disabled).toBe(false)
    await fireEvent.click(rst('Сбросить: скругление'))
    expect(api.SetPlaceRound).toHaveBeenCalledExactlyOnceWith('s1', ['p'], 0)
    cleanup()
    setup({ round: 0 })
    render(ShapeToggle)
    expect(rst('Сбросить: скругление').disabled).toBe(true)
  })
})

describe('Рамочка: ⊗ у «Толщины» и «Размера»', () => {
  it('толщина: SetBorder с толщиной по умолчанию, без серии (merge свой, после сброса серия закрыта)', async () => {
    setup({ border: border() })
    render(InspectorBorder)
    await fireEvent.click(rst('Сбросить: толщина рамочки'))
    await vi.waitFor(() => expect(borderAct.session.merge('border:s1:width:p')).toBe('')) // протяжка после ⊗ начнёт новый шаг
    expect(api.SetBorder).toHaveBeenCalledTimes(1)
    expect(api.SetBorder.mock.calls[0].slice(0, 3)).toEqual(['s1', ['p'], { width: 1.5 }])
    expect(api.SetBorder.mock.calls[0][3]).not.toBe('')
  })

  it('толщина по умолчанию уже стоит или Go её не прислал — ⊗ серая', () => {
    setup({ border: border({ width: 1.5 }) })
    render(InspectorBorder)
    expect(rst('Сбросить: толщина рамочки').disabled).toBe(true)
    cleanup()
    setup({ border: border({ defWidth: 0 }) })
    render(InspectorBorder)
    expect(rst('Сбросить: толщина рамочки').disabled).toBe(true)
  })

  it('размер своей рамочки: SetBorder со scale 1 (100 %); на 100 % — серая', async () => {
    setup({ border: border({ pic: true, preset: '', defWidth: 0, scale: 1.4 }) })
    render(InspectorBorder)
    await fireEvent.click(rst('Сбросить: размер рамочки'))
    expect(api.SetBorder.mock.calls[0].slice(0, 3)).toEqual(['s1', ['p'], { scale: 1 }])
    cleanup()
    setup({ border: border({ pic: true, preset: '', defWidth: 0, scale: 1 }) })
    render(InspectorBorder)
    expect(rst('Сбросить: размер рамочки').disabled).toBe(true)
  })
})

describe('Наклон фона: ⊗', () => {
  const bd = (r: number) =>
    ({
      scope: 'spread',
      photo: 'a',
      lib: '',
      dpi: 0,
      flipH: false,
      flipV: false,
      box: { x: 0, y: 0, w: 100, h: 100, rot: 0 },
      frame: { s: 1, x: 0, y: 0, r },
      img: { w: 100, h: 100 },
    }) as never

  it('сбрасывает в 0° одним SetBackdropAngle; сессия протяжки после него закрыта; при 0° — серая', async () => {
    const setAngle = vi.spyOn(bgmode, 'setAngle').mockResolvedValue(undefined)
    render(BackdropFields, { b: bd(7), spread: 's1' })
    await fireEvent.click(rst('Сбросить: наклон'))
    await vi.waitFor(() => expect(backdropTilt.id).toBe(''))
    expect(setAngle).toHaveBeenCalledTimes(1)
    expect(setAngle.mock.calls[0][0]).toBe(0)
    expect(setAngle.mock.calls[0][1]).not.toBe('') // свой шаг со своей сессией
    cleanup()
    render(BackdropFields, { b: bd(0), spread: 's1' })
    expect(rst('Сбросить: наклон').disabled).toBe(true)
  })
})
