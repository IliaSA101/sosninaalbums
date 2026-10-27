import { afterEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { MOUSE_HELP } from '../menu/mousehelp'
import { ARTICLES } from './articles'
import { articleText } from './articletext'
import { menuPath } from '../guide/show'

// Статьи F1 про «Во весь экран» (режим F) и «Крупный просмотр фото» (окно, 128-11, Т9).
afterEach(() => dropCatalog())

const art = (id: string) => ARTICLES.find((a) => a.id === id)!

describe('статья про режим F', () => {
  it('называется «Во весь экран»; старое имя находится по словам', () => {
    const a = art('big')
    expect(a.title).toBe('Во весь экран')
    expect(a.where).toBe('Кнопка в шапке; «Вид ▸ Во весь экран»; F.')
    expect(a.words).toContain('крупный просмотр')
    expect(a.keys).toEqual(['toggleBig'])
  })
  it('путь меню в статье совпадает с настоящим пунктом «Вид»', () => {
    expect(menuPath('big')).toContain('Во весь экран')
  })
  it('поиск в ленте: ленты нет «в режиме «Во весь экран»»', () => {
    useCatalog()
    expect(articleText(art('name-search').where ?? '')).toContain('в режиме «Во весь экран» и при открытой галерее')
  })
  it('слов «Крупный просмотр» как имени режима F нигде в статьях нет', () => {
    for (const a of ARTICLES.filter((x) => x.id !== 'photo-preview'))
      expect(`${a.title} ${a.where ?? ''}`, a.id).not.toMatch(/в крупном просмотре|Вид ▸ Крупный просмотр/)
  })
})

describe('статья «Крупный просмотр фото» (Т9)', () => {
  it('раздел «Фото и лента», на любом уровне, жест и клавиша из реестра', () => {
    const a = art('photo-preview')
    expect(a).toMatchObject({
      title: 'Крупный просмотр фото',
      group: 'photos',
      feature: 'base',
      keys: ['Ctrl+правый щелчок по фото в ленте', 'photoPreview'],
    })
    useCatalog()
    expect(articleText(a.where ?? '')).toBe(
      'Ctrl+правый щелчок по фото в ленте (на Mac — ⌘+правый); правый щелчок ▸ «Крупный просмотр»; Z.',
    )
    expect(a.how).toBe('Если Ctrl (на Mac — ⌘) ещё зажат после выделения, правый щелчок откроет просмотр, а не меню.')
    expect(a.what).toContain('выбрать лучший кадр')
  })
  it('клавиша снята или переназначена — метка уходит целиком, без «; .»', () => {
    const a = art('photo-preview')
    useCatalog({ photoPreview: ['KeyB'] })
    expect(articleText(a.where ?? '')).toContain('«Крупный просмотр»; B.')
    useCatalog({ photoPreview: [] })
    expect(articleText(a.where ?? '')).toContain('«Крупный просмотр».')
  })
  it('жест — справочная строка мыши на всех уровнях (ключ статьи)', () => {
    const row = MOUSE_HELP.find((r) => r.show === 'Ctrl+правый щелчок по фото в ленте')
    expect(row).toMatchObject({ label: 'Крупный просмотр фото (на Mac — ⌘)', group: 'photos' })
    expect(row?.feature).toBeUndefined()
  })
})

describe('статья «Обложка» (132-22)', () => {
  it('шаблоны сторонки привязаны к уровню «Полный»; Пробел — про лицевую сторонку, без общей строки про разворот', () => {
    const a = art('cover')
    expect(a.how).toMatch(/На уровне «Полный»: «Сохранить как шаблон…»[^.]*✎[^.]*×/)
    expect(a.how).toContain('Пробел и Shift+Пробел — следующая и предыдущая раскладка лицевой сторонки')
    expect(a.keys).toBeUndefined() // общая строка Пробела говорит «раскладка разворота»
  })
})
