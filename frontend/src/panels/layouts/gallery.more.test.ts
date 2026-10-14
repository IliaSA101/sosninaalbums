import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go подменён: MoreLayouts отвечает по очереди, вызовы записываются.
const calls: { id: string; have: number }[] = []
let answer: (id: string, have: number) => Promise<unknown> = async () => ({})

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  MoreLayouts: (id: string, have: number) => {
    calls.push({ id, have })
    return answer(id, have)
  },
  LayoutChoices: async () => ({}),
  Marks: async () => ({}),
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { gallery } = await import('./gallery.svelte')

const row = (n: number, more: boolean) => ({
  inOrder: Array.from({ length: n }, (_, i) => ({ key: `k${i}`, rects: [] })),
  rearranged: [],
  block: 'many',
  more,
})
const choices = (spread: string, rev: number, n: number, more: boolean) => ({ spread, rev, whole: row(n, more), left: null, right: null })
const tick = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  calls.length = 0
  store.toast = null
  store.apply({ rev: 5, marksRev: 0, folders: [], spreads: [{ id: 's', photos: [] }] } as never)
  gallery.cur = { spread: 's', scope: 'spread', tab: 'layouts' }
  gallery.choices = choices('s', 5, 12, true) as never
  gallery.moreBusy = false
})

describe('gallery.loadMore («Ещё»)', () => {
  it('просит Go досчитать после уже показанных и подменяет ряд ответом', async () => {
    answer = async () => choices('s', 5, 24, true)
    await gallery.loadMore()
    expect(calls).toEqual([{ id: 's', have: 12 }])
    expect(gallery.choices?.whole.inOrder).toHaveLength(24)
    expect(gallery.moreBusy).toBe(false)
  })

  it('пока ответа нет, второй щелчок не шлёт второй запрос', async () => {
    let done!: (v: unknown) => void
    answer = () => new Promise((r) => (done = r))
    const first = gallery.loadMore()
    expect(gallery.moreBusy).toBe(true)
    await gallery.loadMore()
    expect(calls).toHaveLength(1)
    done(choices('s', 5, 24, false))
    await first
    expect(gallery.choices?.whole.more).toBe(false)
  })

  it('ответ на устаревший rev выбрасывается', async () => {
    answer = async () => choices('s', 4, 24, true)
    await gallery.loadMore()
    expect(gallery.choices?.whole.inOrder).toHaveLength(12)
  })

  it('галерея закрыта или ушла на другой разворот — ответ не применяется', async () => {
    let done!: (v: unknown) => void
    answer = () => new Promise((r) => (done = r))
    const p = gallery.loadMore()
    gallery.cur = null
    done(choices('s', 5, 24, true))
    await p
    expect(gallery.choices?.whole.inOrder).toHaveLength(12)
  })

  it('ошибка Go — тост, плитка снова доступна', async () => {
    answer = async () => {
      throw 'нет такого разворота'
    }
    await gallery.loadMore()
    await tick()
    expect(store.toast?.text).toBe('Нет такого разворота')
    expect(gallery.moreBusy).toBe(false)
  })
})

describe('gallery: фильтр семейств и «Похожие» (041)', () => {
  it('по умолчанию «Все», похожие свёрнуты', () => {
    gallery.close()
    expect(gallery.family).toBe('all')
    expect(gallery.openTwins).toEqual([])
  })

  it('«Похожие» раскрываются и сворачиваются по ряду, фильтр запоминается', () => {
    gallery.setFamily('air')
    gallery.toggleTwins('В порядке ленты')
    gallery.toggleTwins('Другие варианты')
    expect(gallery.family).toBe('air')
    expect(gallery.openTwins).toEqual(['В порядке ленты', 'Другие варианты'])
    gallery.toggleTwins('В порядке ленты')
    expect(gallery.openTwins).toEqual(['Другие варианты'])
  })

  it('закрытие галереи сбрасывает фильтр и раскрытые ряды', () => {
    gallery.setFamily('hero')
    gallery.toggleTwins('В порядке ленты')
    gallery.close()
    expect(gallery.family).toBe('all')
    expect(gallery.openTwins).toEqual([])
  })
})
