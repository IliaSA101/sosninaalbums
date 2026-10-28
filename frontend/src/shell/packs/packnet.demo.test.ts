import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  DEMO_BACK_MORE,
  DEMO_BACK_TEXT,
  DEMO_RESET_HINT,
  DEMO_SKIP_LINE,
  SKIP_RESET_HINT,
  resetText,
  skipHint,
  skipLine,
  skipText,
} from './packnet'

// Пример темы (142) в сети окна «Наборы»: строка пропуска, тост «Пример вернулся», занятая служба. Go подменён.
const api = vi.hoisted(() => ({
  LibraryPacks: vi.fn(),
  SetLibraryNet: vi.fn(),
  UpdateLibrary: vi.fn(),
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
afterEach(() => packsUi.close())

describe('сеть: пример темы («Вернуть»)', () => {
  const withSkip = (n: number, o: Record<string, unknown> = {}) => list({ net: netOf({ allowed: true, skipped: n, ...o }) })

  it('тексты: удалён только пример — своя фраза и подсказка; с другими — прежняя строка с числом', () => {
    expect(DEMO_SKIP_LINE).toBe('Пример в программе удалён вами')
    expect(DEMO_BACK_TEXT).toBe('Пример вернулся')
    expect(DEMO_BACK_MORE).toBe('Пример вернулся, «Скачать все» снова ставит остальные наборы')
    expect(skipText({ skipped: 1, demoSkipped: true })).toBe(DEMO_SKIP_LINE)
    expect(skipText({ skipped: 2, demoSkipped: true })).toBe(skipLine(2))
    expect(skipText({ skipped: 1, demoSkipped: false })).toBe(skipLine(1))
    expect(skipText({ skipped: 1 })).toBe(skipLine(1)) // старый Go без поля
    expect(skipHint({ skipped: 1, demoSkipped: true })).toBe(DEMO_RESET_HINT)
    expect(skipHint({ skipped: 3, demoSkipped: true })).toBe(SKIP_RESET_HINT)
  })

  it('resetText: пример встал один — «Пример вернулся»; были и другие — с оговоркой; не встал — прежний', () => {
    expect(resetText({ skipped: 1 }, { demoBack: true })).toBe(DEMO_BACK_TEXT)
    expect(resetText({ skipped: 2 }, { demoBack: true })).toBe(DEMO_BACK_MORE)
    expect(resetText({ skipped: 2 }, { demoBack: false })).toBe('Пометка снята: «Скачать все» снова ставит эти наборы')
    expect(resetText({ skipped: 1 }, {})).toBe('Пометка снята: «Скачать все» снова ставит эти наборы')
  })

  it('demoSkipped, demoOffer приходят из LibraryPacks().net; у старого Go — ложь', async () => {
    api.LibraryPacks.mockResolvedValue(withSkip(1, { demoSkipped: true, demoOffer: true }))
    await packsUi.load()
    expect(net.net).toMatchObject({ skipped: 1, demoSkipped: true, demoOffer: true })
    api.LibraryPacks.mockResolvedValue(withSkip(0))
    await packsUi.load()
    expect(net.net.demoSkipped).toBeFalsy()
    expect(net.net.demoOffer).toBeFalsy()
  })

  it('«Вернуть» при удалённом одном примере: demoBack — тост «Пример вернулся», пропуск снят', async () => {
    api.LibraryPacks.mockResolvedValue(withSkip(1, { demoSkipped: true }))
    await packsUi.load()
    api.ResetLibrarySkip.mockResolvedValue(netOf({ allowed: true, skipped: 0, demoBack: true }))
    await net.resetSkip()
    expect(toast()).toMatchObject({ text: 'Пример вернулся', kind: 'info' })
    expect(net.net).toMatchObject({ skipped: 0, demoBack: true })
  })

  it('«Вернуть» с примером и другими: тост про «Скачать все» для остальных', async () => {
    api.LibraryPacks.mockResolvedValue(withSkip(3, { demoSkipped: true }))
    await packsUi.load()
    api.ResetLibrarySkip.mockResolvedValue(netOf({ allowed: true, skipped: 0, demoBack: true }))
    await net.resetSkip()
    expect(toast()?.text).toBe('Пример вернулся, «Скачать все» снова ставит остальные наборы')
  })

  it('«Вернуть» без файла примера: demoBack нет — прежний тост, строка «Скачать» вернётся с перечитанным списком', async () => {
    api.LibraryPacks.mockResolvedValue(withSkip(1, { demoSkipped: true }))
    await packsUi.load()
    api.ResetLibrarySkip.mockResolvedValue(netOf({ allowed: true, skipped: 0, demoOffer: true }))
    api.LibraryPacks.mockClear()
    await net.resetSkip()
    expect(toast()?.text).toBe('Пометка снята: «Скачать все» снова ставит эти наборы')
    expect(net.net.demoOffer).toBe(true)
    expect(api.LibraryPacks).toHaveBeenCalled()
  })

  it('служба занята: ошибка Go «библиотека уже обновляется» — красный тост с заглавной, пропуск прежний, список перечитан', async () => {
    api.LibraryPacks.mockResolvedValue(withSkip(1, { demoSkipped: true }))
    await packsUi.load()
    api.ResetLibrarySkip.mockRejectedValue('библиотека уже обновляется')
    api.LibraryPacks.mockClear()
    await net.resetSkip()
    expect(toast()).toMatchObject({ text: 'Библиотека уже обновляется', kind: 'error' })
    expect(net.net).toMatchObject({ skipped: 1, demoSkipped: true })
    expect(api.LibraryPacks).toHaveBeenCalled()
  })

  it('повторное «Вернуть» после ответа: второй тост не «Пример вернулся» (demoBack — только в ответе того вызова)', async () => {
    api.LibraryPacks.mockResolvedValue(withSkip(1, { demoSkipped: true }))
    await packsUi.load()
    api.ResetLibrarySkip.mockResolvedValueOnce(netOf({ allowed: true, skipped: 0, demoBack: true }))
    await net.resetSkip()
    api.ResetLibrarySkip.mockResolvedValueOnce(netOf({ allowed: true, skipped: 0 }))
    await net.resetSkip()
    expect(toast()?.text).toBe('Пометка снята: «Скачать все» снова ставит эти наборы')
  })
})
