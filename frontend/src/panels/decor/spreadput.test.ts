import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Окно «Развороты» (067 Ф1): Go подменён; вопрос, постановка, повтор, ошибки и «Сохранить разворот…».
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  PlanSpreadTpl: vi.fn(),
  ApplySpreadTpl: vi.fn(),
  SaveSpreadTpl: vi.fn(),
  RenameDecor: vi.fn(),
  HideDecor: vi.fn(),
  Undo: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { decor } = await import('./decor.svelte')
const { spreadSave } = await import('./tplsave.svelte')
const { store } = await import('../../base/core/store.svelte')

const IVORY = 'sa:tpl-ivory-hero'
const OWN = 'my:0123456789abcdef'
const catalog = () => ({
  kind: 'spread',
  themes: [
    { id: 'recent', name: 'Недавние', count: 0 },
    { id: 'all', name: 'Все', count: 2 },
    { id: 'wedding', name: 'Свадьба', count: 1 },
    { id: 'mine', name: 'Мои', count: 1 },
  ],
  items: [
    { id: IVORY, theme: 'wedding', name: 'Айвори', url: `/media/tpl/${IVORY}?v=1`, own: false, aspect: 0, w: 0, h: 0, pages: 2, places: 3 },
    { id: OWN, theme: 'mine', name: 'Мой', url: `/media/tpl/${OWN}?v=1`, own: true, aspect: 0, w: 0, h: 0, pages: 1, places: 2 },
  ],
  recent: [],
  readOnly: false,
})
const view = (rev = 1) => ({
  rev,
  folders: [],
  settings: { pageW: 300, pageH: 300 },
  spreads: [{ id: 's1', photos: [], w: 600, h: 300, mid: 300, frames: [], texts: [], captions: [], bg: ['#fff', '#fff'], bgSet: ['', ''] }],
})
const plan = (over: Record<string, number> = {}) => ({ frozen: 0, left: 0, texts: 0, stickers: 0, ...over })

const open = async (scope: 'spread' | 'left' | 'right' = 'spread') => {
  decor.show('spread', 's1', scope)
  await vi.waitFor(() => expect(decor.cat).not.toBeNull())
}
const tile = (id: string) => decor.cat!.items.find((i) => i.id === id)!

beforeEach(() => {
  vi.clearAllMocks()
  decor.close()
  decor.setTheme('')
  spreadSave.close()
  store.toast = null
  api.DecorCatalog.mockImplementation(async () => catalog())
  api.PlanSpreadTpl.mockResolvedValue(plan())
  api.ApplySpreadTpl.mockResolvedValue({ view: view(2), left: 0, missing: 0 })
  store.apply(view() as never)
})
afterEach(() => decor.close())

describe('вход и каталог', () => {
  it('show читает каталог вида spread; без недавних открывается «Все»', async () => {
    await open()
    expect(api.DecorCatalog).toHaveBeenCalledWith('spread')
    expect(decor.cur).toEqual({ kind: 'spread', spread: 's1', scope: 'spread' })
    expect(decor.theme).toBe('all')
  })
  it('повторный вход из того же вида оставляет выбранную тему', async () => {
    await open()
    decor.setTheme('mine')
    decor.close()
    await open()
    expect(decor.theme).toBe('mine')
  })
})

describe('щелчок по плитке без вопроса (сц. 2)', () => {
  it('Plan → Apply с областью и ID; окно закрыто; тост «Вернуть»; версия выросла', async () => {
    await open('left')
    await decor.pick(tile(IVORY))
    expect(api.PlanSpreadTpl).toHaveBeenCalledWith('s1', 'left', IVORY)
    expect(api.ApplySpreadTpl).toHaveBeenCalledWith('s1', 'left', IVORY)
    expect(decor.cur).toBeNull()
    expect(decor.ask).toBeNull()
    expect(store.view.rev).toBe(2)
    expect(store.toast?.text).toBe('Разворот поставлен')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })
  it('итог: фото вернулись в ленту и слои не найдены — в тосте', async () => {
    api.ApplySpreadTpl.mockResolvedValue({ view: view(2), left: 2, missing: 1 })
    await open()
    await decor.pick(tile(IVORY))
    expect(store.toast?.text).toBe('Разворот поставлен · 2 фото вернулись в ленту · Нет в библиотеке: 1')
  })
  it('повтор: версия не выросла — «Здесь уже такой разворот», без «Вернуть»', async () => {
    api.ApplySpreadTpl.mockResolvedValue({ view: view(1), left: 0, missing: 0 })
    await open()
    await decor.pick(tile(IVORY))
    expect(decor.cur).toBeNull()
    expect(store.toast?.text).toBe('Здесь уже такой разворот')
    expect(store.toast?.action).toBeUndefined()
  })
  it('«Вернуть» в тосте — один Undo', async () => {
    api.Undo.mockResolvedValue(view(3))
    await open()
    await decor.pick(tile(IVORY))
    store.toast?.action?.run()
    await vi.waitFor(() => expect(api.Undo).toHaveBeenCalledTimes(1))
  })
})

describe('вопрос с числами (сц. 3)', () => {
  it('уходят фото и меняются стикеры: окно остаётся, Apply ещё не вызван', async () => {
    api.PlanSpreadTpl.mockResolvedValue(plan({ left: 2, stickers: 1 }))
    await open()
    await decor.pick(tile(IVORY))
    expect(decor.ask).toEqual({ kind: 'put', id: IVORY, text: '2 фото вернутся в ленту, 1 стикер заменится. Поставить?' })
    expect(decor.cur).not.toBeNull()
    expect(api.ApplySpreadTpl).not.toHaveBeenCalled()
  })
  it('«Поставить»: Apply, оба окна закрыты', async () => {
    api.PlanSpreadTpl.mockResolvedValue(plan({ left: 2 }))
    await open()
    await decor.pick(tile(IVORY))
    await decor.confirmPut()
    expect(api.ApplySpreadTpl).toHaveBeenCalledWith('s1', 'spread', IVORY)
    expect(decor.ask).toBeNull()
    expect(decor.cur).toBeNull()
  })
  it('«Отмена»: назад к плиткам, Apply не вызывался, альбом прежний', async () => {
    api.PlanSpreadTpl.mockResolvedValue(plan({ texts: 1 }))
    await open()
    await decor.pick(tile(IVORY))
    decor.dismiss()
    expect(decor.ask).toBeNull()
    expect(decor.cur).not.toBeNull()
    expect(api.ApplySpreadTpl).not.toHaveBeenCalled()
    expect(store.view.rev).toBe(1)
  })
  it('«Поставить» без вопроса и после «Отмены» молчит', async () => {
    await open()
    await decor.confirmPut()
    expect(api.ApplySpreadTpl).not.toHaveBeenCalled()
  })
  it('общий commit() вопроса не трогает: «Убрать» не вызывается', async () => {
    api.PlanSpreadTpl.mockResolvedValue(plan({ left: 1 }))
    await open()
    await decor.pick(tile(IVORY))
    expect(await decor.commit()).toBe(false)
    expect(api.HideDecor).not.toHaveBeenCalled()
    expect(decor.ask?.kind).toBe('put')
  })
})

describe('ошибки и быстрые щелчки', () => {
  it('Plan отказал — тост, окно открыто, повторный щелчок возможен', async () => {
    api.PlanSpreadTpl.mockRejectedValueOnce('разворот не встанет на страницу')
    await open()
    await decor.pick(tile(IVORY))
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Разворот не встанет на страницу' })
    expect(decor.cur).not.toBeNull()
    expect(decor.busy).toBe(false)
    await decor.pick(tile(IVORY))
    expect(api.ApplySpreadTpl).toHaveBeenCalledTimes(1)
  })
  it('Apply отказал — тост, версия прежняя', async () => {
    api.ApplySpreadTpl.mockRejectedValue('нет такого разворота')
    await open()
    await decor.pick(tile(IVORY))
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Нет такого разворота' })
    expect(store.view.rev).toBe(1)
  })
  it('второй щелчок, пока идёт расчёт вопроса, игнорируется', async () => {
    let done!: (v: unknown) => void
    api.PlanSpreadTpl.mockImplementationOnce(() => new Promise((r) => (done = r)))
    await open()
    const first = decor.pick(tile(IVORY))
    await decor.pick(tile(OWN))
    expect(api.PlanSpreadTpl).toHaveBeenCalledTimes(1)
    done(plan())
    await first
    expect(api.ApplySpreadTpl).toHaveBeenCalledTimes(1)
    expect(decor.busy).toBe(false)
  })
  it('окно закрыли, пока шёл расчёт: нет окна — нет постановки', async () => {
    let done!: (v: unknown) => void
    api.PlanSpreadTpl.mockImplementationOnce(() => new Promise((r) => (done = r)))
    await open()
    const p = decor.pick(tile(IVORY))
    decor.close()
    done(plan({ left: 1 }))
    await p
    expect(decor.ask).toBeNull() // вопрос закрытому окну не показываем
    expect(api.ApplySpreadTpl).not.toHaveBeenCalled()
  })
})

describe('✎ и × у своих (сц. 5)', () => {
  it('переименовать и убрать идут с видом spread и перечитывают каталог', async () => {
    await open()
    decor.askRename(tile(OWN))
    decor.name = '  Новое  '
    await decor.commit()
    expect(api.RenameDecor).toHaveBeenCalledWith('spread', OWN, 'Новое')
    decor.askHide(tile(OWN))
    await decor.commit()
    expect(api.HideDecor).toHaveBeenCalledWith('spread', OWN)
    expect(api.DecorCatalog).toHaveBeenCalledTimes(3)
  })
})

describe('«Сохранить разворот…» (сц. 4)', () => {
  const fill = async () => {
    await open('left')
    decor.setTheme('wedding')
    spreadSave.show()
    spreadSave.name = '  Айвори с веткой '
  }

  it('форма берёт «Куда» и открытую тему; места включены, надписи выключены', async () => {
    await fill()
    expect(spreadSave).toMatchObject({ open: true, scope: 'left', theme: 'wedding', places: true, texts: false })
  })
  it('из «Все» и «Недавних» тема — «Мои»', async () => {
    await open()
    for (const t of ['all', 'recent']) {
      decor.setTheme(t)
      spreadSave.show()
      expect(spreadSave.theme).toBe('mine')
    }
  })
  it('вне окна «Развороты» форма не открывается', () => {
    spreadSave.show()
    expect(spreadSave.open).toBe(false)
  })
  it('«Сохранить»: аргументы Go, тост, форма закрыта, каталог перечитан, открыта тема сохранения', async () => {
    api.SaveSpreadTpl.mockResolvedValue({ id: OWN })
    await fill()
    spreadSave.theme = 'mine'
    spreadSave.texts = true
    expect(await spreadSave.commit()).toBe(true)
    expect(api.SaveSpreadTpl).toHaveBeenCalledWith('s1', 'left', 'mine', 'Айвори с веткой', true, true)
    expect(spreadSave.open).toBe(false)
    expect(store.toast?.text).toBe('Разворот сохранён в библиотеку')
    expect(decor.theme).toBe('mine')
    expect(api.DecorCatalog).toHaveBeenCalledTimes(2)
    expect(store.view.rev).toBe(1) // альбом не меняется: отмены нет
  })
  it('пустое имя — Go не зовётся, форма остаётся', async () => {
    await open()
    spreadSave.show()
    spreadSave.name = '   '
    expect(await spreadSave.commit()).toBe(false)
    expect(api.SaveSpreadTpl).not.toHaveBeenCalled()
    expect(spreadSave.open).toBe(true)
  })
  it('«Сохранять нечего» и прочие отказы — тост как есть, форма остаётся', async () => {
    api.SaveSpreadTpl.mockRejectedValue('сохранять нечего: в области нет ни мест, ни оформления')
    await fill()
    expect(await spreadSave.commit()).toBe(false)
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Сохранять нечего: в области нет ни мест, ни оформления' })
    expect(spreadSave.open).toBe(true)
    expect(spreadSave.busy).toBe(false)
  })
  it('повторный Enter, пока идёт сохранение, второго вызова не даёт', async () => {
    let done!: (v: unknown) => void
    api.SaveSpreadTpl.mockImplementationOnce(() => new Promise((r) => (done = r)))
    await fill()
    const first = spreadSave.commit()
    expect(await spreadSave.commit()).toBe(false)
    done({ id: OWN })
    await first
    expect(api.SaveSpreadTpl).toHaveBeenCalledTimes(1)
  })
})
