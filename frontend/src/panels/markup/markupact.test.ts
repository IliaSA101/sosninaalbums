import { beforeEach, describe, expect, it, vi } from 'vitest'

// Блок «Разметка типографии» (066): диалог файла — на стороне Go, здесь моки вызовов.
const calls: string[] = []
let next: () => unknown = () => view(1, '')
let fail = ''

const view = (rev: number, spread: string, cover = '', key = '') => ({
  rev,
  folders: [],
  settings: {},
  spreads: [],
  spreadMarkup: spread ? { name: spread, key: key || spread, ok: true, image: true } : null,
  cover: cover ? { sheet: { id: 'cover', photos: [] }, spec: {}, markup: { name: cover, key: cover, ok: true, image: true } } : undefined,
})

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
const go = (name: string) => async () => {
  calls.push(name)
  if (fail) throw fail
  return next()
}
vi.mock('../../../wailsjs/go/app/App', () => ({
  PickSpreadGuide: go('PickSpreadGuide'),
  ClearSpreadGuide: go('ClearSpreadGuide'),
  PickCoverGuide: go('PickCoverGuide'),
  ClearCoverGuide: go('ClearCoverGuide'),
}))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { markupAct } = await import('./markupact.svelte')
const { markupUi } = await import('../../editor/markup/markupui.svelte')

beforeEach(() => {
  calls.length = 0
  fail = ''
  store.toast = null
  store.apply(view(1, '') as never)
})

describe('markupAct.pick', () => {
  it('развороты: PickSpreadGuide, новый файл — номер адреса не трогаем (его сменит track по имени)', async () => {
    const before = markupUi.rev.spreads
    next = () => view(2, 'spread_30x30.psd')
    await markupAct.pick('spreads')
    expect(calls).toEqual(['PickSpreadGuide'])
    expect(store.view.spreadMarkup?.name).toBe('spread_30x30.psd')
    expect(markupUi.rev.spreads).toBe(before)
  })

  it('обложка: PickCoverGuide, развороты не задеты', async () => {
    const before = markupUi.rev.spreads
    next = () => view(2, '', 'Cover.psd')
    await markupAct.pick('cover')
    expect(calls).toEqual(['PickCoverGuide'])
    expect(store.cover?.markup?.name).toBe('Cover.psd')
    expect(markupUi.rev.spreads).toBe(before)
  })

  it('тот же файл выбрали заново — адрес картинки новый (файл могли заменить)', async () => {
    store.apply(view(5, 'spread_30x30.psd') as never)
    const before = markupUi.rev.spreads
    next = () => view(5, 'spread_30x30.psd') // Go правку не записал, rev тот же
    await markupAct.pick('spreads')
    expect(markupUi.rev.spreads).toBeGreaterThan(before)
  })

  it('одноимённый файл из другой папки (другой key) — номер не трогаем: сменит track', async () => {
    store.apply(view(5, 'spread_30x30.psd', '', 'dir1') as never)
    const before = markupUi.rev.spreads
    next = () => view(6, 'spread_30x30.psd', '', 'dir2')
    await markupAct.pick('spreads')
    expect(markupUi.rev.spreads).toBe(before)
  })

  it('диалог закрыт без файла и разметки нет — ничего не меняется', async () => {
    const before = markupUi.rev.spreads
    next = () => view(1, '')
    await markupAct.pick('spreads')
    expect(markupUi.rev.spreads).toBe(before)
    expect(store.toast).toBeNull()
  })

  it('отказ Go (PSB): тост с заглавной, вид и адрес прежние', async () => {
    store.apply(view(5, 'old.psd') as never)
    const before = markupUi.rev.spreads
    fail = 'файл PSB (большой документ Photoshop) не поддерживается — сохраните его как PSD'
    await markupAct.pick('spreads')
    expect(store.toast?.text).toBe('Файл PSB (большой документ Photoshop) не поддерживается — сохраните его как PSD')
    expect(store.toast?.kind).toBe('error')
    expect(store.view.spreadMarkup?.name).toBe('old.psd')
    expect(markupUi.rev.spreads).toBe(before)
  })
})

describe('markupAct.clear', () => {
  it('убирает разметку своего вида одним вызовом', async () => {
    store.apply(view(5, 'a.psd', 'c.psd') as never)
    next = () => view(6, '', 'c.psd')
    await markupAct.clear('spreads')
    expect(calls).toEqual(['ClearSpreadGuide'])
    expect(store.view.spreadMarkup).toBeNull()
    expect(store.cover?.markup?.name).toBe('c.psd')
    next = () => view(7, '')
    await markupAct.clear('cover')
    expect(calls).toEqual(['ClearSpreadGuide', 'ClearCoverGuide'])
    expect(store.cover).toBeFalsy()
  })
})
