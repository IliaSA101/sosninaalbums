// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'

// Окно «Наборы»: описание набора обрезано line-clamp: 2, полный текст — в title (design 129 §6, Т8).
const api = vi.hoisted(() => ({
  LibraryPacks: vi.fn(),
  OpenLink: vi.fn(),
  DownloadLibraryPack: vi.fn(),
  DownloadAllLibrary: vi.fn(),
  SetLibraryNet: vi.fn(),
  ResetLibrarySkip: vi.fn(),
}))
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
  api.DownloadLibraryPack.mockReset()
  api.DownloadAllLibrary.mockReset()
  api.SetLibraryNet.mockReset()
  api.ResetLibrarySkip.mockReset()
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

// Раздел «Можно скачать» (137): строки каталога под стоящими, «Скачать» в строке, «Скачать все», согласие, серые кнопки.
describe('PacksDialog: «Можно скачать» (137)', () => {
  const row = (o: Record<string, unknown> = {}) => ({
    id: 'school',
    title: 'Школа',
    about: 'Осень',
    license: 'CC0-1.0',
    version: 1,
    bg: 12,
    sticker: 30,
    spread: 0,
    frame: 0,
    print: 0,
    bytes: 41 * 1048576,
    addonTitles: [],
    donor: '',
    why: '',
    removed: false,
    ...o,
  })
  const withAvail = (available: unknown[], o: Record<string, unknown> = {}, allowed = true) => ({
    packs: [info({ id: 'builtin', origin: 'builtin', removable: false })],
    net: { allowed, atStart: false, checked: 0, skipped: 0 },
    busy: false,
    readOnly: false,
    available,
    all: { count: available.length, bytes: 41 * 1048576 },
    availAt: new Date(2026, 9, 27, 12).getTime(),
    ...o,
  })
  const open = async (l: unknown) => {
    api.LibraryPacks.mockResolvedValue(l)
    render(PacksDialog)
    await packsUi.show()
    await vi.waitFor(() => expect(document.querySelector('.list')).not.toBeNull())
  }
  const section = () => document.querySelector<HTMLElement>('section[aria-label="Можно скачать"]')
  const buttons = () => Array.from(section()?.querySelectorAll<HTMLButtonElement>('li button') ?? [])
  const text = (el: Element | null | undefined) => (el?.textContent ?? '').replace(/\s+/g, ' ').trim()

  it('строка: название, версия · состав · размер, лицензия, описание; подпись с датой и ссылка «Скачать все»', async () => {
    await open(withAvail([row()]))
    const li = section()?.querySelector('li')
    expect(text(li)).toContain('Школа')
    expect(text(li)).toContain('версия 1 · 12 фонов · 30 стикеров · 41,0 МБ')
    expect(text(li)).toContain('Лицензия: CC0-1.0')
    expect(li?.querySelector('.about')?.getAttribute('title')).toBe('Осень')
    expect(text(section()?.querySelector('.head'))).toBe('Можно скачать по списку от 27.10 Скачать все (1, 41,0 МБ)')
    expect(buttons()[0].getAttribute('title')).toBe('Скачать набор «Школа» (41,0 МБ)')
  })

  it('«Скачать» в строке зовёт DownloadLibraryPack(id); итог — тост, «Скачать все» не звали', async () => {
    api.DownloadLibraryPack.mockResolvedValue({ added: 1, updated: 0, skipped: [], bytes: 1, picked: true })
    await open(withAvail([row(), row({ id: 'wedding', title: 'Свадьба' })]))
    await fireEvent.click(buttons()[1])
    await vi.waitFor(() => expect(api.DownloadLibraryPack).toHaveBeenCalledWith('wedding'))
    expect(api.DownloadLibraryPack).toHaveBeenCalledTimes(1)
    expect(api.DownloadAllLibrary).not.toHaveBeenCalled()
  })

  it('без согласия «Скачать» открывает вопрос с «Разрешить и скачать»; «Отмена» — Go не звали', async () => {
    await open(withAvail([row()], {}, false))
    await fireEvent.click(buttons()[0])
    const dlg = Array.from(document.querySelectorAll('[role="dialog"]')).find(
      (d) => d.getAttribute('aria-label') === 'Обратиться к GitHub?',
    )
    expect(dlg).toBeTruthy()
    expect(Array.from(dlg!.querySelectorAll('button')).map((b) => text(b))).toContain('Разрешить и скачать')
    expect(api.DownloadLibraryPack).not.toHaveBeenCalled()
    await fireEvent.click(Array.from(dlg!.querySelectorAll('button')).find((b) => text(b) === 'Отмена')!)
    expect(api.DownloadLibraryPack).not.toHaveBeenCalled()
  })

  // Тот же вопрос в «Наборах» и у роликов F1 (NetConsent, 140): слова, флажок и порядок кнопок — общие
  it('вопрос согласия: заголовок, текст про наборы и ролики, флажок, «Отмена» слева — «Разрешить и скачать» справа; «Обновить» — «Разрешить и обновить»', async () => {
    await open(withAvail([row()], {}, false))
    await fireEvent.click(buttons()[0])
    const ask = () =>
      Array.from(document.querySelectorAll('[role="dialog"]')).find((d) => d.getAttribute('aria-label') === 'Обратиться к GitHub?')!
    expect(text(ask().querySelector('p'))).toBe(
      'Программа скачает список наборов и роликов, картинки-образцы наборов, новые наборы и выбранное вами с github.com. Отправляется только обычный запрос: альбомы, фото и сведения о вас не передаются.',
    )
    expect(text(ask().querySelector('label'))).toBe('Проверять при запуске (раз в день)')
    expect(Array.from(ask().querySelectorAll('.actions button')).map((b) => text(b))).toEqual(['Отмена', 'Разрешить и скачать'])
    await fireEvent.click(Array.from(ask().querySelectorAll('button')).find((b) => text(b) === 'Отмена')!)
    expect(document.querySelector('[aria-label="Обратиться к GitHub?"]')).toBeNull()

    await fireEvent.click(Array.from(document.querySelectorAll('button')).find((b) => text(b) === 'Обновить библиотеку')!)
    expect(Array.from(ask().querySelectorAll('.actions button')).map((b) => text(b))).toEqual(['Отмена', 'Разрешить и обновить'])
    await fireEvent.click(ask().querySelector('.actions button')!)
  })

  it('«Скачать все (N, M)» зовёт DownloadAllLibrary', async () => {
    api.DownloadAllLibrary.mockResolvedValue({ added: 1, updated: 0, skipped: [], bytes: 1 })
    await open(withAvail([row()]))
    await fireEvent.click(section()!.querySelector<HTMLButtonElement>('.head button')!)
    await vi.waitFor(() => expect(api.DownloadAllLibrary).toHaveBeenCalledTimes(1))
  })

  it('во время хода кнопки строк и «Скачать все» серые с подсказкой «Библиотека обновляется»', async () => {
    let done: (v: unknown) => void = () => {}
    api.DownloadLibraryPack.mockReturnValue(new Promise((r) => (done = r)))
    await open(withAvail([row(), row({ id: 'b', title: 'B' })]))
    await fireEvent.click(buttons()[0])
    await vi.waitFor(() => expect(buttons()[1].disabled).toBe(true))
    expect(buttons()[1].getAttribute('title')).toBe('Библиотека обновляется')
    const all = section()!.querySelector<HTMLButtonElement>('.head button')!
    expect(all.disabled).toBe(true)
    done({ added: 1, updated: 0, skipped: [], bytes: 1, picked: true })
    await vi.waitFor(() => expect(buttons()[1].disabled).toBe(false))
  })

  it('серая строка: кнопка отключена, причина — текстом в строке и подсказкой', async () => {
    await open(withAvail([row({ why: 'нужна версия Sosnina Albums 0.30.0 или новее' })]))
    expect(buttons()[0].disabled).toBe(true)
    expect(buttons()[0].getAttribute('title')).toBe('Нужна версия Sosnina Albums 0.30.0 или новее')
    expect(text(section()?.querySelector('li'))).toContain('Нужна версия Sosnina Albums 0.30.0 или новее')
    await fireEvent.click(buttons()[0])
    expect(api.DownloadLibraryPack).not.toHaveBeenCalled()
  })

  it('«удалён вами», «дополнение к», «вместе с» — во второй строке; кнопка активна', async () => {
    await open(withAvail([row({ removed: true, donor: 'Свадьба', addonTitles: ['Свадьба: макеты'], license: '' })]))
    expect(text(section()?.querySelector('li'))).toContain('дополнение к «Свадьба» · удалён вами · вместе с: «Свадьба: макеты»')
    expect(buttons()[0].disabled).toBe(false)
  })

  it('нет списка (availAt = 0): вместо раздела приглашение, строк и «Скачать все» нет', async () => {
    await open(withAvail([], { availAt: 0 }))
    expect(text(section())).toBe('Наборы из интернета появятся здесь после «Обновить библиотеку»')
    expect(section()?.querySelector('li')).toBeNull()
  })

  it('список есть, но нечего скачать и стоят одни встроенные — «Других наборов пока нет»; при сетевых — раздела нет', async () => {
    await open(withAvail([]))
    expect(text(section())).toContain('Других наборов пока нет')
    cleanup()
    packsUi.close()
    await open(withAvail([], { packs: [info({ origin: 'net' })] }))
    expect(section()).toBeNull()
  })

  it('«Показать»: окно прокручивается к разделу и просьба гаснет', async () => {
    const scroll = vi.fn()
    Element.prototype.scrollIntoView = scroll
    await open(withAvail([row()]))
    packsUi.reveal = true
    await vi.waitFor(() => expect(scroll).toHaveBeenCalledTimes(1))
    expect(packsUi.reveal).toBe(false)
  })

  it('просьба прокрутки при пустом разделе гаснет без прокрутки', async () => {
    const scroll = vi.fn()
    Element.prototype.scrollIntoView = scroll
    await open(withAvail([], { packs: [info({ origin: 'net' })] }))
    packsUi.reveal = true
    await vi.waitFor(() => expect(packsUi.reveal).toBe(false))
    expect(scroll).not.toHaveBeenCalled()
  })

  it('один блок прокрутки на оба раздела: стоящие и «Можно скачать» внутри .scroll', async () => {
    await open(withAvail([row()]))
    const sc = document.querySelector('.scroll')
    expect(sc?.querySelector('.list[aria-label="Наборы"]')).not.toBeNull()
    expect(sc?.contains(section())).toBe(true)
  })
})

// Пример темы (142): первая строка «Можно скачать» (и до первой проверки), удалён вами, согласие.
describe('PacksDialog: пример темы (142)', () => {
  const demoRow = () => ({
    id: 'wedding-navy-watercolor-demo',
    title: 'Свадьба — синяя акварель (пример)',
    about: 'Часть набора «Свадьба — синяя акварель» с рамками и макетом альбома. Пример уйдёт сам, когда встанет весь набор',
    license: 'CC0-1.0',
    version: 1,
    bg: 4,
    sticker: 7,
    spread: 4,
    frame: 2,
    print: 0,
    bytes: 17 * 1048576,
    addonTitles: [],
    donor: '',
    why: '',
    removed: false,
  })
  const l = (o: Record<string, unknown> = {}, net: Record<string, unknown> = {}) => ({
    packs: [info({ id: 'builtin', origin: 'builtin', removable: false })],
    net: { allowed: false, atStart: false, checked: 0, skipped: 0, demoOffer: true, ...net },
    busy: false,
    readOnly: false,
    available: [demoRow()],
    all: { count: 0, bytes: 0 },
    availAt: 0,
    ...o,
  })
  const open = async (v: unknown) => {
    api.LibraryPacks.mockResolvedValue(v)
    render(PacksDialog)
    await packsUi.show()
    await vi.waitFor(() => expect(document.querySelector('.list')).not.toBeNull())
  }
  const section = () => document.querySelector<HTMLElement>('section[aria-label="Можно скачать"]')
  const text = (el: Element | null | undefined) => (el?.textContent ?? '').replace(/\s+/g, ' ').trim()

  it('availAt = 0: строка примера есть, под ней прежнее приглашение, «Скачать все» нет', async () => {
    await open(l())
    const li = section()?.querySelector('li')
    expect(text(li)).toContain('Свадьба — синяя акварель (пример)')
    expect(text(li)).toContain('версия 1 · 4 фона · 7 стикеров · 4 разворота · 2 рамочки · 17,0 МБ')
    expect(text(section())).toContain('Наборы из интернета появятся здесь после «Обновить библиотеку»')
    expect(section()?.querySelector('.head button')).toBeNull()
    expect(section()?.querySelector('li button')?.textContent).toBe('Скачать')
  })

  it('«Скачать» без согласия: вопрос «Обратиться к GitHub?», Go не звали; «Разрешить и скачать» — SetLibraryNet и DownloadLibraryPack(id)', async () => {
    api.SetLibraryNet.mockResolvedValue({ allowed: true, atStart: false, checked: 0, skipped: 0, demoOffer: true })
    api.DownloadLibraryPack.mockResolvedValue({ added: 1, updated: 0, skipped: [], bytes: 1, picked: true })
    await open(l())
    await fireEvent.click(section()!.querySelector('li button')!)
    const dlg = Array.from(document.querySelectorAll('[role="dialog"]')).find(
      (d) => d.getAttribute('aria-label') === 'Обратиться к GitHub?',
    )
    expect(dlg).toBeTruthy()
    expect(api.DownloadLibraryPack).not.toHaveBeenCalled()
    await fireEvent.click(Array.from(dlg!.querySelectorAll('button')).find((b) => text(b) === 'Разрешить и скачать')!)
    await vi.waitFor(() => expect(api.DownloadLibraryPack).toHaveBeenCalledWith('wedding-navy-watercolor-demo'))
    expect(api.DownloadAllLibrary).not.toHaveBeenCalled()
  })

  it('стоящий пример: подпись «Пример в программе», «Удалить» есть, подписи каталога нет', async () => {
    const demo = info({
      id: 'wedding-navy-watercolor-demo',
      title: 'Свадьба — синяя акварель (пример)',
      origin: 'demo',
      author: '',
      source: '',
    })
    await open(l({ packs: [info({ id: 'builtin', origin: 'builtin', removable: false }), demo], available: [] }, { demoOffer: false }))
    const li = Array.from(document.querySelectorAll('li')).find((x) => text(x).includes('(пример)'))
    expect(text(li)).toContain('Лицензия: CC0-1.0 · Пример в программе')
    expect(li?.querySelector('button')?.textContent).toBe('Удалить')
    expect(Array.from(document.querySelectorAll('.skip')).some((p) => text(p).includes('Наборы каталога'))).toBe(false)
  })

  it('удалён один пример: «Пример в программе удалён вами · Вернуть»; «Вернуть» зовёт ResetLibrarySkip', async () => {
    api.ResetLibrarySkip.mockResolvedValue({ allowed: false, atStart: false, checked: 0, skipped: 0, demoBack: true })
    await open(l({ available: [] }, { skipped: 1, demoSkipped: true, demoOffer: false }))
    const p = Array.from(document.querySelectorAll('.skip')).find((x) => text(x).includes('Вернуть'))
    expect(text(p)).toBe('Пример в программе удалён вами · Вернуть')
    expect(p?.querySelector('button')?.getAttribute('title')).toBe('Вернуть пример в библиотеку')
    await fireEvent.click(p!.querySelector('button')!)
    await vi.waitFor(() => expect(api.ResetLibrarySkip).toHaveBeenCalledTimes(1))
  })

  it('пример и другие в пропуске: прежняя строка с числом', async () => {
    await open(l({ available: [] }, { skipped: 3, demoSkipped: true, demoOffer: false }))
    expect(Array.from(document.querySelectorAll('.skip')).map((x) => text(x))).toContain(
      'Удалённые вами наборы «Скачать все» пропускает (3) · Вернуть',
    )
  })
})
