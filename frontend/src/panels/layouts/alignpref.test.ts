// Опора «Относительно» по числу выделенных (спека 100 п. 4): выбор, запоминание каждой, старый ключ.
import { beforeEach, describe, expect, it, vi } from 'vitest'

const sel = vi.hoisted(() => ({ targetIds: [] as string[] }))
const m = vi.hoisted(() => ({
  Api: {} as Record<string, unknown>,
  store: {} as Record<string, unknown>,
  gridAct: {} as Record<string, unknown>,
  edit: {} as Record<string, unknown>,
}))
vi.mock('../../base/core/store.svelte', () => ({ Api: m.Api, store: m.store }))
vi.mock('./gridact.svelte', () => ({ gridAct: m.gridAct }))
vi.mock('../../editor/text/edit.svelte', () => ({ edit: m.edit }))
vi.mock('../../editor/places/placesel.svelte', () => ({ placesel: sel }))
vi.mock('../../base/prefs/prefs.svelte', () => ({ prefs: { can: () => true } }))

let mem: Map<string, string>
function useStorage(init: Record<string, string> = {}) {
  mem = new Map(Object.entries(init))
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => mem.get(k) ?? null,
    setItem: (k: string, v: string) => void mem.set(k, v),
  })
}

// Опора читает хранилище при создании, поэтому модуль грузится заново.
async function fresh(init: Record<string, string> = {}) {
  useStorage(init)
  vi.resetModules()
  return (await import('./align.svelte')).align
}
const pick = (n: number) => void (sel.targetIds = Array.from({ length: n }, (_, i) => 'p' + i))

describe('align.to по числу выделенных', () => {
  beforeEach(() => pick(0))

  it('по умолчанию: у одного «Страницы», у нескольких «Выделения»', async () => {
    const a = await fresh()
    pick(1)
    expect(a.to).toBe('page')
    pick(2)
    expect(a.to).toBe('sel')
    pick(5)
    expect(a.to).toBe('sel')
  })

  it('выбор при одном не трогает опору нескольких, и наоборот; обе запоминаются', async () => {
    const a = await fresh()
    pick(1)
    a.setTo('spread')
    expect(a.to).toBe('spread')
    pick(3)
    expect(a.to).toBe('sel')
    a.setTo('page')
    pick(1)
    expect(a.to).toBe('spread')
    pick(2)
    expect(a.to).toBe('page')
    expect(mem.get('alignToOne')).toBe('spread')
    expect(mem.get('alignTo')).toBe('page')
    // Новая сессия читает обе.
    const b = await fresh(Object.fromEntries(mem))
    pick(1)
    expect(b.to).toBe('spread')
    pick(2)
    expect(b.to).toBe('page')
  })

  it('переход 1 ↔ много: опоры возвращаются как были', async () => {
    const a = await fresh()
    pick(1)
    a.setTo('spread')
    for (let i = 0; i < 3; i++) {
      pick(2)
      expect(a.to).toBe('sel')
      pick(1)
      expect(a.to).toBe('spread')
    }
  })

  it('«Выделения» при одном — отказ: опора и хранилище не меняются', async () => {
    const a = await fresh()
    pick(1)
    a.setTo('sel')
    expect(a.to).toBe('page')
    expect(mem.has('alignToOne')).toBe(false)
    expect(mem.has('alignTo')).toBe(false)
  })

  it('повторный выбор той же опоры — без изменений', async () => {
    const a = await fresh()
    pick(2)
    a.setTo('spread')
    a.setTo('spread')
    expect(a.to).toBe('spread')
    expect(mem.get('alignTo')).toBe('spread')
    expect(mem.has('alignToOne')).toBe(false)
    a.setTo('sel')
    a.setTo('sel')
    expect(a.to).toBe('sel')
    expect(mem.get('alignTo')).toBe('sel')
  })

  it('run: опора берётся вместе с ids до ожидания (выделение меняется за await)', async () => {
    const a = await fresh()
    pick(1)
    const align = vi.fn()
    m.store.spread = { id: 's1' }
    m.edit.idle = async () => pick(2)
    m.gridAct.placeEdit = async () => true
    m.Api.AlignItems = align
    await a.run('left')
    expect(align).toHaveBeenCalledWith('s1', ['p0'], 'left', 'page')
  })

  it('старый ключ alignTo — опора нескольких, у одного по умолчанию «Страницы»', async () => {
    const a = await fresh({ alignTo: 'spread' })
    pick(3)
    expect(a.to).toBe('spread')
    pick(1)
    expect(a.to).toBe('page')
  })

  it('старый alignTo = sel и чужие значения — как прежде', async () => {
    const a = await fresh({ alignTo: 'sel', alignToOne: 'sel' })
    pick(2)
    expect(a.to).toBe('sel')
    pick(1)
    expect(a.to).toBe('page')
    const c = await fresh({ alignTo: 'чужое', alignToOne: 'чужое' })
    pick(2)
    expect(c.to).toBe('sel')
    pick(1)
    expect(c.to).toBe('page')
  })

  it('хранилище недоступно: опоры по умолчанию, выбор живёт в сессии', async () => {
    vi.stubGlobal('localStorage', {
      getItem: () => {
        throw new Error('нет')
      },
      setItem: () => {
        throw new Error('нет')
      },
    })
    vi.resetModules()
    const a = (await import('./align.svelte')).align
    pick(1)
    expect(a.to).toBe('page')
    a.setTo('spread')
    expect(a.to).toBe('spread')
  })

  it('ничего не выделено — опора одного, выбор пишется в неё', async () => {
    const a = await fresh()
    pick(0)
    expect(a.to).toBe('page')
    a.setTo('spread')
    pick(1)
    expect(a.to).toBe('spread')
  })
})
