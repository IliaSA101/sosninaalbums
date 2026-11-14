import { describe, expect, it } from 'vitest'
import { emptyText, deleteText, type DecorItem } from './decor'
import {
  FILTERED_EMPTY,
  pageRatio,
  putQuestion,
  putToast,
  saveScopes,
  saveTheme,
  saveThemes,
  sheetRatio,
  tileLook,
  tplFits,
  tplNote,
  tplSideItems,
  tplSrc,
  tplTitle,
  tplVisible,
} from './spreadtpl'

const two = { w: 600, h: 300, mid: 300 } // разворот 30×30
const sheet = { w: 300, h: 300, mid: 0 } // лист 031
const item = (id: string, pages: number, extra: Partial<DecorItem> = {}): DecorItem => ({
  id,
  theme: 'wedding',
  name: id,
  url: `/media/tpl/${id}?v=1`,
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  pages,
  ...extra,
})
const A = item('sa:tpl-a', 2)
const B = item('sa:tpl-b', 1)
const plan = (over: Partial<{ frozen: number; left: number; texts: number; stickers: number }> = {}) => ({
  frozen: 0,
  left: 0,
  texts: 0,
  stickers: 0,
  ...over,
})

describe('«Куда» отбирает плитки (сц. 1)', () => {
  it('весь разворот: и разворот, и страница', () => {
    expect(tplVisible([A, B], 'spread', two)).toEqual([A, B])
  })
  it('«Левая» и «Правая»: только страницы', () => {
    expect(tplVisible([A, B], 'left', two)).toEqual([B])
    expect(tplVisible([A, B], 'right', two)).toEqual([B])
  })
  it('лист без шва: плитка-разворот не встанет даже на «весь лист»', () => {
    expect(tplFits(A, 'spread', sheet)).toBe(false)
    expect(tplFits(B, 'spread', sheet)).toBe(true)
  })
  it('пусто — пусто; подсказка про «Весь разворот» нужна окну, когда отбор опустошил тему', () => {
    expect(tplVisible([], 'left', two)).toEqual([])
    expect(FILTERED_EMPTY).toContain('Весь разворот')
  })
})

describe('картинка плитки', () => {
  it('пропорции: страница — половина листа со швом, лист без шва — весь', () => {
    expect(sheetRatio(two)).toBe(2)
    expect(pageRatio(two)).toBe(1)
    expect(pageRatio(sheet)).toBe(1)
  })
  it('разворот на «весь разворот»: кнопка и картинка одной пропорции, ширина по плитке', () => {
    expect(tileLook(A, two, 'spread', 168, 1)).toEqual({ box: 2, r: 2, px: 192, page: false })
  })
  it('страница в «весь разворот»: картинка уже кнопки, PNG заказан поуже', () => {
    const l = tileLook(B, two, 'spread', 168, 1)
    expect(l).toMatchObject({ box: 2, r: 1, page: true })
    expect(l.px).toBe(128) // 84 CSS px → шаг 64
  })
  it('«Левая»: кнопка — страница, картинка во всю кнопку', () => {
    expect(tileLook(B, two, 'left', 168, 1)).toEqual({ box: 1, r: 1, px: 192, page: false })
  })
  it('экран 2× — вдвое больше точек; не больше предела Go; r в пределах Go', () => {
    expect(tileLook(A, two, 'spread', 168, 2).px).toBe(384)
    expect(tileLook(A, two, 'spread', 2000, 3).px).toBe(1024)
    expect(tileLook(A, { w: 100, h: 1000, mid: 50 }, 'spread', 168, 1).r).toBe(0.2)
    expect(tileLook(A, { w: 3000, h: 100, mid: 1500 }, 'spread', 168, 1).r).toBe(5)
  })
  it('адрес: px и r дописываются к ?v=1', () => {
    expect(tplSrc('/media/tpl/sa:tpl-a?v=1', 256, 2)).toBe('/media/tpl/sa:tpl-a?v=1&px=256&r=2')
    expect(tplSrc('/media/tpl/x', 64, 0.666666)).toBe('/media/tpl/x?px=64&r=0.6667')
  })
  it('подпись: «страница» у pages 1; набор — в подсказке плитки', () => {
    expect(tplNote(B)).toBe('страница')
    expect(tplNote(A)).toBeUndefined()
    expect(tplTitle(A)).toBe('sa:tpl-a')
    expect(tplTitle(item('x', 2, { name: 'Зима', pack: 'Айвори' }))).toBe('Зима · набор «Айвори»')
  })
  it('подсказка плитки: «страница» есть, порядок имя · страница · набор; фоны и стикеры (без pages) — имя и набор', () => {
    expect(tplTitle(B)).toBe('sa:tpl-b · страница')
    expect(tplTitle(item('x', 1, { name: 'Акварель', pack: 'Айвори' }))).toBe('Акварель · страница · набор «Айвори»')
    expect(tplTitle({ name: 'Роза', pack: 'Айвори' })).toBe('Роза · набор «Айвори»')
    expect(tplTitle({ name: 'Роза' })).toBe('Роза')
  })
})

describe('вопрос перед постановкой (сц. 3)', () => {
  it('нечего терять — без вопроса; «Авто» → сетка вопроса не требует', () => {
    expect(putQuestion(plan(), { mid: 300 })).toBeNull()
    expect(putQuestion(plan({ frozen: 1 }), { mid: 300 })).toBeNull()
  })
  it('слова и числа: фото в ленту и стикеры', () => {
    expect(putQuestion(plan({ left: 2, stickers: 1 }), { mid: 300 })).toBe('2 фото вернутся в ленту, 1 стикер заменится. Поставить?')
  })
  it('склонения: 1, 2, 5; надписи', () => {
    expect(putQuestion(plan({ left: 1 }), { mid: 300 })).toBe('1 фото вернётся в ленту. Поставить?')
    expect(putQuestion(plan({ stickers: 5, texts: 2 }), { mid: 300 })).toBe('2 блока текста заменятся, 5 стикеров заменятся. Поставить?')
  })
  it('виньетка и лист 031: места не меняются — строка только когда в шаблоне есть места', () => {
    const vig = { mid: 0, kind: 'vignette' as const, vig: {} as never }
    expect(putQuestion(plan({ stickers: 1 }), vig, 3)).toBe('1 стикер заменится. Места виньетки не меняются. Поставить?')
    expect(putQuestion(plan({ stickers: 1 }), vig, 0)).toBe('1 стикер заменится. Поставить?')
    expect(putQuestion(plan({ stickers: 1 }), { mid: 0 }, 2)).toBe('1 стикер заменится. Места листа не меняются. Поставить?')
    expect(putQuestion(plan({ stickers: 1 }), { mid: 300 }, 2)).toBe('1 стикер заменится. Поставить?')
  })
})

describe('тост итога (сц. 2)', () => {
  it('поставлено: фото в ленту и пропавшие слои — через « · »', () => {
    expect(putToast(true, 0, 0)).toEqual({ text: 'Разворот поставлен', undo: true })
    expect(putToast(true, 2, 1)).toEqual({ text: 'Разворот поставлен · 2 фото вернулись в ленту · Нет в библиотеке: 1', undo: true })
    expect(putToast(true, 1, 0).text).toBe('Разворот поставлен · 1 фото вернулось в ленту')
  })
  it('повтор: версия не выросла — без «Вернуть»', () => {
    expect(putToast(false, 0, 0)).toEqual({ text: 'Здесь уже такой разворот', undo: false })
    expect(putToast(false, 0, 2).text).toBe('Здесь уже такой разворот · Нет в библиотеке: 2')
  })
  it('обложка (131 «Поставить на обложку»): те же правила, слово — обложка', () => {
    expect(putToast(true, 0, 0, true)).toEqual({ text: 'Обложка поставлена', undo: true })
    expect(putToast(true, 1, 1, true).text).toBe('Обложка поставлена · 1 фото вернулось в ленту · Нет в библиотеке: 1')
    expect(putToast(false, 0, 0, true)).toEqual({ text: 'Здесь уже такая обложка', undo: false })
  })
})

describe('«Сохранить разворот…» и пустые состояния', () => {
  const themes = [
    { id: 'recent', name: 'Недавние', count: 1 },
    { id: 'all', name: 'Все', count: 3 },
    { id: 'wedding', name: 'Свадьба', count: 2 },
    { id: 'mine', name: 'Мои', count: 1 },
  ]
  it('темы для сохранения — без выборок', () => {
    expect(saveThemes({ themes }).map((t) => t.id)).toEqual(['wedding', 'mine'])
    expect(saveThemes(null)).toEqual([])
  })
  it('тема по умолчанию: открытая; из выборок — «Мои»', () => {
    expect(saveTheme('wedding')).toBe('wedding')
    expect(saveTheme('recent')).toBe('mine')
    expect(saveTheme('all')).toBe('mine')
    expect(saveTheme('')).toBe('mine')
  })
  it('«Что» — без сторонок обложки', () => {
    expect(
      saveScopes([
        { id: 'spread', label: 'Весь разворот' },
        { id: 'left', label: 'Левая' },
        { id: 'spine', label: 'Корешок' },
      ]).map((i) => i.id),
    ).toEqual(['spread', 'left'])
  })
  it('пустые «Мои» зовут сохранить разворот, а не загружать файл', () => {
    expect(emptyText('mine', false, 'spread')).toBe('Сохраните свой разворот кнопкой «Сохранить разворот…»')
    expect(emptyText('mine', true, 'spread')).toBe('Здесь появятся ваши развороты')
    expect(emptyText('mine', false)).toBe('Здесь появятся ваши фоны: нажмите «Добавить свой фон…»')
  })
  it('убрать из библиотеки: у разворота — про поставленные развороты', () => {
    expect(deleteText('Мой', 'spread')).toBe('«Мой» будет убран из библиотеки. Уже поставленные развороты не изменятся.')
    expect(deleteText('Мой')).toContain('Где он уже стоит')
  })
})

describe('счётчики тем идут по тому же отбору, что и плитки', () => {
  const cat = {
    kind: 'spread' as const,
    themes: [
      { id: 'recent', name: 'Недавние', count: 3 },
      { id: 'all', name: 'Все', count: 3 },
      { id: 'wedding', name: 'Свадьба', count: 2 },
      { id: 'mine', name: 'Мои', count: 1 },
    ],
    items: [item('a', 2), item('b', 1), item('c', 2, { theme: 'mine' })],
    recent: ['a', 'b', 'c'],
    new: [],
    readOnly: false,
  }
  const counts = (scope: 'spread' | 'left', sp: { mid: number }) => tplSideItems(cat, scope, sp).map((t) => t.count)
  it('весь разворот: все плитки', () => expect(counts('spread', two)).toEqual([3, 3, 2, 1]))
  it('«Левая»: только страницы — «Недавние 1», а не 3', () => expect(counts('left', two)).toEqual([1, 1, 1, 0]))
  it('лист без шва: только страницы', () => expect(counts('spread', sheet)).toEqual([1, 1, 1, 0]))
  it('нет каталога — пусто', () => expect(tplSideItems(null, 'spread', two)).toEqual([]))
})
