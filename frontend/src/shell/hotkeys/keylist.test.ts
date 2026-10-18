import { describe, expect, it } from 'vitest'
import { catalogView, catalogWith } from '../../base/keys/keymap.fixture'
import { emptyText, groupRows, listed, matches, norm, rowTag } from './keylist'

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
