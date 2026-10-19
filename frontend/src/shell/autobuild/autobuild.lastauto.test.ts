import { beforeEach, describe, expect, it, vi } from 'vitest'

// 078 U2: последняя сборка (store.lastAuto), тост «Пересобрать…», openAgain и пересборка через Api.AutoBuild.
const builds: { ids: string[]; at: number; o: { variant?: number; again?: boolean; keep?: string[] } }[] = []
let marksAnswer: Record<string, unknown> = {}
let answer: (ids: string[], again: boolean) => object = () => ({})
const RANGE = { auto: true, minPer: 1, maxPer: 64, series: true }
const view = (rev: number, again: boolean, spreads: { id: string; photos: string[] }[] = []) => ({
  rev,
  marksRev: 1,
  again,
  folders: [],
  spreads,
})

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  Marks: async () => marksAnswer,
  AutoBuild: async (ids: string[], at: number, o: { variant?: number; again?: boolean; keep?: string[] }) => {
    builds.push({ ids, at, o })
    return answer(ids, o.again === true)
  },
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('../boot/wire')
const { ownMarks } = await import('../../panels/marks/ownmarks.svelte')
const { savedOf } = await import('./autoapi')
wire()

const tick = () => new Promise((r) => setTimeout(r, 0))
const ph = (id: string) => ({ id, path: id, name: id, w: 100, h: 100 })
const OPTS = (variant = 1, again = false, keep: string[] = []) => ({ similar: 0, keep, gray: 0, variant, again })

// Три фото, два свободных; первая сборка встаёт в конец (at = 1: на s уже стоит a)
beforeEach(async () => {
  builds.length = 0
  marksAnswer = {}
  store.photos = ['a', 'b', 'c', 'd'].map(ph) as never
  ownMarks.invalidate()
  store.autoDraft = null
  store.toast = null
  store.selected.clear()
  store.busy = false
  await store.replaceState(Promise.resolve({ view: view(1, false, [{ id: 's', photos: ['a'] }]), photos: store.photos }))
  store.photos = ['a', 'b', 'c', 'd'].map(ph) as never
  answer = () =>
    view(2, true, [
      { id: 's', photos: ['a'] },
      { id: 'n1', photos: ['b'] },
      { id: 'n2', photos: ['c', 'd'] },
    ])
  await tick()
})

async function firstBuild(variant = 1) {
  store.openAuto()
  expect(store.autoDraft?.ids).toEqual(['b', 'c', 'd'])
  return store.autoBuild(2, RANGE, 1, [], OPTS(variant), 0)
}

describe('lastAuto после «Собрать»', () => {
  it('запоминает фото, позицию вставки, настройки и вариант; тост с «Пересобрать…»', async () => {
    expect(store.lastAuto).toBeNull()
    expect(await firstBuild()).toBe(true)
    const l = store.lastAuto
    expect(l?.pick.ids).toEqual(['b', 'c', 'd'])
    expect(l?.at).toBe(1)
    expect(l?.variant).toBe(1)
    expect(l?.opts.spreads).toBe(2)
    expect(l?.rev).toBe(2)
    expect(store.toast?.text).toBe('Собрано разворотов: 2')
    expect(store.toast?.action?.label).toBe('Пересобрать…')
    expect(store.toast?.rev).toBeUndefined() // звёзды и метки тост не гасят
  })

  it('сборка без ph (старые вызовы): вариант 1, again=false', async () => {
    store.openAuto()
    await store.autoBuild(1, RANGE)
    expect(store.lastAuto?.variant).toBe(1)
    expect(store.lastAuto?.opts.ph).toEqual(OPTS())
  })

  it('ошибка Go — lastAuto не меняется', async () => {
    await firstBuild()
    const was = store.lastAuto
    answer = () => {
      throw 'ошибка'
    }
    store.autoDraft = { ids: ['b'], skipped: 0, from: 'selected', inAlbum: 0, filtered: false }
    expect(await store.autoBuild(1, RANGE)).toBe(false)
    expect(store.lastAuto).toBe(was)
  })

  it('открытие другого альбома сбрасывает lastAuto', async () => {
    await firstBuild()
    await store.replaceState(Promise.resolve({ view: view(9, false), photos: [] }))
    expect(store.lastAuto).toBeNull()
  })
})

describe('openAgain', () => {
  it('жива: окно пересборки с теми же фото, без отклонённых после сборки', async () => {
    await firstBuild()
    marksAnswer = { c: { flag: -1 } }
    ownMarks.invalidate()
    store.apply(view(3, true, [{ id: 's', photos: ['a'] }]) as never)
    await tick()
    store.openAgain()
    expect(store.autoDraft).toMatchObject({ ids: ['b', 'd'], again: true })
  })

  it('кнопка тоста — то же; звезда (rev растёт, again жива) её не закрывает', async () => {
    await firstBuild()
    store.apply({
      ...view(3, true, [
        { id: 's', photos: ['a'] },
        { id: 'n1', photos: [] },
      ]),
      marksRev: 2,
    } as never)
    expect(store.toast?.action?.label).toBe('Пересобрать…')
    store.toast?.action?.run()
    expect(store.autoDraft?.again).toBe(true)
  })

  it('не жива (Go снял again): окно не открывается, тост «пересобрать нельзя»', async () => {
    await firstBuild()
    store.apply(view(3, false) as never) // правка альбома, Ctrl+Z
    store.toast?.action?.run()
    expect(store.autoDraft).toBeNull()
    expect(store.toast?.text).toContain('пересобрать нельзя')
    expect(store.toast?.text).toContain('Отменить сборку')
  })

  it('сборки ещё не было — окно не открывается', () => {
    store.openAgain()
    expect(store.autoDraft).toBeNull()
    expect(store.toast?.text).toContain('пересобрать нельзя')
  })

  it('все фото сборки отклонены — тост, окна нет', async () => {
    await firstBuild()
    marksAnswer = { b: { flag: -1 }, c: { flag: -1 }, d: { flag: -1 } }
    ownMarks.invalidate()
    store.apply(view(3, true) as never)
    await tick()
    store.openAgain()
    expect(store.autoDraft).toBeNull()
    expect(store.toast?.text).toContain('отклонены')
  })

  it('идёт сборка (busy) — игнорируется', async () => {
    await firstBuild()
    store.busy = true
    store.openAgain()
    expect(store.autoDraft).toBeNull()
  })
})

describe('«Автосборка» без свободных фото', () => {
  async function allPlaced(again: boolean) {
    await firstBuild()
    store.apply(view(3, again, [{ id: 's', photos: ['a', 'b', 'c', 'd'] }]) as never)
    await tick()
    store.toast = null
  }

  it('пересборка жива → открывается она, тост «нет фото» не остаётся (сц. 12)', async () => {
    await allPlaced(true)
    store.openAuto()
    expect(store.autoDraft?.again).toBe(true)
    expect(store.toast).toBeNull()
  })

  it('после Ctrl+Z (не жива) — прежний тост «нет фото»', async () => {
    await allPlaced(false)
    store.openAuto()
    expect(store.autoDraft).toBeNull()
    expect(store.toast?.text).toBe('Все фото уже в альбоме')
  })

  it('есть выделение — обычная новая сборка, даже если пересборка жива', async () => {
    await allPlaced(true)
    store.selected.add('b')
    store.openAuto()
    expect(store.autoDraft?.again).toBeUndefined()
    expect(store.autoDraft?.ids).toEqual(['b'])
  })

  it('первой сборки не было, фото нет — тост как в 0.19', () => {
    store.apply(view(5, false, [{ id: 's', photos: ['a', 'b', 'c', 'd'] }]) as never)
    store.openAuto()
    expect(store.autoDraft).toBeNull()
    expect(store.toast?.text).toBe('Все фото уже в альбоме')
  })
})

describe('пересборка: AutoBuild(again)', () => {
  async function again(variant: number, next: object) {
    store.openAgain()
    answer = () => next
    return store.autoBuild(2, RANGE, 1, [], OPTS(variant, true), 0)
  }

  it('позиция вставки — первой сборки; тост «Пересобрано: вариант k»; Пересобрать… снова', async () => {
    await firstBuild()
    // за время между сборками число разворотов в виде сменилось (отметки не меняют альбом, но at всё равно первый)
    store.apply(
      view(3, true, [
        { id: 's', photos: ['a'] },
        { id: 'n1', photos: ['b'] },
        { id: 'n2', photos: ['c', 'd'] },
      ]) as never,
    )
    expect(
      await again(
        2,
        view(4, true, [
          { id: 's', photos: ['a'] },
          { id: 'n1', photos: ['d'] },
          { id: 'n2', photos: ['b', 'c'] },
        ]),
      ),
    ).toBe(true)
    expect(builds[1]).toMatchObject({ at: 1, o: { variant: 2, again: true } })
    expect(store.toast?.text).toBe('Пересобрано: вариант 2')
    expect(store.toast?.action?.label).toBe('Пересобрать…')
    expect(store.lastAuto).toMatchObject({ at: 1, have: 1, variant: 2, rev: 4 }) // have — альбом до первой сборки, не текущие 3
    expect(store.lastAuto?.opts.ph.again).toBe(false) // признак — не часть «настроек сборки»
  })

  it('вариант совпал (rev тот же) — «не отличается», пересборка остаётся доступной', async () => {
    await firstBuild()
    expect(
      await again(
        2,
        view(2, true, [
          { id: 's', photos: ['a'] },
          { id: 'n1', photos: ['b'] },
          { id: 'n2', photos: ['c', 'd'] },
        ]),
      ),
    ).toBe(true)
    expect(store.toast?.text).toBe('Этот вариант не отличается — попробуйте следующий')
    expect(store.toast?.action?.label).toBe('Пересобрать…')
    expect(store.lastAuto?.variant).toBe(2)
  })

  it('переход не прыгает: текущий разворот остаётся (ID прошлого варианта сохраняются)', async () => {
    await firstBuild()
    store.goId('n2')
    await again(
      3,
      view(4, true, [
        { id: 's', photos: ['a'] },
        { id: 'n1', photos: ['b', 'c'] },
        { id: 'n2', photos: ['d'] },
      ]),
    )
    expect(store.currentId).toBe('n2')
  })

  it('первая сборка после пересборки — новая цепочка: again=false, позиция по текущему альбому', async () => {
    await firstBuild()
    await again(
      2,
      view(4, true, [
        { id: 's', photos: ['a'] },
        { id: 'n1', photos: ['b'] },
      ]),
    )
    store.autoDraft = { ids: ['c'], skipped: 0, from: 'selected', inAlbum: 0, filtered: false }
    answer = () =>
      view(6, true, [
        { id: 's', photos: ['a'] },
        { id: 'n1', photos: ['b'] },
        { id: 'x', photos: ['c'] },
      ])
    await store.autoBuild(1, RANGE, 1, [], OPTS(1, false), 0)
    expect(builds[2]).toMatchObject({ at: 2, o: { again: false } })
    expect(store.lastAuto?.at).toBe(2)
    expect(store.lastAuto?.have).toBe(2) // новая цепочка — снимок по текущему альбому
    expect(store.toast?.text).toBe('Собрано разворотов: 1')
  })

  it('ошибка пересборки (ErrAgainStale от Go) — тост ошибки, lastAuto прежний', async () => {
    await firstBuild()
    const was = store.lastAuto
    store.openAgain()
    answer = () => {
      throw 'после сборки альбом менялся — пересобрать нельзя'
    }
    expect(await store.autoBuild(2, RANGE, 1, [], OPTS(2, true), 0)).toBe(false)
    expect(store.toast?.kind).toBe('error')
    expect(store.lastAuto).toBe(was)
  })
})

describe('savedOf: начальные значения окна', () => {
  it('без пересборки — память окна', async () => {
    const d = { ids: ['b'], skipped: 0, from: 'free' as const, inAlbum: 0, filtered: false }
    const s = savedOf(d, true, true)
    expect(s).not.toHaveProperty('again')
    expect(s).not.toHaveProperty('variant')
  })

  it('пересборка — настройки последней сборки, вариант + 1, «Вернуть» сохраняются', async () => {
    store.openAuto()
    await store.autoBuild(
      2,
      { auto: false, minPer: 2, maxPer: 5, series: false },
      2,
      ['dense'],
      { ...OPTS(3, false, ['c']), similar: 2, gray: 1 },
      0,
    )
    store.openAgain()
    const s = savedOf(store.autoDraft!, true, true)
    expect(s).toMatchObject({
      spreads: 2,
      range: [2, 5],
      series: false,
      variety: 2,
      families: ['dense'],
      similar: 2,
      gray: 1,
      keep: ['c'],
      variant: 4,
      again: true,
    })
  })

  it('уровень «Простой»: ч/б и семейства в окно не возвращаются', async () => {
    store.openAuto()
    await store.autoBuild(2, RANGE, 1, ['dense'], { ...OPTS(1, false), gray: 2 }, 0)
    store.openAgain()
    const s = savedOf(store.autoDraft!, false, false)
    expect(s.families).toEqual([])
    expect(s.gray).not.toBe(2)
  })
})
