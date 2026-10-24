// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Баг G13 (q113-offer): тост итога обновления не должен вытесняться предложением тура.
const setTour = vi.hoisted(() =>
  vi.fn(async (_id: string, _state: string) => ({ level: 'full', showStart: false, first: false, tourDue: false, tourMore: true })),
)
const info = vi.hoisted(() => ({ UpdateInfo: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SetTour: setTour, OpenLink: async () => {}, UpdateInfo: info.UpdateInfo }))

const { default: Tour } = await import('./Tour.svelte')
const { tour } = await import('./tourstate.svelte')
const { MORE_TOAST } = await import('./steps')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { UpdateUi } = await import('../update/update.svelte')
const { NO_UPDATE } = await import('../update/updateapi')
const { justText } = await import('../update/updatetext')

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms))
const shown = () => store.toast?.text // функцией: TS не сужает store.toast по присваиванию null
const spreadWithPhoto = { id: 's1', photos: ['p1'], texts: [], grid: false } as never

beforeEach(() => {
  prefs.load({ level: 'full', showStart: false, first: false, custom: null, tourMore: true })
  store.loaded = true
  store.toast = null
  tour.started = true
  ;(tour as unknown as { offered: boolean }).offered = false
  setTour.mockClear()
})
afterEach(() => {
  cleanup()
  store.view = { ...store.view, spreads: [] }
  store.toast = null
})

describe('предложение тура и тост итога обновления', () => {
  it('«Программа обновлена» не вытесняется; предложение показывается после его ухода, done — только тогда', async () => {
    info.UpdateInfo.mockResolvedValue({ ...NO_UPDATE, mode: 'install', phase: 'idle', just: '0.25.1' })
    await new UpdateUi().start()
    store.view = { ...store.view, spreads: [spreadWithPhoto] }
    render(Tour)
    flushSync()
    expect(shown()).toBe(justText('0.25.1'))
    expect(setTour).not.toHaveBeenCalled()
    store.toast = null // тост истёк
    flushSync()
    expect(shown()).toBe(MORE_TOAST)
    expect(setTour).toHaveBeenCalledWith('spread', 'done')
  })

  it('тоста нет — предложение показывается сразу', async () => {
    store.view = { ...store.view, spreads: [spreadWithPhoto] }
    render(Tour)
    flushSync()
    await wait(0)
    expect(shown()).toBe(MORE_TOAST)
    expect(setTour).toHaveBeenCalledWith('spread', 'done')
  })

  it('прямой вызов offerMore при видимом тосте ничего не пишет и не показывает', () => {
    store.notify('Другой тост')
    tour.offerMore()
    expect(shown()).toBe('Другой тост')
    expect(setTour).not.toHaveBeenCalled()
  })
})
