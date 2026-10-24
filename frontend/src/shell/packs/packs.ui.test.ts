import { beforeEach, describe, expect, it, vi } from 'vitest'

// Состояние окна «Наборы библиотеки»: Go подменён, вызовы — с проверкой аргументов; итоги — тосты.
const api = vi.hoisted(() => ({
  LibraryPacks: vi.fn(),
  AddPackFile: vi.fn(),
  AddPackPath: vi.fn(),
  ConfirmPack: vi.fn(),
  PlanRemovePack: vi.fn(),
  RemovePack: vi.fn(),
  DecorCatalog: vi.fn(),
  ShowLibraryFolder: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { packsUi } = await import('./packs.svelte')
const { store } = await import('../../base/core/store.svelte')
const { decor } = await import('../../panels/decor/decor.svelte')

const info = (o: Record<string, unknown> = {}) => ({
  id: 'ivory',
  title: 'Айвори',
  about: '',
  author: 'Марина',
  license: 'CC0-1.0',
  source: '',
  origin: 'file',
  version: 3,
  bg: 12,
  sticker: 0,
  spread: 8,
  frame: 0,
  print: 0,
  bytes: 1048576,
  removable: true,
  ...o,
})
const builtin = info({ id: 'builtin', title: 'Встроенные', origin: 'builtin', removable: false, version: 1, bytes: 0 })
const list = (packs: unknown[] = [builtin, info()], o: Record<string, unknown> = {}) => ({
  packs,
  net: { allowed: false, atStart: false, checked: 0, skipped: 0 },
  busy: false,
  readOnly: false,
  ...o,
})
const added = (o: Record<string, unknown> = {}) => ({ pack: info(), ask: '', fromVersion: 0, token: '', skipped: [], ...o })
const toast = () => store.toast as { text: string; kind?: string } | null

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.LibraryPacks.mockResolvedValue(list())
  api.DecorCatalog.mockResolvedValue({ kind: 'bg', themes: [], items: [], recent: [], readOnly: false })
  packsUi.close()
  packsUi.busy = false
  store.toast = null
  decor.close()
})

describe('packsUi: открытие и список', () => {
  it('show читает список, встроенные первыми', async () => {
    api.LibraryPacks.mockResolvedValue(list([info(), builtin]))
    await packsUi.show()
    expect(packsUi.open).toBe(true)
    expect(packsUi.packs.map((p) => p.id)).toEqual(['builtin', 'ivory'])
    expect(packsUi.loading).toBe(false)
  })

  it('повторное «Наборы…» при открытом окне — тот же список, окно одно', async () => {
    await packsUi.show()
    await packsUi.show()
    expect(packsUi.open).toBe(true)
    expect(packsUi.packs).toHaveLength(2)
  })

  it('ошибка чтения списка остаётся в окне', async () => {
    api.LibraryPacks.mockRejectedValue('библиотека недоступна')
    await packsUi.show()
    expect(packsUi.error).toBe('Библиотека недоступна')
    expect(packsUi.packs).toEqual([])
  })

  it('пустой ответ Go (null вместо списка) — пустой список без падения', async () => {
    api.LibraryPacks.mockResolvedValue(list(null as unknown as unknown[]))
    await packsUi.show()
    expect(packsUi.packs).toEqual([])
  })

  it('busy и readOnly от Go блокируют кнопки операций', async () => {
    api.LibraryPacks.mockResolvedValue(list([builtin, info()], { busy: true }))
    await packsUi.show()
    expect(packsUi.locked).toBe(true)
    await packsUi.addFile()
    await packsUi.askRemove(packsUi.packs[1])
    expect(api.AddPackFile).not.toHaveBeenCalled()
    expect(api.PlanRemovePack).not.toHaveBeenCalled()
    api.LibraryPacks.mockResolvedValue(list([builtin, info()], { readOnly: true }))
    await packsUi.load()
    await packsUi.addFile()
    expect(api.AddPackFile).not.toHaveBeenCalled()
  })

  it('закрытие окна отбрасывает запоздавший ответ списка', async () => {
    let done: (v: unknown) => void = () => {}
    api.LibraryPacks.mockReturnValue(new Promise((r) => (done = r)))
    const p = packsUi.show()
    packsUi.close()
    done(list())
    await p
    expect(packsUi.packs).toEqual([])
  })
})

describe('packsUi: добавить из файла', () => {
  beforeEach(async () => {
    await packsUi.show()
  })

  it('поставлен: тост с составом, список перечитан', async () => {
    api.AddPackFile.mockResolvedValue(added())
    api.LibraryPacks.mockClear()
    await packsUi.addFile()
    expect(toast()?.text).toBe('Набор «Айвори» добавлен: 12 фонов, 8 разворотов')
    expect(api.LibraryPacks).toHaveBeenCalled()
    expect(packsUi.busy).toBe(false)
  })

  it('диалог закрыт без выбора — ни тоста, ни вопроса', async () => {
    api.AddPackFile.mockResolvedValue(added({ pack: info({ id: '', title: '' }) }))
    await packsUi.addFile()
    expect(toast()).toBeNull()
    expect(packsUi.ask).toBeNull()
  })

  it('плохой архив — тост-ошибка с причиной, список не меняется', async () => {
    api.AddPackFile.mockRejectedValue('набор не прошёл проверку: хэш не сходится')
    await packsUi.addFile()
    expect(toast()).toMatchObject({ text: 'Набор не прошёл проверку: хэш не сходится', kind: 'error' })
    expect(packsUi.packs).toHaveLength(2)
    expect(packsUi.busy).toBe(false)
  })

  it('«Уже есть» и «Уже стоит версия новее» — тосты, вопроса нет', async () => {
    api.AddPackPath.mockResolvedValueOnce(added({ ask: 'same' }))
    await packsUi.addPath('D:\\a.zip')
    expect(toast()?.text).toBe('Набор «Айвори» уже есть')
    api.AddPackPath.mockResolvedValueOnce(added({ ask: 'older', fromVersion: 5 }))
    await packsUi.addPath('D:\\a.zip')
    expect(toast()?.text).toBe('Набор «Айвори»: уже стоит версия новее (5)')
    expect(packsUi.ask).toBeNull()
  })

  it('новее стоящего: вопрос «Заменить?», «Отмена» ничего не вызывает', async () => {
    api.AddPackFile.mockResolvedValue(added({ ask: 'update', fromVersion: 2, token: 'tk' }))
    await packsUi.addFile()
    expect(packsUi.ask).toMatchObject({ kind: 'replace', token: 'tk', text: 'Заменить набор «Айвори» версии 2 версией 3?' })
    expect(toast()).toBeNull()
    packsUi.dismiss()
    expect(packsUi.ask).toBeNull()
    expect(api.ConfirmPack).not.toHaveBeenCalled()
  })

  it('«Заменить» зовёт ConfirmPack с токеном; тост «обновлён до версии»', async () => {
    api.AddPackFile.mockResolvedValue(added({ ask: 'update', fromVersion: 2, token: 'tk' }))
    await packsUi.addFile()
    api.ConfirmPack.mockResolvedValue(added())
    await packsUi.confirmReplace()
    expect(api.ConfirmPack).toHaveBeenCalledWith('tk')
    expect(packsUi.ask).toBeNull()
    expect(toast()?.text).toBe('Набор «Айвори» обновлён до версии 3: 12 фонов, 8 разворотов')
  })

  it('вопрос устарел — ошибка Go тостом, окно живо', async () => {
    api.AddPackFile.mockResolvedValue(added({ ask: 'update', fromVersion: 2, token: 'tk' }))
    await packsUi.addFile()
    api.ConfirmPack.mockRejectedValue('вопрос устарел — добавьте набор ещё раз')
    await packsUi.confirmReplace()
    expect(toast()).toMatchObject({ text: 'Вопрос устарел — добавьте набор ещё раз', kind: 'error' })
    expect(packsUi.open).toBe(true)
    expect(packsUi.busy).toBe(false)
  })

  it('«Заменить» без вопроса молчит; вторая операция при занятых кнопках не стартует', async () => {
    await packsUi.confirmReplace()
    expect(api.ConfirmPack).not.toHaveBeenCalled()
    packsUi.busy = true
    await packsUi.addFile()
    expect(api.AddPackFile).not.toHaveBeenCalled()
  })

  it('пропущенное в архиве — в тосте итога', async () => {
    api.AddPackFile.mockResolvedValue(added({ skipped: [{ name: 'x.svg', why: 'SVG не поддерживается' }] }))
    await packsUi.addFile()
    expect(toast()?.text).toBe('Набор «Айвори» добавлен: 12 фонов, 8 разворотов; пропущено: 1 (x.svg — SVG не поддерживается)')
  })
})

describe('packsUi: удалить набор', () => {
  beforeEach(async () => {
    await packsUi.show()
  })

  it('«Удалить» спрашивает числа у Go и открывает вопрос', async () => {
    api.PlanRemovePack.mockResolvedValue({ files: 20, kept: 3, bytes: 1048576 })
    await packsUi.askRemove(packsUi.packs[1])
    expect(api.PlanRemovePack).toHaveBeenCalledWith('ivory')
    const a = packsUi.ask
    expect(a?.kind).toBe('remove')
    expect(a && 'note' in a && a.note).toContain('3 файла останутся')
    expect(a?.text).toContain('Удалить набор «Айвори»?')
    expect(api.RemovePack).not.toHaveBeenCalled()
  })

  it('встроенный не удаляется', async () => {
    await packsUi.askRemove(packsUi.packs[0])
    expect(api.PlanRemovePack).not.toHaveBeenCalled()
  })

  it('«Отмена» — набор на месте, Go не звали', async () => {
    api.PlanRemovePack.mockResolvedValue({ files: 1, kept: 0, bytes: 1 })
    await packsUi.askRemove(packsUi.packs[1])
    packsUi.dismiss()
    expect(packsUi.ask).toBeNull()
    expect(api.RemovePack).not.toHaveBeenCalled()
    expect(packsUi.packs).toHaveLength(2)
  })

  it('«Удалить» в вопросе: RemovePack, список из ответа, тост', async () => {
    api.PlanRemovePack.mockResolvedValue({ files: 1, kept: 0, bytes: 1 })
    await packsUi.askRemove(packsUi.packs[1])
    api.RemovePack.mockResolvedValue(list([builtin]))
    await packsUi.confirmRemove()
    expect(api.RemovePack).toHaveBeenCalledWith('ivory')
    expect(packsUi.packs.map((p) => p.id)).toEqual(['builtin'])
    expect(toast()?.text).toBe('Набор «Айвори» удалён')
    expect(packsUi.ask).toBeNull()
  })

  it('ошибка удаления — тост-ошибка и свежий список', async () => {
    api.PlanRemovePack.mockResolvedValue({ files: 1, kept: 0, bytes: 1 })
    await packsUi.askRemove(packsUi.packs[1])
    api.RemovePack.mockRejectedValue('библиотека уже обновляется')
    api.LibraryPacks.mockClear()
    await packsUi.confirmRemove()
    expect(toast()).toMatchObject({ text: 'Библиотека уже обновляется', kind: 'error' })
    expect(api.LibraryPacks).toHaveBeenCalled()
    expect(packsUi.busy).toBe(false)
  })

  it('ошибка расчёта — тост, вопроса нет', async () => {
    api.PlanRemovePack.mockRejectedValue('набор не найден')
    await packsUi.askRemove(packsUi.packs[1])
    expect(toast()?.kind).toBe('error')
    expect(packsUi.ask).toBeNull()
    expect(packsUi.busy).toBe(false)
  })

  it('окно закрыли, пока считалось, — вопрос не появляется', async () => {
    let done: (v: unknown) => void = () => {}
    api.PlanRemovePack.mockReturnValue(new Promise((r) => (done = r)))
    const p = packsUi.askRemove(packsUi.packs[1])
    packsUi.close()
    done({ files: 1, kept: 0, bytes: 1 })
    await p
    expect(packsUi.ask).toBeNull()
  })
})

describe('packsUi: library:changed', () => {
  it('открытое окно «Наборы» перечитывает список', async () => {
    await packsUi.show()
    api.LibraryPacks.mockClear()
    packsUi.changed()
    expect(api.LibraryPacks).toHaveBeenCalledTimes(1)
  })

  it('закрытое окно список не читает', () => {
    packsUi.changed()
    expect(api.LibraryPacks).not.toHaveBeenCalled()
    expect(api.DecorCatalog).not.toHaveBeenCalled()
  })

  it('названия поставленных наборов уходят окнам «Фоны/Стикеры/Развороты» (073-1)', () => {
    const note = vi.spyOn(decor, 'note')
    packsUi.changed(['Айвори'])
    expect(note).toHaveBeenCalledWith(['Айвори'])
    packsUi.changed() // старое событие без данных
    packsUi.changed([]) // удаление набора
    expect(note).toHaveBeenCalledTimes(3)
    note.mockRestore()
  })

  it('открытое окно «Фоны» перечитывает каталог', () => {
    decor.cur = { kind: 'bg', spread: 's1', scope: 'spread' }
    packsUi.changed()
    expect(api.DecorCatalog).toHaveBeenCalledWith('bg')
  })
})

describe('packsUi: «Показать папку»', () => {
  it('зовёт Go без аргументов, тоста нет', async () => {
    api.ShowLibraryFolder.mockResolvedValue(undefined)
    await packsUi.show()
    await packsUi.showFolder()
    expect(api.ShowLibraryFolder).toHaveBeenCalledWith()
    expect(toast()).toBeNull()
  })

  it('ошибка Go — тост с заглавной буквы, окно остаётся', async () => {
    api.ShowLibraryFolder.mockRejectedValue(new Error('проводник не открылся'))
    await packsUi.show()
    await packsUi.showFolder()
    expect(toast()?.kind).toBe('error')
    expect(toast()?.text).toBe('Проводник не открылся')
    expect(packsUi.open).toBe(true)
  })

  it('не зависит от busy и занятости Go: только открывает папку', async () => {
    api.ShowLibraryFolder.mockResolvedValue(undefined)
    packsUi.busy = true
    packsUi.goBusy = true
    await packsUi.showFolder()
    expect(api.ShowLibraryFolder).toHaveBeenCalledTimes(1)
    packsUi.goBusy = false
  })
})
