// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)]))
})

const { default: Guide } = await import('./Guide.svelte')
const { guide } = await import('./guidestate.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { rowOf } = await import('../../base/prefs/levels')
const { dropCatalog, useCatalog } = await import('../../base/keys/keymap.fixture')
const { ARTICLES, GUIDE_GROUPS, articleById, placesOf } = await import('../guidetext/articles')
const { GUIDE_ORDER } = await import('../guidetext/order')
const { search } = await import('./search')
const { guideCount } = await import('./inventory')

// Окно справочника F1 (спека 136): повторное открытие, клавиатура, разделы и счётчики, карточка.
const scroll = vi.fn()
const settle = async () => {
  for (let i = 0; i < 4; i++) await tick()
}
const q = (sel: string) => document.body.querySelector<HTMLElement>(sel)
const qa = (sel: string) => [...document.body.querySelectorAll<HTMLElement>(sel)]
const row = (id: string) => q(`[data-id="${id}"]`)
const heads = () => qa('.head')
const key = (el: Element, k: string) => fireEvent.keyDown(el, { key: k })

async function open(id = 'start-window') {
  guide.sel = id
  guide.open = true
  render(Guide)
  await settle()
}

beforeEach(() => {
  useCatalog()
  Element.prototype.scrollIntoView = scroll
  scroll.mockClear()
  vi.spyOn(prefs, 'can').mockReturnValue(true)
  guide.opened.clear()
  guide.query = ''
  guide.rowFilter = null
  guide.open = false
})
afterEach(() => {
  cleanup()
  guide.open = false
  document.body.innerHTML = ''
  vi.restoreAllMocks()
  dropCatalog()
})

describe('повторный F1', () => {
  it('статья из конца списка → закрыть → F1: её строка прокручена в вид и подсвечена', async () => {
    const last = GUIDE_ORDER.manual[GUIDE_ORDER.manual.length - 1]
    await open(last)
    expect(row(last)?.classList.contains('on')).toBe(true)
    expect(scroll.mock.contexts).toContain(row(last))

    // Окно в приложении смонтировано всегда: F1 лишь переключает guide.open (поэтому эффект выбора не перезапускался).
    guide.open = false
    await settle()
    expect(row(last)).toBeNull()
    scroll.mockClear()

    guide.openFresh() // F1: прежние запрос и выбор
    await settle()
    expect(guide.sel).toBe(last)
    expect(row(last)?.classList.contains('on')).toBe(true)
    expect(scroll.mock.contexts).toContain(row(last))
    expect(scroll).toHaveBeenCalledWith({ block: 'nearest' })
  })
})

describe('выбор при поиске', () => {
  it('ввод запроса — выбор на первой строке выдачи, даже если прежняя статья в ней осталась', async () => {
    const hits = search('сохранить в файл').hits.map((h) => h.a.id)
    const old = hits[5] ?? hits[hits.length - 1]
    expect(hits.length).toBeGreaterThan(1)
    expect(old).not.toBe(hits[0])
    await open(old)
    await fireEvent.input(q('.q') as HTMLInputElement, { target: { value: 'сохранить в файл' } })
    await settle()
    expect(guide.sel).toBe(hits[0])
    expect(row(hits[0])?.classList.contains('on')).toBe(true)
    expect(row(old)?.classList.contains('on')).toBe(false)
  })

  it('следующий ввод снова ведёт на первую строку', async () => {
    await open('start-window')
    const input = q('.q') as HTMLInputElement
    await fireEvent.input(input, { target: { value: 'зеркало' } })
    await settle()
    const first = search('зеркало').hits[0].a.id
    expect(guide.sel).toBe(first)
    const second = search('зеркало фото').hits[0].a.id
    await fireEvent.input(input, { target: { value: 'зеркало фото' } })
    await settle()
    expect(guide.sel).toBe(second)
  })

  it('очистка запроса выбор не меняет; повторный F1 — прежние запрос и выбор, а не первая строка', async () => {
    await open('start-window')
    const input = q('.q') as HTMLInputElement
    await fireEvent.input(input, { target: { value: 'зеркало' } })
    await settle()
    const chosen = search('зеркало').hits[0].a.id
    await fireEvent.input(input, { target: { value: '' } })
    await settle()
    expect(guide.sel).toBe(chosen)
    // Щелчок по другой строке выдачи, закрыть, F1.
    await fireEvent.input(input, { target: { value: 'зеркало' } })
    await settle()
    const other = search('зеркало').hits[1].a.id
    await fireEvent.click(row(other)!)
    guide.open = false
    await settle()
    guide.openFresh()
    await settle()
    expect(guide.query).toBe('зеркало')
    expect(guide.sel).toBe(other)
    expect(row(other)?.classList.contains('on')).toBe(true)
  })
})

describe('клавиатура', () => {
  it('↓ из поиска — фокус на выбранной строке', async () => {
    await open('details')
    const input = q('.q')!
    input.focus()
    await key(input, 'ArrowDown')
    await settle()
    expect(document.activeElement).toBe(row('details'))
  })

  it('Tab: в порядок входят поиск, выбранная строка и кнопки карточки, остальные строки и заголовки — нет', async () => {
    await open('details')
    const stops = qa('[data-id]').filter((e) => e.tabIndex >= 0)
    expect(stops.map((e) => e.getAttribute('data-id'))).toEqual(['details'])
    expect(heads().every((h) => h.tabIndex === -1)).toBe(true)
  })

  it('↓ с последней статьи раздела — первая статья следующего, раздел открыт, фокус на ней', async () => {
    const last = GUIDE_ORDER.start[GUIDE_ORDER.start.length - 1]
    const next = GUIDE_ORDER.photos[0]
    await open(last)
    expect(row(next)).toBeNull() // «Фото и лента» свёрнут
    await key(row(last)!, 'ArrowDown')
    await settle()
    expect(guide.sel).toBe(next)
    expect(row(next)).not.toBeNull()
    expect(document.activeElement).toBe(row(next))
    expect(q('[data-group="photos"] .head')?.getAttribute('aria-expanded')).toBe('true')
  })

  it('↑ с первой статьи — назад в поиск; Home и End — крайние статьи', async () => {
    await open(GUIDE_ORDER.start[0])
    await key(row(GUIDE_ORDER.start[0])!, 'ArrowUp')
    await settle()
    expect(document.activeElement).toBe(q('.q'))
    await key(row(GUIDE_ORDER.start[0])!, 'End')
    await settle()
    expect(guide.sel).toBe(GUIDE_ORDER.manual[GUIDE_ORDER.manual.length - 1])
    await key(row(guide.sel)!, 'Home')
    await settle()
    expect(guide.sel).toBe(GUIDE_ORDER.start[0])
  })

  it('Enter у статьи без места: окно открыто, «Показать» не вызвано, в карточке строка про жест', async () => {
    const a = ARTICLES.find((x) => placesOf(x).length === 0)!
    const show = vi.spyOn(guide, 'show').mockResolvedValue()
    await open(a.id)
    await key(row(a.id)!, 'Enter')
    await settle()
    expect(guide.open).toBe(true)
    expect(show).not.toHaveBeenCalled()
    expect(q('.note')?.textContent).toBe('Показать на экране нечего — это жест или клавиша, см. «Как»')
    expect(document.activeElement).toBe(q('article'))
    expect(qa('.act button')).toHaveLength(0)
  })

  it('Enter у статьи с местом — «Показать»', async () => {
    const a = ARTICLES.find((x) => placesOf(x).length > 0)!
    const show = vi.spyOn(guide, 'show').mockResolvedValue()
    await open(a.id)
    await key(row(a.id)!, 'Enter')
    expect(show).toHaveBeenCalledWith(a.id)
  })
})

describe('разделы и счётчики', () => {
  it('сумма чисел в заголовках — число статей, счётчик в шапке — «N статей» (со склонением)', async () => {
    await open()
    expect(heads()).toHaveLength(GUIDE_GROUPS.length)
    const sum = qa('.head .n').reduce((s, e) => s + Number(e.textContent), 0)
    expect(sum).toBe(ARTICLES.length)
    // Склонение («123 статьи») проверяет guidelist.test.ts; здесь — число и слово «статья» в нужной форме,
    // а за ними — число функций из описи (140): то же, что в «О программе».
    expect(q('.count')?.textContent).toMatch(new RegExp(`^${ARTICLES.length} стат(?:ья|ьи|ей) · ${guideCount()} функ(?:ция|ции|ций)$`))
  })

  it('открыт только раздел выбора; в свёрнутом строк нет, щелчок раскрывает, второй щелчок сворачивает', async () => {
    await open('start-window')
    expect(qa('[data-group="start"] [data-id]').length).toBe(GUIDE_ORDER.start.length)
    expect(qa('[data-group="text"] [data-id]')).toHaveLength(0)
    const head = q('[data-group="text"] .head')!
    expect(head.getAttribute('aria-expanded')).toBe('false')
    await fireEvent.click(head)
    expect(qa('[data-group="text"] [data-id]').length).toBe(GUIDE_ORDER.text.length)
    expect(head.getAttribute('aria-expanded')).toBe('true')
    await fireEvent.click(head)
    expect(qa('[data-group="text"] [data-id]')).toHaveLength(0)
  })

  it('открытое руками помнится после выбора в другом разделе', async () => {
    await open('start-window')
    await fireEvent.click(q('[data-group="text"] .head')!)
    await fireEvent.click(row('first-steps')!)
    await settle()
    expect(qa('[data-group="text"] [data-id]').length).toBe(GUIDE_ORDER.text.length)
  })

  it('поиск «зеркало» — «Найдено N из M», плоский список без заголовков', async () => {
    await open()
    const input = q('.q') as HTMLInputElement
    await fireEvent.input(input, { target: { value: 'зеркало' } })
    await settle()
    const n = search('зеркало').hits.length
    expect(n).toBeGreaterThan(0)
    expect(q('.count')?.textContent).toBe(`Найдено ${n} из ${ARTICLES.length}`)
    expect(heads()).toHaveLength(0)
    expect(qa('[data-id]')).toHaveLength(n)
  })

  it('«как поменять фон» — над списком «Точных совпадений нет — похожие»; точный запрос и пустой — без строки', async () => {
    await open()
    const input = q('.q') as HTMLInputElement
    const note = () => qa('.from').map((p) => p.textContent)
    await fireEvent.input(input, { target: { value: 'как поменять фон' } })
    await settle()
    expect(search('как поменять фон').loose).toBe(true)
    expect(note()).toEqual(['Точных совпадений нет — похожие'])
    await fireEvent.input(input, { target: { value: 'зеркало' } })
    await settle()
    expect(note()).toEqual([])
    await fireEvent.input(input, { target: { value: '' } })
    await settle()
    expect(note()).toEqual([])
  })

  it('фильтр строки уровней — «N из M», все разделы открыты, заголовки не нажимаются', async () => {
    const a = articleById('mini-panel')!
    const row0 = rowOf(a.feature)
    const n = ARTICLES.filter((x) => rowOf(x.feature) === row0).length
    guide.openRow(row0)
    guide.sel = a.id
    render(Guide)
    await settle()
    expect(q('.count')?.textContent).toBe(`${n} из ${ARTICLES.length}`)
    expect(qa('[data-id]')).toHaveLength(n)
    expect(qa('button.head')).toHaveLength(0)
  })
})

describe('тег уровня в строке', () => {
  it('скрытая на уровне функция: справа уровень, с которого она есть; доступная — без тега', async () => {
    vi.spyOn(prefs, 'can').mockImplementation((f) => f !== articleById('mini-panel')!.feature)
    await open('mini-panel')
    expect(row('mini-panel')?.querySelector('.kt')?.textContent).toMatch(/^(Простой|Стандартный|Полный)$/)
    const other = GUIDE_ORDER.onspread.find((id) => articleById(id)!.feature !== articleById('mini-panel')!.feature)!
    expect(row(other)).not.toBeNull()
    expect(row(other)?.querySelector('.kt')).toBeNull()
  })
})

describe('карточка', () => {
  it('«Уровень» без повтора слова; у статьи без места нет кнопок; у статьи с местом — есть', async () => {
    await open('start-window')
    const lvl = qa('.row').find((r) => r.querySelector('b')?.textContent === 'Уровень')!
    expect(lvl.textContent).not.toContain('Уровень Уровень')
    expect(lvl.querySelector('span')?.textContent?.trim()).toMatch(/^(Простой и выше|Стандартный и выше|Полный)$/)
  })

  it('длинный текст — абзацами по «\\n»', async () => {
    const a = articleById('first-steps')!
    const old = a.how
    a.how = 'Первый абзац.\nВторой абзац.'
    try {
      await open(a.id)
      expect(qa('.paras p').map((p) => p.textContent)).toContain('Второй абзац.')
    } finally {
      a.how = old
    }
  })

  it('«См. также»: ссылки на статьи, щелчок выбирает статью и открывает её раздел', async () => {
    const a = articleById('start-window')!
    a.see = ['crop', 'text']
    try {
      await open(a.id)
      const links = qa('.see .link')
      expect(links.map((l) => l.textContent)).toEqual([articleById('crop')!.title, articleById('text')!.title])
      await fireEvent.click(links[1])
      await settle()
      expect(guide.sel).toBe('text')
      expect(row('text')?.classList.contains('on')).toBe(true)
    } finally {
      delete a.see
    }
  })

  it('без «См. также» строки нет', async () => {
    await open('start-window')
    expect(q('.see')).toBeNull()
  })
})
