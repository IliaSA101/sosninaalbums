// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// 107 U2/U3: строка «Собрать» и галочка (AutoMode), вопрос «Пересобрать альбом?» и путь «Пересобрать…» → вопрос → Go.
const builds: { ids: string[]; o: { album?: boolean; again?: boolean } }[] = []
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  Marks: async () => ({}),
  AutoBuild: async (ids: string[], _at: number, o: { album?: boolean; again?: boolean }) => {
    builds.push({ ids, o })
    return { rev: 3, folders: [], spreads: [{ id: 's1', photos: ['a'] }] }
  },
}))
vi.mock('../editor/text/fonts', () => ({ loadFonts: async () => {} }))

const { default: AutoMode } = await import('./AutoMode.svelte')
const { default: RebuildAskDialog } = await import('./RebuildAskDialog.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { AutoDialog } = await import('./autobuild.svelte')
const { goBuild } = await import('./autogo')
const { rebuildAsk } = await import('./rebuildask.svelte')
const { unitOf } = await import('../../base/geom/sheet')

const FREE = { ids: ['d', 'e'], skipped: 0, from: 'free', inAlbum: 0, filtered: false } as const
const ALBUM = { ids: ['a', 'b'], skipped: 0, from: 'album', inAlbum: 0, filtered: false, freeIds: ['d', 'e'], addFree: false } as const
const A = { spreads: 2, photos: 5, free: 0, manual: 1, vignettes: 0, decor: 0, blank: 0 }
const PLAN = { photos: 5, spreads: 2, minSpreads: 1, maxSpreads: 5, avg: 3, min: 2, max: 3, reuse: 0, from: 2, to: 3, cuts: 0, album: A }
const settle = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  builds.length = 0
  store.view = { ...store.view, rev: 1, folders: [], spreads: [{ id: 's1', photos: [] }] } as never
  store.autoDraft = null
  store.busy = false
  rebuildAsk.cancel()
})
afterEach(cleanup)

describe('AutoMode: строка «Собрать»', () => {
  it('свободные + режим альбома; щелчок по альбому переключает окно, повторный — нет', async () => {
    store.autoDraft = { ...FREE, ids: [...FREE.ids] }
    render(AutoMode, { d: store.autoDraft, plan: null })
    expect(screen.getByRole('button', { name: 'Свободные фото: 2' }).getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(screen.getByRole('button', { name: 'Весь альбом заново' }))
    expect(store.autoDraft?.from).toBe('album')
    const d = store.autoDraft
    await fireEvent.click(screen.getByRole('button', { name: 'Весь альбом заново' }))
    expect(store.autoDraft).toBe(d)
  })

  it('режим альбома: щелчок по «Свободные фото» возвращает выбор; видна галочка и она ставится', async () => {
    store.autoDraft = { ...ALBUM, ids: [...ALBUM.ids], freeIds: [...ALBUM.freeIds], base: { ...FREE, ids: [...FREE.ids] } }
    const { rerender } = render(AutoMode, { d: store.autoDraft!, plan: { ...PLAN } as never })
    expect(screen.getByText('Добавить свободные фото ленты: 2')).toBeTruthy()
    await fireEvent.click(screen.getByRole('checkbox'))
    expect(store.autoDraft).toMatchObject({ addFree: true, ids: [] }) // ленты в тесте нет: ids из неё не берутся
    await rerender({ d: store.autoDraft!, plan: { ...PLAN } as never })
    await fireEvent.click(screen.getByRole('button', { name: 'Свободные фото: 2' }))
    expect(store.autoDraft?.from).toBe('free')
  })

  it('режим без выбора (свободных нет): строки «Собрать» нет, только источник', () => {
    render(AutoMode, { d: { ...ALBUM, ids: [...ALBUM.ids], freeIds: [] }, plan: { ...PLAN } as never })
    expect(screen.queryByRole('group', { name: 'Собрать' })).toBeNull()
    expect(screen.queryByRole('checkbox')).toBeNull()
    expect(screen.getByText(/Весь альбом: 5 фото на 2 разворотах; не трогаю: 1 ручной/)).toBeTruthy()
  })

  it('разворотов нет — выбирать не из чего', () => {
    store.view = { ...store.view, spreads: [] } as never
    render(AutoMode, { d: { ...FREE, ids: [...FREE.ids] }, plan: null })
    expect(screen.queryByRole('group', { name: 'Собрать' })).toBeNull()
  })

  it('пересборка альбома: галочки нет', () => {
    render(AutoMode, {
      d: { ...ALBUM, ids: [], freeIds: [...ALBUM.freeIds], again: true, base: { ...FREE, ids: [...FREE.ids] } },
      plan: null,
    })
    expect(screen.queryByRole('checkbox')).toBeNull()
    expect(screen.getByRole('group', { name: 'Собрать' })).toBeTruthy()
  })
})

describe('«Пересобрать…» → вопрос → Go', () => {
  async function dialog(again = false) {
    store.autoDraft = { ...ALBUM, ids: [...ALBUM.ids], freeIds: [], again }
    const d = new AutoDialog(['a', 'b'], async () => PLAN as never, { spreads: null, album: true, again })
    d.start()
    await settle()
    return d
  }

  it('сперва вопрос с числами плана; Go не звали', async () => {
    const d = await dialog()
    await goBuild(d, { famOn: true, grayOn: true, unit: unitOf(undefined) })
    expect(builds).toEqual([])
    expect(rebuildAsk.ask?.lines[0]).toBe('2 разворота соберутся заново: раскладки и фото на них сменятся')
    expect(rebuildAsk.ask?.lines.at(-1)).toMatch(/^Отменить — /)
  })

  it('«Отмена» — вопрос закрыт, сборки нет; «Пересобрать» — сборка с album, память N не тронута', async () => {
    modes.autoSpreads = 7
    const d = await dialog()
    const o = { famOn: true, grayOn: true, unit: unitOf(undefined) }
    await goBuild(d, o)
    render(RebuildAskDialog)
    flushSync()
    expect(screen.getByRole('dialog').getAttribute('aria-label')).toBe('Пересобрать альбом?')
    expect(document.activeElement?.textContent).toBe('Отмена')
    await fireEvent.click(screen.getByText('Отмена'))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(builds).toEqual([])

    await goBuild(d, o)
    flushSync()
    await fireEvent.click(screen.getByText('Пересобрать'))
    await settle()
    expect(builds[0]).toMatchObject({ ids: ['a', 'b'], o: { album: true } })
    expect(modes.autoSpreads).toBe(7)
    expect(rebuildAsk.ask).toBeNull()
  })

  it('пересборка цепочки (вариант +1) — без вопроса', async () => {
    const d = await dialog(true)
    await goBuild(d, { famOn: true, grayOn: true, unit: unitOf(undefined) })
    expect(rebuildAsk.ask).toBeNull()
    expect(builds[0].o).toMatchObject({ album: true, again: true })
  })

  it('голых нет: вопрос не открывается', async () => {
    store.autoDraft = { ...ALBUM, ids: [], freeIds: [] }
    const d = new AutoDialog([], async () => ({ ...PLAN, photos: 0, spreads: 0, album: { ...A, spreads: 0, photos: 0 } }) as never, {
      spreads: null,
      album: true,
    })
    d.start()
    await settle()
    await goBuild(d, { famOn: true, grayOn: true, unit: unitOf(undefined) })
    expect(rebuildAsk.ask).toBeNull()
    expect(builds).toEqual([])
  })
})
