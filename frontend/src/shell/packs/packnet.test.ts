import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { CONSENT_TEXT, isCancelled, progressPercent, progressText, skipLine, startText, updateText } from './packnet'

const MB = 1048576

describe('packnet: тексты', () => {
  it('ход: список, скачивание в МБ, установка, неизвестное', () => {
    expect(progressText({ stage: 'catalog', title: '', done: 0, total: 0 })).toBe('Получаем список наборов…')
    expect(progressText({ stage: 'download', title: 'Айвори', done: 12 * MB, total: 40 * MB })).toBe('Скачиваем «Айвори» — 12 из 40 МБ')
    expect(progressText({ stage: 'download', title: 'Малый', done: 100 * 1024, total: 300 * 1024 })).toBe(
      'Скачиваем «Малый» — 100 КБ из 300 КБ',
    )
    expect(progressText({ stage: 'download', title: 'Х', done: 0, total: 0 })).toBe('Скачиваем «Х»…')
    expect(progressText({ stage: 'install', title: 'Айвори', done: 1, total: 1 })).toBe('Ставим «Айвори»…')
    expect(progressText({ stage: '???', title: '', done: 0, total: 0 })).toBe('Обновляем библиотеку…')
    expect(progressText(null)).toBe('Обновляем библиотеку…')
  })

  it('updateText: без нулевых частей', () => {
    const one = (added: number, updated: number) => updateText({ added, updated, skipped: [], bytes: 1 })
    expect(one(2, 0)).toBe('Библиотека обновлена: новых наборов 2')
    expect(one(0, 1)).toBe('Библиотека обновлена: обновлено наборов 1')
    expect(one(2, 1)).toBe('Библиотека обновлена: новых наборов 2, обновлено 1')
    expect(one(1, 3)).not.toMatch(/\b0\b/)
  })

  it('полоса: доля только у скачивания с известным размером, зажата 0…100', () => {
    expect(progressPercent({ stage: 'download', title: 'a', done: 10, total: 40 })).toBe(25)
    expect(progressPercent({ stage: 'download', title: 'a', done: 50, total: 40 })).toBe(100)
    expect(progressPercent({ stage: 'download', title: 'a', done: -5, total: 40 })).toBe(0)
    expect(progressPercent({ stage: 'download', title: 'a', done: 5, total: 0 })).toBeNull()
    expect(progressPercent({ stage: 'catalog', title: '', done: 0, total: 0 })).toBeNull()
    expect(progressPercent(null)).toBeNull()
  })

  it('итог: новых и обновлено; ничего — «Всё уже свежее»; пропущенное в конце', () => {
    expect(updateText({ added: 2, updated: 1, skipped: [], bytes: 1 })).toBe('Библиотека обновлена: новых наборов 2, обновлено 1')
    expect(updateText({ added: 0, updated: 3, skipped: null, bytes: 1 })).toBe('Библиотека обновлена: обновлено наборов 3')
    expect(updateText({ added: 0, updated: 0, skipped: [], bytes: 0 })).toBe('Всё уже свежее')
    expect(updateText({ added: 0, updated: 0, skipped: null, bytes: 0 })).toBe('Всё уже свежее')
    expect(updateText({ added: 1, updated: 0, skipped: [{ name: 'Новый', why: 'нужна новая версия программы' }], bytes: 1 })).toBe(
      'Библиотека обновлена: новых наборов 1; пропущено: 1 (Новый — нужна новая версия программы)',
    )
  })

  it('тост запуска: части через «; », первая с заглавной, нулевых нет', () => {
    const z = { count: 0, bytes: 0 }
    expect(startText({ updated: 1, offer: z, fresh: { count: 2, bytes: 160 * MB } })).toBe(
      'Обновлено наборов: 1; есть новые наборы: 2 (160,0 МБ)',
    )
    expect(startText({ updated: 0, offer: { count: 2, bytes: 120 * MB }, fresh: z })).toBe('Есть обновления наборов: 2 (120,0 МБ)')
    expect(startText({ updated: 0, offer: { count: 1, bytes: 120 * MB }, fresh: { count: 3, bytes: 5 * MB } })).toBe(
      'Есть обновления наборов: 1 (120,0 МБ); есть новые наборы: 3 (5,0 МБ)',
    )
    expect(startText({ updated: 0, offer: z, fresh: z })).toBe('')
  })

  it('отмена узнаётся по ответу Go (строка и Error), прочее — нет', () => {
    expect(isCancelled('обновление библиотеки отменено')).toBe(true)
    expect(isCancelled(new Error('Обновление библиотеки отменено'))).toBe(true)
    expect(isCancelled('нет связи с GitHub')).toBe(false)
    expect(isCancelled(null)).toBe(false)
  })
})

// Состояние сети: Go подменён.
const api = vi.hoisted(() => ({
  LibraryPacks: vi.fn(),
  SetLibraryNet: vi.fn(),
  UpdateLibrary: vi.fn(),
  DownloadLibraryPack: vi.fn(),
  CancelLibrary: vi.fn(),
  ResetLibrarySkip: vi.fn(),
  DecorCatalog: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { packsUi } = await import('./packs.svelte')
const { store } = await import('../../base/core/store.svelte')
const net = packsUi.net

const builtin = {
  id: 'builtin',
  title: 'Встроенные',
  origin: 'builtin',
  removable: false,
  version: 1,
  bg: 1,
  sticker: 0,
  spread: 0,
  frame: 0,
  print: 0,
  bytes: 0,
}
const netOf = (o: Record<string, unknown> = {}) => ({ allowed: false, atStart: false, checked: 0, skipped: 0, ...o })
const list = (o: Record<string, unknown> = {}) => ({ packs: [builtin], net: netOf(), busy: false, readOnly: false, ...o })
const upd = (o: Record<string, unknown> = {}) => ({ added: 0, updated: 0, skipped: [], bytes: 0, ...o })
const toast = () => store.toast as { text: string; kind?: string; action?: { label: string; run: () => void } } | null

beforeEach(async () => {
  for (const f of Object.values(api)) f.mockReset()
  api.LibraryPacks.mockResolvedValue(list())
  api.DecorCatalog.mockResolvedValue({ kind: 'bg', themes: [], items: [], recent: [], readOnly: false })
  packsUi.close()
  packsUi.busy = false
  packsUi.goBusy = false
  store.toast = null
  await packsUi.show()
})
afterEach(() => {
  vi.useRealTimers()
  packsUi.close()
})

describe('сеть: согласие', () => {
  it('нет согласия: «Обновить» спрашивает, Go не зовёт; «Отмена» — ничего не меняется', () => {
    net.update()
    expect(net.consent).toBe(true)
    expect(net.consentStart).toBe(false)
    net.dismissConsent()
    expect(net.consent).toBe(false)
    expect(api.SetLibraryNet).not.toHaveBeenCalled()
    expect(api.UpdateLibrary).not.toHaveBeenCalled()
  })

  it('«Разрешить и обновить»: SetLibraryNet(true, флажок) → UpdateLibrary, итог тостом', async () => {
    api.SetLibraryNet.mockResolvedValue(netOf({ allowed: true, atStart: true }))
    api.LibraryPacks.mockResolvedValue(list({ net: netOf({ allowed: true, atStart: true }) })) // так Go ответит и после
    api.UpdateLibrary.mockResolvedValue(upd({ added: 2, updated: 1 }))
    net.update()
    net.consentStart = true
    await net.confirmConsent()
    expect(api.SetLibraryNet).toHaveBeenCalledWith(true, true)
    expect(api.UpdateLibrary).toHaveBeenCalledTimes(1)
    expect(net.net).toMatchObject({ allowed: true, atStart: true })
    expect(toast()?.text).toBe('Библиотека обновлена: новых наборов 2, обновлено 1')
    expect(net.consent).toBe(false)
  })

  it('флажок в вопросе не отмечен — проверка при запуске выключена', async () => {
    api.SetLibraryNet.mockResolvedValue(netOf({ allowed: true }))
    api.UpdateLibrary.mockResolvedValue(upd())
    net.update()
    await net.confirmConsent()
    expect(api.SetLibraryNet).toHaveBeenCalledWith(true, false)
  })

  it('согласие флажком при только чтении: вопрос закрыт, подсказка, ни SetLibraryNet, ни UpdateLibrary', async () => {
    await net.setStart(true)
    expect(net.consent).toBe(true)
    packsUi.readOnly = true
    await net.confirmConsent()
    packsUi.readOnly = false
    expect(net.consent).toBe(false)
    expect(toast()?.text).toMatch(/более новая версия программы/)
    expect(api.SetLibraryNet).not.toHaveBeenCalled()
    expect(api.UpdateLibrary).not.toHaveBeenCalled()
  })

  it('согласие флажком, пока Go занят: молча, Go не зовётся', async () => {
    await net.setStart(true)
    packsUi.goBusy = true
    await net.confirmConsent()
    packsUi.goBusy = false
    expect(net.consent).toBe(false)
    expect(toast()).toBeNull()
    expect(api.SetLibraryNet).not.toHaveBeenCalled()
    expect(api.UpdateLibrary).not.toHaveBeenCalled()
  })

  it('«Разрешить и обновить» вне вопроса молчит', async () => {
    await net.confirmConsent()
    expect(api.SetLibraryNet).not.toHaveBeenCalled()
  })

  it('ошибка SetLibraryNet — тост, обновление не стартует', async () => {
    api.SetLibraryNet.mockRejectedValue('не удалось записать настройки')
    net.update()
    await net.confirmConsent()
    expect(api.UpdateLibrary).not.toHaveBeenCalled()
    expect(toast()).toMatchObject({ text: 'Не удалось записать настройки', kind: 'error' })
  })

  it('со согласием «Обновить» идёт сразу, без вопроса', async () => {
    api.LibraryPacks.mockResolvedValue(list({ net: netOf({ allowed: true }) }))
    await packsUi.load()
    api.UpdateLibrary.mockResolvedValue(upd())
    net.update()
    expect(net.consent).toBe(false)
    await vi.waitFor(() => expect(toast()?.text).toBe('Всё уже свежее'))
    expect(api.SetLibraryNet).not.toHaveBeenCalled()
  })

  it('согласие читается из LibraryPacks().net; у старого Go поля нет — согласия нет', async () => {
    api.LibraryPacks.mockResolvedValue(list({ net: netOf({ allowed: true, atStart: true, checked: 5 }) }))
    await packsUi.load()
    expect(net.net).toEqual({ allowed: true, atStart: true, checked: 5, skipped: 0 })
    api.LibraryPacks.mockResolvedValue({ packs: [builtin], busy: false, readOnly: false })
    await packsUi.load()
    expect(net.net.allowed).toBe(false)
  })

  it('закрытие окна гасит вопрос', () => {
    net.update()
    packsUi.close()
    expect(net.consent).toBe(false)
  })

  it('занято или только чтение — «Обновить» молчит', () => {
    packsUi.goBusy = true
    net.update()
    packsUi.goBusy = false
    packsUi.readOnly = true
    net.update()
    packsUi.readOnly = false
    expect(net.consent).toBe(false)
    expect(api.UpdateLibrary).not.toHaveBeenCalled()
  })
})

describe('сеть: «Проверять при запуске» и «Не обращаться к сети»', () => {
  beforeEach(async () => {
    api.LibraryPacks.mockResolvedValue(list({ net: netOf({ allowed: true }) }))
    await packsUi.load()
  })

  it('флажок при согласии: SetLibraryNet(true, on), вид — из ответа Go', async () => {
    api.SetLibraryNet.mockResolvedValue(netOf({ allowed: true, atStart: true }))
    await net.setStart(true)
    expect(api.SetLibraryNet).toHaveBeenCalledWith(true, true)
    expect(net.net.atStart).toBe(true)
    api.SetLibraryNet.mockResolvedValue(netOf({ allowed: true }))
    await net.setStart(false)
    expect(api.SetLibraryNet).toHaveBeenLastCalledWith(true, false)
    expect(net.net.atStart).toBe(false)
  })

  it('ошибка флажка — тост, вид прежний', async () => {
    api.SetLibraryNet.mockRejectedValue('нет доступа к настройкам')
    await net.setStart(true)
    expect(net.net.atStart).toBe(false)
    expect(toast()?.kind).toBe('error')
  })

  it('без согласия флажок открывает вопрос с отмеченным флажком; снятие — ничего', async () => {
    api.LibraryPacks.mockResolvedValue(list())
    await packsUi.load()
    await net.setStart(true)
    expect(net.consent).toBe(true)
    expect(net.consentStart).toBe(true)
    expect(api.SetLibraryNet).not.toHaveBeenCalled()
    net.dismissConsent()
    await net.setStart(false)
    expect(net.consent).toBe(false)
  })

  it('«Не обращаться к сети» снимает согласие и флажок; следующее «Обновить» спросит снова', async () => {
    api.SetLibraryNet.mockResolvedValue(netOf())
    await net.revoke()
    expect(api.SetLibraryNet).toHaveBeenCalledWith(false, false)
    expect(net.net.allowed).toBe(false)
    expect(toast()?.text).toContain('в сеть не обращается')
    net.update()
    expect(net.consent).toBe(true)
  })

  it('ошибка отзыва — тост-ошибка', async () => {
    api.SetLibraryNet.mockRejectedValue('диск только для чтения')
    await net.revoke()
    expect(toast()).toMatchObject({ text: 'Диск только для чтения', kind: 'error' })
    expect(net.net.allowed).toBe(true)
  })
})

describe('сеть: ход, «Отменить», итоги', () => {
  beforeEach(async () => {
    api.LibraryPacks.mockResolvedValue(list({ net: netOf({ allowed: true }) }))
    await packsUi.load()
  })

  it('идёт: кнопки операций заблокированы, ход из library:progress, после — всё сброшено и список перечитан', async () => {
    let done: (v: unknown) => void = () => {}
    api.UpdateLibrary.mockReturnValue(new Promise((r) => (done = r)))
    api.LibraryPacks.mockClear()
    net.update()
    expect(net.updating).toBe(true)
    expect(packsUi.locked).toBe(true)
    expect(net.progress?.stage).toBe('catalog')
    net.progressed({ stage: 'download', title: 'Айвори', done: 5, total: 10 })
    expect(net.progress?.title).toBe('Айвори')
    net.update() // повторное нажатие — второго запроса нет
    expect(api.UpdateLibrary).toHaveBeenCalledTimes(1)
    done(upd({ added: 1 }))
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(net.progress).toBeNull()
    expect(packsUi.locked).toBe(false)
    expect(api.LibraryPacks).toHaveBeenCalled()
    expect(toast()?.text).toBe('Библиотека обновлена: новых наборов 1')
  })

  it('«Отменить»: CancelLibrary; ответ «отменено» — обычный тост, не красный', async () => {
    let fail: (e: unknown) => void = () => {}
    api.UpdateLibrary.mockReturnValue(new Promise((_, r) => (fail = r)))
    api.CancelLibrary.mockResolvedValue(undefined)
    net.update()
    await net.cancel()
    expect(api.CancelLibrary).toHaveBeenCalledTimes(1)
    expect(net.cancelling).toBe(true)
    await net.cancel() // повторное нажатие — не зовёт Go ещё раз
    expect(api.CancelLibrary).toHaveBeenCalledTimes(1)
    fail('обновление библиотеки отменено')
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(toast()).toMatchObject({ text: 'Обновление отменено', kind: 'info' })
    expect(net.cancelling).toBe(false)
  })

  it('«Отменить» без обновления молчит', async () => {
    await net.cancel()
    expect(api.CancelLibrary).not.toHaveBeenCalled()
  })

  it('ошибки: нет связи и «уже обновляется» — красным тостом с заглавной', async () => {
    api.UpdateLibrary.mockRejectedValueOnce('нет связи с GitHub — проверьте интернет и повторите')
    net.update()
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(toast()).toMatchObject({ text: 'Нет связи с GitHub — проверьте интернет и повторите', kind: 'error' })
    api.UpdateLibrary.mockRejectedValueOnce('библиотека уже обновляется')
    net.update()
    await vi.waitFor(() => expect(toast()?.text).toBe('Библиотека уже обновляется'))
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(toast()?.kind).toBe('error')
  })

  it('«Не обращаться к сети» во время загрузки: ответ «отменено» — как отмена', async () => {
    let fail: (e: unknown) => void = () => {}
    api.UpdateLibrary.mockReturnValue(new Promise((_, r) => (fail = r)))
    net.update()
    api.SetLibraryNet.mockResolvedValue(netOf())
    api.LibraryPacks.mockResolvedValue(list())
    await net.revoke()
    fail('обновление библиотеки отменено')
    await vi.waitFor(() => expect(net.updating).toBe(false))
    // один тост — отзыв согласия; «Обновление отменено» не показывается (решение 0.17 №3)
    expect(toast()).toMatchObject({ kind: 'info' })
    expect(toast()?.text).toContain('в сеть не обращается')
    expect(net.net.allowed).toBe(false)
  })

  it('отзыв во время загрузки: «отменено» пришло раньше ответа Go — тоста отмены нет, потом тост отзыва', async () => {
    let fail: (e: unknown) => void = () => {}
    api.UpdateLibrary.mockReturnValue(new Promise((_, r) => (fail = r)))
    net.update()
    let ok: (v: unknown) => void = () => {}
    api.SetLibraryNet.mockReturnValue(new Promise((r) => (ok = r)))
    const p = net.revoke()
    fail('обновление библиотеки отменено')
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(toast()).toBeNull()
    ok(netOf())
    await p
    expect(toast()?.text).toContain('в сеть не обращается')
  })

  it('отзыв без загрузки не гасит «Обновление отменено» следующей загрузки', async () => {
    api.SetLibraryNet.mockResolvedValue(netOf({ allowed: true }))
    await net.revoke()
    let fail: (e: unknown) => void = () => {}
    api.UpdateLibrary.mockReturnValue(new Promise((_, r) => (fail = r)))
    net.update()
    fail('обновление библиотеки отменено')
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(toast()?.text).toBe('Обновление отменено')
  })

  it('окно закрыто: тост «Скачиваем…» в начале и итог в конце, список не читается', async () => {
    packsUi.close()
    api.LibraryPacks.mockClear()
    api.UpdateLibrary.mockResolvedValue(upd({ updated: 2 }))
    await net.run()
    expect(api.LibraryPacks).not.toHaveBeenCalled()
    expect(toast()?.text).toBe('Библиотека обновлена: обновлено наборов 2')
  })
})

const none = { count: 0, bytes: 0 }

describe('сеть: library:start', () => {
  it('только обновлено — обычный тост без кнопки', () => {
    net.started({ updated: 2, offer: none, fresh: none })
    expect(toast()).toMatchObject({ text: 'Обновлено наборов: 2', kind: 'info' })
    expect(toast()?.action).toBeUndefined()
  })

  it('нечего сказать — тоста нет', () => {
    net.started({ updated: 0, offer: none, fresh: none })
    expect(toast()).toBeNull()
  })

  it('обновления и новые вместе — один тост, кнопка «Скачать» (приоритет предложения)', () => {
    net.started({ updated: 1, offer: { count: 1, bytes: 120 * MB }, fresh: { count: 2, bytes: 160 * MB } })
    expect(toast()?.text).toBe('Обновлено наборов: 1; есть обновления наборов: 1 (120,0 МБ); есть новые наборы: 2 (160,0 МБ)')
    expect(toast()?.action?.label).toBe('Скачать')
  })

  it('только новые — кнопка «Показать» открывает окно «Наборы» с прокруткой к разделу', async () => {
    packsUi.close()
    net.started({ updated: 0, offer: none, fresh: { count: 2, bytes: 160 * MB } })
    expect(toast()).toMatchObject({ text: 'Есть новые наборы: 2 (160,0 МБ)', kind: 'info' })
    expect(toast()?.action?.label).toBe('Показать')
    toast()?.action?.run()
    await vi.waitFor(() => expect(packsUi.reveal).toBe(true))
    expect(packsUi.open).toBe(true)
    expect(api.UpdateLibrary).not.toHaveBeenCalled()
  })

  it('«Показать» при открытом окне не перечитывает его заново', async () => {
    api.LibraryPacks.mockClear()
    net.started({ updated: 0, offer: none, fresh: { count: 1, bytes: MB } })
    toast()?.action?.run()
    await vi.waitFor(() => expect(packsUi.reveal).toBe(true))
    expect(api.LibraryPacks).not.toHaveBeenCalled()
  })
})

describe('сеть: предложение при запуске и чужая операция', () => {
  const offer = (count: number) => net.started({ updated: 0, offer: { count, bytes: 120 * MB }, fresh: none })

  it('предложение: тост с «Скачать», кнопка зовёт UpdateLibrary без вопроса', async () => {
    offer(2)
    expect(toast()).toMatchObject({ text: 'Есть обновления наборов: 2 (120,0 МБ)', kind: 'info' })
    expect(toast()?.action?.label).toBe('Скачать')
    api.UpdateLibrary.mockResolvedValue(upd({ added: 2 }))
    toast()?.action?.run()
    await vi.waitFor(() => expect(toast()?.text).toBe('Библиотека обновлена: новых наборов 2'))
    expect(api.UpdateLibrary).toHaveBeenCalledTimes(1)
    expect(api.SetLibraryNet).not.toHaveBeenCalled()
  })

  it('«Скачать» из тоста при только чтении: подсказка про новую версию, Go не зовётся', async () => {
    offer(1)
    const act = toast()?.action
    packsUi.readOnly = true
    act?.run()
    packsUi.readOnly = false
    expect(toast()?.text).toMatch(/более новая версия программы/)
    expect(api.UpdateLibrary).not.toHaveBeenCalled()
  })

  it('«Скачать» из тоста, пока Go занят: молча, без красного тоста и без вызова Go', () => {
    offer(1)
    const act = toast()?.action
    packsUi.goBusy = true
    act?.run()
    packsUi.goBusy = false
    expect(toast()?.kind).toBe('info')
    expect(toast()?.text).toBe('Есть обновления наборов: 1 (120,0 МБ)')
    expect(api.UpdateLibrary).not.toHaveBeenCalled()
  })

  it('чужая загрузка (проверка при запуске): окно серое, полоса видна, окно следит и отпускает', async () => {
    vi.useFakeTimers()
    api.LibraryPacks.mockResolvedValue(list({ busy: true }))
    net.progressed({ stage: 'download', title: 'Айвори', done: 1, total: 2 })
    expect(packsUi.goBusy).toBe(true)
    expect(net.progress).not.toBeNull()
    expect(net.updating).toBe(false)
    api.LibraryPacks.mockResolvedValue(list())
    await vi.advanceTimersByTimeAsync(2100)
    expect(packsUi.goBusy).toBe(false)
    expect(net.progress).toBeNull()
  })

  it('ход при закрытом окне игнорируется', () => {
    packsUi.close()
    net.progressed({ stage: 'download', title: 'x', done: 1, total: 2 })
    expect(net.progress).toBeNull()
    expect(packsUi.goBusy).toBe(false)
  })
})

describe('сеть: пропуск удалённых наборов («Вернуть»)', () => {
  const withSkip = (n: number, o: Record<string, unknown> = {}) => list({ net: netOf({ allowed: true, skipped: n }), ...o })

  it('skipLine: число в скобках', () => {
    expect(skipLine(2)).toBe('Удалённые вами наборы «Скачать все» пропускает (2)')
  })

  it('строка нужна только при skipped > 0: число из LibraryPacks().net, у старого Go — 0', async () => {
    expect(net.net.skipped).toBe(0)
    api.LibraryPacks.mockResolvedValue(withSkip(2))
    await packsUi.load()
    expect(net.net.skipped).toBe(2)
    api.LibraryPacks.mockResolvedValue(list({ net: { allowed: true, atStart: false, checked: 0 } }))
    await packsUi.load()
    expect(net.net.skipped).toBe(0)
  })

  it('«Вернуть»: Go зовётся, net обновляется из ответа, тост', async () => {
    api.LibraryPacks.mockResolvedValue(withSkip(2))
    await packsUi.load()
    api.ResetLibrarySkip.mockResolvedValue(netOf({ allowed: true, skipped: 0 }))
    await net.resetSkip()
    expect(api.ResetLibrarySkip).toHaveBeenCalledTimes(1)
    expect(net.net).toMatchObject({ allowed: true, skipped: 0 })
    expect(toast()).toMatchObject({ text: 'Пометка снята: «Скачать все» снова ставит эти наборы', kind: 'info' })
  })

  it('ошибка «Вернуть» — красный тост, число прежнее', async () => {
    api.LibraryPacks.mockResolvedValue(withSkip(2))
    await packsUi.load()
    api.ResetLibrarySkip.mockRejectedValue('диск только для чтения')
    await net.resetSkip()
    expect(toast()).toMatchObject({ text: 'Диск только для чтения', kind: 'error' })
    expect(net.net.skipped).toBe(2)
  })

  it('занято или только чтение: Go не зовётся', async () => {
    api.LibraryPacks.mockResolvedValue(withSkip(1))
    await packsUi.load()
    packsUi.goBusy = true
    await net.resetSkip()
    packsUi.goBusy = false
    expect(api.ResetLibrarySkip).not.toHaveBeenCalled()
    api.LibraryPacks.mockResolvedValue(withSkip(1, { readOnly: true }))
    await packsUi.load()
    await net.resetSkip()
    expect(api.ResetLibrarySkip).not.toHaveBeenCalled()
  })
})

describe('сеть: «Скачать и поставить» из галереи (144)', () => {
  const pick = (o: Record<string, unknown> = {}) => upd({ added: 1, picked: true, ...o })
  beforeEach(async () => {
    api.LibraryPacks.mockResolvedValue(list({ net: netOf({ allowed: true }) }))
    await packsUi.load()
  })

  it('fetchOne возвращает итог Go и показывает тост «Скачать» из окна «Наборы»', async () => {
    const r = pick()
    api.DownloadLibraryPack.mockResolvedValue(r)
    expect(await net.fetchOne({ id: 'school', title: 'Школа — осень' })).toBe(r)
    expect(api.DownloadLibraryPack).toHaveBeenCalledWith('school')
    expect(toast()).toMatchObject({ text: 'Набор «Школа — осень» скачан', kind: 'info' })
    expect(net.updating).toBe(false)
    expect(net.progress).toBeNull()
  })

  it('ход виден в галерее: окно «Наборы» закрыто, тост «Скачиваем наборы…» не нужен', async () => {
    packsUi.close()
    let done: (v: unknown) => void = () => {}
    api.DownloadLibraryPack.mockReturnValue(new Promise((r) => (done = r)))
    const p = net.fetchOne({ id: 'school', title: 'Школа' })
    expect(net.updating).toBe(true)
    expect(toast()).toBeNull()
    done(pick())
    await p
    expect(toast()?.text).toBe('Набор «Школа» скачан')
  })

  it('не встал: итог возвращается с picked=false, тост с причиной Go', async () => {
    api.DownloadLibraryPack.mockResolvedValue(upd({ picked: false, why: 'нужна новая версия программы' }))
    const r = await net.fetchOne({ id: 'school', title: 'Школа' })
    expect(r?.picked).toBe(false)
    expect(toast()?.text).toBe('Набор «Школа» не поставлен: нужна новая версия программы')
  })

  it('отмена и сбой — null; отмена обычным тостом, сбой красным', async () => {
    api.DownloadLibraryPack.mockRejectedValueOnce('обновление библиотеки отменено')
    expect(await net.fetchOne({ id: 'a', title: 'А' })).toBeNull()
    expect(toast()).toMatchObject({ text: 'Обновление отменено' })
    api.DownloadLibraryPack.mockRejectedValueOnce('нет связи с github.com')
    expect(await net.fetchOne({ id: 'a', title: 'А' })).toBeNull()
    expect(toast()).toMatchObject({ text: 'Нет связи с github.com', kind: 'error' })
  })

  it('идёт другой ход: второго запроса к Go нет, тост «Библиотека обновляется»', async () => {
    let done: (v: unknown) => void = () => {}
    api.DownloadLibraryPack.mockReturnValue(new Promise((r) => (done = r)))
    const first = net.fetchOne({ id: 'a', title: 'А' })
    expect(await net.fetchOne({ id: 'b', title: 'Б' })).toBeNull()
    expect(api.DownloadLibraryPack).toHaveBeenCalledTimes(1)
    expect(toast()?.text).toBe('Библиотека обновляется')
    done(pick())
    await first
  })

  it('текст согласия называет ролики и образцы (0.31-144-21)', () => {
    expect(CONSENT_TEXT).toBe(
      'Программа скачает список наборов и роликов, картинки-образцы наборов, новые наборы и выбранное вами с github.com. Отправляется только обычный запрос: альбомы, фото и сведения о вас не передаются.',
    )
  })
})
