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

describe('статьи про макеты альбома (131 U2)', () => {
  it('четыре статьи в «Развороты и раскладки», на любом уровне, все ведут к «Файл ▸ Макеты альбома…»', () => {
    const ids = ['looks', 'looks-built', 'looks-step', 'looks-format']
    for (const id of ids) {
      const a = art(id)
      expect(a, id).toMatchObject({ group: 'spreads', feature: 'base', menu: ['looks'], spot: 'menu:looks' })
    }
    expect(art('looks').title).toBe('Макеты альбома')
    expect(menuPath('looks')).toContain('Макеты альбома…')
  })
  it('слова спеки: входы, «Что сделать», «Поставить на разворот N», «Разворот макета», формат', () => {
    expect(art('looks').where).toContain('или начните с макета альбома')
    expect(art('looks').where).toContain('Альбом по макету…')
    expect(art('looks-built').what).toContain('«Фоны по серии»')
    expect(art('looks-built').where).toContain('«Весь альбом заново» их не пересоберёт')
    expect(art('looks-step').what).toContain('«Поставить на разворот N»')
    expect(art('looks-step').where).toContain('«Разворот макета «…»»')
    expect(art('looks-format').what).toContain('20×30 и 40×30')
  })
  it('Отмена в статье — клавиша из реестра, снята — без скобок', () => {
    useCatalog()
    expect(articleText(art('looks').where ?? '')).toContain('Отмена (Ctrl+Z) возвращает его')
    useCatalog({ undo: [] })
    expect(articleText(art('looks').where ?? '')).toContain('Отмена возвращает его')
  })
})

describe('статьи про заготовки и сохранение макета (131 U3)', () => {
  it('«Заготовки макета» и «Сохранить альбом как макет» — в «Развороты и раскладки», на любом уровне', () => {
    expect(art('looks-draft')).toMatchObject({ group: 'spreads', feature: 'base', menu: ['looks'], spot: 'menu:looks' })
    expect(art('looks-save')).toMatchObject({ group: 'spreads', feature: 'base', menu: ['saveLook'], spot: 'menu:saveLook' })
    expect(menuPath('saveLook')).toContain('Сохранить альбом как макет…')
  })
  it('слова спеки: пунктир, «Сделать обычной надписью», вопрос выгрузки, флажки, предел 24', () => {
    const d = art('looks-draft')
    expect(d.what).toContain('рамке-пунктире')
    expect(d.where).toContain('«Сделать обычной надписью»')
    expect(d.where).toContain('«… пустые места: 3, заготовки: 2»')
    expect(d.where).toContain('на любом уровне')
    const s = art('looks-save')
    for (const w of ['«Места для фото»', '«Надписи — заготовками»', '«Обложка»', 'не больше 24', '«Повторяемые»'])
      expect(s.where, w).toContain(w)
    expect(s.what).toContain('«Мои»')
  })
})
