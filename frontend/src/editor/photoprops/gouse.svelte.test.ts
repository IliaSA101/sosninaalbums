import { beforeEach, describe, expect, it, vi } from 'vitest'

// Переход к вхождению фото (106): лист открыт, объект выделен или включён режим фона; фон альбома — «Настройки альбома».
const m = vi.hoisted(() => ({
  select: vi.fn(),
  pickSticker: vi.fn(),
  enter: vi.fn(),
  show: vi.fn(),
  can: vi.fn(() => true),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../places/placesel.svelte', () => ({ placesel: { select: m.select, pickSticker: m.pickSticker } }))
vi.mock('../backdrop/bgmode.svelte', () => ({ bgmode: { enter: m.enter } }))
vi.mock('../../shell/guide/guidestate.svelte', () => ({ guide: { show: m.show } }))
vi.mock('../../base/prefs/prefs.svelte', () => ({ prefs: { can: m.can } }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { goUse } = await import('./gouse.svelte')
type Line = Parameters<typeof goUse>[0]

const sp = (id: string) => ({ id, photos: [], grid: false })
const line = (kind: Line['kind'], spread: string, pick = '', page = 'Разворот', what = ''): Line => ({
  kind,
  spread,
  sheet: 0,
  page,
  what,
  pick,
})

beforeEach(() => {
  vi.clearAllMocks()
  m.can.mockReturnValue(true)
  store.view = { ...store.view, rev: 1, spreads: [sp('s1'), sp('s2'), sp('s3')], cover: undefined } as never
  store.go(0)
})

describe('goUse', () => {
  it('место и рамочка: лист открыт, выделена запись (pick)', () => {
    goUse(line('place', 's3', 'f1-2', 'Разворот 3', 'место'))
    expect(store.current).toBe(2)
    expect(m.select).toHaveBeenCalledWith(['f1-2'])
    goUse(line('border', 's2', 'pl1', 'Разворот 2', 'рамочка'))
    expect(store.current).toBe(1)
    expect(m.select).toHaveBeenLastCalledWith(['pl1'])
  })

  it('стикер: лист открыт, выделен один стикер', () => {
    goUse(line('sticker', 's2', 'st1'))
    expect(store.current).toBe(1)
    expect(m.pickSticker).toHaveBeenCalledWith('st1', false)
    expect(m.select).not.toHaveBeenCalled()
  })

  it('фон: лист открыт, режим фона нужной области', () => {
    goUse(line('backdrop', 's2', 'left'))
    expect(store.current).toBe(1)
    expect(m.enter).toHaveBeenCalledWith('s2', 'left')
  })

  it('фон без функции «Фото на фоне» — только лист', () => {
    m.can.mockReturnValue(false)
    goUse(line('backdrop', 's2', 'spread'))
    expect(m.can).toHaveBeenCalledWith('bgPhoto')
    expect(store.current).toBe(1)
    expect(m.enter).not.toHaveBeenCalled()
  })

  it('фон альбома: справочник «Настройки альбома», место album-bg; лист не меняется', () => {
    goUse({ kind: 'albumBg', spread: '', sheet: -1, page: 'Фон альбома', what: '', pick: '' })
    expect(m.show).toHaveBeenCalledWith('settings', 'ui:album-bg')
    expect(store.current).toBe(0)
  })

  it('листа нет к щелчку — ничего', () => {
    goUse(line('place', 'gone', 'f1'))
    expect(store.current).toBe(0)
    expect(m.select).not.toHaveBeenCalled()
    expect(m.enter).not.toHaveBeenCalled()
  })

  it('объекта нет (pick пуст) — лист открыт, выделения нет', () => {
    goUse(line('sticker', 's2', ''))
    expect(store.current).toBe(1)
    expect(m.pickSticker).not.toHaveBeenCalled()
    expect(m.select).not.toHaveBeenCalled()
  })

  it('обложка: открывается по COVER_ID', () => {
    store.view = { ...store.view, cover: { sheet: sp('cover') } } as never
    goUse(line('place', 'cover', 'f1', 'Обложка', 'место'))
    expect(store.onCover).toBe(true)
    expect(m.select).toHaveBeenCalledWith(['f1'])
  })
})
