import { beforeEach, describe, expect, it, vi } from 'vitest'

// Сужение набора функций сбрасывает состояния верхних групп (narrow.ts). Go и события Wails подменены;
// notify ставит таймер через window (тест идёт в окружении node).
vi.stubGlobal('window', globalThis)

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  SetLevel: async (l: string) => ({ level: l, showStart: true, first: false }),
  SetCustom: async () => ({}),
  SetShowStart: async () => ({}),
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))

const { prefs } = await import('../../base/prefs/prefs.svelte')
const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('./wire')
const { placesel } = await import('../../editor/places/placesel.svelte')
const { gallery } = await import('../../panels/layouts/gallery.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { libfilter } = await import('../../panels/marks/libfilter.svelte')

wire()

const tick = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  prefs.load({ level: 'standard', showStart: false, first: false, custom: null })
  store.toast = null
  gallery.cur = null
  modes.big = false
  libfilter.reset()
})

describe('prefs: набор «Своего» (спека 011 U2)', () => {
  it('сужение набора снимает выделение и режимы; расширение — нет', async () => {
    const clear = vi.spyOn(placesel, 'clear')
    prefs.toggle('rotate', true)
    await tick()
    expect(clear).not.toHaveBeenCalled()
    prefs.toggle('manual', false)
    await tick()
    expect(clear).toHaveBeenCalledTimes(1)
    clear.mockRestore()
  })

  it('потеря «Вариантов» закрывает галерею, «Разреза и склейки» — крупный просмотр', async () => {
    gallery.cur = { spread: 's1', scope: 'spread', tab: 'layouts' }
    modes.big = true
    prefs.toggle('variants', false)
    await tick()
    expect(gallery.cur).toBeNull()
    expect(modes.big).toBe(true)
    prefs.toggle('timeline', false)
    await tick()
    expect(modes.big).toBe(false)
  })

  it('потеря «Отметок» сбрасывает фильтр ленты, остальные выключения его не трогают', async () => {
    libfilter.minRating = 3
    libfilter.flag = 'picked'
    prefs.toggle('flip', false)
    await tick()
    expect(libfilter.active).toBe(true)
    prefs.toggle('marks', false)
    await tick()
    expect(libfilter.active).toBe(false)
  })
})
