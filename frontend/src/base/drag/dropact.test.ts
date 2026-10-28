import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Перенос разворота (096 сц. 5): одна правка MoveSpread, перенесённый — текущий; dropOn сообщает, прошла ли правка.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  MoveSpread: vi.fn(),
  AddPhotos: vi.fn(),
  MovePhoto: vi.fn(),
  NewSpread: vi.fn(),
  PlacePhotos: vi.fn(),
  DropPhotos: vi.fn(),
  CreateCoverLook: vi.fn(),
  ForgetFileNames: vi.fn(),
}))
vi.stubGlobal('window', globalThis)

const Api = await import('../../../wailsjs/go/app/App')
const { store } = await import('../core/store.svelte')
const { dropOn, dropHooks } = await import('./dropact')
const { dropLanded } = await import('../prefs/cardgroups.svelte')
const { noHooks } = await import('../core/hooks')

const MoveSpread = vi.mocked(Api.MoveSpread)
const gate = vi.fn()
const sp = (id: string) => ({ id, photos: [], grid: false })
const view = (rev: number, ids: string[]) => ({ rev, marksRev: 0, folders: [], spreads: ids.map(sp), settings: { sheet: false } }) as never

beforeEach(() => {
  vi.restoreAllMocks()
  MoveSpread.mockReset()
  gate.mockReset()
  gate.mockImplementation(async (ids: string[]) => ids)
  store.plug({ ...noHooks, gate })
  for (const f of [Api.AddPhotos, Api.MovePhoto, Api.NewSpread, Api.PlacePhotos, Api.DropPhotos]) vi.mocked(f).mockReset()
  store.apply(view(1, ['a', 'b', 'c', 'd']))
  store.go(0)
})

describe('dropOn: перенос разворота', () => {
  it('перенесённый — текущий', async () => {
    MoveSpread.mockResolvedValueOnce(view(2, ['b', 'c', 'a', 'd']))
    const go = vi.spyOn(store, 'go')
    const ok = await dropOn({ kind: 'spread', id: 'a', index: 0 }, { kind: 'gap', at: 3, to: 2 })
    expect(ok).toBe(true)
    expect(MoveSpread).toHaveBeenCalledOnce()
    expect(MoveSpread).toHaveBeenCalledWith('a', 2)
    expect(go).toHaveBeenCalledWith(2)
    expect(store.current).toBe(2)
    expect(store.currentId).toBe('a')
  })

  it('перенесённый — текущий, даже если до этого текущим был другой', async () => {
    store.go(3) // текущий — d
    MoveSpread.mockResolvedValueOnce(view(2, ['b', 'a', 'c', 'd']))
    expect(await dropOn({ kind: 'spread', id: 'a', index: 0 }, { kind: 'gap', at: 2, to: 1 })).toBe(true)
    expect(store.currentId).toBe('a')
    expect(store.current).toBe(1)
  })

  it('ошибка — без перехода', async () => {
    store.go(3)
    MoveSpread.mockRejectedValueOnce(new Error('нельзя'))
    const go = vi.spyOn(store, 'go')
    const ok = await dropOn({ kind: 'spread', id: 'a', index: 0 }, { kind: 'gap', at: 3, to: 2 })
    expect(ok).toBe(false)
    expect(go).not.toHaveBeenCalled()
    expect(store.currentId).toBe('d')
    store.toast = null
  })

  it('своё место — ни правки, ни перехода', async () => {
    const go = vi.spyOn(store, 'go')
    expect(await dropOn({ kind: 'spread', id: 'b', index: 1 }, { kind: 'gap', at: 1, to: 1 })).toBe(false)
    expect(MoveSpread).not.toHaveBeenCalled()
    expect(go).not.toHaveBeenCalled()
  })
})

// 098: вопрос «Фото уже в альбоме» — только при броске из ленты; с холста и ленты разворотов (fromSpread) — без вопроса.
describe('dropOn: ворота копии (098)', () => {
  const AddPhotos = vi.mocked(Api.AddPhotos)
  const NewSpread = vi.mocked(Api.NewSpread)
  const PlacePhotos = vi.mocked(Api.PlacePhotos)
  const strip = (...ids: string[]) => ({ kind: 'photos', ids }) as never
  const canvas = (...ids: string[]) => ({ kind: 'photos', ids, fromSpread: 'b' }) as never

  it('из ленты на лист: ворота получают ID, Go — то, что они вернули (запрос копии)', async () => {
    gate.mockResolvedValue(['f1-new'])
    AddPhotos.mockResolvedValueOnce(view(2, ['a', 'b', 'c', 'd']))
    await dropOn(strip('f1'), { kind: 'spread', spread: 'a' })
    expect(gate).toHaveBeenCalledExactlyOnceWith(['f1'])
    expect(AddPhotos).toHaveBeenCalledExactlyOnceWith('a', ['f1-new'], -1)
  })

  it('«Отмена» (null) — ни одного вызова Go, выделение не снято', async () => {
    gate.mockResolvedValue(null)
    store.selected.add('f1')
    await dropOn(strip('f1'), { kind: 'spread', spread: 'a' })
    expect(AddPhotos).not.toHaveBeenCalled()
    expect(store.selected.has('f1')).toBe(true)
    store.selected.clear()
  })

  it('с холста и из ленты разворотов (fromSpread) — ворот нет, ID уходят как есть', async () => {
    AddPhotos.mockResolvedValueOnce(view(2, ['a', 'b', 'c', 'd']))
    await dropOn(canvas('f1-2'), { kind: 'spread', spread: 'a' })
    expect(gate).not.toHaveBeenCalled()
    expect(AddPhotos).toHaveBeenCalledExactlyOnceWith('a', ['f1-2'], -1)
  })

  it('на «+ Новый разворот» из ленты — ворота, NewSpread с ответом; отмена — разворота нет', async () => {
    gate.mockResolvedValueOnce(['f1-new', 'f2'])
    NewSpread.mockResolvedValueOnce(view(2, ['a', 'b', 'c', 'd', 'e']))
    await dropOn(strip('f1', 'f2'), { kind: 'new', at: 4 })
    expect(NewSpread).toHaveBeenCalledExactlyOnceWith(4, ['f1-new', 'f2'])
    NewSpread.mockClear()
    gate.mockResolvedValueOnce(null)
    await dropOn(strip('f1'), { kind: 'new', at: 4 })
    expect(NewSpread).not.toHaveBeenCalled()
  })

  it('на «+ Новый разворот» с холста (strip: false) — ворот нет', async () => {
    NewSpread.mockResolvedValueOnce(view(2, ['a', 'b', 'c', 'd', 'e']))
    await dropOn(canvas('f1-2'), { kind: 'new', at: 4 })
    expect(gate).not.toHaveBeenCalled()
    expect(NewSpread).toHaveBeenCalledExactlyOnceWith(4, ['f1-2'])
  })

  it('на место обложки из ленты — ворота; с холста — нет', async () => {
    store.apply({
      ...(view(3, ['a']) as object),
      cover: { sheet: { id: 'cover', photos: ['c1'], places: [{ id: 'pl', photo: 'c1' }], grid: false } },
    } as never)
    gate.mockResolvedValue(['f1-new'])
    PlacePhotos.mockResolvedValue({ view: view(4, ['a']), left: 0 } as never)
    await dropOn(strip('f1'), { kind: 'place', spread: 'cover', place: 'pl', photo: 'c1' })
    expect(PlacePhotos).toHaveBeenCalledExactlyOnceWith('cover', 'pl', ['f1-new'])
    PlacePhotos.mockClear()
    gate.mockClear()
    await dropOn(canvas('f2'), { kind: 'place', spread: 'cover', place: 'pl', photo: 'c1' })
    expect(gate).not.toHaveBeenCalled()
    expect(PlacePhotos).toHaveBeenCalledExactlyOnceWith('cover', 'pl', ['f2'])
  })

  it('на фон и на плитку «+ Обложка» — без вопроса (фото с места не уходит)', async () => {
    vi.mocked(Api.CreateCoverLook).mockResolvedValueOnce({ view: view(2, ['a', 'b', 'c', 'd']), styled: false, look: '' } as never)
    await dropOn(strip('f1'), { kind: 'cover' })
    await dropOn(strip('f1'), { kind: 'backdrop', spread: 'a', scope: 'spread' })
    expect(Api.CreateCoverLook).toHaveBeenCalledExactlyOnceWith('', 'f1')
    expect(gate).not.toHaveBeenCalled()
  })
})

// 114: перенос своего фото в щель своего разворота — одна правка MovePhoto; «Авто» или ручной, решает Go.
describe('dropOn: перестановка в ленте (114)', () => {
  const MovePhoto = vi.mocked(Api.MovePhoto)
  const chip = (spread: string) => ({ kind: 'chip', spread, index: 3, photo: 'x', side: 'before' }) as never
  const own = (spread: string) => ({ kind: 'photos', ids: ['f1'], fromSpread: spread }) as never
  const withGrid = (grid: boolean) =>
    ({ rev: 2, marksRev: 0, folders: [], spreads: [{ id: 'a', photos: ['f1'], grid }], settings: { sheet: false } }) as never

  for (const grid of [true, false]) {
    it(`move — одна правка MovePhoto (${grid ? 'ручной' : 'Авто'})`, async () => {
      store.apply({ ...(withGrid(grid) as object), rev: 1 } as never)
      store.go(0)
      MovePhoto.mockResolvedValueOnce(withGrid(grid))
      expect(await dropOn(own('a'), chip('a'))).toBe(true)
      expect(MovePhoto).toHaveBeenCalledExactlyOnceWith('f1', 'a', 3)
      expect(Api.DropPhotos).not.toHaveBeenCalled()
      expect(Api.AddPhotos).not.toHaveBeenCalled()
    })
  }

  it('чужое фото в щель — по-прежнему AddPhotos', async () => {
    vi.mocked(Api.AddPhotos).mockResolvedValueOnce(view(2, ['a', 'b', 'c', 'd']))
    await dropOn(own('b'), chip('a'))
    expect(MovePhoto).not.toHaveBeenCalled()
    expect(Api.AddPhotos).toHaveBeenCalledExactlyOnceWith('a', ['f1'], 3)
  })
})

// 0.26-124-23: группа «Фон» карточки узнаёт об удачном броске по dropLanded; отказ Go её не трогает.
describe('dropOn: бросок фото на фон', () => {
  const photo = { kind: 'photos', ids: ['f1'] } as never
  const zone = { kind: 'backdrop', spread: 'a', scope: 'left' } as never
  const saved = dropHooks.setBackdrop
  afterEach(() => {
    dropHooks.setBackdrop = saved
  })

  it('Go принял — dropLanded.n растёт, со спредом и областью', async () => {
    dropHooks.setBackdrop = vi.fn(async () => true)
    const n = dropLanded.n
    await dropOn(photo, zone)
    expect(dropHooks.setBackdrop).toHaveBeenCalledExactlyOnceWith('a', 'left', 'f1')
    expect([dropLanded.n, dropLanded.spread, dropLanded.scope]).toEqual([n + 1, 'a', 'left'])
  })

  it('правка не прошла — счётчик на месте', async () => {
    dropHooks.setBackdrop = vi.fn(async () => false)
    const n = dropLanded.n
    await dropOn(photo, zone)
    expect(dropLanded.n).toBe(n)
  })
})
