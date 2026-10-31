// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Заглушки биндингов: файл отметок — массив file; MarkGuide возвращает его целиком, как Go.
const api = vi.hoisted(() => ({ file: [] as string[], GuideLearned: vi.fn(), MarkGuide: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})

const { default: Guide } = await import('./Guide.svelte')
const { guide } = await import('./guidestate.svelte')
const { learned } = await import('./learned.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { dropCatalog, useCatalog } = await import('../../base/keys/keymap.fixture')
const { ARTICLES, articleById } = await import('../guidetext/articles')
const { GUIDE_ORDER } = await import('../guidetext/order')

// Отметки «Изучил» в окне F1 (спека 140, раздел A): флажок, ✓ в строке, «5 / 18», «Изучено N из M».
const settle = async () => {
  for (let i = 0; i < 4; i++) await tick()
}
const q = (sel: string) => document.body.querySelector<HTMLElement>(sel)
const qa = (sel: string) => [...document.body.querySelectorAll<HTMLElement>(sel)]
const row = (id: string) => q(`[data-id="${id}"]`)
const mark = (id: string) => row(id)?.querySelector('.ok')?.textContent
const flag = () => q('.card input[type="checkbox"]') as HTMLInputElement
const prog = () => q('.prog')?.textContent
const groupN = (g: string) => q(`[data-group="${g}"] .head .n`)?.textContent

async function open(id = 'start-window') {
  guide.sel = id
  guide.open = true
  render(Guide)
  await settle()
}

beforeEach(() => {
  useCatalog()
  Element.prototype.scrollIntoView = vi.fn()
  api.file = []
  api.GuideLearned.mockReset().mockImplementation(async () => [...api.file])
  api.MarkGuide.mockReset().mockImplementation(async (ids: string[], on: boolean) => {
    api.file = on ? [...api.file, ...ids.filter((x) => !api.file.includes(x))] : api.file.filter((x) => !ids.includes(x))
    return [...api.file]
  })
  learned.ids.clear()
  vi.spyOn(prefs, 'can').mockReturnValue(true)
  guide.opened.clear()
  guide.query = ''
  guide.rowFilter = null
  guide.open = false
})
afterEach(() => {
  guide.stop() // «Показать» из теста не оставляет поиск места: его таймеры пережили бы разборку окружения
  cleanup()
  guide.open = false
  document.body.innerHTML = ''
  vi.restoreAllMocks()
  dropCatalog()
})

describe('флажок «Изучил»', () => {
  it('ставит и снимает отметку через MarkGuide; ✓ в строке следует за ответом', async () => {
    await open()
    expect(flag().checked).toBe(false)
    expect(mark('start-window')).toBe('')

    await fireEvent.click(flag())
    await settle()
    expect(api.MarkGuide).toHaveBeenLastCalledWith(['start-window'], true)
    expect(flag().checked).toBe(true)
    expect(mark('start-window')).toBe('✓')
    expect(row('start-window')?.hasAttribute('aria-label')).toBe(false)
    expect(row('start-window')?.textContent).toContain(`${articleById('start-window')!.title}`)
    expect(row('start-window')?.querySelector('.vh')?.textContent).toBe(', изучено')

    await fireEvent.click(flag())
    await settle()
    expect(api.MarkGuide).toHaveBeenLastCalledWith(['start-window'], false)
    expect(flag().checked).toBe(false)
    expect(mark('start-window')).toBe('')
    expect(row('start-window')?.querySelector('.vh')).toBeNull()
  })

  it('у каждой строки есть место под ✓ — пустое у неизученной', async () => {
    await open()
    const rows = qa('button.item')
    expect(rows.length).toBeGreaterThan(0)
    for (const r of rows) expect(r.querySelector('.ok')).not.toBeNull()
  })

  it('отметка из файла видна при открытии окна', async () => {
    api.file = ['crop', 'undo']
    await open()
    expect(mark('crop')).toBe('✓')
    expect(mark('undo')).toBe('✓')
    expect(mark('open')).toBe('')
  })

  it('отказ записи: отметка не появляется', async () => {
    api.MarkGuide.mockRejectedValueOnce('диск недоступен')
    await open()
    await fireEvent.click(flag())
    await settle()
    expect(flag().checked).toBe(false)
    expect(mark('start-window')).toBe('')
  })
})

describe('отметку ставит только флажок', () => {
  it('выбор строки, двойной щелчок и «Показать» MarkGuide не вызывают', async () => {
    await open('open')
    await fireEvent.click(row('save')!)
    await settle()
    await fireEvent.dblClick(row('save')!)
    await settle()
    for (const b of qa('.act button')) await fireEvent.click(b)
    await settle()
    expect(api.MarkGuide).not.toHaveBeenCalled()
    expect(learned.ids.size).toBe(0)
  })
  it('Enter в списке — тоже (окно закрывается «Показать», отметки нет)', async () => {
    await open('open')
    await fireEvent.keyDown(row('open')!, { key: 'Enter' })
    await settle()
    expect(api.MarkGuide).not.toHaveBeenCalled()
    expect(learned.ids.size).toBe(0)
  })
  it('Enter в поиске — тоже', async () => {
    await open('open')
    await fireEvent.keyDown(q('.q')!, { key: 'Enter' })
    await settle()
    expect(api.MarkGuide).not.toHaveBeenCalled()
    expect(learned.ids.size).toBe(0)
  })
})

describe('«Показать» и конец теста', () => {
  it('stop() снимает ожидание поиска места: таймеров не остаётся, документ после него не читается', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      await open('open')
      await fireEvent.click(qa('.act button')[0])
      await settle()
      expect(vi.getTimerCount()).toBeGreaterThan(0) // в happy-dom кнопки нет на экране — поиск ждёт кадры
      guide.stop()
      await settle()
      expect(vi.getTimerCount()).toBe(0)
      expect(guide.busy).toBe(false)
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('«5 / 18» в заголовке раздела', () => {
  it('без изученных — число статей; с изученными — «k / n»; снятие возвращает число', async () => {
    const start = GUIDE_ORDER.start
    await open('open')
    expect(groupN('start')).toBe(String(start.length))
    await fireEvent.click(flag())
    await settle()
    expect(groupN('start')).toBe(`1 / ${start.length}`)
    expect(groupN('photos')).toBe(String(GUIDE_ORDER.photos.length)) // другой раздел не затронут
    await fireEvent.click(flag())
    await settle()
    expect(groupN('start')).toBe(String(start.length))
  })
})

describe('«Изучено N из M» внизу окна', () => {
  it('без отметок строки нет; с отметками — N из всех доступных статей', async () => {
    await open()
    expect(prog()).toBeUndefined()
    await fireEvent.click(flag())
    await settle()
    expect(prog()).toBe(`Изучено 1 из ${ARTICLES.length}`)
  })
  it('M считается от уровня; изученная, но скрытая уровнем в N не входит, ✓ у строки остаётся', async () => {
    const hide = articleById('crop')!.feature
    expect(articleById('start-window')!.feature).not.toBe(hide)
    vi.spyOn(prefs, 'can').mockImplementation((f) => f !== hide)
    api.file = ['crop', 'start-window']
    await open('start-window')
    expect(prog()).toBe(`Изучено 1 из ${ARTICLES.filter((a) => a.feature !== hide).length}`)
    expect(mark('crop')).toBe('✓')
  })
  it('все изученные скрыты уровнем — строки нет', async () => {
    vi.spyOn(prefs, 'can').mockReturnValue(false)
    api.file = ['crop']
    await open('crop')
    expect(prog()).toBeUndefined()
  })
  it('мусорный и неизвестный id в файле: не показывается и не считается', async () => {
    api.file = ['no-such-article', 'crop']
    await open('crop')
    expect(prog()).toBe(`Изучено 1 из ${ARTICLES.length}`)
    expect(mark('crop')).toBe('✓')
  })
})

describe('прежние id (RENAMED)', () => {
  it('в файле прежний id — отметка засчитана новому, снятие уходит вместе с ним', async () => {
    const { RENAMED } = await import('../guidetext/renamed')
    RENAMED['old-crop'] = 'crop'
    try {
      api.file = ['old-crop']
      await open('crop')
      expect(mark('crop')).toBe('✓')
      expect(flag().checked).toBe(true)
      await fireEvent.click(flag()) // снятие уходит вместе с прежним id
      await settle()
      expect(api.MarkGuide).toHaveBeenLastCalledWith(['crop', 'old-crop'], false)
      expect(mark('crop')).toBe('')
    } finally {
      delete RENAMED['old-crop']
    }
  })
})

describe('поиск: плоский список', () => {
  it('✓ видна и в выдаче поиска', async () => {
    api.file = ['crop']
    await open('crop')
    guide.query = articleById('crop')!.title
    await settle()
    expect(row('crop')).not.toBeNull()
    expect(mark('crop')).toBe('✓')
  })
})
