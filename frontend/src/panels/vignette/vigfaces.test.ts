import { beforeEach, describe, expect, it, vi } from 'vitest'

// Store, Go и выделение подменены: проверяем, что уходит в Go, что делается с ответом и что видно человеку.
// gridAct настоящий — окно 098 живёт в нём.
const state = vi.hoisted(() => ({ rev: 5, currentId: 'a', progress: null as null | ((d: number, t: number) => void) }))
vi.mock('../../base/core/store.svelte', () => ({
  store: {
    view: {
      get rev() {
        return state.rev
      },
      spreads: [
        { id: 'a', grid: true },
        { id: 'b', grid: true },
      ],
    },
    get currentId() {
      return state.currentId
    },
    take: vi.fn(async () => true),
    notify: vi.fn(),
    goId: vi.fn(),
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
const select = vi.hoisted(() => vi.fn())
vi.mock('../../editor/places/placesel.svelte', () => ({ placesel: { select } }))
const off = vi.hoisted(() => vi.fn())
const fit = vi.hoisted(() => vi.fn())
vi.mock('./facesapi', () => ({
  fitFaces: fit,
  onFacesProgress: (fn: (d: number, t: number) => void) => {
    state.progress = fn
    return off
  },
}))

import { store } from '../../base/core/store.svelte'
import { gridAct } from '../layouts/gridact.svelte'
import { FACES_ASK, faceShowIds, faceText, facesLabel, facesTotal } from './vigfaces'
import { vigFaces } from './vigfaces.svelte'
import type { FaceReport } from './facesapi'
import { articleById } from '../../shell/guidetext/articles'
import { framelive } from '../../editor/photo/framelive.svelte'

const rep = (o: Partial<FaceReport> = {}): FaceReport => ({
  fitted: 0,
  noFace: 0,
  many: 0,
  missing: 0,
  small: 0,
  big: 0,
  edge: 0,
  contain: 0,
  manual: 0,
  changed: 0,
  check: [],
  short: [],
  asked: false,
  ...o,
})
const ok = (o: Partial<FaceReport> = {}, rev = 6) => ({ view: { rev }, report: rep(o) })

beforeEach(() => {
  vi.clearAllMocks()
  gridAct.cancelAuto()
  state.rev = 5
  state.currentId = 'a'
  state.progress = null
})

describe('facesLabel: ход на кнопке', () => {
  it('без поиска — «Подогнать по лицам», в ходу — «Лица 12 / 25», число мест ещё неизвестно — «Лица…»', () => {
    expect(facesLabel(null)).toBe('Подогнать по лицам')
    expect(facesLabel({ done: 12, total: 25 })).toBe('Лица 12 / 25')
    expect(facesLabel({ done: 0, total: 0 })).toBe('Лица…')
  })
})

describe('faceText: три группы', () => {
  it('все группы по спеке: 20 из 25, нулевые не пишутся', () => {
    const r = rep({ fitted: 20, noFace: 2, many: 1, small: 1, contain: 1 })
    expect(faceText(r)).toBe('Подогнано 20 из 25 · Проверьте 3 · Не по норме 1 (мелкое) · Не тронуто 1 («Целиком»)')
    expect(facesTotal(r)).toBe(25)
  })

  it('причина одна на группу — в скобках; две — без скобок', () => {
    expect(faceText(rep({ fitted: 10, noFace: 2 }))).toBe('Подогнано 10 из 12 · Проверьте 2 (без лица)')
    expect(faceText(rep({ fitted: 10, noFace: 1, missing: 1 }))).toBe('Подогнано 10 из 12 · Проверьте 2')
  })

  it('«Не по норме» и «Не тронуто» собирают свои причины', () => {
    const r = rep({ fitted: 1, small: 1, big: 1, edge: 1, manual: 2, changed: 1 })
    expect(faceText(r)).toBe('Подогнано 1 из 7 · Не по норме 3 · Не тронуто 3')
  })

  it('пример спеки (20 из 25, три группы) короткий; без скобок, когда причин в группе несколько', () => {
    const t = faceText(rep({ fitted: 20, noFace: 1, many: 1, missing: 1, small: 1, big: 1, contain: 1, manual: 1 }))
    expect(t).toBe('Подогнано 20 из 27 · Проверьте 3 · Не по норме 2 · Не тронуто 2')
    expect(t.length).toBeLessThanOrEqual(65)
  })

  it('холостая: «Лица уже подогнаны: менять нечего» и те же группы', () => {
    expect(faceText(rep({ fitted: 22, noFace: 3 }), true)).toBe('Лица уже подогнаны: менять нечего · Проверьте 3 (без лица)')
    expect(faceText(rep({ fitted: 25 }), true)).toBe('Лица уже подогнаны: менять нечего')
  })

  it('на листе нет фото для подгона', () => {
    expect(faceText(rep())).toBe('На листе нет фото для подгона')
    expect(faceText(rep(), true)).toBe('На листе нет фото для подгона')
  })

  it('«Показать» выделяет первые две группы без повторов', () => {
    expect(faceShowIds(rep({ check: ['p1', 'p2'], short: ['p3', 'p1'] }))).toEqual(['p1', 'p2', 'p3'])
    expect(faceShowIds(rep())).toEqual([])
  })
})

describe('vigFaces.fit: поиск и итог', () => {
  it('поиск уходит в Go без образца и режима; итог — тост на 20 с с «Показать»', async () => {
    fit.mockResolvedValue(ok({ fitted: 20, noFace: 3, check: ['p1', 'p2', 'p3'] }))
    expect(await vigFaces.fit('a')).toBe(true)
    expect(fit).toHaveBeenCalledWith('a', '', '')
    expect(store.take).toHaveBeenCalledWith({ rev: 6 })
    const [text, kind, action, ms] = vi.mocked(store.notify).mock.calls[0]
    expect(text).toBe('Подогнано 20 из 23 · Проверьте 3 (без лица)')
    expect(kind).toBe('info')
    expect(action?.label).toBe('Показать')
    expect(ms).toBe(20000)
  })

  it('нечего проверять — тост тоже 20 с, но без «Показать»', async () => {
    fit.mockResolvedValue(ok({ fitted: 25, manual: 0 }))
    await vigFaces.fit('a')
    const [, , action, ms] = vi.mocked(store.notify).mock.calls[0]
    expect(action).toBeUndefined()
    expect(ms).toBe(20000)
  })

  it('повтор: Go ревизию не поменял — тост «Лица уже подогнаны: менять нечего»', async () => {
    fit.mockResolvedValue(ok({ fitted: 24, noFace: 1, check: ['p1'] }, 5))
    await vigFaces.fit('a')
    expect(vi.mocked(store.notify).mock.calls[0][0]).toBe('Лица уже подогнаны: менять нечего · Проверьте 1 (без лица)')
  })

  it('правка прошла: событие "view" подняло ревизию до ответа — тост «Подогнано», не «менять нечего»', async () => {
    fit.mockImplementation(async () => {
      state.rev = 6 // store принял вид из события раньше ответа
      return ok({ fitted: 20, noFace: 3, check: ['p1'] }, 6)
    })
    await vigFaces.fit('a')
    expect(vi.mocked(store.notify).mock.calls[0][0]).toBe('Подогнано 20 из 23 · Проверьте 3 (без лица)')
  })

  it('ход: «12 / 25» только на своём листе; отписка и свободная кнопка после ответа', async () => {
    let done: (v: unknown) => void = () => {}
    fit.mockReturnValue(new Promise((r) => (done = r)))
    const p = vigFaces.fit('a')
    expect(vigFaces.busy).toBe(true)
    expect(vigFaces.progress('a')).toEqual({ done: 0, total: 0 })
    state.progress?.(12, 25)
    expect(vigFaces.progress('a')).toEqual({ done: 12, total: 25 })
    expect(vigFaces.progress('b')).toBeNull()
    done(ok({ fitted: 25 }))
    await p
    expect(off).toHaveBeenCalledTimes(1)
    expect(vigFaces.busy).toBe(false)
    expect(vigFaces.progress('a')).toBeNull()
  })

  it('занятость: повторное нажатие (и на другом листе) во время поиска пропускается, Go зовётся один раз', async () => {
    let done: (v: unknown) => void = () => {}
    fit.mockReturnValue(new Promise((r) => (done = r)))
    const first = vigFaces.fit('a')
    expect(await vigFaces.fit('a')).toBe(false)
    expect(await vigFaces.fit('b')).toBe(false)
    expect(fit).toHaveBeenCalledTimes(1)
    done(ok({ fitted: 1 }))
    await first
    fit.mockResolvedValue(ok({ fitted: 1 }))
    expect(await vigFaces.fit('b')).toBe(true)
    expect(fit).toHaveBeenCalledTimes(2)
  })

  it('ошибка Go — красный тост, вид не принят, кнопка свободна', async () => {
    fit.mockRejectedValue('Уже ищу лица')
    expect(await vigFaces.fit('a')).toBe(false)
    expect(store.take).not.toHaveBeenCalled()
    expect(store.notify).toHaveBeenCalledWith('Уже ищу лица', 'error')
    expect(vigFaces.busy).toBe(false)
    expect(off).toHaveBeenCalledTimes(1)
  })
})

describe('окно 098: правленые кадры', () => {
  const asked = () => fit.mockResolvedValueOnce(ok({ asked: true, manual: 7 }, 5))

  it('правленых нет — окна нет; есть — окно с вопросом спеки, N в галочке, правки и тоста нет', async () => {
    asked()
    await vigFaces.fit('a')
    const a = gridAct.autoAsk
    expect(a?.glue).toMatchObject({
      title: 'Подогнать по лицам?',
      ask: 'Кадр у части фото уже настроен. Подогнать по лицам?',
      ok: 'Подогнать',
    })
    expect(a?.glue?.title).toBe(FACES_ASK.title)
    expect(a?.crops).toBe(7)
    expect(a?.keep).toBe(true)
    expect(store.take).not.toHaveBeenCalled()
    expect(store.notify).not.toHaveBeenCalled()
    expect(vigFaces.busy).toBe(false)
  })

  it('«Подогнать» с галочкой — второй вызов keep; без галочки — all', async () => {
    asked()
    await vigFaces.fit('a')
    fit.mockResolvedValue(ok({ fitted: 18, manual: 7 }))
    await gridAct.confirmAuto()
    expect(fit).toHaveBeenLastCalledWith('a', '', 'keep')
    expect(vi.mocked(store.notify).mock.calls[0][0]).toBe('Подогнано 18 из 25 · Не тронуто 7 (вручную)')

    asked()
    await vigFaces.fit('a')
    gridAct.setKeep(false)
    await gridAct.confirmAuto()
    expect(fit).toHaveBeenLastCalledWith('a', '', 'all')
  })

  it('«Отмена» — второго вызова нет, лист не тронут', async () => {
    asked()
    await vigFaces.fit('a')
    gridAct.cancelAuto()
    expect(fit).toHaveBeenCalledTimes(1)
    expect(gridAct.autoAsk).toBeNull()
    expect(store.take).not.toHaveBeenCalled()
  })
})

describe('«Показать»', () => {
  it('на том же листе — выделяет места без перехода', () => {
    vigFaces.show('a', ['p1', 'p2'])
    expect(store.goId).not.toHaveBeenCalled()
    expect(select).toHaveBeenCalledWith(['p1', 'p2'])
  })

  it('с другого листа — сначала переход, потом выделение', () => {
    state.currentId = 'b'
    const order: string[] = []
    vi.mocked(store.goId).mockImplementation(() => void order.push('go'))
    select.mockImplementation(() => void order.push('select'))
    vigFaces.show('a', ['p1'])
    expect(order).toEqual(['go', 'select'])
    expect(store.goId).toHaveBeenCalledWith('a')
  })

  it('кнопка тоста ведёт к местам листа, где шёл подгон', async () => {
    fit.mockResolvedValue(ok({ fitted: 3, noFace: 1, small: 1, check: ['p1'], short: ['p2'] }))
    await vigFaces.fit('a')
    state.currentId = 'b'
    vi.mocked(store.notify).mock.calls[0][2]?.run()
    expect(store.goId).toHaveBeenCalledWith('a')
    expect(select).toHaveBeenCalledWith(['p1', 'p2'])
  })
})

describe('130 U3: силуэт узнаёт о смене нормы листа', () => {
  it('ответ подгона поднимает framelive.normRev (rev нормы не меняет), ошибка — нет', async () => {
    const n = framelive.normRev
    fit.mockResolvedValue(ok({ fitted: 3 }))
    await vigFaces.fit('a', '', 'p1')
    expect(framelive.normRev).toBe(n + 1)
    fit.mockRejectedValue('сбой')
    await vigFaces.fit('a')
    expect(framelive.normRev).toBe(n + 1)
  })
})

describe('статья F1 vig-silhouette', () => {
  it('есть, привязана к пункту меню «Вид ▸ Силуэт под лицо» и коротко описана', () => {
    const a = articleById('vig-silhouette')
    expect(a?.feature).toBe('vignette')
    expect(a?.menu).toEqual(['faceGuide'])
    expect((a?.what ?? '').length).toBeLessThanOrEqual(220)
    expect(a?.how).toContain('В выгрузку не попадает')
  })
})

describe('статья F1 vig-faces', () => {
  it('есть, привязана к кнопке карточки; про одно фото — «Сбросить кадр», затем кнопка; три группы названы', () => {
    const a = articleById('vig-faces')
    expect(a?.spot).toBe('ui:vig-faces')
    expect(a?.how).toContain('Вернуть одно фото к норме — «Сбросить кадр», затем кнопка')
    for (const g of ['Проверьте', 'Не по норме', 'Не тронуто']) expect(a?.how).toContain(g)
    expect(a?.how).toContain('Нажмите ещё раз — отчёт появится снова')
  })

  it('130 U2: назван путь к пункту «Кадр» ▸ «Остальные — как это лицо»', () => {
    expect(articleById('vig-faces')?.how).toContain('правый щелчок по нему ▸ «Кадр» ▸ «Остальные — как это лицо»')
  })
})
