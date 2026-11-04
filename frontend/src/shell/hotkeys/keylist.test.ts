import { describe, expect, it } from 'vitest'
import { catalogView, catalogWith } from '../../base/keys/keymap.fixture'
import { emptyText, groupRows, listed, matches, navKey, norm, rowTag, stepRow } from './keylist'

const all = () => true
const ids = (rows: { id: string }[]) => rows.map((e) => e.id)
const cmd = (id: string) => catalogView.commands.find((e) => e.id === id)!
const find = (query: string, extra: Partial<Parameters<typeof listed>[1]> = {}) =>
  ids(listed(catalogView, { query, onlyOwn: false, visible: all, ...extra }))

describe('поиск в окне клавиш', () => {
  it('ё = е и без регистра', () => {
    expect(norm('ЁлкА')).toBe('елка')
    expect(matches({ ...cmd('toggleBig'), label: 'Всё в кадре' }, 'все')).toBe(true)
  })
  it('по названию и по тексту клавиши: «ctrl+d», «ctrl d», «del»', () => {
    expect(find('Отменить')).toContain('undo')
    expect(find('ctrl+z')).toContain('undo')
    expect(find('ctrl z')).toContain('undo')
    expect(find('CTRL+Z')).toContain('undo')
    expect(find('del')).toEqual(expect.arrayContaining(['removeCell', 'bgRemove']))
  })
  it('по русской букве на клавише: «в» — клавиша D, а не все команды со словом на «в»', () => {
    const own = catalogWith({ toggleBig: ['KeyD'] })
    const r = ids(listed(own, { query: 'в', onlyOwn: false, visible: all }))
    expect(r).toContain('toggleBig')
    expect(r).not.toContain('fitZoom') // «Вписать разворот в окно» содержит «в», но это слово, а не клавиша
  })
  it('одна латинская буква или цифра — клавиша с таким именем', () => {
    expect(find('f')).toContain('toggleBig')
    expect(find('f')).not.toContain('save')
    expect(find('0')).toEqual(expect.arrayContaining(['markRating0']))
  })
  it('пустой запрос — все; несуществующий — никого', () => {
    expect(find('   ')).toHaveLength(catalogView.commands.length)
    expect(find('ъъъъъ')).toEqual([])
  })
  it('«Только свои» и уровень', () => {
    const own = catalogWith({ toggleBig: ['KeyB'] })
    expect(ids(listed(own, { query: '', onlyOwn: true, visible: all }))).toEqual(['toggleBig'])
    expect(ids(listed(own, { query: '', onlyOwn: false, visible: (e) => e.id !== 'save' }))).not.toContain('save')
  })
})

describe('группы списка', () => {
  const groups = groupRows(catalogView, listed(catalogView, { query: '', onlyOwn: false, visible: all }))
  it('группы по порядку Go, пустых нет', () => {
    expect(groups.map((g) => g.id)).toEqual(catalogView.groups.map((g) => g.id))
    expect(groupRows(catalogView, [])).toEqual([])
  })
  it('одна область на всю группу — подпись в заголовке, а не в строках', () => {
    const bg = groups.find((g) => g.id === 'backdrop')!
    expect(bg.scopeTag).toBe('режим фона')
    expect(rowTag(bg.rows[0], bg)).toBe('')
    const album = groups.find((g) => g.id === 'album')!
    expect(album.scopeTag).toBe('') // «везде» — без подписи
  })
  it('разные области — подпись у каждой строки', () => {
    const photos = groups.find((g) => g.id === 'photos')!
    expect(photos.scopeTag).toBe('')
    const lib = photos.rows.find((e) => e.scope === 'library')!
    expect(rowTag(lib, photos)).toBe('в ленте фото')
  })
})

describe('пустой список', () => {
  it('«Только свои» без запроса — «Своих сочетаний пока нет»; иначе — «Ничего не нашлось»', () => {
    expect(emptyText({ query: '', onlyOwn: true })).toBe('Своих сочетаний пока нет.')
    expect(emptyText({ query: 'я', onlyOwn: true })).toMatch(/^Ничего не нашлось/)
    expect(emptyText({ query: 'х', onlyOwn: false })).toMatch(/^Ничего не нашлось/)
  })
})

describe('хождение стрелками по строкам', () => {
  const order = ['a', 'b', 'c']
  const bare = { ctrlKey: false, shiftKey: false, altKey: false, metaKey: false }
  it('↓/↑ — соседняя строка, на краях остаёмся на месте (без круга)', () => {
    expect(stepRow(order, 'a', 'ArrowDown')).toBe('b')
    expect(stepRow(order, 'b', 'ArrowUp')).toBe('a')
    expect(stepRow(order, 'c', 'ArrowDown')).toBe('c')
    expect(stepRow(order, 'a', 'ArrowUp')).toBe('a')
  })
  it('Home/End — первая и последняя', () => {
    expect(stepRow(order, 'b', 'Home')).toBe('a')
    expect(stepRow(order, 'b', 'End')).toBe('c')
    expect(stepRow(order, null, 'End')).toBe('c')
  })
  it('одна строка: стрелки никуда не уводят', () => {
    for (const k of ['ArrowUp', 'ArrowDown', 'Home', 'End'] as const) expect(stepRow(['x'], 'x', k)).toBe('x')
  })
  it('пусто (поиск ничего не нашёл) — null', () => {
    expect(stepRow([], 'a', 'ArrowDown')).toBeNull()
    expect(stepRow([], null, 'Home')).toBeNull()
  })
  it('текущей строки нет среди видимых (скрыта поиском) — вход с начала или с конца', () => {
    expect(stepRow(order, 'gone', 'ArrowDown')).toBe('a')
    expect(stepRow(order, 'gone', 'ArrowUp')).toBe('c')
    expect(stepRow(order, null, 'ArrowDown')).toBe('a')
  })
  it('порядок берётся из групп и поиска: скрытые строки пропускаются', () => {
    const groups = groupRows(catalogView, listed(catalogView, { query: 'ctrl', onlyOwn: false, visible: all }))
    const rows = groups.flatMap((g) => g.rows.map((e) => e.id))
    expect(rows.length).toBeGreaterThan(1)
    expect(stepRow(rows, rows[0], 'ArrowDown')).toBe(rows[1])
    expect(stepRow(rows, rows[0], 'End')).toBe(rows[rows.length - 1])
  })
  it('navKey: только голые ↑ ↓ Home End; с модификаторами и другие клавиши — null', () => {
    expect(navKey({ key: 'ArrowDown', ...bare })).toBe('ArrowDown')
    expect(navKey({ key: 'Home', ...bare })).toBe('Home')
    expect(navKey({ key: 'ArrowDown', ...bare, ctrlKey: true })).toBeNull()
    expect(navKey({ key: 'End', ...bare, shiftKey: true })).toBeNull()
    expect(navKey({ key: 'ArrowLeft', ...bare })).toBeNull()
    expect(navKey({ key: 'a', ...bare })).toBeNull()
  })
})
