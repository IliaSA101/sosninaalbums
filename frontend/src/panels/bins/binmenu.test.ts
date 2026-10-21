import { beforeEach, describe, expect, it, vi } from 'vitest'

// Состояние окна «Подборки»: строка имени (новая, переименование), ошибка под полем. Go подменён.
const calls: unknown[][] = []
let answer: () => Promise<unknown> = async () => ({})
const BINS = [
  { id: 'b1', name: 'Обязательно' },
  { id: 'b2', name: 'Запас' },
]
const view = (rev: number, bins = BINS) => ({ rev, marksRev: rev, folders: [], spreads: [], bins })

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Marks: async () => ({}), GetView: async () => ({}) }))
vi.mock('./binapi', () => ({
  NewBin: (...a: unknown[]) => (calls.push(['new', ...a]), answer()),
  RenameBin: (...a: unknown[]) => (calls.push(['rename', ...a]), answer()),
  DeleteBin: async () => ({}),
  SetBin: async () => ({}),
}))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { binsel } = await import('./binsel.svelte')
const { BinMenuState, NEW } = await import('./binstate.svelte')

let m: InstanceType<typeof BinMenuState>
beforeEach(() => {
  calls.length = 0
  binsel.reset()
  store.apply(view(1) as never)
  m = new BinMenuState()
})

describe('новая подборка', () => {
  it('Enter: создаётся вместе с выделенными, строка закрывается', async () => {
    answer = async () => view(2, [...BINS, { id: 'b3', name: 'Подборка 1' }])
    m.startEdit(NEW)
    await m.submitNew('Подборка 1', ['a', 'b'])
    expect(calls).toEqual([['new', 'Подборка 1', ['a', 'b']]])
    expect(m.editing).toBe('')
    expect(binsel.target).toBe('b3')
  })

  it('Esc: строка закрыта, Go не звали, ничего не создано', () => {
    m.startEdit(NEW)
    m.stop()
    expect(m.editing).toBe('')
    expect(calls).toEqual([])
  })

  it('имя занято — ошибка под полем, строка остаётся; новый ввод стирает ошибку', async () => {
    m.startEdit(NEW)
    await m.submitNew('запас', [])
    expect(m.err).toBe('Такая подборка уже есть')
    expect(m.editing).toBe(NEW)
    m.startEdit(NEW)
    expect(m.err).toBe('')
  })

  it('без выделения — подборка пустая (ids пуст)', async () => {
    answer = async () => view(2, [...BINS, { id: 'b3', name: 'Новая' }])
    await m.submitNew('Новая', [])
    expect(calls).toEqual([['new', 'Новая', []]])
  })
})

describe('переименование', () => {
  it('Enter с новым именем: вызов и закрытие поля', async () => {
    answer = async () => view(2)
    m.startEdit('b2')
    expect(m.editing).toBe('b2')
    await m.submitRename('b2', 'Резерв')
    expect(calls).toEqual([['rename', 'b2', 'Резерв']])
    expect(m.editing).toBe('')
  })

  it('Enter с пустым или прежним именем: поле закрыто, шага нет', async () => {
    m.startEdit('b2')
    await m.submitRename('b2', '')
    expect(m.editing).toBe('')
    m.startEdit('b2')
    await m.submitRename('b2', 'Запас')
    expect(m.editing).toBe('')
    expect(calls).toEqual([])
  })

  it('имя другой подборки: «Такая подборка уже есть», поле остаётся', async () => {
    m.startEdit('b2')
    await m.submitRename('b2', 'ОБЯЗАТЕЛЬНО')
    expect(m.err).toBe('Такая подборка уже есть')
    expect(m.editing).toBe('b2')
    expect(calls).toEqual([])
  })

  it('Esc: имя прежнее, ошибка стёрта', async () => {
    m.startEdit('b2')
    await m.submitRename('b2', 'обязательно')
    m.stop()
    expect([m.editing, m.err]).toEqual(['', ''])
  })

  it('ошибка Go (подборки больше нет) остаётся под полем', async () => {
    answer = async () => {
      throw new Error('подборки больше нет')
    }
    m.startEdit('b2')
    await m.submitRename('b2', 'Резерв')
    expect(m.err).toBe('подборки больше нет')
  })
})
