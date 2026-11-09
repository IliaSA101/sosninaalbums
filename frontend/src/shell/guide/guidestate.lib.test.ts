// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'

// F1 при пустой ленте (124 Q1): «фото нет вообще» и «скрыты поиском или фильтром» различаются по ctx().libAll.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)]))
})

const { store } = await import('../../base/core/store.svelte')
const { noHooks } = await import('../../base/core/hooks')
const { guide } = await import('./guidestate.svelte')
const { articleById } = await import('../guidetext/articles')
const { planShow } = await import('./show')

afterEach(() => {
  store.photos = []
  store.plug(noHooks)
})

const photos = (n: number) => Array.from({ length: n }, (_, i) => ({ id: `p${i}`, name: `IMG_${i}.jpg`, w: 10, h: 10 })) as never

describe('ctx(): фото в ленте и в библиотеке', () => {
  it('поиск скрыл все: libPhotos 0, libAll — сколько фото в библиотеке, подсказка про сброс', () => {
    store.photos = photos(5)
    store.plug({ ...noHooks, named: () => [] })
    const c = guide.ctx()
    expect(c.libPhotos).toBe(0)
    expect(c.libAll).toBe(5)
    const p = planShow(articleById('marks')!, c)
    expect(p.hint).toBe('Лента пуста из-за поиска или фильтра: сбросьте их')
  })

  it('фото нет вообще: libAll 0 — «добавьте папку»', () => {
    const c = guide.ctx()
    expect([c.libPhotos, c.libAll]).toEqual([0, 0])
    expect(planShow(articleById('marks')!, c).hint).toMatch(/добавьте папку/)
  })
})
