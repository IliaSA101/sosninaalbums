import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go и store подменены: проверяем, что уходит в Go, какие тосты и когда обновляется список шаблонов.
vi.mock('../../base/core/store.svelte', () => ({
  store: { apply: vi.fn(), notify: vi.fn(), undoToast: vi.fn() },
  Api: {
    SaveVignetteTemplate: vi.fn(),
    RenameGridTemplate: vi.fn(),
    DeleteGridTemplate: vi.fn(),
    ResetVignette: vi.fn(),
  },
}))

import { Api, store } from '../../base/core/store.svelte'
import type { VigChoice } from '../../base/core/types'
import {
  NAME_MAX,
  canEditOwn,
  deleteText,
  deleteVigTemplate,
  firstOwn,
  isOwnTpl,
  nameOf,
  ownLabel,
  pickedForm,
  pickedRatio,
  tileForm,
  fitBox,
  renameVigTemplate,
  resetVignette,
  saveVigTemplate,
  tileLines,
  tplLabel,
} from './vigown'
import { vigOwn } from './vigown.svelte'

const ch = (o: Partial<VigChoice> = {}): VigChoice => ({
  id: 'head-t-left',
  name: 'Шапка, учителя слева',
  boxes: [],
  roles: [],
  pupilW: 35,
  small: false,
  over: false,
  ...o,
})
const mine = (o: Partial<VigChoice> = {}) => ch({ id: 'my:a1', name: 'Мой', own: true, ratio: '4:5', ...o })

beforeEach(() => {
  vi.clearAllMocks()
  vigOwn.close()
  vigOwn.rev = 0
  vi.mocked(Api.SaveVignetteTemplate).mockResolvedValue()
  vi.mocked(Api.RenameGridTemplate).mockResolvedValue()
  vi.mocked(Api.DeleteGridTemplate).mockResolvedValue()
})

describe('раздел «Мои шаблоны»', () => {
  it('firstOwn: индекс первого своего, своих нет — −1', () => {
    expect(firstOwn([ch(), ch({ id: 'b' })])).toBe(-1)
    expect(firstOwn([ch(), mine(), mine({ id: 'my:b2' })])).toBe(1)
  })

  it('ownLabel: удалённый из файла шаблон помечен', () => {
    expect(ownLabel(mine())).toBe('Мой')
    expect(ownLabel(mine({ gone: true }))).toBe('Мой (шаблон удалён)')
  })

  it('✎ и × — только у живого своего и при уровне «Шаблоны»', () => {
    expect(canEditOwn(mine(), true)).toBe(true)
    expect(canEditOwn(mine(), false)).toBe(false)
    expect(canEditOwn(ch(), true)).toBe(false)
    expect(canEditOwn(mine({ gone: true }), true)).toBe(false)
  })

  it('tileLines: у удалённого — строка «удалён»', () => {
    expect(tileLines(mine())).toEqual(['35 мм'])
    expect(tileLines(mine({ gone: true }))).toEqual(['35 мм', 'удалён'])
  })
})

describe('tplLabel и isOwnTpl', () => {
  it('имя берётся из списка Go: после переименования строка и карточка показывают новое', () => {
    expect(tplLabel([mine({ name: 'Новое имя' })], 'my:a1', 'Старое (снимок)')).toBe('Новое имя')
    expect(tplLabel([mine({ gone: true })], 'my:a1', 'Старое')).toBe('Мой (шаблон удалён)')
  })
  it('списка или карточки нет — запасное имя листа, иначе пусто', () => {
    expect(tplLabel([], 'my:a1', 'Старое')).toBe('Старое')
    expect(tplLabel([ch()], 'my:a1')).toBe('')
  })
  it('isOwnTpl: только my:', () => {
    expect(isOwnTpl('my:a1')).toBe(true)
    expect(isOwnTpl('head-t-left')).toBe(false)
  })
})

describe('pickedRatio', () => {
  const list = [
    ch(),
    mine(),
    mine({ id: 'my:same', ratio: '3:4' }),
    mine({ id: 'my:bad', ratio: '7:7' }),
    mine({ id: 'my:none', ratio: '' }),
  ]

  it('свой шаблон с другим портретом — его портрет', () => {
    expect(pickedRatio(list, 'my:a1', '3:4')).toBe('4:5')
  })
  it('тот же портрет, каталожный, неизвестный и пустой — менять нечего', () => {
    expect(pickedRatio(list, 'my:same', '3:4')).toBeNull()
    expect(pickedRatio(list, 'head-t-left', '2:3')).toBeNull()
    expect(pickedRatio(list, 'my:bad', '3:4')).toBeNull()
    expect(pickedRatio(list, 'my:none', '3:4')).toBeNull()
    expect(pickedRatio(list, 'нет такого', '3:4')).toBeNull()
  })
})

describe('pickedForm (049, решение №122)', () => {
  const round = ch({ id: 'round', shape: 'oval', ratio: '1:1', family: 'round' })
  const round2 = ch({ id: 'head-round', shape: 'oval', ratio: '1:1', family: 'round' })
  const rect = ch({ id: 'grid', family: 'inrow' })
  const list = [rect, round, round2, mine(), mine({ id: 'my:sq', ratio: '1:1' })]

  it('шаблон «круги»: форма «Круг» и портрет 1:1', () => {
    expect(pickedForm(list, 'grid', 'round', { ratio: '3:4', shape: '' })).toEqual({ shape: 'oval', ratio: '1:1' })
  })
  it('круг уже стоит — менять нечего; между шаблонами кругов форма не сбрасывается', () => {
    expect(pickedForm(list, 'round', 'head-round', { ratio: '1:1', shape: 'oval' })).toBeNull()
  })
  it('уход с кругов на шаблон без подсказки — прямоугольник, портрет не трогается', () => {
    expect(pickedForm(list, 'round', 'grid', { ratio: '1:1', shape: 'oval' })).toEqual({ shape: '' })
    expect(pickedForm(list, 'round', 'grid', { ratio: '4:5', shape: 'oval' })).toEqual({ shape: '' })
  })
  it('форму меняли руками (не круг) — уход с кругов её не трогает', () => {
    expect(pickedForm(list, 'round', 'grid', { ratio: '3:4', shape: '' })).toBeNull()
  })
  it('круг выбрали вручную на прямоугольном шаблоне — смена на другой прямоугольный его не сбрасывает', () => {
    expect(pickedForm(list, 'grid', 'head-t-left', { ratio: '1:1', shape: 'oval' })).toBeNull()
  })
  it('свой шаблон — только его портрет, как раньше', () => {
    expect(pickedForm(list, 'grid', 'my:a1', { ratio: '3:4', shape: '' })).toEqual({ ratio: '4:5' })
    expect(pickedForm(list, 'grid', 'my:sq', { ratio: '1:1', shape: '' })).toBeNull()
  })
  it('нет такого шаблона — менять нечего', () => {
    expect(pickedForm(list, 'grid', 'нет такого', { ratio: '3:4', shape: '' })).toBeNull()
  })
})

describe('tileForm: плитка показывает результат щелчка по ней', () => {
  const round = ch({ id: 'round', shape: 'oval', ratio: '1:1', family: 'round' })
  const top = ch({ id: 'top', family: 'head' })
  const row = ch({ id: 'row', family: 'inrow' })
  const list = [top, row, round, mine({ id: 'my:o', shape: 'oval' })]
  type F = { ratio: string; shape?: string; round?: number }
  // Вид плиток для формы окна f при выбранном шаблоне cur
  const view = (cur: string, f: F) =>
    Object.fromEntries(list.map((c) => [c.id, (({ shape, round }) => ({ shape, round }))(tileForm(list, cur, c.id, f))]))
  // Применить щелчок по плитке к форме окна, как делает окно
  const click = (cur: string, id: string, f: F): F => ({ ...f, ...pickedForm(list, cur, id, f) })

  it('«круги» всегда круг; прочие при прямоугольнике — прямоугольник', () => {
    const v = view('top', { ratio: '3:4', shape: '', round: 0 })
    expect(v.round).toEqual({ shape: 'oval', round: 0 })
    expect(v.top).toEqual({ shape: '', round: 0 })
    expect(v.row).toEqual({ shape: '', round: 0 })
  })
  it('прямоугольник → круги → верхняя → обратно: плитки не прыгают', () => {
    let f: F = { ratio: '3:4', shape: '', round: 0 }
    const before = view('top', f)
    f = click('top', 'round', f)
    expect(f.shape).toBe('oval')
    const after = view('round', f)
    // все, кроме выбранной, выглядят как до выбора
    expect(after.top).toEqual(before.top)
    expect(after.row).toEqual(before.row)
    expect(after.round).toEqual(before.round)
    f = click('round', 'top', f)
    expect(view('top', f)).toEqual(before)
  })
  it('скруглённые → круги → верхняя → скруглённые: радиус сохраняется', () => {
    let f: F = { ratio: '3:4', shape: '', round: 20 }
    const before = view('top', f)
    expect(before.row).toEqual({ shape: '', round: 20 })
    f = click('top', 'round', f)
    const after = view('round', f)
    expect(after.top).toEqual({ shape: '', round: 20 })
    expect(after.row).toEqual(before.row)
    f = click('round', 'top', f)
    expect(f).toMatchObject({ shape: '', round: 20 })
    expect(view('top', f)).toEqual(before)
  })
  it('ручной «Круг» на не-круглом шаблоне: щелчок форму не меняет — не-круглые плитки кругом', () => {
    const v = view('top', { ratio: '3:4', shape: 'oval', round: 0 })
    expect(v.row).toEqual({ shape: 'oval', round: 0 })
    expect(v.top).toEqual({ shape: 'oval', round: 0 })
  })
  it('свой шаблон с формой — как после выбора (pickedShape)', () => {
    expect(view('top', { ratio: '3:4', shape: '', round: 0 })['my:o']).toEqual({ shape: 'oval', round: 0 })
  })
})

describe('tileForm: пропорция результата щелчка (№186)', () => {
  const round = ch({ id: 'round', shape: 'oval', ratio: '1:1', family: 'round' })
  const top = ch({ id: 'top', family: 'head' })
  const list = [top, round, mine({ id: 'my:r', ratio: '2:3' }), mine({ id: 'my:o', shape: 'oval', ratio: '1:1' })]
  const t = (cur: string, id: string, ratio: string, shape = '') => tileForm(list, cur, id, { ratio, shape, round: 0 })

  it('портрет 3:4: «круги» — 1:1 при коробках 3:4, прочие — 3:4 как есть', () => {
    expect(t('top', 'round', '3:4')).toMatchObject({ ratio: '1:1', from: '3:4', shape: 'oval' })
    expect(t('top', 'top', '3:4')).toMatchObject({ ratio: '3:4', from: '3:4', shape: '' })
  })
  it('портрет уже 1:1 — вписывать нечего', () => {
    expect(t('round', 'round', '1:1', 'oval')).toMatchObject({ ratio: '1:1', from: '1:1' })
  })
  it('после выбора кругов (1:1, круг) остальные каталожные плитки остаются 1:1: уход портрет не меняет (№122)', () => {
    expect(t('round', 'top', '1:1', 'oval')).toMatchObject({ ratio: '1:1', from: '1:1', shape: '' })
  })
  it('свой шаблон — его портрет, коробки уже в нём', () => {
    expect(t('top', 'my:r', '3:4')).toMatchObject({ ratio: '2:3', from: '2:3' })
    expect(t('top', 'my:o', '3:4')).toMatchObject({ ratio: '1:1', from: '1:1', shape: 'oval' })
  })
})

describe('fitBox: место нужного портрета внутри коробки', () => {
  // Лист 100×100 px: коробка 40×60 px (3:4 не нужен, берём простые числа)
  const b = { x: 0.1, y: 0.2, w: 0.4, h: 0.6 }
  it('круг 1:1 в портретной коробке — квадрат по ширине, по центру', () => {
    const r = fitBox(b, 100, 100, '1:1')
    expect(r.w * 100).toBeCloseTo(40)
    expect(r.h * 100).toBeCloseTo(40)
    expect(r.x).toBeCloseTo(0.1)
    expect(r.y).toBeCloseTo(0.3)
  })
  it('широкая коробка — квадрат по высоте', () => {
    const r = fitBox({ x: 0, y: 0, w: 0.6, h: 0.3 }, 100, 100, '1:1')
    expect([r.w, r.h]).toEqual([expect.closeTo(0.3), expect.closeTo(0.3)])
    expect(r.x).toBeCloseTo(0.15)
  })
  it('тот же портрет — коробка не меняется', () => {
    const r = fitBox({ x: 0, y: 0, w: 0.3, h: 0.4 }, 100, 100, '3:4')
    expect([r.x, r.y, r.w, r.h]).toEqual([0, 0, expect.closeTo(0.3), expect.closeTo(0.4)])
  })
  it('мусорный портрет — как есть', () => {
    expect(fitBox(b, 100, 100, 'abc')).toBe(b)
  })
})

describe('тексты', () => {
  it('deleteText: «Листы» на листе, «Развороты» в альбоме', () => {
    expect(deleteText('Мой', 'Лист')).toBe('Шаблон «Мой» будет удалён из списка. Листы, где он уже применён, не изменятся.')
    expect(deleteText('Мой', 'Разворот')).toContain('Развороты, где')
  })
  it('nameOf: пробелы по краям убираются, длинное обрезается по знакам', () => {
    expect(nameOf('  Класс  ')).toBe('Класс')
    expect(Array.from(nameOf('я'.repeat(100)))).toHaveLength(NAME_MAX)
  })
})

describe('вызовы Go', () => {
  it('saveVigTemplate: id листа и имя в Go, тост «Шаблон сохранён»', async () => {
    expect(await saveVigTemplate('s1', ' Выпуск ')).toBe(true)
    expect(Api.SaveVignetteTemplate).toHaveBeenCalledWith('s1', 'Выпуск')
    expect(store.notify).toHaveBeenCalledWith('Шаблон сохранён')
  })

  it('saveVigTemplate: ошибка Go («Это не виньетка») — тост-ошибка, «сохранён» нет', async () => {
    vi.mocked(Api.SaveVignetteTemplate).mockRejectedValue('Это не виньетка')
    expect(await saveVigTemplate('s1', '')).toBe(false)
    expect(store.notify).toHaveBeenCalledWith('Это не виньетка', 'error')
    expect(store.notify).not.toHaveBeenCalledWith('Шаблон сохранён')
  })

  it('переименование и удаление зовут Go с id карточки как есть («my:…»)', async () => {
    expect(await renameVigTemplate('my:a1', ' Новое ')).toBe(true)
    expect(Api.RenameGridTemplate).toHaveBeenCalledWith('my:a1', 'Новое')
    expect(await deleteVigTemplate('my:a1')).toBe(true)
    expect(Api.DeleteGridTemplate).toHaveBeenCalledWith('my:a1')
  })

  it('«шаблоны сохранены более новой версией» — понятный тост', async () => {
    vi.mocked(Api.DeleteGridTemplate).mockRejectedValue('шаблоны сохранены более новой версией программы')
    expect(await deleteVigTemplate('my:a1')).toBe(false)
    expect(vi.mocked(store.notify).mock.calls[0][0]).toMatch(/Обновите программу/)
  })

  it('resetVignette: ответ — в store.apply, лишние фото — тост «Вернуть»', async () => {
    const view = { rev: 9 }
    vi.mocked(Api.ResetVignette).mockResolvedValue({ view, left: 2 } as never)
    expect(await resetVignette('s1')).toBe(true)
    expect(Api.ResetVignette).toHaveBeenCalledWith('s1')
    expect(store.apply).toHaveBeenCalledWith(view)
    expect(store.undoToast).toHaveBeenCalledWith('Убрано в ленту: 2')
  })

  it('resetVignette: left = 0 — без тоста; ошибка — тост, вид не меняется', async () => {
    vi.mocked(Api.ResetVignette).mockResolvedValueOnce({ view: {}, left: 0 } as never)
    await resetVignette('s1')
    expect(store.undoToast).not.toHaveBeenCalled()
    vi.mocked(Api.ResetVignette).mockRejectedValueOnce('Лист не найден')
    vi.mocked(store.apply).mockClear()
    expect(await resetVignette('s1')).toBe(false)
    expect(store.apply).not.toHaveBeenCalled()
    expect(store.notify).toHaveBeenCalledWith('Лист не найден', 'error')
  })
})

describe('vigOwn: окна', () => {
  it('save: пустое имя, commit зовёт Go и поднимает rev', async () => {
    vigOwn.save('s1')
    expect(vigOwn.ask).toEqual({ kind: 'save', spread: 's1' })
    expect(vigOwn.name).toBe('')
    vigOwn.name = 'Класс А'
    expect(await vigOwn.commit()).toBe(true)
    expect(Api.SaveVignetteTemplate).toHaveBeenCalledWith('s1', 'Класс А')
    expect(vigOwn.ask).toBeNull()
    expect(vigOwn.rev).toBe(1)
  })

  it('rename: в поле — прежнее имя; commit переименовывает', async () => {
    vigOwn.rename({ id: 'my:a1', name: 'Мой' })
    expect(vigOwn.name).toBe('Мой')
    vigOwn.name = 'Другой'
    await vigOwn.commit()
    expect(Api.RenameGridTemplate).toHaveBeenCalledWith('my:a1', 'Другой')
    expect(vigOwn.rev).toBe(1)
  })

  it('delete: вопрос с именем; commit удаляет по id карточки', async () => {
    vigOwn.del({ id: 'my:a1', name: 'Мой' })
    expect(vigOwn.ask).toEqual({ kind: 'delete', id: 'my:a1', name: 'Мой' })
    await vigOwn.commit()
    expect(Api.DeleteGridTemplate).toHaveBeenCalledWith('my:a1')
    expect(vigOwn.rev).toBe(1)
  })

  it('Отмена (close): в Go не идёт, rev прежний; ошибка Go — rev прежний', async () => {
    vigOwn.save('s1')
    vigOwn.close()
    expect(await vigOwn.commit()).toBe(false)
    expect(Api.SaveVignetteTemplate).not.toHaveBeenCalled()

    vi.mocked(Api.DeleteGridTemplate).mockRejectedValue('Шаблон не найден')
    vigOwn.del({ id: 'my:x', name: 'X' })
    expect(await vigOwn.commit()).toBe(false)
    expect(vigOwn.rev).toBe(0)
    expect(vigOwn.ask).toBeNull()
  })
})
