import { beforeEach, describe, expect, it, vi } from 'vitest'

// Слой lookApi (131 U4): каждый метод — ровно один вызов биндинга Go с теми же аргументами, ответ и ошибка идут как есть.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  DecorCatalog: vi.fn(),
  LibraryPacks: vi.fn(),
  LookInfo: vi.fn(),
  LookCount: vi.fn(),
  PlanLook: vi.fn(),
  ApplyLook: vi.fn(),
  PlanLookStep: vi.fn(),
  ApplyLookStep: vi.fn(),
  NewLookSpread: vi.fn(),
  RenameDecor: vi.fn(),
  HideDecor: vi.fn(),
  SaveLook: vi.fn(),
  PlanSaveLook: vi.fn(),
}))
vi.stubGlobal('window', globalThis)

const Api = await import('../../../wailsjs/go/app/App')
const { lookApi } = await import('./lookapi')

const ID = 'my:0a1b2c3d4e5f6071'
const o = { mode: 'series' as const, spreads: 9 }

beforeEach(() => vi.clearAllMocks())

describe('lookApi: вызовы биндингов', () => {
  it('каталог — вид album общего DecorCatalog', async () => {
    vi.mocked(Api.DecorCatalog).mockResolvedValue({ kind: 'album' } as never)
    expect(await lookApi.catalog()).toEqual({ kind: 'album' })
    expect(Api.DecorCatalog).toHaveBeenCalledExactlyOnceWith('album')
  })

  it('demoOffer: net.demoOffer из LibraryPacks; поля нет (старый Go) или ответа нет — ложь', async () => {
    vi.mocked(Api.LibraryPacks).mockResolvedValue({ net: { demoOffer: true } } as never)
    expect(await lookApi.demoOffer()).toBe(true)
    vi.mocked(Api.LibraryPacks).mockResolvedValue({ net: {} } as never)
    expect(await lookApi.demoOffer()).toBe(false)
    vi.mocked(Api.LibraryPacks).mockResolvedValue(undefined as never)
    expect(await lookApi.demoOffer()).toBe(false)
  })

  it('карточка, числа и план: id и параметры без изменений', async () => {
    vi.mocked(Api.LookInfo).mockResolvedValue({ min: 3 } as never)
    vi.mocked(Api.LookCount).mockResolvedValue({ n: 9 } as never)
    vi.mocked(Api.PlanLook).mockResolvedValue({ spreads: 12 } as never)
    expect(await lookApi.info(ID)).toEqual({ min: 3 })
    expect(await lookApi.count(ID, 9, 40)).toEqual({ n: 9 })
    expect(await lookApi.plan(ID, o)).toEqual({ spreads: 12 })
    expect(Api.LookInfo).toHaveBeenCalledExactlyOnceWith(ID)
    expect(Api.LookCount).toHaveBeenCalledExactlyOnceWith(ID, 9, 40)
    expect(Api.PlanLook).toHaveBeenCalledExactlyOnceWith(ID, o)
  })

  it('постановка: макет с запросом; шаг на лист (обложка — «cover» и шаг 0); «Разворот макета»', async () => {
    vi.mocked(Api.ApplyLook).mockResolvedValue({ view: { rev: 2 }, plan: {} } as never)
    vi.mocked(Api.PlanLookStep).mockResolvedValue({ frozen: 1 } as never)
    vi.mocked(Api.ApplyLookStep).mockResolvedValue({ view: { rev: 3 } } as never)
    vi.mocked(Api.NewLookSpread).mockResolvedValue({ rev: 4 } as never)
    expect((await lookApi.apply(ID, o)).view.rev).toBe(2)
    expect(await lookApi.planStep('cover', ID, 0)).toEqual({ frozen: 1 })
    expect((await lookApi.applyStep('s3', ID, 2)).view.rev).toBe(3)
    expect((await lookApi.newSpread()).rev).toBe(4)
    expect(Api.ApplyLook).toHaveBeenCalledExactlyOnceWith(ID, o)
    expect(Api.PlanLookStep).toHaveBeenCalledExactlyOnceWith('cover', ID, 0)
    expect(Api.ApplyLookStep).toHaveBeenCalledExactlyOnceWith('s3', ID, 2)
    expect(Api.NewLookSpread).toHaveBeenCalledExactlyOnceWith()
  })

  it('✎ и ×: общие RenameDecor и HideDecor вида album', async () => {
    await lookApi.rename(ID, 'Новое')
    await lookApi.hide(ID)
    expect(Api.RenameDecor).toHaveBeenCalledExactlyOnceWith('album', ID, 'Новое')
    expect(Api.HideDecor).toHaveBeenCalledExactlyOnceWith('album', ID)
  })

  it('ошибка Go доходит до вызывающего (окно покажет красный тост)', async () => {
    vi.mocked(Api.ApplyLook).mockRejectedValue(new Error('Макета нет'))
    await expect(lookApi.apply(ID, o)).rejects.toThrow('Макета нет')
  })

  it('save: ровно один SaveLook с теми же опциями, ответ Go — как есть', async () => {
    const opts = { theme: 'mine', name: 'Мой', ids: ['s1', 's2'], repFrom: 2, repTo: 1, places: true, drafts: true, cover: false }
    vi.mocked(Api.SaveLook).mockResolvedValue({ id: 'my:1', name: 'Мой' } as never)
    expect(await lookApi.save(opts)).toEqual({ id: 'my:1', name: 'Мой' })
    expect(Api.SaveLook).toHaveBeenCalledExactlyOnceWith(opts)
  })

  it('save: ошибка Go (больше 24 разных) доходит текстом Go', async () => {
    vi.mocked(Api.SaveLook).mockRejectedValue(new Error('Не больше 24 разных разворотов — выделите нужные в ленте'))
    await expect(
      lookApi.save({ theme: 'mine', name: 'Б', ids: ['s1'], repFrom: 2, repTo: 1, places: true, drafts: true, cover: false }),
    ).rejects.toThrow('Не больше 24 разных разворотов')
    expect(Api.SaveLook).toHaveBeenCalledTimes(1)
  })

  it('planSave: один PlanSaveLook с теми же опциями; отказ — обычный ответ ok=false, не ошибка', async () => {
    const opts = { theme: 'mine', name: '', ids: ['s1'], repFrom: 2, repTo: 1, places: true, drafts: true, cover: false }
    const r = { ok: false, steps: 0, max: 24, reason: 'Не больше 24 разных разворотов — выделите нужные в ленте' }
    vi.mocked(Api.PlanSaveLook).mockResolvedValue(r as never)
    expect(await lookApi.planSave(opts)).toEqual(r)
    expect(Api.PlanSaveLook).toHaveBeenCalledExactlyOnceWith(opts)
    expect(Api.SaveLook).not.toHaveBeenCalled()
  })
})
