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

describe('gallery: ряд «Вариантов» и источник «Шаблонов» (050)', () => {
  const sp = (rearranged: boolean) =>
    ({
      id: 's',
      photos: [],
      layout: 'k',
      mid: 200,
      rearranged,
      pages: [{ photos: 2, variant: 0, variants: 2, rearranged: true }, null],
    }) as never
  const withSpread = (rearranged: boolean) => store.apply({ rev: 6, marksRev: 0, folders: [], spreads: [sp(rearranged)] } as never)

  it('open: ряд — где текущая раскладка', () => {
    withSpread(true)
    gallery.close()
    gallery.open('spread')
    expect(gallery.order).toBe('rearranged')
    gallery.close()
    gallery.open('left')
    expect(gallery.order).toBe('rearranged') // страница: свой флаг
    gallery.close()
    withSpread(false)
    gallery.open('spread')
    expect(gallery.order).toBe('inOrder')
    gallery.close()
  })

  it('выбор живёт, пока галерея открыта; закрытие сбрасывает ряд и источник', () => {
    withSpread(false)
    gallery.open('spread')
    gallery.setOrder('rearranged')
    gallery.setSource('mine')
    expect(gallery.order).toBe('rearranged')
    expect(gallery.source).toBe('mine')
    gallery.close()
    expect(gallery.order).toBe('inOrder')
    expect(gallery.source).toBe('all')
  })

  it('фильтр видов виньетки (049) живёт, пока галерея открыта; закрытие сбрасывает', () => {
    withSpread(false)
    gallery.open('spread')
    gallery.setVigFamily('big')
    expect(gallery.vigFamily).toBe('big')
    gallery.close()
    expect(gallery.vigFamily).toBe('all')
  })

  it('смена области «Шаблонов» источник не сбрасывает', () => {
    withSpread(false)
    gallery.close()
    gallery.open('spread', 'grids')
    gallery.setSource('builtin')
    gallery.preview = { spread: 's', rects: [] }
    gallery.rescope('left')
    expect(gallery.cur).toEqual({ spread: 's', scope: 'left', tab: 'grids' })
    expect(gallery.source).toBe('builtin')
    expect(gallery.preview).toBeNull()
    gallery.close()
  })
})
