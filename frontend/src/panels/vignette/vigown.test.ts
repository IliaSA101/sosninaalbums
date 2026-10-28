import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go и store подменены: проверяем, что уходит в Go, какие тосты и когда обновляется список шаблонов.
vi.mock('../../base/core/store.svelte', () => ({
  store: { take: vi.fn(async () => true), notify: vi.fn(), undoToast: vi.fn() },
  Api: {
    SaveVignetteTemplate: vi.fn(),
    RenameGridTemplate: vi.fn(),
    DeleteGridTemplate: vi.fn(),
    ResetVignette: vi.fn(),
  },
}))

import { Api, store } from '../../base/core/store.svelte'
import type { VigChoice } from '../../base/core/types'
import {
  NAME_MAX,
  canEditOwn,
  deleteText,
  deleteVigTemplate,
  firstOwn,
  isOwnTpl,
  nameOf,
  ownLabel,
  pickedRatio,
  renameVigTemplate,
  resetVignette,
  saveVigTemplate,
  tileLines,
  tplLabel,
} from './vigown'
import { vigOwn } from './vigown.svelte'

const ch = (o: Partial<VigChoice> = {}): VigChoice => ({
  id: 'head-t-left',
  name: 'Шапка, учителя слева',
  boxes: [],
  roles: [],
  pupilW: 35,
  small: false,
  over: false,
  ...o,
})
const mine = (o: Partial<VigChoice> = {}) => ch({ id: 'my:a1', name: 'Мой', own: true, ratio: '4:5', ...o })

beforeEach(() => {
  vi.clearAllMocks()
  vigOwn.close()
  vigOwn.rev = 0
  vi.mocked(Api.SaveVignetteTemplate).mockResolvedValue()
  vi.mocked(Api.RenameGridTemplate).mockResolvedValue()
  vi.mocked(Api.DeleteGridTemplate).mockResolvedValue()
})

describe('раздел «Мои шаблоны»', () => {
  it('firstOwn: индекс первого своего, своих нет — −1', () => {
    expect(firstOwn([ch(), ch({ id: 'b' })])).toBe(-1)
    expect(firstOwn([ch(), mine(), mine({ id: 'my:b2' })])).toBe(1)
  })

  it('ownLabel: удалённый из файла шаблон помечен', () => {
    expect(ownLabel(mine())).toBe('Мой')
    expect(ownLabel(mine({ gone: true }))).toBe('Мой (шаблон удалён)')
  })

  it('✎ и × — только у живого своего и при уровне «Шаблоны»', () => {
    expect(canEditOwn(mine(), true)).toBe(true)
    expect(canEditOwn(mine(), false)).toBe(false)
    expect(canEditOwn(ch(), true)).toBe(false)
    expect(canEditOwn(mine({ gone: true }), true)).toBe(false)
  })

  it('tileLines: у удалённого — строка «удалён»', () => {
    expect(tileLines(mine())).toEqual(['35 мм'])
    expect(tileLines(mine({ gone: true }))).toEqual(['35 мм', 'удалён'])
  })
})

describe('tplLabel и isOwnTpl', () => {
  it('имя берётся из списка Go: после переименования строка и карточка показывают новое', () => {
    expect(tplLabel([mine({ name: 'Новое имя' })], 'my:a1', 'Старое (снимок)')).toBe('Новое имя')
    expect(tplLabel([mine({ gone: true })], 'my:a1', 'Старое')).toBe('Мой (шаблон удалён)')
  })
  it('списка или карточки нет — запасное имя листа, иначе пусто', () => {
    expect(tplLabel([], 'my:a1', 'Старое')).toBe('Старое')
    expect(tplLabel([ch()], 'my:a1')).toBe('')
  })
  it('isOwnTpl: только my:', () => {
    expect(isOwnTpl('my:a1')).toBe(true)
    expect(isOwnTpl('head-t-left')).toBe(false)
  })
})

describe('pickedRatio', () => {
  const list = [
    ch(),
    mine(),
    mine({ id: 'my:same', ratio: '3:4' }),
    mine({ id: 'my:bad', ratio: '7:7' }),
    mine({ id: 'my:none', ratio: '' }),
  ]

  it('свой шаблон с другим портретом — его портрет', () => {
    expect(pickedRatio(list, 'my:a1', '3:4')).toBe('4:5')
  })
  it('тот же портрет, каталожный, неизвестный и пустой — менять нечего', () => {
    expect(pickedRatio(list, 'my:same', '3:4')).toBeNull()
    expect(pickedRatio(list, 'head-t-left', '2:3')).toBeNull()
    expect(pickedRatio(list, 'my:bad', '3:4')).toBeNull()
    expect(pickedRatio(list, 'my:none', '3:4')).toBeNull()
    expect(pickedRatio(list, 'нет такого', '3:4')).toBeNull()
  })
})

describe('тексты', () => {
  it('deleteText: «Листы» на листе, «Развороты» в альбоме', () => {
    expect(deleteText('Мой', 'Лист')).toBe('Шаблон «Мой» будет удалён из списка. Листы, где он уже применён, не изменятся.')
    expect(deleteText('Мой', 'Разворот')).toContain('Развороты, где')
  })
  it('nameOf: пробелы по краям убираются, длинное обрезается по знакам', () => {
    expect(nameOf('  Класс  ')).toBe('Класс')
    expect(Array.from(nameOf('я'.repeat(100)))).toHaveLength(NAME_MAX)
  })
})

describe('вызовы Go', () => {
  it('saveVigTemplate: id листа и имя в Go, тост «Шаблон сохранён»', async () => {
    expect(await saveVigTemplate('s1', ' Выпуск ')).toBe(true)
    expect(Api.SaveVignetteTemplate).toHaveBeenCalledWith('s1', 'Выпуск')
    expect(store.notify).toHaveBeenCalledWith('Шаблон сохранён')
  })

  it('saveVigTemplate: ошибка Go («Это не виньетка») — тост-ошибка, «сохранён» нет', async () => {
    vi.mocked(Api.SaveVignetteTemplate).mockRejectedValue('Это не виньетка')
    expect(await saveVigTemplate('s1', '')).toBe(false)
    expect(store.notify).toHaveBeenCalledWith('Это не виньетка', 'error')
    expect(store.notify).not.toHaveBeenCalledWith('Шаблон сохранён')
  })

  it('переименование и удаление зовут Go с id карточки как есть («my:…»)', async () => {
    expect(await renameVigTemplate('my:a1', ' Новое ')).toBe(true)
    expect(Api.RenameGridTemplate).toHaveBeenCalledWith('my:a1', 'Новое')
    expect(await deleteVigTemplate('my:a1')).toBe(true)
    expect(Api.DeleteGridTemplate).toHaveBeenCalledWith('my:a1')
  })

  it('«шаблоны сохранены более новой версией» — понятный тост', async () => {
    vi.mocked(Api.DeleteGridTemplate).mockRejectedValue('шаблоны сохранены более новой версией программы')
    expect(await deleteVigTemplate('my:a1')).toBe(false)
    expect(vi.mocked(store.notify).mock.calls[0][0]).toMatch(/Обновите программу/)
  })

  it('resetVignette: ответ — в store.take, лишние фото — тост «Вернуть»', async () => {
    const view = { rev: 9 }
    vi.mocked(Api.ResetVignette).mockResolvedValue({ view, left: 2 } as never)
    expect(await resetVignette('s1')).toBe(true)
    expect(Api.ResetVignette).toHaveBeenCalledWith('s1')
    expect(store.take).toHaveBeenCalledWith(view)
    expect(store.undoToast).toHaveBeenCalledWith('Убрано в ленту: 2')
  })

  it('resetVignette: left = 0 — без тоста; ошибка — тост, вид не меняется', async () => {
    vi.mocked(Api.ResetVignette).mockResolvedValueOnce({ view: {}, left: 0 } as never)
    await resetVignette('s1')
    expect(store.undoToast).not.toHaveBeenCalled()
    vi.mocked(Api.ResetVignette).mockRejectedValueOnce('Лист не найден')
    vi.mocked(store.take).mockClear()
    expect(await resetVignette('s1')).toBe(false)
    expect(store.take).not.toHaveBeenCalled()
    expect(store.notify).toHaveBeenCalledWith('Лист не найден', 'error')
  })
})

describe('vigOwn: окна', () => {
  it('save: пустое имя, commit зовёт Go и поднимает rev', async () => {
    vigOwn.save('s1')
    expect(vigOwn.ask).toEqual({ kind: 'save', spread: 's1' })
    expect(vigOwn.name).toBe('')
    vigOwn.name = 'Класс А'
    expect(await vigOwn.commit()).toBe(true)
    expect(Api.SaveVignetteTemplate).toHaveBeenCalledWith('s1', 'Класс А')
    expect(vigOwn.ask).toBeNull()
    expect(vigOwn.rev).toBe(1)
  })

  it('rename: в поле — прежнее имя; commit переименовывает', async () => {
    vigOwn.rename({ id: 'my:a1', name: 'Мой' })
    expect(vigOwn.name).toBe('Мой')
    vigOwn.name = 'Другой'
    await vigOwn.commit()
    expect(Api.RenameGridTemplate).toHaveBeenCalledWith('my:a1', 'Другой')
    expect(vigOwn.rev).toBe(1)
  })

  it('delete: вопрос с именем; commit удаляет по id карточки', async () => {
    vigOwn.del({ id: 'my:a1', name: 'Мой' })
    expect(vigOwn.ask).toEqual({ kind: 'delete', id: 'my:a1', name: 'Мой' })
    await vigOwn.commit()
    expect(Api.DeleteGridTemplate).toHaveBeenCalledWith('my:a1')
    expect(vigOwn.rev).toBe(1)
  })

  it('Отмена (close): в Go не идёт, rev прежний; ошибка Go — rev прежний', async () => {
    vigOwn.save('s1')
    vigOwn.close()
    expect(await vigOwn.commit()).toBe(false)
    expect(Api.SaveVignetteTemplate).not.toHaveBeenCalled()

    vi.mocked(Api.DeleteGridTemplate).mockRejectedValue('Шаблон не найден')
    vigOwn.del({ id: 'my:x', name: 'X' })
    expect(await vigOwn.commit()).toBe(false)
    expect(vigOwn.rev).toBe(0)
    expect(vigOwn.ask).toBeNull()
  })
})
