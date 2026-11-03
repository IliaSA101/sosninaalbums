import { beforeEach, describe, expect, it, vi } from 'vitest'

// Подменён только Go (App.FitFaces) и шина событий: facesapi и vigFaces — настоящие. Store и выделение — заглушки.
const go = vi.hoisted(() => ({
  fit: vi.fn(),
  handlers: {} as Record<string, (...a: number[]) => void>,
  off: vi.fn(),
  rev: 5,
}))
vi.mock('../../../wailsjs/go/app/App', () => ({ FitFaces: go.fit }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, fn: (...a: number[]) => void) => {
    go.handlers[name] = fn
    return go.off
  },
}))
vi.mock('../../base/core/store.svelte', () => ({
  store: {
    view: {
      get rev() {
        return go.rev
      },
      spreads: [{ id: 'a', grid: true }],
    },
    get currentId() {
      return 'a'
    },
    take: vi.fn(async () => true),
    notify: vi.fn(),
    goId: vi.fn(),
  },
}))
vi.mock('../../editor/places/placesel.svelte', () => ({ placesel: { select: vi.fn() } }))

import { store } from '../../base/core/store.svelte'
import { gridAct } from '../layouts/gridact.svelte'
import { fitFaces, onFacesProgress } from './facesapi'
import { vigFaces } from './vigfaces.svelte'

const report = (o = {}) => ({
  fitted: 0,
  noFace: 0,
  many: 0,
  missing: 0,
  small: 0,
  big: 0,
  edge: 0,
  contain: 0,
  manual: 0,
  changed: 0,
  check: [],
  short: [],
  asked: false,
  ...o,
})
const ok = (o = {}, rev = 6) => ({ view: { rev }, report: report(o) })

beforeEach(() => {
  vi.clearAllMocks()
  gridAct.cancelAuto()
  go.rev = 5
  go.handlers = {}
})

describe('facesapi: вызов и событие', () => {
  it('fitFaces передаёт лист, образец и режим в App.FitFaces как есть и отдаёт ответ', async () => {
    const r = ok({ fitted: 3 })
    go.fit.mockResolvedValue(r)
    expect(await fitFaces('s1', 'p7', 'keep')).toBe(r)
    expect(go.fit).toHaveBeenCalledWith('s1', 'p7', 'keep')
  })

  it('faces:progress — позиционные (done, total); возвращается отписка Wails', () => {
    const got: [number, number][] = []
    const off = onFacesProgress((d, t) => got.push([d, t]))
    go.handlers['faces:progress'](3, 25)
    expect(got).toEqual([[3, 25]])
    expect(off).toBe(go.off)
  })
})

describe('vigFaces на настоящем facesapi', () => {
  it('успех: ход 0/25 → 25/25 до ответа, затем вид принят и итог показан', async () => {
    const seen: ({ done: number; total: number } | null)[] = []
    go.fit.mockImplementation(async () => {
      go.handlers['faces:progress'](0, 25)
      seen.push(vigFaces.progress('a'))
      go.handlers['faces:progress'](25, 25)
      seen.push(vigFaces.progress('a'))
      return ok({ fitted: 20, noFace: 5, check: ['p1'] })
    })
    expect(await vigFaces.fit('a')).toBe(true)
    expect(go.fit).toHaveBeenCalledWith('a', '', '')
    expect(seen).toEqual([
      { done: 0, total: 25 },
      { done: 25, total: 25 },
    ])
    expect(store.take).toHaveBeenCalledWith({ rev: 6 })
    expect(vi.mocked(store.notify).mock.calls[0][0]).toBe('Подогнано 20 из 25 · Проверьте 5 (без лица)')
    expect(go.off).toHaveBeenCalledTimes(1)
    expect(vigFaces.busy).toBe(false)
  })

  it('asked: правки нет, окно 098; «Подогнать» повторяет вызов с тем же листом и образцом и режимом', async () => {
    go.fit.mockResolvedValueOnce(ok({ asked: true, manual: 4 }, 5))
    await vigFaces.fit('a', '', 'p7')
    expect(go.fit).toHaveBeenLastCalledWith('a', 'p7', '')
    expect(store.take).not.toHaveBeenCalled()
    expect(gridAct.autoAsk?.crops).toBe(4)

    go.fit.mockResolvedValueOnce(ok({ fitted: 16, manual: 4 }))
    await gridAct.confirmAuto()
    expect(go.fit).toHaveBeenLastCalledWith('a', 'p7', 'keep')
    expect(go.fit).toHaveBeenCalledTimes(2)

    go.fit.mockResolvedValueOnce(ok({ asked: true, manual: 4 }, 6))
    go.rev = 6
    await vigFaces.fit('a', '', 'p7')
    gridAct.setKeep(false)
    go.fit.mockResolvedValueOnce(ok({ fitted: 20 }, 7))
    await gridAct.confirmAuto()
    expect(go.fit).toHaveBeenLastCalledWith('a', 'p7', 'all')
  })

  it.each([
    'Уже ищу лица',
    'Идёт экспорт: подгоните по лицам после него',
    'Нет такого разворота',
    'Альбом сменился во время поиска лиц',
    'Выбранное фото не найдено на листе',
  ])('ошибка Go строкой «%s» — красный тост как у команд, вид не принят, кнопка свободна', async (text) => {
    go.fit.mockRejectedValue(text)
    expect(await vigFaces.fit('a')).toBe(false)
    expect(store.notify).toHaveBeenCalledWith(text, 'error')
    expect(store.take).not.toHaveBeenCalled()
    expect(vigFaces.busy).toBe(false)
    expect(go.off).toHaveBeenCalledTimes(1)
  })

  it('лист без фото: событий хода нет, ответ нулевой — «На листе нет фото для подгона»', async () => {
    go.fit.mockResolvedValue(ok({}, 5))
    expect(await vigFaces.fit('a')).toBe(true)
    expect(Object.keys(go.handlers)).toEqual(['faces:progress'])
    expect(vigFaces.progress('a')).toBeNull()
    expect(vi.mocked(store.notify).mock.calls[0][0]).toBe('На листе нет фото для подгона')
  })

  it('чек-лист и короткий список приходят пустыми массивами — «Показать» не рисуется', async () => {
    go.fit.mockResolvedValue(ok({ fitted: 10 }))
    await vigFaces.fit('a')
    expect(vi.mocked(store.notify).mock.calls[0][2]).toBeUndefined()
  })
})
