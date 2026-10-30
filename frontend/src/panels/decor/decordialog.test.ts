import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Состояние окна «Фоны»: Go подменён, каталог — из заготовки, правки — вызовами с проверкой аргументов.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  ImportDecor: vi.fn(),
  AddDecorPNG: vi.fn(),
  RenameDecor: vi.fn(),
  HideDecor: vi.fn(),
  ApplyBackground: vi.fn(),
  AddSticker: vi.fn(),
  ReplaceSticker: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { decor } = await import('./decor.svelte')
const { store } = await import('../../base/core/store.svelte')
const { bgmode } = await import('../../editor/backdrop/bgmode.svelte')

const OWN = 'my:0123456789abcdef'
const catalog = (over: Record<string, unknown> = {}) => ({
  kind: 'bg',
  themes: [
    { id: 'recent', name: 'Недавние', count: 1 },
    { id: 'all', name: 'Все', count: 2 },
    { id: 'mine', name: 'Мои', count: 1 },
  ],
  items: [
    { id: 'sa:dots', theme: 'basic', name: 'Горох', url: '/media/gen/pat?k=dots&v=1', own: false, aspect: 0, w: 0, h: 0, color: '#fff' },
    { id: OWN, theme: 'mine', name: 'Моё', url: '/media/lib/t/0123456789abcdef?v=1', own: true, aspect: 1.5, w: 3000, h: 2000 },
  ],
  recent: ['sa:dots'],
  readOnly: false,
  ...over,
})

const spreadView = (backdrop = false) => ({
  rev: 1,
  folders: [],
  settings: { pageW: 300, pageH: 300 },
  spreads: [
    {
      id: 's1',
      photos: [],
      stickers: [{ id: 'k1' }], // заменяемый в тестах «Заменить…»
      w: 600,
      h: 300,
      mid: 300,
      frames: [],
      texts: [],
      captions: [],
      bg: ['#fff', '#fff'],
      bgSet: ['', ''],
      backdrops: backdrop
        ? [
            {
              scope: 'spread',
              photo: 'lib1',
              lib: OWN,
              area: { x: 0, y: 0, w: 600, h: 300 },
              box: { x: 0, y: 0, w: 600, h: 300, rot: 0 },
              fade: 0,
            },
          ]
        : null,
    },
  ],
})

beforeEach(() => {
  vi.clearAllMocks()
  decor.close()
  store.toast = null
  api.DecorCatalog.mockImplementation(async () => catalog())
  store.apply(spreadView() as never)
})

describe('окно «Фоны»: открытие', () => {
  it('show читает каталог и выбирает «Недавние»', async () => {
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
    expect(api.DecorCatalog).toHaveBeenCalledWith('bg')
    expect(decor.cur).toEqual({ kind: 'bg', spread: 's1', scope: 'spread' })
    expect(decor.theme).toBe('recent')
    expect(decor.loading).toBe(false)
  })

  it('закрыли, пока каталог в пути: запоздавший ответ окну не нужен', async () => {
    let done!: (v: unknown) => void
    api.DecorCatalog.mockImplementationOnce(() => new Promise((r) => (done = r)))
    const before = decor.cat
    decor.show('bg', 's1', 'spread')
    decor.close()
    done(catalog({ themes: [] }))
    await new Promise((r) => setTimeout(r, 0))
    expect(decor.cat).toBe(before) // ответ отброшен
    expect(decor.cur).toBeNull()
    expect(decor.loading).toBe(false)
  })

  it('«Куда» меняется без закрытия окна', async () => {
    decor.show('bg', 's1', 'spread')
    decor.setScope('right')
    expect(decor.cur?.scope).toBe('right')
  })
})

describe('окно после установки набора (073-1)', () => {
  afterEach(() => {
    decor.cat = null // каталог с набором другим тестам не нужен
    decor.setTheme('')
  })

  const withPack = () =>
    catalog({
      themes: [
        { id: 'recent', name: 'Недавние', count: 1 },
        { id: 'all', name: 'Все', count: 3 },
        { id: 'basic', name: 'Универсальные', count: 1 },
        { id: 'ivory', name: 'Айвори', count: 2 },
      ],
      items: [
        { id: 'sa:dots', theme: 'basic', name: 'Горох', url: '/a', own: false, aspect: 0, w: 0, h: 0 },
        { id: 'pk:1', theme: 'ivory', name: 'Лён', url: '/b', own: false, aspect: 0, w: 0, h: 0, pack: 'Айвори' },
        { id: 'pk:2', theme: 'ivory', name: 'Шёлк', url: '/c', own: false, aspect: 0, w: 0, h: 0, pack: 'Айвори' },
      ],
    })

  it('после установки — тема набора, открытое окно не прыгает', async () => {
    api.DecorCatalog.mockImplementation(async () => withPack())
    decor.note(['Айвори'])
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.loading).toBe(false))
    expect(decor.theme).toBe('ivory')
    // Пока окно открыто: ещё один набор и перечитывание каталога — тема остаётся, на новый набор не переходит.
    decor.setTheme('basic')
    decor.note(['Айвори'])
    await decor.refresh()
    expect(decor.theme).toBe('basic')
    // Следующее открытие каждого окна — на теме набора, один раз; потом забыто.
    for (const kind of ['sticker', 'spread', 'frame', 'bg'] as const) {
      decor.close()
      decor.setTheme('basic')
      decor.show(kind, 's1', 'spread')
      await vi.waitFor(() => expect(decor.loading).toBe(false))
      expect(decor.theme).toBe('ivory')
    }
    decor.close()
    decor.setTheme('basic')
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.loading).toBe(false))
    expect(decor.theme).toBe('basic') // свежее показано всем четырём видам и забыто
  })

  it('повторное открытие того же вида не прыгает; удаление набора (пусто) и набор без плиток вида — тоже', async () => {
    api.DecorCatalog.mockImplementation(async () => withPack())
    decor.setTheme('')
    decor.note([]) // удаление набора
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.loading).toBe(false))
    expect(decor.theme).toBe('recent')
    decor.close()
    decor.note(['Нет такого'])
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.loading).toBe(false))
    expect(decor.theme).toBe('recent') // плиток набора в этом виде нет — как раньше
    decor.close()
    decor.show('bg', 's1', 'spread') // этот вид уже получил своё
    await vi.waitFor(() => expect(decor.loading).toBe(false))
    expect(decor.theme).toBe('recent')
    for (const kind of ['sticker', 'spread'] as const) {
      decor.close()
      decor.show(kind, 's1', 'spread')
      await vi.waitFor(() => expect(decor.loading).toBe(false))
    }
    decor.close()
  })
})

describe('щелчок по плитке', () => {
  it('закрывает окно сразу и зовёт ApplyBackground с областью и ключом плитки', async () => {
    api.ApplyBackground.mockResolvedValue({ ...spreadView(), rev: 2 })
    decor.show('bg', 's1', 'left')
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
    const p = decor.pick(decor.cat!.items[0])
    expect(decor.cur).toBeNull() // окно закрыто до ответа Go
    await p
    expect(api.ApplyBackground).toHaveBeenCalledWith('s1', 'left', 'sa:dots')
    expect(store.view.rev).toBe(2)
    expect(bgmode.cur).toBeNull() // узор — не фото: режим фона не открывается
  })

  it('своё — как «Свой файл…»: сразу режим фона области', async () => {
    api.ApplyBackground.mockResolvedValue({ ...spreadView(true), rev: 2 })
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
    await decor.pick(decor.cat!.items[1])
    expect(api.ApplyBackground).toHaveBeenCalledWith('s1', 'spread', OWN)
    expect(bgmode.cur).toEqual({ spread: 's1', scope: 'spread' })
    bgmode.exit()
  })

  it('тот же фон повторно (rev не вырос) — окно закрыто, режим не открывается', async () => {
    api.ApplyBackground.mockResolvedValue(spreadView(true)) // rev 1, как был
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
    await decor.pick(decor.cat!.items[1])
    expect(decor.cur).toBeNull()
    expect(bgmode.cur).toBeNull()
  })

  it('ошибка Go — тост, окно уже закрыто', async () => {
    api.ApplyBackground.mockRejectedValue('нет такого фона')
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
    await decor.pick(decor.cat!.items[0])
    expect(store.toast?.kind).toBe('error')
    expect(store.toast?.text).toBe('Нет такого фона')
  })
})

describe('окно «Стикеры» (048 Ф2)', () => {
  const STK = 'my:fedcba9876543210'
  const stickers = () =>
    catalog({
      kind: 'sticker',
      themes: [
        { id: 'all', name: 'Все', count: 2 },
        { id: 'mine', name: 'Мои', count: 1 },
      ],
      items: [
        { id: 'sa:heart', theme: 'wedding', name: 'Сердце', url: '/media/gen/art?k=heart&v=1', own: false, aspect: 1, w: 0, h: 0 },
        { id: STK, theme: 'mine', name: 'Моя наклейка', url: '/media/lib/t/fedcba9876543210?v=1', own: true, aspect: 1, w: 800, h: 800 },
      ],
      recent: [],
    })
  const open = async (replace?: string) => {
    api.DecorCatalog.mockImplementation(async () => stickers())
    decor.show('sticker', 's1', 'spread', replace)
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
  }

  it('show читает каталог стикеров и запоминает его для карточки', async () => {
    await open()
    expect(api.DecorCatalog).toHaveBeenCalledWith('sticker')
    expect(decor.cur).toEqual({ kind: 'sticker', spread: 's1', scope: 'spread' })
    expect(decor.stickerItem(STK)?.name).toBe('Моя наклейка')
    expect(decor.stickerItem('нет')).toBeUndefined()
    expect(decor.stickerItem(undefined)).toBeUndefined()
  })

  it('щелчок по плитке: окно закрыто до ответа, AddSticker на страницу под курсором, новый стикер выделен', async () => {
    const { placesel } = await import('../../editor/places/placesel.svelte')
    placesel.clear()
    placesel.page = 1
    api.AddSticker.mockResolvedValue({ view: { ...spreadView(), rev: 2 }, id: 'k9' })
    await open()
    const p = decor.pick(decor.cat!.items[0])
    expect(decor.cur).toBeNull()
    await p
    expect(api.AddSticker).toHaveBeenCalledWith('s1', 1, 'sa:heart')
    expect(api.ApplyBackground).not.toHaveBeenCalled()
    expect(store.view.rev).toBe(2)
    expect(placesel.stickers).toEqual(['k9'])
    placesel.page = 0
    placesel.clear()
  })

  it('«Заменить…»: ReplaceSticker с ID стикера, добавления нет', async () => {
    api.ReplaceSticker.mockResolvedValue({ ...spreadView(), rev: 3 })
    await open('k1')
    expect(decor.cur).toEqual({ kind: 'sticker', spread: 's1', scope: 'spread', replace: 'k1' })
    await decor.pick(decor.cat!.items[1])
    expect(api.ReplaceSticker).toHaveBeenCalledWith('s1', 'k1', STK)
    expect(api.AddSticker).not.toHaveBeenCalled()
    expect(store.view.rev).toBe(3)
  })

  // 0.28-133-78: Ctrl+Z под окном убрал заменяемый стикер — окно остаётся и ведёт себя как «Добавить…»; возврат (Redo) — снова «Заменить…»
  it('«Заменить…»: стикер убрали отменой — окно открыто, постановка добавляет; вернули — снова заменяет', async () => {
    const { placesel } = await import('../../editor/places/placesel.svelte')
    placesel.clear()
    placesel.page = 1
    api.AddSticker.mockResolvedValue({ view: { ...spreadView(), rev: 4 }, id: 'k9' })
    api.ReplaceSticker.mockResolvedValue({ ...spreadView(), rev: 6 })
    await open('k1')
    expect(decor.replacing()).toBe('k1')
    const withK1 = store.view
    store.view = { ...withK1, rev: 2, spreads: [{ ...withK1.spreads[0], stickers: [] }] } as never // Ctrl+Z
    expect(decor.cur?.replace).toBe('k1') // окно не закрылось, запись режима цела
    expect(decor.replacing()).toBeUndefined()
    expect(decor.standing('spread')).toBe('')
    store.view = { ...withK1, rev: 3 } as never // Ctrl+Shift+Z вернул стикер
    expect(decor.replacing()).toBe('k1')
    store.view = { ...withK1, rev: 2, spreads: [{ ...withK1.spreads[0], stickers: [] }] } as never
    await decor.pick(decor.cat!.items[0])
    expect(api.AddSticker).toHaveBeenCalledWith('s1', 1, 'sa:heart')
    expect(api.ReplaceSticker).not.toHaveBeenCalled()
    expect(decor.cur).toBeNull()
    placesel.page = 0
    placesel.clear()
  })

  it('отказ Go — тост, окно закрыто, выделение прежнее', async () => {
    api.AddSticker.mockRejectedValue('на листе уже 64 стикера')
    await open()
    await decor.pick(decor.cat!.items[0])
    expect(decor.cur).toBeNull()
    expect(store.toast?.kind).toBe('error')
    expect(store.toast?.text).toBe('На листе уже 64 стикера')
  })

  it('загрузка своих идёт с видом «sticker» и темой «Мои»', async () => {
    api.ImportDecor.mockResolvedValue({ added: [{ id: STK }], skipped: [] })
    await open()
    await decor.upload()
    expect(api.ImportDecor).toHaveBeenCalledWith('sticker', 'mine')
  })

  it('смена вида (стикеры → фоны) начинает с первой темы нового каталога', async () => {
    await open()
    decor.setTheme('mine')
    api.DecorCatalog.mockImplementation(async () => catalog())
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.cat?.kind).toBe('bg'))
    expect(decor.theme).toBe('recent')
  })

  it('каталог стикеров не подменяет плитки фонов в карточке узора', async () => {
    api.DecorCatalog.mockImplementation(async () => catalog())
    await decor.ensureNames()
    const bg = decor.bgItems
    await open()
    expect(decor.bgItems).toBe(bg)
  })
})

describe('«Загрузить своё…»', () => {
  const open = async () => {
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
  }

  it('из «Все» и «Недавних» — в «Мои»; итог тостом; каталог перечитан', async () => {
    api.ImportDecor.mockResolvedValue({
      added: [{ id: OWN }, { id: 'my:1' }, { id: 'my:2' }],
      skipped: [{ name: 'x.png', why: 'уже есть' }],
    })
    await open()
    expect(decor.theme).toBe('recent')
    await decor.upload()
    expect(api.ImportDecor).toHaveBeenCalledWith('bg', 'mine')
    expect(store.toast?.text).toBe('Добавлено: 3, пропущено: 1 (x.png — уже есть)')
    expect(api.DecorCatalog).toHaveBeenCalledTimes(2)
    expect(decor.busy).toBe(false)
    expect(decor.theme).toBe('mine') // 100-1: после загрузки окно в теме «Мои»
  })

  it('из своей темы — в неё', async () => {
    api.ImportDecor.mockResolvedValue({ added: [{ id: 'my:1' }], skipped: [] })
    await open()
    decor.setTheme('mine')
    await decor.upload()
    expect(api.ImportDecor).toHaveBeenCalledWith('bg', 'mine')
    expect(store.toast?.text).toBe('Добавлено: 1')
  })

  it('диалог закрыли ничего не выбрав — без тоста', async () => {
    api.ImportDecor.mockResolvedValue({ added: [], skipped: [] })
    await open()
    const was = decor.theme
    await decor.upload()
    expect(store.toast).toBeNull()
    expect(decor.theme).toBe(was) // ничего не добавлено — тема прежняя
  })

  it('библиотека только для чтения — загрузки нет', async () => {
    api.DecorCatalog.mockImplementation(async () => catalog({ readOnly: true }))
    await open()
    await decor.upload()
    expect(api.ImportDecor).not.toHaveBeenCalled()
  })

  it('ошибка — тост, кнопка снова живая', async () => {
    api.ImportDecor.mockRejectedValue('библиотека недоступна')
    await open()
    await decor.upload()
    expect(store.toast?.text).toBe('Библиотека недоступна')
    expect(decor.busy).toBe(false)
  })
})

describe('«Загрузить своё…»: SVG-стикеры', () => {
  const svg = (n: number) => ({ token: `t${n}`, name: `s${n}`, data: 'QQ==', w: 200, h: 100 })
  const open = async () => {
    decor.show('sticker', 's1', 'spread')
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
  }
  // Картинка и холст подменены: рисуется «PNG», у которого в имени токен.
  const stubBrowser = (failImg = (_src: string) => false) => {
    class Img {
      src = ''
      async decode() {
        if (failImg(this.src)) throw new Error('битый')
      }
    }
    vi.stubGlobal('Image', Img)
    vi.stubGlobal('document', {
      createElement: () => ({
        getContext: () => ({ drawImage: () => {} }),
        toDataURL: () => 'data:image/png;base64,UE5H',
      }),
    })
  }
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.stubGlobal('window', globalThis)
  })

  it('SVG рисуются по очереди, в Go уходит PNG по токену; один тост на всё', async () => {
    api.ImportDecor.mockResolvedValue({ added: [{ id: 'my:a' }], skipped: [], svg: [svg(1), svg(2)] })
    api.AddDecorPNG.mockImplementation(async (t: string) => ({ id: `my:${t}` }))
    stubBrowser()
    await open()
    await decor.upload()
    expect(api.ImportDecor).toHaveBeenCalledWith('sticker', 'mine')
    expect(api.AddDecorPNG.mock.calls).toEqual([
      ['t1', 'UE5H'],
      ['t2', 'UE5H'],
    ])
    expect(store.toast?.text).toBe('Добавлено: 3')
    expect(api.DecorCatalog).toHaveBeenCalledTimes(2)
    expect(decor.busy).toBe(false)
  })

  it('один из двух не открылся — пропуск в том же тосте, второй добавлен', async () => {
    api.ImportDecor.mockResolvedValue({ added: [], skipped: [{ name: 'x.png', why: 'уже есть' }], svg: [svg(1), svg(2)] })
    api.AddDecorPNG.mockImplementation(async (t: string) => ({ id: `my:${t}` }))
    let n = 0
    stubBrowser(() => n++ === 0) // первый decode — отказ
    await open()
    await decor.upload()
    expect(api.AddDecorPNG).toHaveBeenCalledTimes(1)
    expect(store.toast?.text).toBe('Добавлено: 1, пропущено: 2 (x.png — уже есть; s1 — не удалось прочитать рисунок)')
  })

  it('все SVG пропущены — тост-ошибка с причиной', async () => {
    api.ImportDecor.mockResolvedValue({ added: [], skipped: [], svg: [svg(1)] })
    stubBrowser(() => true)
    await open()
    await decor.upload()
    expect(api.AddDecorPNG).not.toHaveBeenCalled()
    expect(store.toast?.text).toBe('Добавлено: 0, пропущено: 1 (s1 — не удалось прочитать рисунок)')
    expect(store.toast?.kind).toBe('error')
  })

  it('пустой список SVG — прежний итог без рисования', async () => {
    api.ImportDecor.mockResolvedValue({ added: [{ id: 'my:a' }], skipped: [], svg: [] })
    await open()
    await decor.upload()
    expect(api.AddDecorPNG).not.toHaveBeenCalled()
    expect(store.toast?.text).toBe('Добавлено: 1')
  })
})

describe('✎ и × у своего', () => {
  const open = async () => {
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
  }
  const own = { id: OWN, name: 'Моё' }

  it('✎ спрашивает имя, «Сохранить» переименовывает и перечитывает каталог', async () => {
    api.RenameDecor.mockResolvedValue(undefined)
    await open()
    decor.askRename(own)
    expect(decor.ask).toEqual({ kind: 'rename', id: OWN })
    expect(decor.name).toBe('Моё')
    decor.name = '  Лес  '
    expect(await decor.commit()).toBe(true)
    expect(api.RenameDecor).toHaveBeenCalledWith('bg', OWN, 'Лес')
    expect(decor.ask).toBeNull()
    expect(api.DecorCatalog).toHaveBeenCalledTimes(2)
  })

  it('× спрашивает «Убрать из библиотеки?», «Убрать» скрывает, а «Отмена» ничего не зовёт', async () => {
    api.HideDecor.mockResolvedValue(undefined)
    await open()
    decor.askHide(own)
    expect(decor.ask).toEqual({ kind: 'hide', id: OWN, name: 'Моё' })
    decor.dismiss()
    expect(decor.ask).toBeNull()
    expect(api.HideDecor).not.toHaveBeenCalled()
    decor.askHide(own)
    expect(await decor.commit()).toBe(true)
    expect(api.HideDecor).toHaveBeenCalledWith('bg', OWN)
  })

  it('ошибка Go — тост, каталог не перечитывается', async () => {
    api.RenameDecor.mockRejectedValue('пустое имя')
    await open()
    decor.askRename(own)
    expect(await decor.commit()).toBe(false)
    expect(store.toast?.text).toBe('Пустое имя')
    expect(api.DecorCatalog).toHaveBeenCalledTimes(1)
  })

  it('закрытие окна снимает вопрос: Esc ничего не правит', async () => {
    await open()
    decor.askHide(own)
    decor.close()
    expect(decor.ask).toBeNull()
    expect(await decor.commit()).toBe(false)
    expect(api.HideDecor).not.toHaveBeenCalled()
  })
})

describe('имена своих для карточки фона', () => {
  it('каталог читается один раз; имя по ключу', async () => {
    vi.resetModules() // свежее состояние: прежние тесты уже читали каталог
    const { decor: fresh } = await import('./decor.svelte')
    api.DecorCatalog.mockImplementation(async () => catalog())
    await fresh.ensureNames()
    await fresh.ensureNames()
    expect(api.DecorCatalog).toHaveBeenCalledTimes(1)
    expect(fresh.nameOf(OWN)).toBe('Моё')
    expect(fresh.nameOf('my:ffffffffffffffff')).toBeUndefined()
    expect(fresh.nameOf(undefined)).toBeUndefined()
  })

  it('каталог недоступен — имени нет, следующая попытка прочитает снова', async () => {
    vi.resetModules()
    const { decor: fresh } = await import('./decor.svelte')
    api.DecorCatalog.mockRejectedValueOnce('нет библиотеки')
    await fresh.ensureNames()
    expect(fresh.nameOf(OWN)).toBeUndefined()
    await fresh.ensureNames()
    expect(fresh.nameOf(OWN)).toBe('Моё')
  })

  it('плитки каталога доступны карточке узора без окна', async () => {
    vi.resetModules()
    const { decor: fresh } = await import('./decor.svelte')
    api.DecorCatalog.mockImplementation(async () => catalog())
    expect(fresh.bgItems).toEqual([])
    await fresh.ensureNames()
    expect(fresh.bgItems.map((i) => i.id)).toEqual(['sa:dots', OWN])
  })
})
