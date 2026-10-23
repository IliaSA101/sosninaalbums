import { beforeEach, describe, expect, it, vi } from 'vitest'

// Бросок на ручной разворот (042): вопрос «что с остальными», второй вызов с rest, тосты и выделение новых мест.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  DropPhotos: vi.fn(),
  PlacePhotos: vi.fn(),
  AddPhotos: vi.fn(),
  FillPlaces: vi.fn(),
  ForgetFileNames: vi.fn(),
}))
vi.stubGlobal('window', globalThis)

const Api = await import('../../../wailsjs/go/app/App')
const { store } = await import('../core/store.svelte')
const { noHooks } = await import('../core/hooks')
const { dropAsk, dropAskText, dropGrid, answerDrop, cancelDrop } = await import('./dropgrid.svelte')
const { dropOn, dropHooks } = await import('./dropact')
const { unitOf } = await import('../geom/sheet')

const DropPhotos = vi.mocked(Api.DropPhotos)
const select = vi.fn()

const grid = (id: string, photos: string[], vig?: object) => ({
  id,
  photos,
  grid: true,
  places: [],
  ...(vig ? { kind: 'vignette', vig } : {}),
})
const view = (rev: number, spreads: object[] = [grid('g', ['p0']), { id: 'a', photos: ['x'], grid: false }]) =>
  ({ rev, marksRev: 0, folders: [], spreads, settings: { sheet: false } }) as never
const reply = (o: Partial<Record<string, unknown>> = {}) =>
  ({ view: view(2), ask: false, fit: 0, total: 0, fresh: [], grown: 0, out: false, left: 0, small: false, named: [], ...o }) as never

beforeEach(() => {
  vi.clearAllMocks()
  cancelDrop()
  store.toast = null
  store.selected.clear()
  store.plug({ ...noHooks, select })
  store.apply(view(1))
})

describe('вопрос об остальных', () => {
  it('Ask: проект не меняется, окно открыто, второго вызова нет', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ ask: true, fit: 3, total: 7, view: view(1) }))
    await dropGrid('g', '', ['a', 'b'], 0.7, false)
    expect(DropPhotos).toHaveBeenCalledOnce()
    expect(DropPhotos).toHaveBeenCalledWith('g', '', ['a', 'b'], 0.7, '')
    expect(dropAsk.cur).toMatchObject({ spread: 'g', fit: 3, total: 7, grow: false })
    expect(store.view.rev).toBe(1)
  })

  it.each([['add' as const], ['skip' as const]])('ответ «%s» — второй вызов с rest, окно закрыто', async (rest) => {
    DropPhotos.mockResolvedValueOnce(reply({ ask: true, fit: 3, total: 7, view: view(1) }))
    await dropGrid('g', 'pl', ['a'], -1, false)
    DropPhotos.mockResolvedValueOnce(reply())
    await answerDrop(rest)
    expect(DropPhotos).toHaveBeenLastCalledWith('g', 'pl', ['a'], -1, rest)
    expect(dropAsk.cur).toBeNull()
    expect(store.view.rev).toBe(2)
  })

  it('Esc и × — без вызова Go', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ ask: true, fit: 1, total: 2, view: view(1) }))
    await dropGrid('g', '', ['a', 'b'], -1, false)
    cancelDrop()
    await answerDrop('add')
    expect(dropAsk.cur).toBeNull()
    expect(DropPhotos).toHaveBeenCalledOnce()
    expect(store.view.rev).toBe(1)
  })

  it('нетронутая виньетка — grow, ручная — нет', async () => {
    store.apply(view(1, [grid('g', [], { pupils: 25, manual: false })]))
    DropPhotos.mockResolvedValue(reply({ ask: true, fit: 0, total: 2, view: view(1) }))
    await dropGrid('g', '', ['a', 'b'], -1, false)
    expect(dropAsk.cur?.grow).toBe(true)
    cancelDrop()
    store.apply(view(1, [grid('g', [], { pupils: 25, manual: true })]))
    await dropGrid('g', '', ['a', 'b'], -1, false)
    expect(dropAsk.cur?.grow).toBe(false)
  })

  it('вопрос не переживает другую правку и пропажу листа', async () => {
    DropPhotos.mockResolvedValue(reply({ ask: true, fit: 1, total: 2, view: view(1) }))
    await dropGrid('g', '', ['a', 'b'], -1, false)
    dropAsk.prune(view(1))
    expect(dropAsk.cur).not.toBeNull()
    dropAsk.prune(view(2))
    expect(dropAsk.cur).toBeNull()
    await dropGrid('g', '', ['a', 'b'], -1, false)
    dropAsk.prune(view(1, []))
    expect(dropAsk.cur).toBeNull()
  })

  it('бросок при открытом вопросе игнорируется (Enter под окном)', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ ask: true, fit: 1, total: 2, view: view(1) }))
    await dropGrid('g', '', ['a', 'b'], -1, false)
    await dropGrid('g', '', ['c'], -1, false)
    expect(DropPhotos).toHaveBeenCalledOnce()
    expect(dropAsk.cur).toMatchObject({ ids: ['a', 'b'] })
  })

  it('rev вопроса — из ответа Go, а не из store', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ ask: true, fit: 1, total: 2, view: view(5) }))
    await dropGrid('g', '', ['a', 'b'], -1, false)
    expect(dropAsk.cur?.rev).toBe(5)
  })

  it('тексты: разворот и лист, обычный лист и виньетка', () => {
    const t = dropAskText({ fit: 3, total: 7, grow: false }, unitOf({ sheet: false }))
    expect(t).toEqual({
      title: 'Не хватает свободных мест',
      ask: 'На свободные места встанут 3 из 7 фото. Что сделать с остальными?',
      skip: 'Не добавлять на разворот',
      add: 'Вставить на разворот в свободной форме',
    })
    const s = dropAskText({ fit: 0, total: 2, grow: false }, unitOf({ sheet: true }))
    expect([s.skip, s.add]).toEqual(['Не добавлять на лист', 'Вставить на лист в свободной форме'])
    expect(dropAskText({ fit: 0, total: 2, grow: true }, unitOf({ sheet: false })).add).toBe('Добавить учеников в виньетку')
  })
})

describe('после правки', () => {
  it('новые места выделяются на текущем листе, не на соседе', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ fresh: ['n1', 'n2'] }))
    await dropGrid('g', '', ['a'], -1, false)
    expect(select).toHaveBeenCalledWith(['n1', 'n2'])
    select.mockClear()
    DropPhotos.mockResolvedValueOnce(reply({ fresh: ['n3'], view: view(3) }))
    await dropGrid('a', '', ['b'], -1, false)
    expect(select).not.toHaveBeenCalled()
  })

  it('бросок на миниатюру (go) переходит на лист и выделяет новые места', async () => {
    store.apply(view(1, [{ id: 'a', photos: [], grid: false }, grid('g', ['p0'])]))
    store.goId('a')
    DropPhotos.mockResolvedValueOnce(reply({ fresh: ['n1'], view: view(2, [{ id: 'a', photos: [], grid: false }, grid('g', ['p0'])]) }))
    await dropGrid('g', '', ['x'], -1, false, true)
    expect(store.currentId).toBe('g')
    expect(select).toHaveBeenCalledWith(['n1'])
  })

  it('замена: тост с «Вернуть»', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ out: true }))
    await dropGrid('g', 'pl', ['a'], -1, false)
    expect(store.toast?.text).toBe('Прежнее фото вернулось в ленту')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('виньетка выросла: число учеников из нового View', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ grown: 2, view: view(2, [grid('g', [], { pupils: 27, manual: false })]) }))
    await dropGrid('g', '', ['a', 'b'], -1, false)
    expect(store.toast?.text).toBe('Учеников в виньетке: 27')
    expect(store.toast?.action).toBeTruthy()
  })

  it('мелкие места: подсказка про «+ Новый разворот»', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ small: true }))
    await dropGrid('g', '', ['a'], -1, false)
    expect(store.toast?.text).toBe('Места вышли мелкими — много фото удобнее бросить на «+ Новый разворот»')
  })

  it('несколько тостов складываются в один', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ out: true, small: true }))
    await dropGrid('g', 'pl', ['a', 'b'], -1, false)
    expect(store.toast?.text).toBe('Прежнее фото вернулось в ленту. Места вышли мелкими — много фото удобнее бросить на «+ Новый разворот»')
  })

  it('«Не добавлять» без тоста', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ left: 4 }))
    await dropGrid('g', '', ['a'], -1, false)
    expect(store.toast).toBeNull()
  })

  it('из ленты: выделение снято, «Перенесено из …» — только про фото, что встали на лист', async () => {
    store.selected.add('x')
    store.selected.add('y')
    DropPhotos.mockResolvedValueOnce(reply({ view: view(2, [grid('g', ['p0', 'x']), { id: 'a', photos: ['y'], grid: false }]) }))
    // x пришло с листа a, y — тоже, но на g не встало («Не добавлять»)
    store.apply(view(1, [grid('g', ['p0']), { id: 'a', photos: ['x', 'y'], grid: false }]))
    await dropGrid('g', '', ['x', 'y'], -1, false)
    expect(store.selected.size).toBe(0)
    expect(store.toast?.text).toBe('Фото перенесено из разворота 2')
  })

  it('из ленты + замена: «Перенесено из …» не теряется в общем тосте', async () => {
    store.apply(view(1, [grid('g', ['p0']), { id: 'a', photos: ['x'], grid: false }]))
    DropPhotos.mockResolvedValueOnce(reply({ out: true, view: view(2, [grid('g', ['p0', 'x']), { id: 'a', photos: [], grid: false }]) }))
    await dropGrid('g', 'pl', ['x'], -1, false)
    expect(store.toast?.text).toBe('Фото перенесено из разворота 2. Прежнее фото вернулось в ленту')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('ошибка Go — красный тост, вопроса нет, повтор возможен', async () => {
    DropPhotos.mockRejectedValueOnce('на развороте не может быть больше 64 мест')
    await dropGrid('g', '', ['a'], -1, false)
    expect(store.toast).toMatchObject({ kind: 'error', text: 'На развороте не может быть больше 64 мест' })
    expect(dropAsk.cur).toBeNull()
    DropPhotos.mockResolvedValueOnce(reply())
    await dropGrid('g', '', ['a'], -1, false)
    expect(DropPhotos).toHaveBeenCalledTimes(2)
  })
})

describe('dropOn и Enter', () => {
  it('на ручной лист — DropPhotos с долей ширины, не AddPhotos', async () => {
    DropPhotos.mockResolvedValueOnce(reply())
    await dropOn({ kind: 'photos', ids: ['a'] }, { kind: 'spread', spread: 'g', at: 0.8 })
    expect(DropPhotos).toHaveBeenCalledWith('g', '', ['a'], 0.8, '')
    expect(Api.AddPhotos).not.toHaveBeenCalled()
  })

  it('сосед без точки — at = −1', async () => {
    DropPhotos.mockResolvedValueOnce(reply())
    await dropOn({ kind: 'photos', ids: ['a'] }, { kind: 'spread', spread: 'g' })
    expect(DropPhotos).toHaveBeenCalledWith('g', '', ['a'], -1, '')
  })

  it('на место ручного листа — DropPhotos с id места', async () => {
    DropPhotos.mockResolvedValueOnce(reply())
    await dropOn({ kind: 'photos', ids: ['a', 'b'] }, { kind: 'place', spread: 'g', place: 'pl', photo: null })
    expect(DropPhotos).toHaveBeenCalledWith('g', 'pl', ['a', 'b'], -1, '')
    expect(Api.PlacePhotos).not.toHaveBeenCalled()
  })

  it('лист «Авто» — прежний AddPhotos', async () => {
    vi.mocked(Api.AddPhotos).mockResolvedValueOnce(view(2) as never)
    await dropOn({ kind: 'photos', ids: ['a'] }, { kind: 'spread', spread: 'a' })
    expect(Api.AddPhotos).toHaveBeenCalledWith('a', ['a'], -1)
    expect(DropPhotos).not.toHaveBeenCalled()
  })

  it('обложка: бросок на место — прежний PlacePhotos без вопроса', async () => {
    store.apply({
      rev: 1,
      folders: [],
      spreads: [],
      settings: { sheet: false },
      cover: { sheet: { id: 'cover', photos: [], grid: true, places: [] }, spine: 14 },
    } as never)
    vi.mocked(Api.PlacePhotos).mockResolvedValueOnce({ view: view(2), left: 0 } as never)
    await dropOn({ kind: 'photos', ids: ['a'] }, { kind: 'place', spread: 'cover', place: 'pl', photo: null })
    expect(Api.PlacePhotos).toHaveBeenCalledWith('cover', 'pl', ['a'])
    expect(DropPhotos).not.toHaveBeenCalled()
  })

  it('Enter / двойной щелчок на ручном листе — через хук, at = −1', async () => {
    const dropGridHook = vi.fn(async (spread: string, ids: string[]) => {
      await dropGrid(spread, '', ids, -1, false)
      return true
    })
    store.plug({ ...noHooks, select, dropGrid: dropGridHook })
    DropPhotos.mockResolvedValueOnce(reply())
    await store.addToCurrent(['a'])
    expect(dropGridHook).toHaveBeenCalledWith('g', ['a'])
    expect(DropPhotos).toHaveBeenCalledWith('g', '', ['a'], -1, '')
    expect(Api.AddPhotos).not.toHaveBeenCalled()
  })

  it('без хука Enter на ручном листе идёт прежним путём', async () => {
    vi.mocked(Api.AddPhotos).mockResolvedValueOnce(view(2) as never)
    await store.addToCurrent(['a'])
    expect(Api.AddPhotos).toHaveBeenCalledWith('g', ['a'], -1)
  })
})

// 060: имена из названий файлов — тост «Убрать имена» (или общий с «Вернуть», если есть свои заметки).
describe('имена из названий файлов (060)', () => {
  const ForgetFileNames = vi.mocked(Api.ForgetFileNames)
  const NAMES = 'Имена взяты из названий файлов: 2'

  it('named пуст — тоста нет', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ named: [] }))
    await dropGrid('g', '', ['a'], -1, false)
    expect(store.toast).toBeNull()
  })

  it('named без заметок — тост с числом и кнопкой «Убрать имена»', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ named: ['a', 'b'] }))
    await dropGrid('g', '', ['a', 'b'], -1, false)
    expect(store.toast?.text).toBe(NAMES)
    expect(store.toast?.action?.label).toBe('Убрать имена')
  })

  it('кнопка зовёт ForgetFileNames один раз с named и применяет View', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ named: ['a', 'b'] }))
    await dropGrid('g', '', ['a', 'b'], -1, false)
    ForgetFileNames.mockResolvedValueOnce(view(3))
    await store.toast!.action!.run()
    expect(ForgetFileNames).toHaveBeenCalledOnce()
    expect(ForgetFileNames).toHaveBeenCalledWith(['a', 'b'])
    expect(store.view.rev).toBe(3)
    expect(store.toast).toBeNull()
  })

  it('после чужой правки кнопка ничего не зовёт', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ named: ['a'] }))
    await dropGrid('g', '', ['a'], -1, false)
    const run = store.toast!.action!.run
    store.view = { ...store.view, rev: 5 } // правка прошла, а тост ещё на экране (событие view без смены)
    await run()
    expect(ForgetFileNames).not.toHaveBeenCalled()
    expect(store.toast?.text).toBe('Уже были другие изменения — имена не тронуты')
  })

  it('тост гаснет, когда ревизия сменилась (Ctrl+Z броска)', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ named: ['a'] }))
    await dropGrid('g', '', ['a'], -1, false)
    store.apply(view(1))
    expect(store.toast).toBeNull()
  })

  it('с заметками — один общий тост с текстом обоих и «Вернуть»', async () => {
    DropPhotos.mockResolvedValueOnce(reply({ out: true, small: true, named: ['a', 'b'] }))
    await dropGrid('g', 'pl', ['a', 'b'], -1, false)
    expect(store.toast?.text).toBe(
      `Прежнее фото вернулось в ленту. Места вышли мелкими — много фото удобнее бросить на «+ Новый разворот». ${NAMES}`,
    )
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('из ленты: «Перенесено из …» не теряется рядом с именами', async () => {
    store.apply(view(1, [grid('g', ['p0']), { id: 'a', photos: ['x'], grid: false }]))
    DropPhotos.mockResolvedValueOnce(reply({ named: ['x'], view: view(2, [grid('g', ['x']), { id: 'a', photos: [], grid: false }]) }))
    await dropGrid('g', '', ['x'], -1, false)
    expect(store.toast?.text).toBe('Фото перенесено из разворота 2. Имена взяты из названий файлов: 1')
    expect(store.toast?.action?.label).toBe('Убрать имена')
  })

  it('обложка: PlacePhotos с named — тот же тост', async () => {
    store.apply({
      rev: 1,
      folders: [],
      spreads: [],
      settings: { sheet: false },
      cover: { sheet: { id: 'cover', photos: [], grid: true, places: [] }, spine: 14 },
    } as never)
    vi.mocked(Api.PlacePhotos).mockResolvedValueOnce({ view: view(2), left: 0, named: ['a'] } as never)
    await dropOn({ kind: 'photos', ids: ['a'] }, { kind: 'place', spread: 'cover', place: 'pl', photo: null })
    expect(store.toast?.text).toBe('Имена взяты из названий файлов: 1')
    expect(store.toast?.action?.label).toBe('Убрать имена')
  })

  describe('обложка: «Перенесено из …» не перетирается', () => {
    const cover = { id: 'cover', photos: [], grid: true, places: [{ id: 'pl', photo: '' }] }
    const src = (photos: string[]) => ({ id: 'a', photos, grid: false })
    // landed — фото, которые встали на обложку после правки.
    const withCover = (rev: number, a: string[], landed: string[] = []) =>
      ({
        rev,
        folders: [],
        spreads: [src(a)],
        settings: { sheet: false },
        cover: { sheet: { ...cover, photos: landed }, spine: 14 },
      }) as never
    const drop = () => dropOn({ kind: 'photos', ids: ['x'] }, { kind: 'place', spread: 'cover', place: 'pl', photo: null })

    it('с именами', async () => {
      store.apply(withCover(1, ['x']))
      vi.mocked(Api.PlacePhotos).mockResolvedValueOnce({ view: withCover(2, [], ['x']), left: 0, named: ['x'] } as never)
      await drop()
      expect(store.toast?.text).toBe('Фото перенесено из разворота 1. Имена взяты из названий файлов: 1')
    })
    it('с «не хватило мест»', async () => {
      store.apply(withCover(1, ['x']))
      vi.mocked(Api.PlacePhotos).mockResolvedValueOnce({ view: withCover(2, ['y'], ['x']), left: 1 } as never)
      await drop()
      expect(store.toast?.text).toBe('Фото перенесено из разворота 1. Не хватило мест: 1 фото остались в ленте')
    })
    it('фото не встало на обложку — «перенесено» не пишем', async () => {
      store.apply(withCover(1, ['x']))
      vi.mocked(Api.PlacePhotos).mockResolvedValueOnce({ view: withCover(2, ['x']), left: 1 } as never)
      await drop()
      expect(store.toast?.text).toBe('Не хватило мест: 1 фото остались в ленте')
    })
    it('из разворота (не из ленты) — текста о переносе нет', async () => {
      store.apply(withCover(1, ['x']))
      store.selected.add('x')
      vi.mocked(Api.PlacePhotos).mockResolvedValueOnce({ view: withCover(2, []), left: 0 } as never)
      await dropOn({ kind: 'photos', ids: ['x'], fromSpread: 'a' }, { kind: 'place', spread: 'cover', place: 'pl', photo: null })
      expect(store.toast).toBeNull()
    })
  })

  it('«Заполнить»: named — тост, «не хватило мест» идёт в тот же текст', async () => {
    store.plug({ ...noHooks, select, pick: () => ({ ids: ['a', 'b'], skipped: 0, from: 'free', inAlbum: 0, filtered: false }) })
    vi.mocked(Api.FillPlaces).mockResolvedValueOnce({ view: view(2), left: 1, named: ['a'] } as never)
    await store.fillPlaces()
    expect(store.toast?.text).toBe('Не хватило мест: 1 фото остались в ленте. Имена взяты из названий файлов: 1')
    expect(store.toast?.action?.label).toBe('Убрать имена')
  })

  it('«Заполнить» без named — тоста нет', async () => {
    store.plug({ ...noHooks, select, pick: () => ({ ids: ['a'], skipped: 0, from: 'free', inAlbum: 0, filtered: false }) })
    vi.mocked(Api.FillPlaces).mockResolvedValueOnce({ view: view(2), left: 0, named: [] } as never)
    await store.fillPlaces()
    expect(store.toast).toBeNull()
  })
})

describe('бросок на кнопку «Подборки» (083)', () => {
  it('фото из ленты уходят в хук подборок; с холста — нет', async () => {
    const hook = vi.spyOn(dropHooks, 'bin').mockResolvedValue()
    await dropOn({ kind: 'photos', ids: ['a', 'b'] }, { kind: 'bin' })
    expect(hook).toHaveBeenCalledWith(['a', 'b'])
    await dropOn({ kind: 'photos', ids: ['a'], fromSpread: 's1' }, { kind: 'bin' })
    expect(hook).toHaveBeenCalledOnce()
    hook.mockRestore()
  })
})

// 098: ворота копии стоят в dropGrid — для броска, двойного щелчка и Enter; с холста (fromSpread) вопроса нет.
describe('ворота копии (098)', () => {
  const gate = vi.fn()
  beforeEach(() => {
    gate.mockReset()
    gate.mockImplementation(async (ids: string[]) => ids)
    store.plug({ ...noHooks, select, gate })
  })

  it('из ленты: Go получает ответ ворот (запрос копии)', async () => {
    gate.mockResolvedValueOnce(['a-new', 'b'])
    DropPhotos.mockResolvedValueOnce(reply())
    await dropGrid('g', '', ['a', 'b'], -1, false)
    expect(gate).toHaveBeenCalledExactlyOnceWith(['a', 'b'])
    expect(DropPhotos).toHaveBeenCalledExactlyOnceWith('g', '', ['a-new', 'b'], -1, '')
  })

  it('«Отмена»: Go не зовём, повторный бросок снова проходит', async () => {
    gate.mockResolvedValueOnce(null)
    await dropGrid('g', '', ['a'], -1, false)
    expect(DropPhotos).not.toHaveBeenCalled()
    DropPhotos.mockResolvedValueOnce(reply())
    await dropGrid('g', '', ['a'], -1, false)
    expect(DropPhotos).toHaveBeenCalledOnce()
  })

  it('с холста (fromSpread) — ворот нет', async () => {
    DropPhotos.mockResolvedValueOnce(reply())
    await dropGrid('g', 'pl', ['a-2'], -1, true)
    expect(gate).not.toHaveBeenCalled()
    expect(DropPhotos).toHaveBeenCalledExactlyOnceWith('g', 'pl', ['a-2'], -1, '')
  })

  it('пока открыт вопрос ворот, второй бросок игнорируется', async () => {
    let answer: (v: string[] | null) => void = () => {}
    gate.mockImplementationOnce(() => new Promise((r) => (answer = r)))
    const first = dropGrid('g', '', ['a'], -1, false)
    await dropGrid('g', '', ['b'], -1, false)
    expect(gate).toHaveBeenCalledOnce()
    DropPhotos.mockResolvedValueOnce(reply())
    answer(['a-new'])
    await first
    expect(DropPhotos).toHaveBeenCalledExactlyOnceWith('g', '', ['a-new'], -1, '')
  })
})
