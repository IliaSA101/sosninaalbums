import { describe, expect, it, vi } from 'vitest'

// Go и store не нужны: окно получает вызовы через VigDeps, vigact тянет store только при настоящем вызове.
vi.mock('../../base/core/store.svelte', () => ({ store: {}, Api: {} }))

import type { SpreadView, VigChoice, VigPreview, VignetteView } from '../../base/core/types'
import {
  CARD_MAX_H,
  CARD_W,
  STRIP_MIN,
  VAR_FIELDS,
  cardBox,
  countsText,
  dialogBlock,
  emptyPlaces,
  fitBox,
  gridStep,
  leftLine,
  placesLine,
  previewBox,
  resetWarning,
  revealX,
  sameAsDialog,
  sameVars,
  setupOf,
  stripMax,
  summaryText,
  varsFormOf,
  varsOf,
} from './vigdialog'
import { autoAskText } from '../layouts/gridact.svelte'
import { Latest, VigSession, vigDialog, type VigDeps } from './vigsession.svelte'

const vig = (o: Partial<VignetteView> = {}): VignetteView => ({
  tpl: 'head-t-left',
  teachers: 1,
  pupils: 23,
  ratio: '3:4',
  places: 24,
  max: 64,
  group: false,
  ...o,
})
const choice = (o: Partial<VigChoice> = {}): VigChoice => ({
  id: 'head-t-left',
  name: 'Шапка, учителя слева',
  boxes: [],
  roles: [],
  pupilW: 35,
  small: false,
  over: false,
  ...o,
})
const sheet = (o: Partial<SpreadView> = {}) =>
  ({ id: 's1', kind: 'vignette', vig: vig(), places: [], ...o }) as unknown as SpreadView & { vig: VignetteView }

describe('сводка карточки: склонения', () => {
  it('учителя: 0 — части нет; 1 / 2 / 5 / 21', () => {
    expect(countsText(vig({ teachers: 0 }))).toBe('23 ученика')
    expect(countsText(vig({ teachers: 1 }))).toBe('1 учитель · 23 ученика')
    expect(countsText(vig({ teachers: 2 }))).toBe('2 учителя · 23 ученика')
    expect(countsText(vig({ teachers: 5 }))).toBe('5 учителей · 23 ученика')
    expect(countsText(vig({ teachers: 21 }))).toBe('21 учитель · 23 ученика')
  })
  it('ученики: 1 / 23 / 25; групповое фото — в конце', () => {
    expect(countsText(vig({ pupils: 1 }))).toBe('1 учитель · 1 ученик')
    expect(countsText(vig({ pupils: 23 }))).toBe('1 учитель · 23 ученика')
    expect(countsText(vig({ pupils: 25, group: true }))).toBe('1 учитель · 25 учеников · групповое')
  })
  it('summaryText: размер портрета из карточки Go; «не поместится» и отсутствие карточки — без него', () => {
    expect(summaryText(vig(), choice())).toBe('1 учитель · 23 ученика · портрет 35 мм')
    expect(summaryText(vig(), choice({ pupilW: 35.24 }))).toBe('1 учитель · 23 ученика · портрет 35,2 мм')
    expect(summaryText(vig(), choice({ over: true }))).toBe('1 учитель · 23 ученика')
    expect(summaryText(vig(), undefined)).toBe('1 учитель · 23 ученика')
  })
  it('пустые места — только роли без фото', () => {
    const places = [{ photo: 'a' }, { photo: '' }, { photo: '' }] as unknown as SpreadView['places']
    expect(emptyPlaces({ places })).toBe(2)
    expect(emptyPlaces({ places: null })).toBe(0)
  })
})

describe('низ окна', () => {
  it('«Мест N из 64» и «в ленту уйдут K фото»', () => {
    expect(placesLine(24, 64)).toBe('Мест 24 из 64')
    expect(leftLine(2)).toBe('в ленту уйдут 2 фото')
    expect(leftLine(0)).toBe('')
  })
})

describe('«Применить»: когда закрыто', () => {
  const ok = { busy: false, tpl: 't', total: 24, max: 64, same: false }
  it('можно, если всё есть и форма изменилась', () => {
    expect(dialogBlock(ok)).toBeNull()
  })
  it('больше 64 мест, без шаблона, идёт применение, ничего не изменилось', () => {
    expect(dialogBlock({ ...ok, total: 65 })).toMatch(/Не больше 64 мест/)
    expect(dialogBlock({ ...ok, tpl: '' })).toBe('Выберите шаблон')
    expect(dialogBlock({ ...ok, busy: true })).toBe('Идёт пересборка…')
    expect(dialogBlock({ ...ok, same: true })).toBe('Ничего не изменилось')
  })
  it('ровно 64 мест — можно', () => {
    expect(dialogBlock({ ...ok, total: 64 })).toBeNull()
  })
})

describe('поля шапки', () => {
  it('четыре поля, образцы не пустые, пустое поле не уходит в Go', () => {
    expect(VAR_FIELDS.map((f) => f.label)).toEqual(['Заголовок', 'Подзаголовок', 'Год', 'Руководитель'])
    expect(varsFormOf(undefined)).toEqual({ title: '', subtitle: '', year: '', lead: '' })
    expect(varsOf({ title: 'Гимназия', subtitle: '', year: '2026', lead: '' })).toEqual({ title: 'Гимназия', year: '2026' })
  })
  it('sameAsDialog: форма и поля совпадают с листом', () => {
    const sp = sheet({ vars: { title: 'А' } })
    const f = { tpl: 'head-t-left', teachers: 1, pupils: 23, ratio: '3:4' as const, ttext: '' }
    expect(sameAsDialog(f, varsFormOf({ title: 'А' }), sp)).toBe(true)
    expect(sameAsDialog(f, varsFormOf({ title: 'Б' }), sp)).toBe(false)
    expect(sameAsDialog({ ...f, pupils: 20 }, varsFormOf({ title: 'А' }), sp)).toBe(false)
    expect(sameVars(varsFormOf({ year: '1' }), varsFormOf({ year: '1' }))).toBe(true)
  })
  it('setupOf: параметры и поля одним запросом', () => {
    const s = setupOf({ tpl: 't', teachers: 2, pupils: 10, ratio: '4:5', ttext: 'x' }, varsFormOf({ lead: 'И. И.' }))
    expect(s).toEqual({ vig: { tpl: 't', teachers: 2, pupils: 10, ratio: '4:5', ttext: 'x', head: '' }, vars: { lead: 'И. И.' } })
  })
})

describe('размеры предпросмотра', () => {
  it('лист 2×300×300 — 560×280, 204×304 — ≈ 295×440', () => {
    expect(previewBox(600, 300, 568, 760)).toEqual({ w: 560, h: 280 })
    expect(previewBox(204, 304, 568, 760)).toEqual({ w: 295, h: 440 })
  })
  it('низкое окно: min(440, окно − 300), но не меньше 240', () => {
    expect(previewBox(204, 304, 568, 640).h).toBe(340)
    expect(previewBox(204, 304, 568, 400).h).toBe(240)
  })
  it('узкая колонка сжимает; нулевой лист — пусто', () => {
    expect(previewBox(600, 300, 400, 760)).toEqual({ w: 400, h: 200 })
    expect(fitBox(0, 300, 100, 100)).toEqual({ w: 0, h: 0 })
  })
  it('резерв под сетку шаблонов: окно 760 — 360, окно 670 — 270, низкое — не меньше 240', () => {
    expect(previewBox(204, 304, 568, 760, 400).h).toBe(360)
    expect(previewBox(204, 304, 568, 670, 400).h).toBe(270)
    expect(previewBox(204, 304, 568, 500, 400).h).toBe(240)
  })
  it('карточка шаблона: пропорция листа в рамке CARD_W × CARD_MAX_H', () => {
    expect(cardBox(600, 300)).toEqual({ w: CARD_W, h: 82 })
    const tall = cardBox(204, 304)
    expect(tall.h).toBe(CARD_MAX_H)
    expect(tall.w).toBeLessThan(CARD_W)
  })
  it('высота сетки: остаток окна под предпросмотром, но не меньше STRIP_MIN', () => {
    expect(stripMax(760, 360)).toBe(238)
    expect(stripMax(400, 240)).toBe(STRIP_MIN)
  })
  it('revealX: прокрутка только если элемент не виден', () => {
    expect(revealX(0, 100, 150, 300)).toBe(0)
    expect(revealX(0, 280, 340, 300)).toBe(40)
    expect(revealX(200, 100, 150, 300)).toBe(100)
  })
})

describe('gridStep: стрелки по сетке', () => {
  const list = ['a', 'b', 'c', 'd', 'e'].map((id) => ({ id, over: id === 'd' }))
  it('← → по кругу, «не поместится» пропускается', () => {
    expect(gridStep(list, 'a', 'ArrowRight', 3)).toBe('b')
    expect(gridStep(list, 'c', 'ArrowRight', 3)).toBe('e')
    expect(gridStep(list, 'a', 'ArrowLeft', 3)).toBe('e')
  })
  it('↑ ↓ на ряд; пропущенное и край — остаёмся', () => {
    expect(gridStep(list, 'a', 'ArrowDown', 3)).toBeNull() // под «a» — «d», она не поместится
    expect(gridStep(list, 'b', 'ArrowDown', 3)).toBe('e')
    expect(gridStep(list, 'e', 'ArrowUp', 3)).toBe('b')
    expect(gridStep(list, 'b', 'ArrowUp', 3)).toBeNull()
    expect(gridStep(list, 'zz', 'ArrowDown', 3)).toBeNull()
  })
})

// Go с медленным ответом: запрос висит, пока тест не вызовет done.
function slow() {
  const pending: { arg: number; done: () => void }[] = []
  const ask = (a: number) =>
    new Promise<string>((res) => {
      pending.push({ arg: a, done: () => res(`ответ ${a}`) })
    })
  return { ask, pending }
}
const flush = () => new Promise((r) => setTimeout(r, 0))

describe('Latest: запросы не копятся', () => {
  it('10 изменений подряд при медленном Go — 2 запроса, виден ответ на последний', async () => {
    const { ask, pending } = slow()
    const l = new Latest(ask)
    for (let i = 1; i <= 10; i++) l.request(i)
    expect(pending.map((p) => p.arg)).toEqual([1]) // один в полёте
    pending[0].done()
    await flush()
    expect(l.value).toBeNull() // ответ на устаревшее значение не показываем
    expect(pending.map((p) => p.arg)).toEqual([1, 10])
    pending[1].done()
    await flush()
    expect(l.calls).toBe(2)
    expect(l.value).toBe('ответ 10')
    expect(l.pending).toBe(false)
  })

  it('ошибка последнего: текст со строчной→заглавной, прежний ответ остаётся', async () => {
    let n = 0
    const l = new Latest(async (a: number) => {
      if (++n === 2) throw 'не больше 64 мест на развороте'
      return `ответ ${a}`
    })
    l.request(1)
    await flush()
    l.request(2)
    await flush()
    expect(l.value).toBe('ответ 1')
    expect(l.error).toBe('Не больше 64 мест на развороте')
    l.request(3)
    await flush()
    expect(l.error).toBe('')
    expect(l.value).toBe('ответ 3')
  })

  it('ошибка устаревшего значения не показывается', async () => {
    const calls: ((ok: boolean) => void)[] = []
    const l = new Latest(
      (a: number) =>
        new Promise<string>((res, rej) => {
          calls.push((ok) => (ok ? res(`ответ ${a}`) : rej('сбой')))
        }),
    )
    l.request(1)
    l.request(2)
    calls[0](false)
    await flush()
    expect(l.error).toBe('')
    calls[1](true)
    await flush()
    expect(l.value).toBe('ответ 2')
  })
})

describe('VigSession', () => {
  const prev = (id: string): VigPreview => ({ spread: { id } as unknown as SpreadView, left: 2 })
  function deps(): VigDeps & { preview: ReturnType<typeof vi.fn>; choices: ReturnType<typeof vi.fn>; apply: ReturnType<typeof vi.fn> } {
    return {
      preview: vi.fn(async (id: string) => prev(id)),
      choices: vi.fn(async () => [choice(), choice({ id: 'other', over: true })]),
      apply: vi.fn(async () => true),
    }
  }

  it('форма и поля берутся из листа; «Применить» закрыто, пока ничего не изменилось', () => {
    const s = new VigSession('s1', sheet({ vars: { title: 'Гимназия' } }), deps())
    expect(s.form).toMatchObject({ tpl: 'head-t-left', teachers: 1, pupils: 23, ratio: '3:4' })
    expect(s.vars.title).toBe('Гимназия')
    expect(s.block).toBe('Ничего не изменилось')
    s.vars.year = '2026'
    expect(s.block).toBeNull()
  })

  it('refresh: предпросмотр с полями формы и карточки шаблонов для чисел', async () => {
    const d = deps()
    const s = new VigSession('s1', sheet(), d)
    s.form.pupils = 20
    s.vars.title = 'Лицей'
    s.refreshPreview()
    s.refreshChoices()
    await flush()
    expect(d.preview).toHaveBeenCalledWith('s1', setupOf(s.form, s.vars))
    expect(d.choices).toHaveBeenCalledWith('s1', { teachers: 1, pupils: 20, ratio: '3:4' })
    expect(s.preview.value?.left).toBe(2)
    expect(s.now?.id).toBe('head-t-left')
  })

  it('«Применить» сверяется с живым листом, а не со снимком при открытии', () => {
    const d = deps()
    let live = sheet()
    d.live = () => live
    const s = new VigSession('s1', sheet(), d)
    s.form.pupils = 20
    expect(s.block).toBeNull() // форма отличается от листа
    live = sheet({ vig: vig({ pupils: 20, places: 21 }) }) // лист сменился (Undo/другая правка) и совпал с формой
    expect(s.block).toBe('Ничего не изменилось')
    s.form.pupils = 23 // совпало со старым снимком, но лист уже другой
    expect(s.block).toBeNull()
  })

  it('больше 64 мест — «Применить» закрыто, Go не зовётся', async () => {
    const d = deps()
    const s = new VigSession('s1', sheet(), d)
    s.form.pupils = 64 // + 1 учитель = 65
    expect(s.total).toBe(65)
    expect(s.block).toMatch(/Не больше 64/)
    expect(await s.apply()).toBe(false)
    expect(d.apply).not.toHaveBeenCalled()
  })

  it('apply: один вызов с полной формой; busy гасится, ошибка Go (false) окно не закрывает', async () => {
    const d = deps()
    const s = new VigSession('s1', sheet(), d)
    s.form.pupils = 20
    expect(await s.apply()).toBe(true)
    expect(d.apply).toHaveBeenCalledTimes(1)
    expect(d.apply).toHaveBeenCalledWith('s1', setupOf(s.form, s.vars))
    expect(s.busy).toBe(false)
    d.apply.mockResolvedValueOnce(false)
    expect(await s.apply()).toBe(false)
  })

  it('удалённый из файла шаблон — обычная карточка: числа сменили — «Применить» открыто', async () => {
    const d = deps()
    d.choices.mockResolvedValue([choice({ id: 'my:g', own: true, gone: true, name: 'Старый' })])
    const s = new VigSession('s1', sheet({ vig: vig({ tpl: 'my:g' }) }), d)
    s.refreshChoices()
    await flush()
    expect(s.now?.gone).toBe(true)
    s.form.pupils = 20
    expect(s.block).toBeNull()
  })

  it('выбранную карточку удалили × — выбор возвращается к шаблону листа', async () => {
    const d = deps()
    d.choices.mockResolvedValue([choice(), choice({ id: 'my:x', own: true })])
    const s = new VigSession('s1', sheet(), d)
    s.form.tpl = 'my:x'
    s.refreshChoices()
    await flush()
    s.fixTpl()
    expect(s.form.tpl).toBe('my:x') // карточка на месте — выбор не трогаем
    d.choices.mockResolvedValue([choice()]) // my:x удалили
    s.refreshChoices()
    await flush()
    s.fixTpl()
    expect(s.form.tpl).toBe('head-t-left')
  })

  it('групповое фото учитывается в «Мест N»', async () => {
    const d = deps()
    d.choices.mockResolvedValue([choice({ id: 'g', roles: ['teacher', 'pupil', 'group'] })])
    const s = new VigSession('s1', sheet({ vig: vig({ tpl: 'g', teachers: 1, pupils: 5, group: true }) }), d)
    s.refreshChoices()
    await flush()
    expect(s.total).toBe(7)
  })

  it('переименование своего шаблона: новый запрос карточек даёт новое имя сразу', async () => {
    const d = deps()
    d.choices.mockResolvedValue([choice({ id: 'my:a', own: true, name: 'Шаблон 1' })])
    const s = new VigSession('s1', sheet({ vig: vig({ tpl: 'my:a' }) }), d)
    s.refreshChoices()
    await flush()
    expect(s.now?.name).toBe('Шаблон 1')
    d.choices.mockResolvedValue([choice({ id: 'my:a', own: true, name: 'Шаблон 1б' })])
    s.refreshChoices() // его зовёт окно по vigOwn.rev
    await flush()
    expect(s.now?.name).toBe('Шаблон 1б')
  })
})

describe('предупреждение о сбросе ручных мест', () => {
  it('ручная виньетка и есть что применять — слова вопроса страницы; иначе пусто', () => {
    expect(resetWarning(true, false)).toBe('Ваши правки мест сбросятся.')
    expect(resetWarning(true, true)).toBe('')
    expect(resetWarning(false, false)).toBe('')
    expect(resetWarning(undefined, false)).toBe('')
  })

  it('то же предложение стоит в вопросе страницы «Вернуть авто?»', () => {
    expect(autoAskText().ask.startsWith(resetWarning(true, false))).toBe(true)
  })
})

describe('свои шаблоны в окне (038)', () => {
  const list = [choice({ id: 'a' }), choice({ id: 'b' }), choice({ id: 'my:1', own: true }), choice({ id: 'my:2', own: true, over: true })]

  it('стрелки проходят по плоскому списку: каталог, затем свои; «не поместится» пропускается', () => {
    expect(gridStep(list, 'b', 'ArrowRight', 4)).toBe('my:1')
    expect(gridStep(list, 'my:1', 'ArrowRight', 4)).toBe('a') // my:2 не поместится, круг замыкается
    expect(gridStep(list, 'a', 'ArrowLeft', 4)).toBe('my:1')
  })
})

describe('vigDialog', () => {
  it('open/close', () => {
    vigDialog.open('x')
    expect(vigDialog.id).toBe('x')
    vigDialog.close()
    expect(vigDialog.id).toBeNull()
  })
})
