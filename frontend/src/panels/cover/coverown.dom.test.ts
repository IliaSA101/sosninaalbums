// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'

// «(своя)» у группы типографий в карточке обложки (123 Б3). Go подменён: пресеты двух групп.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    CoverPresets: vi.fn(async () => [
      { id: 'fab-1', name: 'Тонкие', group: 'Фабрика Фотокниги', spec: { preset: 'fab-1', wrap: 17 } },
      { id: 'own-1-1', name: 'Тонкие', group: 'Альфа', spec: { preset: 'own-1-1', wrap: 17 } },
    ]),
  }
})
const { default: InspectorCover } = await import('./InspectorCover.svelte')
const { store } = await import('../../base/core/store.svelte')
const { printList } = await import('../../shell/printprof/printlist.svelte')

afterEach(() => {
  cleanup()
  printList.infos = []
})

describe('карточка обложки: список «Типография»', () => {
  it('группа своей — с «(своя)»', async () => {
    printList.infos = [
      { id: 'fabrika', name: 'Фабрика Фотокниги', source: 'builtin', version: 1, hasExport: true, formats: [] },
      { id: 'own-1', name: 'Альфа', source: 'own', version: 1, hasExport: true, formats: [] },
    ]
    store.view = {
      rev: 1,
      folders: [],
      spreads: [],
      settings: { pageW: 304, pageH: 203, dpi: 300, margin: 10, gap: 3, safe: 5, bleed: 3, bg: '' },
      cover: {
        preset: 'fab-1',
        spec: { preset: 'fab-1', wrap: 17 },
        sheet: { id: 'cover', w: 600, h: 300, rects: [] },
        zones: [],
        drop: [],
        scopes: [],
        safe: [],
        spine: 12,
        n: 10,
        layout: 'both',
        missing: '',
        fix: '',
      },
    } as never
    render(InspectorCover)
    await new Promise((r) => setTimeout(r, 0))
    const labels = Array.from(document.querySelectorAll('optgroup')).map((g) => g.label)
    expect(labels).toEqual(['Фабрика Фотокниги', 'Альфа (своя)'])
  })
})
