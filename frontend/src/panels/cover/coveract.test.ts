import { beforeEach, describe, expect, it, vi } from 'vitest'

// Выбор типографии в карточке обложки: «Свои размеры» не сбрасывают числа (QA 032).
const calls: string[] = []
const specs: unknown[] = []

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  ApplyCoverPreset: async (id: string) => {
    calls.push(`ApplyCoverPreset:${id}`)
    return null
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

const withCover = (preset: string) =>
  store.apply({
    rev: 1,
    folders: [],
    settings: {},
    spreads: [],
    cover: { sheet: { id: 'cover', photos: [] }, preset, spine: 14, spec: { preset, wrap: 23.5, hinge: 6, steps: [{ upto: 5, mm: 30 }] } },
  } as never)

beforeEach(() => {
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

  it('обычный пресет — ApplyCoverPreset', async () => {
    withCover('custom')
    await coverAct.applyPreset('double')
    expect(calls).toEqual(['ApplyCoverPreset:double'])
  })
})
