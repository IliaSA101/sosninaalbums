import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  MyColors: vi.fn(),
  ImportColors: vi.fn(),
  RememberColor: vi.fn(),
  ForgetColor: vi.fn(),
  SpreadPhotoColors: vi.fn(),
}))
vi.mock('../../../wailsjs/go/app/App', () => api)

function useStorage(init: Record<string, string>) {
  const m = new Map(Object.entries(init))
  const s = {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => void m.set(k, v),
    removeItem: (k: string) => void m.delete(k),
  }
  Object.defineProperty(globalThis, 'localStorage', { value: s, configurable: true, writable: true })
  return s
}

// Свежий экземпляр на тест: «один раз за запуск» живёт в модуле.
async function fresh() {
  vi.resetModules()
  return (await import('./mycolors.svelte')).mycolors
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
})

describe('mycolors', () => {
  it('перенос recentColors один раз', async () => {
    const ls = useStorage({ recentColors: JSON.stringify(['#AA0000', 'bad', '#00bb00']) })
    api.MyColors.mockResolvedValue([])
    api.ImportColors.mockResolvedValue(['#aa0000', '#00bb00'])
    const m = await fresh()
    await m.load()
    await m.load() // повторное открытие окна за запуск Go не спрашивает
    expect(api.ImportColors).toHaveBeenCalledTimes(1)
    expect(api.ImportColors).toHaveBeenCalledWith(['#aa0000', '#00bb00'])
    expect(api.MyColors).toHaveBeenCalledTimes(1)
    expect(m.list).toEqual(['#aa0000', '#00bb00'])
    expect(ls.getItem('recentColors')).toBeNull()
  })

  it('в config.json есть цвета — localStorage не трогается', async () => {
    const ls = useStorage({ recentColors: JSON.stringify(['#aa0000']) })
    api.MyColors.mockResolvedValue(['#123456'])
    const m = await fresh()
    await m.load()
    expect(api.ImportColors).not.toHaveBeenCalled()
    expect(ls.getItem('recentColors')).not.toBeNull()
    expect(m.list).toEqual(['#123456'])
  })

  it('пусто и там и там — Go не зовём, ключ-мусор стирается', async () => {
    const ls = useStorage({ recentColors: '{не json' })
    api.MyColors.mockResolvedValue([])
    const m = await fresh()
    await m.load()
    expect(api.ImportColors).not.toHaveBeenCalled()
    expect(ls.getItem('recentColors')).toBeNull()
    expect(m.list).toEqual([])
  })

  it('Go не ответил на загрузку — следующий load пробует снова', async () => {
    useStorage({})
    api.MyColors.mockRejectedValueOnce(new Error('нет')).mockResolvedValue(['#111111'])
    const m = await fresh()
    await m.load()
    expect(m.list).toEqual([])
    await m.load()
    expect(m.list).toEqual(['#111111'])
  })

  it('remember / add / forget — список из ответа Go', async () => {
    useStorage({})
    const m = await fresh()
    api.RememberColor.mockResolvedValue(['#222222', '#111111'])
    await m.remember('#222222')
    expect(m.list).toEqual(['#222222', '#111111'])
    api.RememberColor.mockResolvedValue(['#333333', '#222222', '#111111'])
    await m.add('#333333')
    expect(api.RememberColor).toHaveBeenLastCalledWith('#333333')
    api.ForgetColor.mockResolvedValue(['#333333', '#111111'])
    await m.forget('#222222')
    expect(m.list).toEqual(['#333333', '#111111'])
  })

  it('ошибка Go в remember — список прежний', async () => {
    useStorage({})
    const m = await fresh()
    api.RememberColor.mockResolvedValueOnce(['#111111'])
    await m.remember('#111111')
    api.RememberColor.mockRejectedValueOnce(new Error('нет'))
    await m.remember('#222222')
    expect(m.list).toEqual(['#111111'])
  })

  it('photoColors — ответ Go', async () => {
    useStorage({})
    api.SpreadPhotoColors.mockResolvedValue(['#d9d3cf'])
    const m = await fresh()
    expect(await m.photoColors('s1')).toEqual(['#d9d3cf'])
    expect(api.SpreadPhotoColors).toHaveBeenCalledWith('s1')
  })
})
