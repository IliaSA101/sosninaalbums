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
const { gallery } = await import('../../panels/gallery/gallery.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { libfilter } = await import('../../panels/marks/libfilter.svelte')
const { gridAct } = await import('../../panels/layouts/gridact.svelte')

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

  it('окно «Копировать раскладку» закрывается с «Шаблонами», «Копировать оформление» — с «Фонами и стикерами»', async () => {
    prefs.load({ level: 'full', showStart: false, first: false, custom: null }) // «Шаблоны» — с «Полного»
    gridAct.copyGrid = { spread: 's1', from: 'spread' }
    prefs.toggle('templates', false)
    await tick()
    expect(gridAct.copyGrid).toBeNull()
    prefs.toggle('templates', true)
    await tick()
    gridAct.copyGrid = { spread: 's1', from: 'spread', mode: 'decor' }
    prefs.toggle('templates', false)
    await tick()
    expect(gridAct.copyGrid).not.toBeNull()
    prefs.toggle('decor', false)
    await tick()
    expect(gridAct.copyGrid).toBeNull()
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
