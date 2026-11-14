import { beforeEach, describe, expect, it, vi } from 'vitest'

// Общие действия «Файл» меню фото и меню миниатюры: имена в буфер, тосты, отказ буфера, «Показать в …».
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ ShowPhotoInExplorer: vi.fn() }))
vi.stubGlobal('window', globalThis)

const Api = await import('../../../wailsjs/go/app/App')
const { store } = await import('../../base/core/store.svelte')
const { copyNames, namesOf, reveal } = await import('./fileact')

const Show = vi.mocked(Api.ShowPhotoInExplorer)
const write = vi.fn()
const notify = vi.spyOn(store, 'notify').mockImplementation(() => {})
const none = () => false

beforeEach(() => {
  write.mockReset()
  notify.mockClear()
  Show.mockReset()
  vi.stubGlobal('navigator', { clipboard: { writeText: write } })
  vi.spyOn(store.byId, 'get').mockImplementation(((id: string) => ({ a: { name: 'a.jpg' }, b: { name: 'b.jpg' } })[id]) as never)
})

describe('namesOf', () => {
  it('только найденные, в порядке выделения; без имени — пропуск', () => {
    const names: Record<string, string> = { a: 'a.jpg', b: 'b.jpg' }
    expect(
      namesOf(
        ['a', 'b', 'c', 'd'],
        (id) => names[id],
        (id) => id === 'b',
      ),
    ).toEqual(['a.jpg'])
  })
})

describe('copyNames', () => {
  it('одно фото — имя и тост в единственном числе', async () => {
    write.mockResolvedValue(undefined)
    await copyNames(['a'], none)
    expect(write).toHaveBeenCalledWith('a.jpg')
    expect(notify).toHaveBeenCalledWith('Имя файла скопировано', 'info')
  })
  it('несколько — по строке на фото, тост во множественном числе', async () => {
    write.mockResolvedValue(undefined)
    await copyNames(['a', 'b'], none)
    expect(write).toHaveBeenCalledWith('a.jpg\nb.jpg')
    expect(notify).toHaveBeenCalledWith('Имена файлов скопированы', 'info')
  })
  it('все пропали или неизвестны — буфер и тост молчат', async () => {
    await copyNames(['a', 'zz'], (id) => id === 'a')
    await copyNames([], none)
    expect(write).not.toHaveBeenCalled()
    expect(notify).not.toHaveBeenCalled()
  })
  it('отказ буфера — свой текст по-русски, без исключения наружу', async () => {
    write.mockRejectedValue(new Error('NotAllowedError: Write permission denied'))
    await copyNames(['a'], none)
    expect(notify).toHaveBeenCalledWith('Не удалось скопировать в буфер обмена', 'error')
  })
})

describe('reveal', () => {
  it('зовёт Go с id фото; успех тостов не даёт', async () => {
    Show.mockResolvedValue(undefined)
    await reveal('a')
    expect(Show).toHaveBeenCalledWith('a')
    expect(notify).not.toHaveBeenCalled()
  })
  it('ошибка Go — тост ошибки', async () => {
    Show.mockRejectedValue(new Error('нет файла'))
    await reveal('a')
    expect(notify).toHaveBeenCalledTimes(1)
    expect(notify.mock.calls[0][1]).toBe('error')
  })
})
