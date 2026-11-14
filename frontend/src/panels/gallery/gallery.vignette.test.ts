import { beforeEach, describe, expect, it, vi } from 'vitest'

// Решение 80: на виньетке галерея и листание работают с шаблонами виньетки, а не с раскладками «Авто».
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  StepLayout: vi.fn(async () => ({})),
  LayoutChoices: vi.fn(async () => ({})),
  SetLayout: vi.fn(async () => ({})),
  SetGrid: vi.fn(async () => ({})),
  SetPlaces: vi.fn(async () => ({})),
  MirrorGrid: vi.fn(async () => ({})),
  DeleteSelection: vi.fn(async () => ({})),
  ResetVignette: vi.fn(async () => ({ view: { rev: 7, marksRev: 0, folders: [], spreads: [] }, left: 0 })),
  SetVignette: vi.fn(async () => ({ view: { rev: 6, marksRev: 0, folders: [], spreads: [] }, left: 0 })),
  VignetteChoices: vi.fn(async () => [
    { id: 'a', over: false },
    { id: 'b', over: false },
  ]),
  Marks: async () => ({}),
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const Api = await import('../../../wailsjs/go/app/App')
const { store } = await import('../../base/core/store.svelte')
const { gallery } = await import('./gallery.svelte')
const { gridAct, autoAskText } = await import('../layouts/gridact.svelte')
const { placesel } = await import('../../editor/places/placesel.svelte')
const { resetVignette } = await import('../vignette/vigown')

const vigSpread = {
  id: 's',
  photos: [],
  layout: '',
  grid: true,
  kind: 'vignette',
  vig: { tpl: 'a', teachers: 1, pupils: 24, ratio: '3:4', places: 25, max: 64, group: false },
}

beforeEach(() => {
  vi.clearAllMocks()
  gallery.close()
  store.apply({ rev: 5, marksRev: 0, folders: [], spreads: [vigSpread] } as never)
})

describe('галерея и листание на виньетке', () => {
  it('step листает шаблоны виньетки, а не раскладки', async () => {
    await gallery.step('spread', 1)
    expect(Api.StepLayout).not.toHaveBeenCalled()
    expect(Api.SetVignette).toHaveBeenCalledWith('s', expect.objectContaining({ tpl: 'b', teachers: 1, pupils: 24 }))
  })

  it('open открывает галерею только раскладок; шаблонов сетки у виньетки нет', () => {
    gallery.open('left', 'grids')
    expect(gallery.cur).toBeNull()
    gallery.open('left')
    expect(gallery.cur).toEqual({ spread: 's', scope: 'spread', tab: 'layouts' })
  })

  it('sync не запрашивает раскладки «Авто»', () => {
    gallery.open('spread')
    gallery.sync()
    expect(Api.LayoutChoices).not.toHaveBeenCalled()
    expect(gallery.cur).not.toBeNull()
  })

  it('setLayout на виньетке ничего не зовёт: «Авто» из галереи недоступно', async () => {
    gallery.open('spread')
    await gallery.setLayout('x')
    expect(Api.SetLayout).not.toHaveBeenCalled()
    expect(Api.SetGrid).not.toHaveBeenCalled()
    expect(gallery.cur).toBeNull()
  })
})

// Ручная виньетка (038): как у страницы, места сбрасываются — сначала вопрос AutoAskDialog.
describe('вопрос на ручной виньетке', () => {
  const manual = { ...vigSpread, vig: { ...vigSpread.vig, manual: true } }
  beforeEach(() => {
    gridAct.cancelAuto()
    store.apply({ rev: 5, marksRev: 0, folders: [], spreads: [manual] } as never)
  })

  it('‹ › и Пробел: вопрос про шаблон, до ответа Go не зовётся', async () => {
    await gallery.step('spread', 1)
    expect(gridAct.autoAsk).toMatchObject({ spread: 's', vig: { kind: 'tpl' } })
    expect(Api.SetVignette).not.toHaveBeenCalled()
  })

  it('«Отмена» ничего не меняет', async () => {
    await gallery.step('spread', 1)
    gridAct.cancelAuto()
    expect(gridAct.autoAsk).toBeNull()
    expect(Api.SetVignette).not.toHaveBeenCalled()
  })

  it('«Сменить шаблон» ставит следующий шаблон', async () => {
    await gallery.step('spread', 1)
    await gridAct.confirmAuto()
    expect(gridAct.autoAsk).toBeNull()
    expect(Api.SetVignette).toHaveBeenCalledWith('s', expect.objectContaining({ tpl: 'b' }))
    expect(Api.SetGrid).not.toHaveBeenCalled()
  })

  it('«Вернуть авто» спрашивает и после «да» зовёт ResetVignette', async () => {
    expect(await gridAct.vigGate('s', 'reset', () => resetVignette('s'))).toBe(false)
    expect(gridAct.autoAsk?.vig?.kind).toBe('reset')
    expect(Api.ResetVignette).not.toHaveBeenCalled()
    await gridAct.confirmAuto()
    expect(Api.ResetVignette).toHaveBeenCalledWith('s')
  })

  it('нетронутая виньетка — без вопроса, как у страницы', async () => {
    store.apply({ rev: 6, marksRev: 0, folders: [], spreads: [vigSpread] } as never)
    const run = vi.fn(async () => {})
    expect(await gridAct.vigGate('s', 'tpl', run)).toBe(true)
    expect(run).toHaveBeenCalledOnce()
    expect(gridAct.autoAsk).toBeNull()
  })

  it('вопрос не переживает лист, ставший нетронутым (Отменить)', async () => {
    await gallery.step('spread', 1)
    gridAct.prune([vigSpread as never])
    expect(gridAct.autoAsk).toBeNull()
  })

  it('тексты: страница — «Вернуть авто?», виньетка — смена шаблона и возврат', () => {
    expect(autoAskText().title).toBe('Вернуть авто?')
    const noop = async () => {}
    expect(autoAskText({ kind: 'tpl', run: noop }).ask).toBe('Ваши правки мест сбросятся. Сменить шаблон?')
    expect(autoAskText({ kind: 'reset', run: noop }).ask).toBe('Ваши правки мест сбросятся. Вернуть авто?')
  })
})

describe('тост первой ручной правки виньетки', () => {
  const after = (manual: boolean) => ({
    rev: 6,
    marksRev: 0,
    folders: [],
    spreads: [{ ...vigSpread, vig: { ...vigSpread.vig, manual } }],
  })

  it('нетронутая стала ручной — «Виньетка стала ручной» с «Вернуть»', async () => {
    vi.mocked(Api.SetPlaces).mockResolvedValueOnce(after(true) as never)
    expect(await gridAct.placeEdit('s', Api.SetPlaces('s', [], ''))).toBe(true)
    expect(store.toast?.text).toMatch(/^Виньетка стала ручной/)
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('уже ручная — повторного тоста нет', async () => {
    store.apply({ ...after(true), rev: 5 } as never)
    store.toast = null
    vi.mocked(Api.SetPlaces).mockResolvedValueOnce({ ...after(true), rev: 6 } as never)
    await gridAct.placeEdit('s', Api.SetPlaces('s', [], ''))
    expect(store.toast).toBeNull()
  })

  it('«Отразить» перевело виньетку в ручную — тот же тост', async () => {
    vi.mocked(Api.MirrorGrid).mockResolvedValueOnce(after(true) as never)
    await gridAct.mirrorGrid('spread')
    expect(store.toast?.text).toMatch(/^Виньетка стала ручной/)
  })

  it('«Удалить» выделение перевело виньетку в ручную — тот же тост', async () => {
    vi.mocked(Api.DeleteSelection).mockResolvedValueOnce(after(true) as never)
    placesel.texts = ['t1']
    await placesel.deleteSelected()
    expect(Api.DeleteSelection).toHaveBeenCalled()
    expect(store.toast?.text).toMatch(/^Виньетка стала ручной/)
  })

  it('«Отразить» без смены ручности — тоста нет', async () => {
    vi.mocked(Api.MirrorGrid).mockResolvedValueOnce(after(false) as never)
    await gridAct.mirrorGrid('spread')
    expect(store.toast?.text ?? '').not.toMatch(/Виньетка стала ручной/)
  })
})
