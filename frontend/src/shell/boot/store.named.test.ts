import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Поиск по имени (0.26-124) в связке со store: стадия named после фильтра отметок, M и «Показано» отметок от поиска не
// меняются, автосборка и «Заполнить места» берут найденные, сброс при смене альбома. Go подменён.
const calls: { name: string; ids: string[] }[] = []
const view = (rev: number, extra: object = {}) => ({ rev, marksRev: 1, folders: ['D:/a'], spreads: [], bins: [], ...extra })

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  Marks: async () => ({}),
  AutoBuild: async (ids: string[]) => {
    calls.push({ name: 'build', ids })
    return { rev: 9, marksRev: 1, folders: [], spreads: [{ id: 'n', photos: ids }], bins: [] }
  },
  FillPlaces: async (ids: string[]) => {
    calls.push({ name: 'fill', ids })
    return { view: { rev: 9, marksRev: 1, folders: [], spreads: [], bins: [] }, left: 0 }
  },
  GetState: async () => ({ view: { rev: 20, marksRev: 1, folders: [], spreads: [], bins: [] }, photos: [] }),
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('./wire')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { libfilter } = await import('../../panels/marks/libfilter.svelte')
const { nameSearch } = await import('../../panels/marks/namefind.svelte')
const { ownMarks } = await import('../../panels/marks/ownmarks.svelte')
const { run } = await import('../menu/commands')

const NAMES = ['IMG_0798.JPG', 'IMG_0801.JPG', 'IMG_0805.JPG', 'IMG_1798.JPG', 'Свадьба_015.JPG']
const ph = (name: string) => ({ id: name, path: `D:/a/${name}`, name, w: 100, h: 100 })
const ids = (l: { id: string }[]) => l.map((p) => p.id)
// Запрос действует сразу (Enter), без паузы набора.
const search = (q: string) => {
  nameSearch.type(q)
  nameSearch.commit()
}
wire()

beforeEach(() => {
  calls.length = 0
  prefs.level = 'standard'
  libfilter.reset()
  nameSearch.reset()
  store.selected.clear()
  store.toast = null
  store.filter = 'all'
  store.photos = NAMES.map(ph) as never
  store.apply(view(1) as never)
  ownMarks.map = { 'IMG_0798.JPG': { rating: 5 }, 'IMG_0801.JPG': { rating: 5 }, 'IMG_1798.JPG': { rating: 5 } }
})
afterEach(() => nameSearch.reset())

describe('стадия named', () => {
  it('поиска нет — лента та же, тот же массив', () => {
    expect(store.visible).toBe(store.marked)
    expect(ids(store.visible)).toEqual(NAMES)
  })

  it('номер режет ленту; M («до поиска») не меняется', () => {
    search('798')
    expect(ids(store.visible)).toEqual(['IMG_0798.JPG'])
    expect(store.marked).toHaveLength(5)
  })

  it('после фильтра отметок: оба условия вместе, «Показано» отметок — без поиска', () => {
    libfilter.minRating = 5 // IMG_0798, IMG_0801, IMG_1798
    search('0805, 0801')
    expect(ids(store.marked)).toEqual(['IMG_0798.JPG', 'IMG_0801.JPG', 'IMG_1798.JPG'])
    expect(ids(store.visible)).toEqual(['IMG_0801.JPG']) // IMG_0805 без звёзд отсеяна фильтром
    expect(store.beforeMarks).toHaveLength(5)
  })

  it('«Не в альбоме» идёт до поиска', () => {
    store.apply(view(2, { spreads: [{ id: 's', photos: ['IMG_0798.JPG'] }] }) as never)
    store.filter = 'unused'
    search('798')
    expect(store.visible).toEqual([])
    expect(store.marked).toHaveLength(4)
    expect(nameSearch.empty).toBe(true)
  })

  it('найдено в библиотеке, но скрыто лентой: «Не в альбоме» — «· скрыто: 805», нет — только то, чего нет вовсе', () => {
    store.photos = [...NAMES, 'IMG_0900.JPG'].map(ph) as never
    store.apply(view(2, { spreads: [{ id: 's', photos: ['IMG_0805.JPG'] }] }) as never)
    store.filter = 'unused'
    search('798, 801, 805, 7777')
    expect(ids(store.visible)).toEqual(['IMG_0798.JPG', 'IMG_0801.JPG'])
    expect(nameSearch.hidden).toEqual(['805'])
    expect(nameSearch.hiddenText).toEqual({
      text: '· скрыто: 805',
      title: 'Есть в библиотеке, но скрыто фильтрами ленты: 805',
    })
    expect(nameSearch.miss).toEqual(['7777'])
  })

  it('скрыто фильтром отметок; сняли фильтр — «скрыто» пропало', () => {
    libfilter.minRating = 5 // IMG_0805 без звёзд
    search('801, 805')
    expect(nameSearch.hidden).toEqual(['805'])
    libfilter.reset()
    expect(nameSearch.hidden).toEqual([])
  })

  it('токен, у которого хоть один кадр в ленте, не скрыт; больше трёх — числом', () => {
    store.photos = [...NAMES, '_MG_0798.CR2', 'IMG_0900.JPG', 'IMG_0901.JPG'].map(ph) as never
    libfilter.minRating = 5 // в ленте: IMG_0798, IMG_0801, IMG_1798
    search('798')
    expect(nameSearch.hidden).toEqual([]) // IMG_0798 в ленте, _MG_0798 — нет
    search('805, 015, 900, 901')
    expect(nameSearch.hidden).toEqual(['805', '015', '900', '901'])
    expect(nameSearch.hiddenText.text).toBe('· скрыто 4')
  })

  it('русская раскладка и ё/е: «шьп_080» и «СВАДЬБА»', () => {
    search('шьп_080')
    expect(ids(store.visible)).toEqual(['IMG_0801.JPG', 'IMG_0805.JPG'])
    search('СВАДЬБА')
    expect(ids(store.visible)).toEqual(['Свадьба_015.JPG'])
  })

  it('новый массив фото — индекс пересобирается (сканирование добавило кадр)', () => {
    search('900')
    expect(ids(store.visible)).toEqual([])
    store.photos = [...NAMES, 'IMG_0900.JPG'].map(ph) as never
    expect(ids(store.visible)).toEqual(['IMG_0900.JPG'])
  })

  it('«ничего не найдено» только если фото в ленте есть; пустая лента — не поиск', () => {
    search('zzz')
    expect(nameSearch.empty).toBe(true)
    expect(nameSearch.missText.text).toBe('· нет: zzz')
    store.photos = []
    expect(nameSearch.empty).toBe(false)
  })

  it('Ctrl+A выделяет найденные', () => {
    search('801, 805')
    run('selectAll')
    expect([...store.selected].sort()).toEqual(['IMG_0801.JPG', 'IMG_0805.JPG'])
    expect(store.selectedIds).toEqual(['IMG_0801.JPG', 'IMG_0805.JPG'])
  })

  it('другой альбом (reloaded) и перезапуск сбрасывают поиск', () => {
    search('798')
    nameSearch.open = true
    store.hooks.reloaded()
    expect(nameSearch.q).toBe('')
    expect(nameSearch.active).toBe(false)
    expect(nameSearch.open).toBe(false)
    expect(ids(store.visible)).toEqual(NAMES)
  })
})

describe('автосборка и «Заполнить места» с поиском', () => {
  it('без выделения — найденные свободные, «фильтр ленты»', () => {
    search('801, 805')
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ ids: ['IMG_0801.JPG', 'IMG_0805.JPG'], from: 'free', filtered: true })
  })

  it('поиска нет — как раньше, filtered: false', () => {
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ filtered: false })
    expect(store.autoDraft?.ids).toHaveLength(5)
  })

  it('с выделением — выделенные', () => {
    search('801, 805')
    store.selected.add('IMG_0801.JPG')
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ ids: ['IMG_0801.JPG'], from: 'selected' })
  })

  it('в Go уходят только найденные', async () => {
    search('805')
    store.openAuto()
    await store.autoBuild(1, { auto: true, minPer: 1, maxPer: 64, series: true })
    expect(calls).toEqual([{ name: 'build', ids: ['IMG_0805.JPG'] }])
    nameSearch.reset()
    search('801')
    await store.fillPlaces()
    expect(calls[1]).toEqual({ name: 'fill', ids: ['IMG_0801.JPG'] })
  })

  it('ничего не нашлось — Go не зовём, тост «Под фильтр не подходит…»', async () => {
    search('zzz')
    await store.fillPlaces()
    expect(calls).toEqual([])
    expect(store.toast?.text).toBe('Под фильтр не подходит ни одно свободное фото')
  })

  it('найденные уже в альбоме — не берутся', async () => {
    store.apply(view(2, { spreads: [{ id: 's', photos: ['IMG_0801.JPG'] }] }) as never)
    search('801, 805')
    store.openAuto()
    expect(store.autoDraft?.ids).toEqual(['IMG_0805.JPG'])
  })
})
