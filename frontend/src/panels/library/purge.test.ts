import { beforeEach, describe, expect, it, vi } from 'vitest'

// «Убрать из альбома» (075-13): один вызов PurgePhotos, не RemovePhotos; тост — только после успеха.
const calls: string[][] = []
let fail = false
const view = (rev: number) => ({ rev, folders: [], spreads: [] })

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  PurgePhotos: async (ids: string[]) => {
    calls.push(ids)
    if (fail) throw 'не удалось убрать фото'
    return view(2)
  },
  RemovePhotos: async () => {
    throw new Error('RemovePhotos не должен вызываться')
  },
}))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { purgeMissing } = await import('./purge')

beforeEach(() => {
  calls.length = 0
  fail = false
  store.toast = null
  store.apply(view(1) as never)
})

describe('purgeMissing', () => {
  it('фото в месте и на фоне дано один раз: одним вызовом, тост с числом', async () => {
    expect(await purgeMissing(['a', 'b', 'a'])).toBe(true)
    expect(calls).toEqual([['a', 'b']])
    expect(store.toast).toMatchObject({ kind: 'info', text: 'Убрано фото: 2' })
    expect(store.view.rev).toBe(2)
  })

  it('пустой список — Go не зовётся, тоста нет', async () => {
    expect(await purgeMissing([])).toBe(false)
    expect(calls).toEqual([])
    expect(store.toast).toBeNull()
  })

  it('ошибка Go — красный тост, «Убрано» не показываем, вид прежний', async () => {
    fail = true
    expect(await purgeMissing(['a'])).toBe(false)
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Не удалось убрать фото' })
    expect(store.view.rev).toBe(1)
  })
})
