import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go и store подменены: проверяем, что окно просит у Go, и что делает с ответом.
const view = { rev: 7, spreads: [{ id: 's1', photos: ['a', 'b'] }] }
vi.mock('../../base/core/store.svelte', () => ({
  store: {
    view: { spreads: [{ id: 's1', photos: ['a', 'b'] }] },
    spread: { id: 's1' },
    apply: vi.fn(),
    notify: vi.fn(),
  },
  Api: {
    Person: vi.fn(async (id: string) =>
      id === 'a' ? { last: 'Иванов', first: 'Пётр', middle: '', fromFile: true } : { last: '', first: '', middle: '', fromFile: false },
    ),
    PasteNames: vi.fn(),
  },
}))
vi.mock('../../editor/text/edit.svelte', () => ({ edit: { idle: vi.fn(async () => {}) } }))

import { Api, store } from '../../base/core/store.svelte'
import { namesUi } from './names.svelte'

beforeEach(() => {
  vi.clearAllMocks()
  namesUi.open = false
})

describe('namesUi', () => {
  it('show открывает окно на листе и узнаёт, у кого уже есть имя', async () => {
    await namesUi.show()
    expect(namesUi.open).toBe(true)
    expect(namesUi.spreadId).toBe('s1')
    expect(namesUi.named).toEqual({ a: true, b: false })
  })

  it('нет такого листа — окно не открывается', async () => {
    await namesUi.show('нет')
    expect(namesUi.open).toBe(false)
  })

  it('apply отдаёт Go текст и «Имя первым», берёт вид, закрывает окно и сообщает итог', async () => {
    vi.mocked(Api.PasteNames).mockResolvedValue({ view, extra: 2, missing: 1 } as never)
    await namesUi.show()
    namesUi.text = 'Иванов Пётр'
    namesUi.firstFirst = true
    expect(await namesUi.apply()).toBe(true)
    expect(Api.PasteNames).toHaveBeenCalledWith('s1', 'Иванов Пётр', true)
    expect(store.apply).toHaveBeenCalledWith(view)
    expect(store.notify).toHaveBeenCalledWith(expect.stringContaining('лишних: 2'))
    expect(namesUi.open).toBe(false)
    namesUi.firstFirst = false
  })

  it('ошибка Go остаётся в окне, правки нет', async () => {
    vi.mocked(Api.PasteNames).mockRejectedValue('список имён слишком длинный')
    await namesUi.show()
    expect(await namesUi.apply()).toBe(false)
    expect(namesUi.open).toBe(true)
    expect(namesUi.error).toBe('Список имён слишком длинный')
    expect(store.apply).not.toHaveBeenCalled()
  })
})
