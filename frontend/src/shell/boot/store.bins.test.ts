import { beforeEach, describe, expect, it, vi } from 'vitest'

// Подборки (083) в связке со store: порядок фильтров папки → подборка → «Не в альбоме» → отметки, автосборка и
// «Заполнить места» из показанной подборки, жизнь выбора. Go подменён.
const calls: { name: string; ids: string[] }[] = []
const view = (rev: number, extra: object = {}) => ({ rev, marksRev: 1, folders: FOLDERS, spreads: [], bins: BINS, ...extra })
const RANGE = { auto: true, minPer: 1, maxPer: 64, series: true }

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

const FOLDERS = ['D:/a', 'D:/b']
const BINS = [
  { id: 'b1', name: 'Обязательно' },
  { id: 'b2', name: 'Запас' },
]

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('./wire')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { binsel } = await import('../../panels/bins/binsel.svelte')
const { binCounts } = await import('../../panels/bins/bins')
const { folderfilter } = await import('../../panels/folders/folderfilter.svelte')
const { buildTree, findNode, nodeKey } = await import('../../panels/folders/tree')
// Узел дерева папки по текущим папкам и фото store (093: фильтр хранит узлы, а не папки списка).
const node = (dir: string) => findNode(buildTree(store.view.folders, store.photos), nodeKey(dir))!
const { libfilter } = await import('../../panels/marks/libfilter.svelte')
const { ownMarks } = await import('../../panels/marks/ownmarks.svelte')

const ph = (id: string, dir: string) => ({ id, path: `${dir}/${id}.jpg`, name: id, w: 100, h: 100 })
const ids = (l: { id: string }[]) => l.map((p) => p.id)
wire()

// a1, a2 (папка a), b1, b2, b3 (папка b). Обязательно = a1, a2, b1; Запас = b2.
beforeEach(() => {
  calls.length = 0
  prefs.level = 'standard'
  folderfilter.reset()
  libfilter.reset()
  binsel.reset()
  store.selected.clear()
  store.toast = null
  store.filter = 'all'
  store.photos = [ph('a1', 'D:/a'), ph('a2', 'D:/a'), ph('b1', 'D:/b'), ph('b2', 'D:/b'), ph('b3', 'D:/b')] as never
  ownMarks.map = { a1: { bins: ['b1'] }, a2: { bins: ['b1'], rating: 5 }, b1: { bins: ['b1'] }, b2: { bins: ['b2'] } }
  store.apply(view(1) as never)
})

describe('лента подборки', () => {
  it('подборка не показана — тот же массив, без пересчёта', () => {
    expect(store.inBin).toBe(store.inFolders)
    expect(ids(store.visible)).toHaveLength(5)
  })

  it('показана — только её фото в порядке ленты', () => {
    binsel.show('b1')
    expect(ids(store.inBin)).toEqual(['a1', 'a2', 'b1'])
    expect(ids(store.visible)).toEqual(['a1', 'a2', 'b1'])
  })

  it('порядок: папки → подборка → «Не в альбоме» → отметки', () => {
    store.apply(view(2, { spreads: [{ id: 's', photos: ['a1'] }] }) as never)
    binsel.show('b1')
    folderfilter.toggle(node('D:/b')) // скрыта папка b: b1 уходит раньше подборки
    expect(ids(store.inFolders)).toEqual(['a1', 'a2'])
    expect(ids(store.inBin)).toEqual(['a1', 'a2'])
    folderfilter.showAll()
    store.filter = 'unused'
    expect(ids(store.free)).toEqual(['a2', 'b1']) // a1 уже в месте
    libfilter.minRating = 5
    expect(ids(store.visible)).toEqual(['a2'])
    expect(store.beforeMarks.length).toBe(2) // «из M» фильтра — после подборки и «Не в альбоме»
  })

  it('«Обязательно» + «Не в альбоме» = что ещё не поставлено', () => {
    store.apply(view(2, { spreads: [{ id: 's', photos: ['a1', 'b2'] }] }) as never)
    binsel.show('b1')
    store.filter = 'unused'
    expect(ids(store.visible)).toEqual(['a2', 'b1'])
  })

  it('выделение вне показанной подборки не входит в selectedIds', () => {
    store.selected.add('a1')
    store.selected.add('b3')
    binsel.show('b1')
    expect(store.selectedIds).toEqual(['a1'])
  })

  it('пустая подборка — пустая лента; «Все фото» возвращает', () => {
    binsel.show('b2')
    ownMarks.map = {}
    expect(store.inBin).toEqual([])
    expect(store.visible).toEqual([])
    binsel.showAll()
    expect(ids(store.visible)).toHaveLength(5)
  })

  it('числа окна не расходятся с «Не в альбоме»: в альбоме = всего − свободные', () => {
    store.apply(view(2, { spreads: [{ id: 's', photos: ['a1'] }], backdropUsed: ['b1'] }) as never)
    const c = binCounts(BINS, store.photos, binsel.index, store.used, store.backdropUsed).get('b1')!
    binsel.show('b1')
    store.filter = 'unused'
    expect(c.all - c.inAlbum).toBe(store.visible.length)
    expect(c).toEqual({ all: 3, inAlbum: 2 })
  })
})

describe('автосборка и «Заполнить места» из подборки', () => {
  it('без выделения — фото показанной подборки, имя — в окне', () => {
    binsel.show('b1')
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ ids: ['a1', 'a2', 'b1'], from: 'free', bin: 'Обязательно' })
  })

  it('подборка не показана — как раньше, без bin', () => {
    store.openAuto()
    expect(store.autoDraft?.ids).toHaveLength(5)
    expect(store.autoDraft?.bin).toBeUndefined()
  })

  it('с выделением — выделенные (подборка не называется)', () => {
    binsel.show('b1')
    store.selected.add('a1')
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ ids: ['a1'], from: 'selected' })
    expect(store.autoDraft?.bin).toBeUndefined()
  })

  it('в Go уходят только фото подборки', async () => {
    binsel.show('b1')
    store.openAuto()
    await store.autoBuild(1, RANGE)
    expect(calls).toEqual([{ name: 'build', ids: ['a1', 'a2', 'b1'] }])
  })

  it('«Заполнить места» — тоже только подборка', async () => {
    binsel.show('b2')
    await store.fillPlaces()
    expect(calls).toEqual([{ name: 'fill', ids: ['b2'] }])
  })

  it('все фото подборки уже в альбоме — Go не зовём, тост с её именем', async () => {
    store.apply(view(2, { spreads: [{ id: 's', photos: ['a1', 'a2', 'b1'] }] }) as never)
    binsel.show('b1')
    await store.fillPlaces()
    expect(calls).toEqual([])
    expect(store.toast?.text).toBe('Все фото подборки «Обязательно» уже в альбоме')
  })

  it('подборка пуста — тост «нет фото»', async () => {
    ownMarks.map = {}
    binsel.show('b2')
    await store.fillPlaces()
    expect(calls).toEqual([])
    expect(store.toast?.text).toBe('В подборке «Запас» нет фото')
  })

  it('подборка + фильтр отметок: «Под фильтр не подходит…»', async () => {
    binsel.show('b1')
    libfilter.minRating = 5
    store.apply(view(2, { spreads: [{ id: 's', photos: ['a2'] }] }) as never)
    await store.fillPlaces()
    expect(store.toast?.text).toBe('Под фильтр не подходит ни одно свободное фото')
  })
})

describe('жизнь выбора', () => {
  it('удалили показанную подборку — «Все фото»', () => {
    binsel.show('b2')
    store.apply(view(2, { bins: [BINS[0]] }) as never)
    expect(binsel.cur).toBe('')
    expect(ids(store.visible)).toHaveLength(5)
  })

  it('Ctrl+Z удаления: подборка вернулась, но «показана» не возвращается сама', () => {
    binsel.show('b2')
    store.apply(view(2, { bins: [BINS[0]] }) as never)
    store.apply(view(3) as never)
    expect(binsel.cur).toBe('')
  })

  it('правка альбома без смены подборок выбор не трогает', () => {
    binsel.show('b1')
    store.apply(view(2, { spreads: [{ id: 's', photos: ['a1'] }] }) as never)
    expect(binsel.cur).toBe('b1')
  })

  it('«Новый»/«Открыть» — все фото, цель забыта', async () => {
    binsel.show('b1')
    binsel.target = 'b2'
    await store.replaceState(Promise.resolve({ view: view(30), photos: store.photos }))
    expect([binsel.cur, binsel.target]).toEqual(['', ''])
  })

  it('перезапуск (GetState) — все фото', async () => {
    binsel.show('b1')
    await store.init()
    expect(binsel.cur).toBe('')
  })
})

describe('уровень «Простой»', () => {
  it('подборка выбрана, но лента не фильтруется; подборки в данных целы', () => {
    binsel.show('b1')
    prefs.level = 'simple'
    expect(store.inBin).toBe(store.inFolders)
    expect(ids(store.visible)).toHaveLength(5)
    expect(store.view.bins).toHaveLength(2)
    store.openAuto()
    expect(store.autoDraft?.bin).toBeUndefined()
  })
})
