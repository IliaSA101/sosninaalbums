import { beforeEach, describe, expect, it, vi } from 'vitest'

// Выбор типографии в карточке обложки: «Свои размеры» не сбрасывают числа (QA 032).
const calls: string[] = []
const specs: unknown[] = []
let spineId = ''

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  ApplyCoverPreset: async (id: string) => {
    calls.push(`ApplyCoverPreset:${id}`)
    return null
  },
  CoverSpineText: async () => {
    calls.push('CoverSpineText')
    spineId = 'tx1'
    return viewWith('custom')
  },
  SetCoverSpec: async (s: unknown) => {
    calls.push('SetCoverSpec')
    specs.push(s)
    return null
  },
}))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { coverAct } = await import('./coveract.svelte')
const { edit } = await import('../../editor/text/edit.svelte')

const viewWith = (preset: string) => ({
  rev: 1,
  folders: [],
  settings: {},
  spreads: [],
  cover: {
    sheet: { id: 'cover', photos: [] },
    preset,
    spine: 14,
    spineText: spineId,
    spec: { preset, wrap: 23.5, hinge: 6, steps: [{ upto: 5, mm: 30 }] },
  },
})
const withCover = (preset: string) => store.apply(viewWith(preset) as never)

beforeEach(() => {
  spineId = ''
  store.toast = null
  edit.sel = null
  calls.length = 0
  specs.length = 0
})

describe('coverAct.applyPreset', () => {
  it('«Свои размеры» из таблицы: формула a = корешок сейчас, b = 0, через SetCoverSpec', async () => {
    withCover('double')
    await coverAct.applyPreset('custom')
    expect(calls).toEqual(['SetCoverSpec'])
    expect(specs[0]).toEqual({ preset: 'custom', wrap: 23.5, hinge: 6, steps: [], a: 14, b: 0 })
  })

  it('уже «Свои размеры» — Go не зовётся', async () => {
    withCover('custom')
    await coverAct.applyPreset('custom')
    expect(calls).toEqual([])
  })

  it('«Исправить» (047): тот же пресет, что стоит, — ApplyCoverPreset(cv.preset)', async () => {
    withCover('fsfera-thin')
    await coverAct.applyPreset(store.cover!.preset)
    expect(calls).toEqual(['ApplyCoverPreset:fsfera-thin'])
  })

  it('обычный пресет — ApplyCoverPreset', async () => {
    withCover('custom')
    await coverAct.applyPreset('double')
    expect(calls).toEqual(['ApplyCoverPreset:double'])
  })
})

describe('coverAct.spineText (058 В)', () => {
  it('надписи нет: добавляет, выделяет её и говорит тостом', async () => {
    withCover('custom')
    await coverAct.spineText()
    expect(calls).toEqual(['CoverSpineText'])
    expect(edit.sel).toEqual({ kind: 'text', spread: 'cover', id: 'tx1' })
    expect(store.toast?.text).toBe('Надпись на корешке добавлена')
  })

  it('надпись есть: только выделяет, Go не зовётся, тоста нет', async () => {
    spineId = 'tx9'
    withCover('custom')
    await coverAct.spineText()
    expect(calls).toEqual([])
    expect(edit.sel).toEqual({ kind: 'text', spread: 'cover', id: 'tx9' })
    expect(store.toast).toBeNull()
  })
})
