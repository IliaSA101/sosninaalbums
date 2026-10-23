import { describe, expect, it } from 'vitest'
import { menuContext, photoMenu, sheetMenu, type MenuCtx, type MenuItem } from './photomenu'
import { can, type FeatureId } from '../../base/prefs/levels'

// Группа буфера в меню правой кнопки (099): фото, стикер, надпись, пустой лист. Слова и порядок — как в «Правке».
const base: MenuCtx = {
  grid: false,
  places: 1,
  texts: 0,
  stickers: 0,
  photos: 1,
  missing: 0,
  borderLost: 0,
  differ: false,
  allContain: false,
  allCover: true,
  allRect: true,
  allRounded: false,
  allOval: false,
  allGray: false,
  framed: false,
  copyable: true,
  pasteable: true,
}
const KEYS: Record<string, string> = { cutItems: 'Ctrl+X', copyItems: 'Ctrl+C', pasteItems: 'Ctrl+V', duplicateItems: 'Ctrl+D' } // X и D в меню не идут
const hint = (cmd: string) => KEYS[cmd] ?? ''
const menu = (c: Partial<MenuCtx>, f: (id: FeatureId) => boolean = () => true) => photoMenu({ ...base, ...c }, f, hint)
const ids = (m: MenuItem[]) => m.map((i) => i.id)
const CLIP = ['clipCopy', 'clipPaste']

describe('группа буфера в меню фото', () => {
  it('Копировать · Вставить — слова и клавиши как в «Правке»; «Вырезать» и «Дублировать» в меню нет', () => {
    const g = menu({}).filter((i) => CLIP.includes(i.id))
    expect(g.map((i) => i.label)).toEqual(['Копировать', 'Вставить'])
    expect(g.map((i) => i.hint)).toEqual(['Ctrl+C', 'Ctrl+V'])
  })
  it('стоит сразу после «Крупного просмотра», отдельной группой', () => {
    const m = menu({})
    expect(ids(m).slice(0, 4)).toEqual(['preview', ...CLIP, 'copy'])
    expect(m.find((i) => i.id === 'clipCopy')?.sep).toBe(true)
    expect(m.find((i) => i.id === 'copy')?.sep).toBe(true) // «Копировать имя файла» отделено от группы
    expect(m.filter((i) => CLIP.includes(i.id) && i.sep)).toHaveLength(1)
  })
  it('нет просмотра (несколько фото) — группа первая и без линии сверху', () => {
    const m = menu({ places: 2, photos: 2 })
    expect(ids(m).slice(0, 2)).toEqual(CLIP)
    expect(m[0].sep).toBeFalsy()
    expect(m[2]).toMatchObject({ id: 'copy', sep: true })
  })
  it('пустое место: группа первая, файловых пунктов нет, линия перед кадром', () => {
    const m = menu({ photos: 0, grid: true })
    expect(ids(m).slice(0, 3)).toEqual([...CLIP, 'rounded'])
    expect(m[2].sep).toBe(true)
  })
  it('серость: нечего копировать — серое «Копировать»; буфер пуст — серая «Вставить»', () => {
    const off = (m: MenuItem[]) => m.filter((i) => CLIP.includes(i.id) && i.disabled).map((i) => i.id)
    expect(off(menu({}))).toEqual([])
    expect(off(menu({ copyable: false }))).toEqual(['clipCopy'])
    expect(off(menu({ pasteable: false }))).toEqual(['clipPaste'])
  })
  it('клавиша снята в настройках — подписи справа нет', () => {
    const m = photoMenu(base, () => true)
    expect(m.find((i) => i.id === 'clipCopy')?.hint).toBeUndefined()
  })
  it('нет функции «Копировать и вставить» — группы нет, линия не висит', () => {
    const m = menu({}, (f) => f !== 'clipboard')
    expect(ids(m).some((id) => CLIP.includes(id))).toBe(false)
    expect(ids(m).slice(0, 2)).toEqual(['preview', 'copy'])
    expect(m[1].sep).toBeFalsy()
  })
  it('на «Простом» меню нет совсем', () => {
    expect(menu({}, (f) => can(f, 'simple'))).toEqual([])
  })
  it('на «Стандартном» группа есть', () => {
    expect(ids(menu({}, (f) => can(f, 'standard')))).toEqual(expect.arrayContaining(CLIP))
  })
})

describe('«Вырезать» и «Дублировать» в меню правой кнопки не идут (099-9, п. 14 бэклога)', () => {
  it('ни у фото, ни у стикера, ни у надписи, ни у листа', () => {
    const all = [
      menu({}),
      menu({ places: 0, photos: 0, stickers: 1 }),
      menu({ places: 0, photos: 0, texts: 1 }),
      sheetMenu(true, () => true, hint),
    ]
    for (const m of all) expect(m.map((i) => i.label)).not.toEqual(expect.arrayContaining(['Вырезать']))
    for (const m of all) expect(m.map((i) => i.label)).not.toEqual(expect.arrayContaining(['Дублировать']))
  })
})

describe('меню стикера: буфер первым', () => {
  const one = { places: 0, photos: 0, stickers: 1 }
  it('один стикер: группа, затем «Заменить…» через линию', () => {
    const m = menu(one)
    expect(ids(m).slice(0, 4)).toEqual([...CLIP, 'replace', 'flipH'])
    expect(m[0].sep).toBeFalsy()
    expect(m[2]).toMatchObject({ id: 'replace', sep: true })
  })
  it('несколько стикеров: группа и «Слой», «Удалить (N)»', () => {
    const m = menu({ ...one, stickers: 3 })
    expect(ids(m)).toEqual([...CLIP, 'layer', 'delete'])
    expect(m[2].sep).toBe(true)
  })
  it('без функции буфера — прежнее меню стикера', () => {
    const m = menu(one, (f) => f !== 'clipboard')
    expect(ids(m)).toEqual(['replace', 'flipH', 'flipV', 'layer', 'delete'])
    expect(m[0].sep).toBeFalsy()
  })
  it('серость берётся из состояния буфера', () => {
    expect(menu({ ...one, pasteable: false }).find((i) => i.id === 'clipPaste')?.disabled).toBe(true)
  })
})

describe('меню надписи', () => {
  const t = { places: 0, photos: 0, texts: 1 }
  it('«Авто»: группа и «Удалить» — слоёв у «Авто» нет', () => {
    expect(ids(menu(t))).toEqual([...CLIP, 'delete'])
  })
  it('«Ручная»: «Слой ▸» с подменю между группой и «Удалить»', () => {
    const m = menu({ ...t, grid: true })
    expect(ids(m)).toEqual([...CLIP, 'layer', 'delete'])
    expect(m.find((i) => i.id === 'layer')).toMatchObject({ sub: true, sep: true })
    expect(m[0].sep).toBeFalsy()
  })
  it('без слоёв на уровне — нет «Слоя»; «Удалить (N)» для группы', () => {
    expect(ids(menu({ ...t, grid: true }, (f) => f !== 'layers'))).toEqual([...CLIP, 'delete'])
    expect(menu({ ...t, texts: 3 }).find((i) => i.id === 'delete')?.label).toBe('Удалить (3)')
  })
  it('без функции буфера у надписи меню нет (раньше его не было)', () => {
    expect(menu(t, (f) => f !== 'clipboard')).toEqual([])
    expect(menu(t, (f) => can(f, 'simple'))).toEqual([])
  })
  it('места или стикеры в выделении — меню фото или стикера, а не надписи', () => {
    expect(ids(menu({ ...t, places: 1, photos: 1 }))).toContain('preview')
    expect(ids(menu({ ...t, stickers: 1 }))).toEqual([...CLIP, 'layer', 'delete']) // меню стикера: «Слой» и на «Авто»
  })
  it('ничего не выделено — меню нет', () => {
    expect(menu({ places: 0, photos: 0 })).toEqual([])
  })
})

describe('меню пустого листа', () => {
  it('одно «Вставить» с клавишей, без линии', () => {
    const m = sheetMenu(true, () => true, hint)
    expect(m).toEqual([{ id: 'clipPaste', label: 'Вставить', hint: 'Ctrl+V' }])
  })
  it('буфер пуст — меню нет', () => {
    expect(sheetMenu(false, () => true, hint)).toEqual([])
  })
  it('нет функции или «Простой» — меню нет', () => {
    expect(sheetMenu(true, (f) => f !== 'clipboard', hint)).toEqual([])
    expect(sheetMenu(true, (f) => can(f, 'simple'), hint)).toEqual([])
  })
})

describe('menuContext: состояние буфера', () => {
  it('по умолчанию копировать можно, вставлять нечего; состояние берётся из аргумента', () => {
    expect(menuContext(undefined, [], [])).toMatchObject({ copyable: true, pasteable: false })
    expect(menuContext(undefined, [], [], 0, 0, { copy: false, paste: true })).toMatchObject({ copyable: false, pasteable: true })
  })
})
