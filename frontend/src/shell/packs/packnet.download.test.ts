import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { consentLabel, failedLabel, oneText, oneTone, startAction, updateText, updateTone } from './packnet'

// 137: итоги с неудачами, «Скачать» в строке и «Скачать все». Go подменён; общие помощники — как в packnet.test.ts.
const api = vi.hoisted(() => ({
  LibraryPacks: vi.fn(),
  SetLibraryNet: vi.fn(),
  UpdateLibrary: vi.fn(),
  DownloadLibraryPack: vi.fn(),
  DownloadAllLibrary: vi.fn(),
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
  packsUi.reveal = false
  store.toast = null
  await packsUi.show()
})
afterEach(() => {
  vi.useRealTimers()
  packsUi.close()
})

// ---- 137: «Скачать», «Скачать все», итоги с неудачами ----

const sk = (name: string, why: string) => ({ name, why })
const ctxOpen = { open: true, noNet: false }

describe('updateText: неудачи, «можно скачать», новые (137)', () => {
  it('неудача одного: красный тон, «не скачались: N (имя — причина)» после головы', () => {
    const r = upd({ added: 2, failed: [sk('Школа', 'скачался с ошибкой, повторите')] })
    expect(updateText(r, ctxOpen)).toBe('Библиотека обновлена: новых наборов 2; не скачались: 1 (Школа — скачался с ошибкой, повторите)')
    expect(updateTone(r)).toBe('error')
    expect(updateTone(upd({ added: 1 }))).toBe('info')
    expect(updateTone(upd({ failed: null }))).toBe('info')
  })

  it('имена неудач — не больше трёх, остальное «и ещё N»', () => {
    const failed = [sk('A', 'x'), sk('B', 'y'), sk('C', 'z'), sk('D', 'w'), sk('E', 'v')]
    expect(failedLabel(failed)).toBe('не скачались: 5 (A — x; B — y; C — z и ещё 2)')
    expect(failedLabel([])).toBe('')
    expect(failedLabel(null)).toBe('')
  })

  it('ничего не встало, но есть неудача: «Всё уже свежее» не пишем, фраза с заглавной', () => {
    const r = upd({ failed: [sk('Школа', 'библиотека заполнена (2 ГБ)')] })
    expect(updateText(r, ctxOpen)).toBe('Не скачались: 1 (Школа — библиотека заполнена (2 ГБ))')
  })

  it('«можно скачать»: после обновления и после «Всё уже свежее»; ниже — окно открыто, иначе — в окне «Наборы»', () => {
    expect(updateText(upd({ updated: 1, available: 3 }), ctxOpen)).toBe(
      'Библиотека обновлена: обновлено наборов 1; можно скачать: 3 — ниже',
    )
    expect(updateText(upd({ available: 3 }), ctxOpen)).toBe('Всё уже свежее; можно скачать: 3 — ниже')
    expect(updateText(upd({ available: 3 }), { open: false, noNet: false })).toBe(
      'Всё уже свежее; можно скачать: 3 — список в окне «Наборы»',
    )
    expect(updateText(upd({ available: 0 }), ctxOpen)).toBe('Всё уже свежее')
  })

  it('новичок (наборов из сети нет): «Список наборов получен»; без «можно скачать» — «Всё уже свежее»', () => {
    expect(updateText(upd({ available: 12 }), { open: true, noNet: true })).toBe('Список наборов получен: можно скачать 12 — ниже')
    expect(updateText(upd({ available: 12 }), { open: false, noNet: true })).toBe(
      'Список наборов получен: можно скачать 12 — список в окне «Наборы»',
    )
    expect(updateText(upd({ available: 0 }), { open: true, noNet: true })).toBe('Всё уже свежее')
  })

  it('«Скачать все»: появились новые — в конце; у «Обновить» этой фразы нет (новые и так в «можно скачать»)', () => {
    const r = upd({ added: 2, available: 1, fresh: 3 })
    expect(updateText(r, { open: true, noNet: false, kind: 'all' })).toBe(
      'Библиотека обновлена: новых наборов 2; можно скачать: 1 — ниже; появились новые: 3 — список обновлён',
    )
    expect(updateText(r, { open: true, noNet: false, kind: 'update' })).not.toContain('появились')
    expect(updateText(upd({ added: 1, fresh: 0 }), { open: true, noNet: false, kind: 'all' })).not.toContain('появились')
  })

  it('пропущенное идёт перед неудачами; поля старого Go (нет failed/available/fresh) не мешают', () => {
    const r = upd({ added: 1, skipped: [sk('Новый', 'нужна новая версия программы')], failed: [sk('Б', 'в')] })
    expect(updateText(r, ctxOpen)).toBe(
      'Библиотека обновлена: новых наборов 1; пропущено: 1 (Новый — нужна новая версия программы); не скачались: 1 (Б — в)',
    )
    expect(updateText({ added: 0, updated: 0, skipped: [], bytes: 0 }, ctxOpen)).toBe('Всё уже свежее')
  })
})

describe('oneText: итог «Скачать» в строке (137)', () => {
  const one = (o: Record<string, unknown> = {}) => upd({ added: 1, picked: true, ...o })

  it('встал: «скачан»; с дополнениями — их число (added − 1)', () => {
    expect(oneText(one(), 'Школа')).toBe('Набор «Школа» скачан')
    expect(oneText(one({ added: 3 }), 'Школа')).toBe('Набор «Школа» скачан, дополнений: 2')
    expect(oneTone(one())).toBe('info')
  })

  it('added = 0 при picked — без «дополнений: -1»', () => {
    expect(oneText(one({ added: 0 }), 'Школа')).toBe('Набор «Школа» скачан')
  })

  it('встал, но дополнение не скачалось: красный тон и «не скачались»', () => {
    const r = one({ added: 1, failed: [sk('Свадьба: макеты', 'скачался с ошибкой, повторите')] })
    expect(oneText(r, 'Свадьба')).toBe('Набор «Свадьба» скачан; не скачались: 1 (Свадьба: макеты — скачался с ошибкой, повторите)')
    expect(oneTone(r)).toBe('error')
  })

  it('не встал со сбоем: «не поставлен: причина» от Go, красный; причины нет — без двоеточия, тон обычный', () => {
    const r = upd({ picked: false, why: 'библиотека заполнена (2 ГБ)', failed: [sk('Школа', 'библиотека заполнена (2 ГБ)')] })
    expect(oneText(r, 'Школа')).toBe('Набор «Школа» не поставлен: библиотека заполнена (2 ГБ)')
    expect(oneTone(r)).toBe('error')
    expect(oneText(upd({ picked: false }), 'Школа')).toBe('Набор «Школа» не поставлен')
    expect(oneTone(upd({ picked: false, why: 'уже стоит' }))).toBe('info')
  })
})

describe('слова согласия и кнопка тоста запуска (137)', () => {
  it('кнопка вопроса говорит, что будет после', () => {
    expect(consentLabel({ kind: 'update' })).toBe('Разрешить и обновить')
    expect(consentLabel({ kind: 'all' })).toBe('Разрешить и скачать')
    expect(consentLabel({ kind: 'one', id: 'a', title: 'A' })).toBe('Разрешить и скачать')
  })

  it('кнопка тоста запуска: предложение главнее новых; ничего — без кнопки', () => {
    const o = { count: 1, bytes: 1 }
    const z = { count: 0, bytes: 0 }
    expect(startAction({ updated: 0, offer: o, fresh: o })).toBe('download')
    expect(startAction({ updated: 0, offer: z, fresh: o })).toBe('show')
    expect(startAction({ updated: 5, offer: z, fresh: z })).toBeNull()
  })
})

describe('сеть: «Скачать» и «Скачать все» (137)', () => {
  const row = { id: 'school', title: 'Школа', why: '' }
  beforeEach(async () => {
    api.LibraryPacks.mockResolvedValue(list({ net: netOf({ allowed: true }) }))
    await packsUi.load()
  })

  it('«Скачать» в строке зовёт DownloadLibraryPack(id), итог — тост «скачан»', async () => {
    api.DownloadLibraryPack.mockResolvedValue(upd({ added: 1, picked: true }))
    net.download(row)
    expect(net.updating).toBe(true)
    await vi.waitFor(() => expect(toast()?.text).toBe('Набор «Школа» скачан'))
    expect(api.DownloadLibraryPack).toHaveBeenCalledWith('school')
    expect(api.UpdateLibrary).not.toHaveBeenCalled()
    expect(toast()?.kind).toBe('info')
  })

  it('не встал без сбоя — обычный тост с причиной; список перечитан', async () => {
    api.DownloadLibraryPack.mockResolvedValue(upd({ picked: false, why: 'нужна версия Sosnina Albums 0.30.0 или новее' }))
    api.LibraryPacks.mockClear()
    net.download(row)
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(toast()).toMatchObject({ text: 'Набор «Школа» не поставлен: нужна версия Sosnina Albums 0.30.0 или новее', kind: 'info' })
    expect(api.LibraryPacks).toHaveBeenCalled()
  })

  it('набора больше нет: текст Go с заглавной, обычный тост (не сбой), раздел перечитан', async () => {
    api.DownloadLibraryPack.mockRejectedValue('набора больше нет в списке — список обновлён')
    api.LibraryPacks.mockClear()
    net.download(row)
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(toast()).toMatchObject({ text: 'Набора больше нет в списке — список обновлён', kind: 'info' })
    expect(api.LibraryPacks).toHaveBeenCalled()
  })

  it('серая строка (why) Go не зовёт; занято и только чтение — тоже', () => {
    net.download({ ...row, why: 'нужна версия 0.30.0' })
    packsUi.goBusy = true
    net.download(row)
    packsUi.goBusy = false
    packsUi.readOnly = true
    net.download(row)
    packsUi.readOnly = false
    expect(api.DownloadLibraryPack).not.toHaveBeenCalled()
    expect(net.consent).toBe(false)
  })

  it('второй щелчок во время хода — второго вызова нет', async () => {
    let done: (v: unknown) => void = () => {}
    api.DownloadLibraryPack.mockReturnValue(new Promise((r) => (done = r)))
    net.download(row)
    net.download({ id: 'b', title: 'B', why: '' })
    net.downloadAll()
    expect(api.DownloadLibraryPack).toHaveBeenCalledTimes(1)
    expect(api.DownloadAllLibrary).not.toHaveBeenCalled()
    done(upd({ added: 1, picked: true }))
    await vi.waitFor(() => expect(net.updating).toBe(false))
  })

  it('без согласия: вопрос с «Разрешить и скачать»; после согласия — именно то, что нажали', async () => {
    api.LibraryPacks.mockResolvedValue(list())
    await packsUi.load()
    net.download(row)
    expect(net.consent).toBe(true)
    expect(consentLabel(net.pending)).toBe('Разрешить и скачать')
    expect(api.DownloadLibraryPack).not.toHaveBeenCalled()
    api.SetLibraryNet.mockResolvedValue(netOf({ allowed: true }))
    api.DownloadLibraryPack.mockResolvedValue(upd({ added: 1, picked: true }))
    await net.confirmConsent()
    expect(api.SetLibraryNet).toHaveBeenCalledWith(true, false)
    expect(api.DownloadLibraryPack).toHaveBeenCalledWith('school')
    expect(api.UpdateLibrary).not.toHaveBeenCalled()
  })

  it('«Скачать все» без согласия: тот же вопрос, после — DownloadAllLibrary; «Обновить» после — снова «обновить»', async () => {
    api.LibraryPacks.mockResolvedValue(list())
    await packsUi.load()
    net.downloadAll()
    expect(consentLabel(net.pending)).toBe('Разрешить и скачать')
    net.dismissConsent()
    net.update()
    expect(consentLabel(net.pending)).toBe('Разрешить и обновить')
    net.dismissConsent()
    net.downloadAll()
    api.SetLibraryNet.mockResolvedValue(netOf({ allowed: true }))
    api.DownloadAllLibrary.mockResolvedValue(upd())
    await net.confirmConsent()
    expect(api.DownloadAllLibrary).toHaveBeenCalledTimes(1)
  })

  it('согласие флажком «Проверять при запуске» после себя обновляет, а не скачивает', async () => {
    api.LibraryPacks.mockResolvedValue(list())
    await packsUi.load()
    net.download(row)
    net.dismissConsent()
    await net.setStart(true)
    expect(consentLabel(net.pending)).toBe('Разрешить и обновить')
  })

  it('«Скачать все»: итог с «можно скачать» и новыми, красный при неудаче; окно прокручивается к разделу', async () => {
    api.DownloadAllLibrary.mockResolvedValue(
      upd({ added: 2, available: 1, fresh: 1, failed: [sk('Школа', 'скачался с ошибкой, повторите')] }),
    )
    net.downloadAll()
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(toast()).toMatchObject({
      text: 'Библиотека обновлена: новых наборов 2; не скачались: 1 (Школа — скачался с ошибкой, повторите); можно скачать: 1 — ниже; появились новые: 1 — список обновлён',
      kind: 'error',
    })
    expect(packsUi.reveal).toBe(true)
  })

  it('«Обновить» без чего скачать — прокрутки нет; «Скачать» одного — тоже', async () => {
    api.UpdateLibrary.mockResolvedValue(upd({ available: 0 }))
    net.update()
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(packsUi.reveal).toBe(false)
    api.DownloadLibraryPack.mockResolvedValue(upd({ added: 1, picked: true, available: 4 }))
    net.download(row)
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(packsUi.reveal).toBe(false)
  })

  it('«Обновить» нашёл, что скачать: окно прокручивается к разделу', async () => {
    api.UpdateLibrary.mockResolvedValue(upd({ updated: 1, available: 2 }))
    net.update()
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(packsUi.reveal).toBe(true)
  })

  it('окно закрыто: итог — «в окне «Наборы»», прокрутки нет', async () => {
    api.LibraryPacks.mockResolvedValue(
      list({ net: netOf({ allowed: true }), packs: [builtin, { ...builtin, id: 'n', title: 'Сеть', origin: 'net', removable: true }] }),
    )
    await packsUi.load()
    packsUi.close()
    api.UpdateLibrary.mockResolvedValue(upd({ available: 3 }))
    await net.run()
    expect(toast()?.text).toBe('Всё уже свежее; можно скачать: 3 — список в окне «Наборы»')
    expect(packsUi.reveal).toBe(false)
  })

  it('новичок: сетевых наборов нет — «Список наборов получен»', async () => {
    api.UpdateLibrary.mockResolvedValue(upd({ available: 12 }))
    net.update()
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(toast()?.text).toBe('Список наборов получен: можно скачать 12 — ниже')
  })

  it('есть набор из сети — «Всё уже свежее; можно скачать»', async () => {
    api.LibraryPacks.mockResolvedValue(
      list({ net: netOf({ allowed: true }), packs: [builtin, { ...builtin, id: 'n', title: 'Сеть', origin: 'net', removable: true }] }),
    )
    await packsUi.load()
    api.UpdateLibrary.mockResolvedValue(upd({ available: 12 }))
    net.update()
    await vi.waitFor(() => expect(net.updating).toBe(false))
    expect(toast()?.text).toBe('Всё уже свежее; можно скачать: 12 — ниже')
  })

  it('«Вернуть» перечитывает раздел (пометки «удалён вами» сняты)', async () => {
    api.LibraryPacks.mockResolvedValue(list({ net: netOf({ allowed: true, skipped: 1 }) }))
    await packsUi.load()
    api.ResetLibrarySkip.mockResolvedValue(netOf({ allowed: true }))
    api.LibraryPacks.mockClear()
    await net.resetSkip()
    expect(api.LibraryPacks).toHaveBeenCalled()
  })
})
