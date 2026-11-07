import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Marks: async () => ({}), GetView: async () => ({}) }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { ownMarks } = await import('../marks/ownmarks.svelte')
const { binsel } = await import('./binsel.svelte')

const BINS = [
  { id: 'b1', name: 'Обязательно' },
  { id: 'b2', name: 'Запас' },
]
const view = (bins: { id: string; name: string }[]) => ({ rev: 1, marksRev: 1, folders: [], spreads: [], bins })

beforeEach(() => {
  prefs.level = 'standard'
  binsel.reset()
  ownMarks.map = {}
  store.apply(view(BINS) as never)
})

describe('показанная и целевая', () => {
  it('по умолчанию — все фото, целевой нет', () => {
    expect(binsel.shown).toBe('')
    expect(binsel.active).toBe(false)
    expect(binsel.targetId()).toBe('')
  })

  it('показанная — целевая по умолчанию; при «Все фото» — последняя, куда добавляли', () => {
    binsel.target = 'b2'
    expect(binsel.targetId()).toBe('b2')
    binsel.show('b1')
    expect(binsel.shown).toBe('b1')
    expect(binsel.targetId()).toBe('b1')
    binsel.showAll()
    expect(binsel.targetId()).toBe('b2')
  })

  it('повторный показ той же подборки ничего не меняет', () => {
    binsel.show('b1')
    binsel.show('b1')
    expect(binsel.shown).toBe('b1')
  })

  it('имя показанной; нет такой — пусто', () => {
    binsel.show('b2')
    expect(binsel.curName(BINS)).toBe('Запас')
    expect(binsel.curName([])).toBe('')
  })
})

describe('prune и reset', () => {
  it('удалили показанную — «Все фото»; целевую — забыта', () => {
    binsel.show('b1')
    binsel.target = 'b1'
    binsel.prune([BINS[1]])
    expect(binsel.shown).toBe('')
    expect(binsel.targetId()).toBe('')
  })

  it('живые подборки не трогаются', () => {
    binsel.show('b1')
    binsel.target = 'b2'
    binsel.prune(BINS)
    expect([binsel.shown, binsel.target]).toEqual(['b1', 'b2'])
  })

  it('цель пропала, показанная осталась', () => {
    binsel.show('b1')
    binsel.target = 'b2'
    binsel.prune([BINS[0]])
    expect([binsel.shown, binsel.target]).toEqual(['b1', ''])
  })

  it('«Новый» / «Открыть» (reset) — все фото, цели нет', () => {
    binsel.show('b1')
    binsel.target = 'b2'
    binsel.reset()
    expect([binsel.cur, binsel.target]).toEqual(['', ''])
  })
})

describe('уровень', () => {
  it('на «Простом» подборка не показана, но выбор помнится', () => {
    binsel.show('b1')
    prefs.level = 'simple'
    expect(binsel.shown).toBe('')
    expect(binsel.active).toBe(false)
    expect(binsel.namesOf(['b1'])).toEqual([])
    prefs.level = 'standard'
    expect(binsel.shown).toBe('b1')
  })
})

describe('членство и имена', () => {
  it('индекс строится от карты своих отметок', () => {
    ownMarks.map = { a: { bins: ['b1'] }, b: { bins: ['b1', 'b2'] } }
    expect([...(binsel.index.get('b1') ?? [])]).toEqual(['a', 'b'])
    ownMarks.map = { a: { bins: ['b2'] } }
    expect(binsel.index.has('b1')).toBe(false)
  })

  it('имена фото по членству; неизвестный ID пропускается', () => {
    expect(binsel.namesOf(['b2', 'b1', 'gone'])).toEqual(['Запас', 'Обязательно'])
    expect(binsel.namesOf(undefined)).toEqual([])
    expect(binsel.namesOf([])).toEqual([])
  })
})

describe('окно и брошенные фото (083 U2)', () => {
  it('close() закрывает окно и забывает брошенные фото, не трогая показанную и целевую', () => {
    binsel.show('b1')
    binsel.target = 'b2'
    binsel.open = true
    binsel.pending = ['a']
    binsel.wantNew = true
    binsel.close()
    expect([binsel.open, binsel.pending, binsel.wantNew]).toEqual([false, [], false])
    expect([binsel.cur, binsel.target]).toEqual(['b1', 'b2'])
  })

  it('reset («Новый»/«Открыть») закрывает окно и забывает брошенные фото', () => {
    binsel.open = true
    binsel.pending = ['a']
    binsel.wantNew = true
    binsel.reset()
    expect([binsel.open, binsel.pending, binsel.wantNew]).toEqual([false, [], false])
  })
})
