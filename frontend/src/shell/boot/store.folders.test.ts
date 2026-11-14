import { beforeEach, describe, expect, it, vi } from 'vitest'

// Фильтр ленты по папкам (054) в связке со store: Go подменён, смотрим, какие ID уходят автосборке и заполнению.
const calls: { name: string; ids: string[] }[] = []
const view = (rev: number, folders: string[], extra: object = {}) => ({ rev, marksRev: 1, folders, spreads: [], ...extra })
const RANGE = { auto: true, minPer: 1, maxPer: 64, series: true }

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  Marks: async () => ({}),
  AutoBuild: async (ids: string[]) => {
    calls.push({ name: 'build', ids })
    return view(9, ['D:/a', 'D:/b', 'D:/c'], { spreads: [{ id: 'n', photos: ids }] })
  },
  FillPlaces: async (ids: string[]) => {
    calls.push({ name: 'fill', ids })
    return { view: view(9, ['D:/a', 'D:/b', 'D:/c']), left: 0 }
  },
  GetState: async () => ({ view: view(20, ['D:/x']), photos: [] }),
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('./wire')
const { folderfilter } = await import('../../panels/folders/folderfilter.svelte')
const { buildTree, findNode, nodeKey } = await import('../../panels/folders/tree')
// Узел дерева папки по текущим папкам и фото store (093: фильтр хранит узлы, а не папки списка).
const node = (dir: string) => findNode(buildTree(store.view.folders, store.photos), nodeKey(dir))!
const { libfilter } = await import('../../panels/marks/libfilter.svelte')

const FOLDERS = ['D:/a', 'D:/b', 'D:/c']
const ph = (id: string, dir: string) => ({ id, path: `${dir}/${id}.jpg`, name: id, w: 100, h: 100 })
wire()

beforeEach(() => {
  calls.length = 0
  folderfilter.reset()
  libfilter.reset()
  store.selected.clear()
  store.toast = null
  store.filter = 'all'
  store.photos = [ph('a1', 'D:/a'), ph('a2', 'D:/a'), ph('b1', 'D:/b'), ph('c1', 'D:/c')] as never
  store.apply(view(1, FOLDERS) as never)
})

const ids = (l: { id: string }[]) => l.map((p) => p.id)

describe('folderfilter в ленте', () => {
  it('ничего не скрыто — лента та же, без копии', () => {
    expect(store.inFolders).toBe(store.photos)
    expect(ids(store.visible)).toEqual(['a1', 'a2', 'b1', 'c1'])
  })

  it('правки без смены папок и фото — тот же массив ленты (без пересчёта)', () => {
    folderfilter.toggle(node('D:/b'))
    const first = store.inFolders
    const vis = store.visible
    store.apply(view(2, [...FOLDERS], { spreads: [{ id: 's', photos: ['a1'] }] }) as never)
    store.apply(view(3, [...FOLDERS]) as never)
    expect(store.inFolders).toBe(first)
    expect(store.visible).toBe(vis)
    // смена скрытых — новый массив
    folderfilter.toggle(node('D:/c'))
    expect(store.inFolders).not.toBe(first)
  })

  it('скрытая папка уходит из ленты сразу; «Показать все» возвращает', () => {
    folderfilter.toggle(node('D:/b'))
    expect(ids(store.visible)).toEqual(['a1', 'a2', 'c1'])
    expect(ids(store.inFolders)).toEqual(['a1', 'a2', 'c1'])
    folderfilter.showAll()
    expect(ids(store.visible)).toHaveLength(4)
  })

  it('«Не в альбоме» и отметки считаются по показанным папкам', () => {
    store.apply(view(2, FOLDERS, { spreads: [{ id: 's', photos: ['a1'] }] }) as never)
    folderfilter.toggle(node('D:/c'))
    store.filter = 'unused'
    expect(ids(store.free)).toEqual(['a2', 'b1'])
    expect(ids(store.visible)).toEqual(['a2', 'b1'])
  })

  it('выделенное в скрытой папке не входит в selectedIds и в Ctrl+A-набор', () => {
    store.selected.add('a1')
    store.selected.add('b1')
    folderfilter.toggle(node('D:/b'))
    expect(store.selectedIds).toEqual(['a1'])
  })

  it('скрыты все папки — лента пуста', () => {
    for (const f of FOLDERS) folderfilter.toggle(node(f))
    expect(store.visible).toEqual([])
    expect(folderfilter.allHidden(store.inFolders.length, store.photos.length)).toBe(true)
  })
})

describe('автосборка и заполнение мест при скрытой папке', () => {
  it('автосборка без выделения берёт только показанные, filtered: true', () => {
    folderfilter.toggle(node('D:/a'))
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ ids: ['b1', 'c1'], filtered: true, from: 'free' })
  })

  it('без скрытых — filtered: false', () => {
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ ids: ['a1', 'a2', 'b1', 'c1'], filtered: false })
  })

  it('автосборка уходит в Go только с показанными', async () => {
    folderfilter.toggle(node('D:/a'))
    store.openAuto()
    await store.autoBuild(1, RANGE)
    expect(calls).toEqual([{ name: 'build', ids: ['b1', 'c1'] }])
  })

  it('заполнение мест — только показанные', async () => {
    folderfilter.toggle(node('D:/c'))
    await store.fillPlaces()
    expect(calls).toEqual([{ name: 'fill', ids: ['a1', 'a2', 'b1'] }])
  })

  it('все папки скрыты — Go не зовём, тост «Под фильтр не подходит…»', async () => {
    for (const f of FOLDERS) folderfilter.toggle(node(f))
    await store.fillPlaces()
    expect(calls).toEqual([])
    expect(store.toast?.text).toBe('Под фильтр не подходит ни одно свободное фото')
  })
})

describe('жизнь фильтра', () => {
  it('убрали скрытую папку — забыта; Ctrl+Z (папка вернулась) — показана', () => {
    folderfilter.toggle(node('D:/b'))
    store.apply(view(2, ['D:/a', 'D:/c']) as never)
    expect(folderfilter.active).toBe(false)
    store.apply(view(3, FOLDERS) as never)
    expect(ids(store.visible)).toHaveLength(4)
  })

  // 093: при одной папке фильтр не снимается — скрыть можно подпапку (054 сбрасывал: при одной папке скрывать было нечего).
  it('осталась одна папка — скрытое под ней остаётся, остальное забыто', () => {
    folderfilter.toggle(node('D:/b'))
    folderfilter.toggle(node('D:/c'))
    store.apply(view(2, ['D:/b']) as never)
    expect([...folderfilter.hidden]).toEqual([nodeKey('D:/b')])
  })

  it('скрыта подпапка — уходят только её фото, фото папки остаются', () => {
    store.photos = [...store.photos, ph('x1', 'D:/b/x')] as never
    folderfilter.toggle(node('D:/b/x'))
    expect(ids(store.visible)).toEqual(['a1', 'a2', 'b1', 'c1'])
    folderfilter.toggle(node('D:/b')) // «—» у папки b: показать всё поддерево
    expect(ids(store.visible)).toHaveLength(5)
  })

  it('новая папка показана', () => {
    folderfilter.toggle(node('D:/b'))
    store.apply(view(2, [...FOLDERS, 'D:/d']) as never)
    expect([...folderfilter.hidden]).toEqual([nodeKey('D:/b')])
    expect(ids(store.visible)).toEqual(['a1', 'a2', 'c1'])
  })

  it('«Новый»/«Открыть» — все показаны, даже если папки те же', async () => {
    folderfilter.toggle(node('D:/b'))
    await store.replaceState(Promise.resolve({ view: view(30, FOLDERS), photos: store.photos }))
    expect(folderfilter.active).toBe(false)
    expect(ids(store.visible)).toHaveLength(4)
  })

  it('перезапуск (GetState) — все показаны', async () => {
    folderfilter.toggle(node('D:/b'))
    await store.init()
    expect(folderfilter.active).toBe(false)
  })
})
