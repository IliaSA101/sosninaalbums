import { beforeEach, describe, expect, it, vi } from 'vitest'

// 107 U1: вход «Весь альбом заново» (design §9.1), строка «Собрать», галочка свободных, цепочка и тост пересборки альбома.
type Opts = { variant?: number; again?: boolean; album?: boolean }
const builds: { ids: string[]; at: number; o: Opts }[] = []
let answer: (o: Opts) => object = () => ({})
const RANGE = { auto: true, minPer: 1, maxPer: 64, series: true }
const spread = (id: string, photos: string[]) => ({ id, photos })
const view = (rev: number, again: boolean, spreads: { id: string; photos: string[] }[]) => ({
  rev,
  marksRev: 1,
  again,
  folders: [],
  spreads,
})

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  Marks: async () => ({}),
  AutoBuild: async (ids: string[], at: number, o: Opts) => {
    builds.push({ ids, at, o })
    return answer(o)
  },
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('../boot/wire')
const { ownMarks } = await import('../../panels/marks/ownmarks.svelte')
wire()

const tick = () => new Promise((r) => setTimeout(r, 0))
const ph = (id: string) => ({ id, path: id, name: id, w: 100, h: 100 })
const PH = (variant = 1, again = false, album = true) => ({ similar: 0, keep: [], gray: 0, variant, again, ...(album ? { album } : {}) })

// Лента a…e; в альбоме a, c (s1), b (s2); свободные d, e
async function setup(spreads = [spread('s1', ['a', 'c']), spread('s2', ['b'])], photos = ['a', 'b', 'c', 'd', 'e']) {
  builds.length = 0
  store.photos = photos.map(ph) as never
  ownMarks.invalidate()
  store.autoDraft = null
  store.toast = null
  store.selected.clear()
  store.busy = false
  await store.replaceState(Promise.resolve({ view: view(1, false, spreads), photos: store.photos }))
  store.photos = photos.map(ph) as never
  await tick()
}

beforeEach(() => setup())

describe('вход «Автосборка» (design §9.1)', () => {
  it('есть свободные — обычное окно «Свободные фото», режим альбома в строке «Собрать»', () => {
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ from: 'free', ids: ['d', 'e'] })
  })

  it('выделенные — окно выделенных, как раньше', () => {
    store.selected.add('a')
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ from: 'selected', ids: ['a'] })
  })

  it('свободных нет, развороты есть — сразу «Весь альбом заново» вместо тоста «нет фото» (сц. 9)', async () => {
    await setup([spread('s1', ['a', 'c', 'e']), spread('s2', ['b', 'd'])])
    store.openAuto()
    // фото альбома — по ленте, не по разворотам
    expect(store.autoDraft).toMatchObject({ from: 'album', ids: ['a', 'b', 'c', 'd', 'e'], freeIds: [], addFree: false })
    expect(store.autoDraft?.base).toBeUndefined() // выбирать не из чего: строки «Собрать» нет
    expect(store.toast).toBeNull()
  })

  it('свободных нет и развороты не нужны: альбом пуст — тост, окна нет', async () => {
    await setup([], [])
    store.openAuto()
    expect(store.autoDraft).toBeNull()
    expect(store.toast?.text).toBe('Все фото уже в альбоме')
  })

  it('идёт сборка — вход игнорируется', async () => {
    await setup([spread('s1', ['a', 'b', 'c', 'd', 'e'])])
    store.busy = true
    store.openAuto()
    expect(store.autoDraft).toBeNull()
  })
})

describe('строка «Собрать» и галочка свободных', () => {
  it('«Весь альбом заново» из окна свободных: фото альбома по ленте, свободные — в freeIds, галочка выключена', () => {
    store.openAuto()
    store.autorun.setSource('album')
    expect(store.autoDraft).toMatchObject({ from: 'album', ids: ['a', 'b', 'c'], freeIds: ['d', 'e'], addFree: false })
    expect(store.autoDraft?.base).toMatchObject({ from: 'free', ids: ['d', 'e'] })
  })

  it('повторный щелчок по выбранному режиму — ничего не меняет', () => {
    store.openAuto()
    const free = store.autoDraft
    store.autorun.setSource('base')
    expect(store.autoDraft).toBe(free)
    store.autorun.setSource('album')
    const album = store.autoDraft
    store.autorun.setSource('album')
    expect(store.autoDraft).toBe(album)
  })

  it('обратно — тот же выбор, что был (со своей ссылкой)', () => {
    store.openAuto()
    const free = store.autoDraft
    store.autorun.setSource('album')
    store.autorun.setSource('base')
    expect(store.autoDraft).toBe(free)
  })

  it('из выделенных тоже: свободные — не выделение', () => {
    store.selected.add('d')
    store.openAuto()
    store.autorun.setSource('album')
    expect(store.autoDraft).toMatchObject({ from: 'album', ids: ['a', 'b', 'c'], freeIds: ['d', 'e'] })
    expect(store.autoDraft?.base?.from).toBe('selected')
  })

  it('галочка: свободные встают в ids по ленте; снять — уходят', () => {
    store.openAuto()
    store.autorun.setSource('album')
    store.autorun.setAddFree(true)
    expect(store.autoDraft).toMatchObject({ addFree: true, ids: ['a', 'b', 'c', 'd', 'e'] })
    store.autorun.setAddFree(true) // повтор — то же
    expect(store.autoDraft?.ids).toEqual(['a', 'b', 'c', 'd', 'e'])
    store.autorun.setAddFree(false)
    expect(store.autoDraft).toMatchObject({ addFree: false, ids: ['a', 'b', 'c'] })
  })

  it('галочка вне режима альбома и в пересборке не действует', () => {
    store.openAuto()
    const free = store.autoDraft
    store.autorun.setAddFree(true)
    expect(store.autoDraft).toBe(free)
  })

  it('разворотов нет — режим альбома недоступен', async () => {
    await setup([], ['a'])
    store.openAuto()
    store.autorun.setSource('album')
    expect(store.autoDraft?.from).toBe('free')
  })
})

describe('сборка и цепочка', () => {
  const REBUILT = () => view(2, true, [spread('s1', ['a', 'c']), spread('n1', ['b'])])

  async function first(spreads = 2) {
    store.openAuto()
    store.autorun.setSource('album')
    answer = REBUILT
    return store.autoBuild(spreads, RANGE, 1, [], PH(), 0)
  }

  it('Go получает album: true и ids альбома; тост «Альбом пересобран: N разворота» с «Пересобрать…»', async () => {
    expect(await first(2)).toBe(true)
    expect(builds[0]).toMatchObject({ ids: ['a', 'b', 'c'], o: { album: true, again: false } })
    expect(store.toast?.text).toBe('Альбом пересобран: 2 разворота')
    expect(store.toast?.action?.label).toBe('Пересобрать…')
    expect(store.lastAuto?.pick.from).toBe('album')
    expect(store.lastAuto?.pick.base).toBeUndefined()
  })

  it('обычная сборка не шлёт album', async () => {
    store.openAuto()
    answer = REBUILT
    await store.autoBuild(1, RANGE, 1, [], PH(1, false, false), 0)
    expect(builds[0].o.album).toBeUndefined()
    expect(store.toast?.text).toBe('Собрано разворотов: 1')
  })

  it('отброшенные похожие — в тосте (withSimilar)', async () => {
    store.openAuto()
    store.autorun.setSource('album')
    answer = REBUILT
    await store.autoBuild(2, RANGE, 1, [], PH(), 37)
    expect(store.toast?.text).toBe('Альбом пересобран: 2 разворота. Похожих кадров не взято: 37 — они в ленте')
  })

  it('текущий разворот остаётся на месте', async () => {
    store.goId('s2')
    store.openAuto()
    store.autorun.setSource('album')
    answer = () => view(2, true, [spread('s1', ['a', 'c']), spread('s2', ['b'])])
    await store.autoBuild(2, RANGE, 1, [], PH(), 0)
    expect(store.currentId).toBe('s2')
  })

  it('«Пересобрать…» из тоста — окно пересборки альбома: те же фото, вариант +1, без галочки', async () => {
    await first()
    store.toast?.action?.run()
    expect(store.autoDraft).toMatchObject({ from: 'album', again: true, ids: ['a', 'b', 'c'] })
    const { savedOf } = await import('./autoapi')
    expect(savedOf(store.autoDraft!, true, true)).toMatchObject({ variant: 2, again: true, album: true, spreads: 2 })
  })

  it('пересборка: again + album, один тост «Пересобрано: вариант 2»', async () => {
    await first()
    store.openAgain()
    answer = () => view(3, true, [spread('s1', ['a', 'c']), spread('n1', ['b'])])
    await store.autoBuild(2, RANGE, 1, [], PH(2, true), 0)
    expect(builds[1]).toMatchObject({ o: { album: true, again: true, variant: 2 } })
    expect(store.toast?.text).toBe('Пересобрано: вариант 2')
  })

  it('«Автосборка» сразу после (свободных нет) — окно пересборки альбома', async () => {
    await setup([spread('s1', ['a', 'b', 'c', 'd', 'e'])])
    store.openAuto()
    answer = () => view(2, true, [spread('n1', ['a', 'b', 'c', 'd', 'e'])])
    await store.autoBuild(1, RANGE, 1, [], PH(), 0)
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ from: 'album', again: true })
  })

  it('свободные есть: «Автосборка» после альбома — окно свободных без ссылки «Пересобрать прошлую»; строка «Собрать» ведёт в пересборку', async () => {
    await first()
    store.toast = null
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ from: 'free', alt: undefined })
    store.autorun.setSource('album')
    expect(store.autoDraft).toMatchObject({ from: 'album', again: true })
  })

  it('правка альбома закрыла цепочку — «Весь альбом заново» снова новая пересборка', async () => {
    await first()
    store.apply(view(3, false, [spread('s1', ['a', 'c']), spread('n1', ['b'])]) as never)
    store.openAuto()
    store.autorun.setSource('album')
    expect(store.autoDraft).toMatchObject({ from: 'album', ids: ['a', 'b', 'c'] })
    expect(store.autoDraft?.again).toBeUndefined()
  })

  it('пустой список ID (лента ещё не загрузилась) альбому не помеха', async () => {
    await setup([spread('s1', ['a'])], [])
    store.openAuto()
    expect(store.autoDraft).toMatchObject({ from: 'album', ids: [] })
    answer = REBUILT
    expect(await store.autoBuild(1, RANGE, 1, [], PH(), 0)).toBe(true)
    expect(builds[0].o.album).toBe(true)
  })

  it('ошибка Go — тост ошибки, цепочки нет', async () => {
    store.openAuto()
    store.autorun.setSource('album')
    answer = () => {
      throw 'больше 1000 разворотов'
    }
    expect(await store.autoBuild(2, RANGE, 1, [], PH(), 0)).toBe(false)
    expect(store.lastAuto).toBeNull()
  })
})
