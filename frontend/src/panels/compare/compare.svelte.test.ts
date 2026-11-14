// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Состояние окна сравнения (138): открытие по выделению и тосты Т2–Т7, «Похожие к этому», «Лучший», закрытие, листание.
const m = vi.hoisted(() => ({ similar: vi.fn(), pick: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => ({
  ...(await orig<typeof import('../../../wailsjs/go/app/App')>()),
  SimilarTo: m.similar,
  PickBest: m.pick,
}))

const { compare, restForBest, SIMILAR_TEXT, WHY_TEXT } = await import('./compare.svelte')
const { store } = await import('../../base/core/store.svelte')
const { photoui } = await import('../../editor/photo/photoui.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')

const IDS = ['a', 'b', 'c', 'd', 'e', 'f']
const select = (...ids: string[]) => {
  store.selected.clear()
  for (const id of ids) store.selected.add(id)
  store.anchor = ids[0] ?? null
}
const toast = () => store.toast?.text

beforeEach(() => {
  useCatalog()
  store.photos = IDS.map((id) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })) as never
  store.toast = null
  photoui.reveal = null
  select()
  m.similar.mockReset()
  m.pick.mockReset()
  vi.spyOn(store, 'run').mockResolvedValue(true)
})
afterEach(() => {
  compare.close()
  dropCatalog()
  vi.restoreAllMocks()
})

describe('открытие по выделению', () => {
  it('0, 5 и один кадр в ленте — тосты Т2, Т3, Т4, окно закрыто', () => {
    expect(compare.openByKey()).toBe(false)
    expect(toast()).toBe(WHY_TEXT.none(0))
    select('a', 'b', 'c', 'd', 'e')
    expect(compare.openByKey()).toBe(false)
    expect(toast()).toBe('Сравнить можно до 4 фото — выделено 5')
    store.photos = [{ id: 'a', name: 'a.jpg', w: 3, h: 2 }] as never
    select('a')
    expect(compare.openByKey()).toBe(false)
    expect(toast()).toBe('Сравнивать не с чем: в ленте одно фото')
    expect(compare.open).toBe(false)
  })

  it('2–4 выделено — эти кадры в порядке ленты; активный — первый', () => {
    select('d', 'b', 'c')
    expect(compare.openByKey()).toBe(true)
    expect(compare.ids).toEqual(['b', 'c', 'd'])
    expect(compare.activeId).toBe('b')
  })

  it('один выделенный — он и следующий; у последнего — предыдущий', () => {
    select('b')
    compare.openByKey()
    expect(compare.ids).toEqual(['b', 'c'])
    compare.close()
    select('f')
    compare.openByKey()
    expect(compare.ids).toEqual(['e', 'f'])
    expect(compare.activeId).toBe('f')
  })
})

describe('«Похожие к этому»', () => {
  const ask = () => {
    select('c')
    return compare.similarByKey()
  }
  it('ответ Go открывает окно: кадр и похожие в порядке ленты, активный — исходный; список ленты уходит в Go', async () => {
    m.similar.mockResolvedValue({ ids: ['b', 'c', 'd'], more: 0, why: '' })
    expect(await ask()).toBe(true)
    expect(m.similar).toHaveBeenCalledExactlyOnceWith('c', IDS)
    expect(compare.ids).toEqual(['b', 'c', 'd'])
    expect(compare.activeId).toBe('c')
    expect(toast()).toBeUndefined()
  })

  it('больше трёх — тост Т5 с числом похожих и окно открыто', async () => {
    m.similar.mockResolvedValue({ ids: ['b', 'c', 'd', 'e'], more: 2, why: '' })
    await ask()
    expect(compare.open).toBe(true)
    expect(toast()).toBe(SIMILAR_TEXT.more(5))
  })

  it('нет похожих — Т6, нет подписи — Т7, окна нет', async () => {
    m.similar.mockResolvedValue({ ids: [], more: 0, why: 'none' })
    await ask()
    expect(toast()).toBe('Похожих кадров рядом в ленте нет')
    m.similar.mockResolvedValue({ ids: [], more: 0, why: 'nosig' })
    await ask()
    expect(toast()).toBe('Похожих пока не найти: миниатюра этого фото ещё готовится')
    expect(compare.open).toBe(false)
  })

  it('ошибка Go — тост ошибки, окна нет; без выделения — Т2 и вызова нет', async () => {
    m.similar.mockRejectedValue(new Error('сбой'))
    await ask()
    expect(store.toast?.kind).toBe('error')
    expect(compare.open).toBe(false)
    select()
    store.toast = null
    expect(await compare.similarByKey()).toBe(false)
    expect(toast()).toBe(WHY_TEXT.none(0))
    expect(m.similar).toHaveBeenCalledOnce()
  })

  it('устаревший ответ (окно открыли, пока ждали) отбрасывается', async () => {
    let done: (v: unknown) => void = () => {}
    m.similar.mockReturnValue(new Promise((r) => (done = r)))
    const p = ask()
    select('a', 'b')
    compare.openByKey()
    done({ ids: ['c', 'd'], more: 0, why: '' })
    expect(await p).toBe(false)
    expect(compare.ids).toEqual(['a', 'b'])
  })
})

describe('«Лучший»', () => {
  it('остальные кадры окна уходят в PickBest через restForBest; выделен лучший; прокрутка к нему', async () => {
    select('a', 'b', 'c')
    compare.openByKey()
    compare.pick(1)
    expect(await compare.best()).toBe(true)
    expect(m.pick).toHaveBeenCalledExactlyOnceWith('b', ['a', 'c'])
    expect([...store.selected]).toEqual(['b'])
    expect(store.anchor).toBe('b')
    expect(photoui.reveal?.id).toBe('b')
    expect(compare.open).toBe(false)
    expect(toast()).toMatch(/^Лучший — b\.jpg; отклонено: 2/)
  })

  it('restForBest — единственное место решения «кого отклонять»', () => {
    expect(restForBest(['x', 'y'])).toEqual(['x', 'y'])
  })

  it('повторное нажатие, пока вызов в пути, не плодит вызовов', async () => {
    select('a', 'b')
    compare.openByKey()
    const first = compare.best()
    expect(await compare.best()).toBe(false)
    await first
    expect(m.pick).toHaveBeenCalledOnce()
  })
})

describe('закрытие и убирание', () => {
  it('закрытие: выделены оставшиеся видимые, якорь — активный, лента прокручена к нему', () => {
    select('a', 'b', 'c')
    compare.openByKey()
    compare.pick(2)
    compare.remove(0)
    compare.close()
    expect([...store.selected]).toEqual(['b', 'c'])
    expect(store.anchor).toBe('c')
    expect(photoui.reveal?.id).toBe('c')
  })

  it('остался один кадр — окно закрыто, выделен он', () => {
    select('a', 'b')
    compare.openByKey()
    compare.remove(0)
    expect(compare.open).toBe(false)
    expect([...store.selected]).toEqual(['b'])
  })

  it('кадр ушёл из альбома — ячейка пропала; ушли все, кроме одного, — окно закрыто', () => {
    select('a', 'b', 'c')
    compare.openByKey()
    store.photos = store.photos.filter((p) => p.id !== 'b') as never
    compare.prune()
    expect(compare.ids).toEqual(['a', 'c'])
    store.photos = store.photos.filter((p) => p.id !== 'a') as never
    compare.prune()
    expect(compare.open).toBe(false)
    expect([...store.selected]).toEqual(['c'])
  })

  it('Tab по кругу', () => {
    select('a', 'b', 'c')
    compare.openByKey()
    compare.cycle(-1)
    expect(compare.activeId).toBe('c')
    compare.cycle(1)
    expect(compare.activeId).toBe('a')
  })
})

describe('листание ← →', () => {
  it('меняет кадр в ячейке, зум остаётся при тех же пропорциях и снимается при других', () => {
    select('a', 'b')
    compare.openByKey()
    compare.loaded('a', { w: 2000, h: 1000 })
    compare.loaded('b', { w: 2000, h: 1000 })
    compare.loupe.sync = { z: 3, cx: 0.4, cy: 0.5 }
    compare.step(1)
    expect(compare.ids).toEqual(['c', 'b'])
    compare.loaded('c', { w: 1000, h: 500 })
    expect(compare.loupe.sync.z).toBe(3)
    compare.step(1)
    compare.loaded('d', { w: 1000, h: 1000 })
    expect(compare.loupe.sync.z).toBe(0)
  })
})
