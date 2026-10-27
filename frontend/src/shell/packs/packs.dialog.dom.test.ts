// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'

// Окно «Наборы»: описание набора обрезано line-clamp: 2, полный текст — в title (design 129 §6, Т8).
const api = vi.hoisted(() => ({ LibraryPacks: vi.fn(), OpenLink: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})

const { default: PacksDialog } = await import('./PacksDialog.svelte')
const { packsUi } = await import('./packs.svelte')

const LONG =
  'Нарисовано ИИ (Magnific), отобрано и обработано вручную. Фоны, рамочки, уголки и стикеры двух стилей. Школа, осень, листья, клён, жёлуди и ещё много всего про осень.'
const info = (o: Record<string, unknown> = {}) => ({
  id: 'school',
  title: 'Школа — осень',
  about: LONG,
  author: 'Марина',
  license: 'CC0-1.0',
  source: '',
  origin: 'file',
  version: 1,
  bg: 1,
  sticker: 0,
  spread: 0,
  frame: 0,
  print: 0,
  bytes: 1024,
  removable: true,
  ...o,
})
const list = (packs: unknown[], skipped = 0) => ({
  packs,
  net: { allowed: false, atStart: false, checked: 0, skipped },
  busy: false,
  readOnly: false,
})
const abouts = () => Array.from(document.querySelectorAll<HTMLElement>('.about'))

beforeEach(() => {
  api.LibraryPacks.mockReset()
  api.OpenLink.mockReset()
  packsUi.close()
})
afterEach(() => {
  cleanup()
  packsUi.close()
})

describe('PacksDialog: описание набора', () => {
  it('длинное описание несёт полный текст в title', async () => {
    api.LibraryPacks.mockResolvedValue(list([info()]))
    render(PacksDialog)
    await packsUi.show()
    await vi.waitFor(() => expect(abouts()).toHaveLength(1))
    expect(abouts()[0].textContent).toBe(LONG)
    expect(abouts()[0].getAttribute('title')).toBe(LONG)
  })

  it('пустое описание — строки и title нет; у каждого набора свой title', async () => {
    api.LibraryPacks.mockResolvedValue(list([info({ id: 'a', about: '' }), info({ id: 'b', about: 'Коротко' })]))
    render(PacksDialog)
    await packsUi.show()
    await vi.waitFor(() => expect(abouts()).toHaveLength(1))
    expect(abouts()[0].getAttribute('title')).toBe('Коротко')
  })
})

describe('PacksDialog: строка «Об авторах» (129 §6)', () => {
  const line = () => Array.from(document.querySelectorAll<HTMLElement>('.skip')).find((p) => p.textContent?.includes('Наборы каталога'))
  const open = async (packs: unknown[], skipped = 0) => {
    api.LibraryPacks.mockResolvedValue(list(packs, skipped))
    render(PacksDialog)
    await packsUi.show()
    await vi.waitFor(() => expect(document.querySelector('.list')).not.toBeNull())
  }

  it('без наборов из сети строки нет (свой файл и встроенный)', async () => {
    await open([info({ origin: 'file' }), info({ id: 'b', origin: 'builtin', removable: false })])
    expect(line()).toBeUndefined()
  })

  it('с одним сетевым набором — подпись и ссылка «Об авторах»', async () => {
    await open([info({ id: 'a', origin: 'file' }), info({ id: 'n', origin: 'net' })])
    expect(line()?.textContent?.replace(/\s+/g, ' ').trim()).toBe('Наборы каталога: Илья и Марина Соснины · Об авторах')
    expect(line()?.querySelector('button.link')?.textContent).toBe('Об авторах')
  })

  it('щелчок по «Об авторах» зовёт OpenLink("authors")', async () => {
    await open([info({ origin: 'net' })])
    line()?.querySelector<HTMLButtonElement>('button.link')?.click()
    await vi.waitFor(() => expect(api.OpenLink).toHaveBeenCalledWith('authors'))
    expect(api.OpenLink).toHaveBeenCalledTimes(1)
  })

  it('порядок: список, «Наборы каталога…», затем «Вернуть удалённые»', async () => {
    await open([info({ origin: 'net' })], 2)
    const ps = Array.from(document.querySelectorAll('.skip'))
    expect(ps).toHaveLength(2)
    expect(ps[0].textContent).toContain('Наборы каталога')
    expect(ps[1].textContent).toContain('Вернуть')
    expect(document.querySelector('.list')?.compareDocumentPosition(ps[0])).toBe(Node.DOCUMENT_POSITION_FOLLOWING)
  })

  it('«Вернуть удалённые» без сетевых наборов — одна строка, подписи каталога нет', async () => {
    await open([info({ origin: 'file' })], 1)
    expect(document.querySelectorAll('.skip')).toHaveLength(1)
    expect(line()).toBeUndefined()
  })
})
