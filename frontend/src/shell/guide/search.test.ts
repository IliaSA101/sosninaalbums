import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { prefs } from '../../base/prefs/prefs.svelte'
import { ARTICLES } from './articles'
import { fromLatin, grouped, norm, search, stem, tokens } from './search'

const titles = (q: string) => search(q).hits.map((h) => h.a.title)

// Клавиши статей — из реестра (074): без него поиск по клавише пуст.
beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

describe('нормализация', () => {
  it('регистр и «ё» не важны', () => {
    expect(norm('ЁЖ Звёзды')).toBe('еж звезды')
    expect(tokens('Ctrl+Z, отмена')).toEqual(['ctrl+z', 'отмена'])
  })
  it('основа: окончания отбрасываются по длине слова', () => {
    expect(stem('виньетку')).toBe('виньет')
    expect(stem('имена')).toBe('имен')
    expect(stem('фото')).toBe('фот')
    expect(stem('са')).toBe('са')
  })
  it('английская раскладка → русская', () => {
    expect(fromLatin('j,kj;rf')).toBe('обложка')
    expect(fromLatin('ghbdtn')).toBe('привет')
  })
})

describe('поиск (079): рамочки и прозрачность', () => {
  it('«рамочка», «паспарту», «полароид» находят «Рамочки»; «прозрачность» — «Прозрачность фото»', () => {
    for (const q of ['рамочка', 'рамочки', 'паспарту', 'полароид', 'обводка']) expect(titles(q), q).toContain('Рамочки')
    expect(titles('прозрачность')[0]).toBe('Прозрачность фото')
  })
})

describe('поиск (спека 033)', () => {
  it('«скруглённое» (076) находит «Добавить место» и «Форма места», ё и е не важны', () => {
    const a = titles('скруглённое')
    expect(a).toEqual(expect.arrayContaining(['Добавить место', 'Форма места']))
    expect(titles('скругленное')).toEqual(a)
  })
  it('«виньетку» → «Настроить виньетку» в первых трёх', () => {
    expect(titles('виньетку').slice(0, 3)).toContain('Настроить виньетку')
  })
  it('«отмечённые» и «отмеченные» — одно и то же', () => {
    const a = titles('отмечённые')
    expect(a.length).toBeGreaterThan(0)
    expect(titles('отмеченные')).toEqual(a)
  })
  it('набрано в английской раскладке: j,kj;rf → «Обложка»', () => {
    const r = search('j,kj;rf')
    expect(r.hits.map((h) => h.a.title)).toContain('Обложка')
    expect(r.from).toBe('обложка')
  })
  it('бросок на ручной разворот (042): «места», «плитка», «не хватает мест» находят статью', () => {
    for (const q of ['места', 'плитка', 'не хватает мест']) expect(titles(q), q).toContain('Фото на ручной разворот')
  })
  it('фото через корешок (064): «layflat», «сгиб», «корешок», «лесенка» находят статью', () => {
    for (const q of ['layflat', 'сгиб', 'корешок', 'лесенка', 'через разворот', 'лейфлэт'])
      expect(
        search(q).hits.map((h) => h.a.id),
        q,
      ).toContain('fold')
  })
  it('«клавиши» → «Горячие клавиши»', () => {
    expect(titles('клавиши')).toContain('Горячие клавиши')
  })
  it('поиск по клавише: Ctrl+Z находит отмену', () => {
    expect(titles('ctrl+z')).toContain('Отменить и повторить')
  })
  it('индекс клавиш пересобирается вместе с реестром: своя клавиша находит статью, пока её нет — нет', () => {
    expect(titles('q')).not.toContain('Отменить и повторить')
    useCatalog({ undo: ['KeyQ'] })
    expect(titles('q')).toContain('Отменить и повторить')
    useCatalog()
    expect(titles('q')).not.toContain('Отменить и повторить')
  })
  it('080-9: «ч/б» на «Простом» находит «Ч/б развороты» (скрытую функцию), а не «Похожие кадры»', () => {
    prefs.load({ level: 'simple', showStart: false, first: false, custom: null })
    const ids = search('ч/б').hits.map((h) => h.a.id)
    expect(ids).toContain('auto-gray')
    expect(ids).not.toContain('similar')
  })
  it('080-9: индекс пересобирается при смене уровня: «можно несколько» (фильтр семейств) в «Автосборке» только пока есть «Варианты раскладки»', () => {
    const has = () => search('несколько').hits.some((h) => h.a.id === 'autobuild')
    prefs.load({ level: 'standard', showStart: false, first: false, custom: null })
    expect(has()).toBe(true)
    prefs.load({ level: 'custom', showStart: false, first: false, custom: { base: 'standard', on: [], off: ['variants'] } })
    expect(has()).toBe(false)
    prefs.load({ level: 'simple', showStart: false, first: false, custom: null })
  })
  it('все слова запроса обязательны', () => {
    expect(titles('виньетка шрифт зеркало')).toEqual([])
    expect(titles('виньетка шаблон')).toContain('Шаблоны виньетки')
  })
  it('«свой шаблон виньетки» → статья «Свои шаблоны виньетки»', () => {
    expect(search('свой шаблон виньетки').hits.map((h) => h.a.id)).toContain('vig-own')
  })
  it('ничего не нашлось — пустой список и без смены раскладки', () => {
    const r = search('ъъъъъ')
    expect(r.hits).toEqual([])
    expect(r.from).toBeNull()
  })
  it('название весомее текста: статья с названием идёт раньше', () => {
    expect(titles('обложка')[0]).toBe('Обложка')
  })
  it('пустой запрос — все статьи', () => {
    expect(search('  ').hits).toHaveLength(ARTICLES.length)
  })
})

describe('список без запроса', () => {
  it('только смысловые разделы по порядку, без «Новое в …»', () => {
    const s = grouped()
    expect(s.map((x) => x.id)).toEqual(['start', 'photos', 'spreads', 'onspread', 'text', 'cover', 'view', 'manual'])
    expect(s.some((x) => x.title.startsWith('Новое') || x.id.startsWith('new-'))).toBe(false)
  })
  it('каждая статья — ровно один раз, в своей группе', () => {
    const s = grouped()
    const ids = s.flatMap((x) => x.items.map((a) => a.id))
    expect(ids).toHaveLength(ARTICLES.length)
    expect(new Set(ids).size).toBe(ids.length)
    for (const x of s) for (const a of x.items) expect(a.group, a.id).toBe(x.id)
    // Бывшие «новые» статьи лежат в смысловых группах.
    const grp = (id: string) => s.find((x) => x.items.some((a) => a.id === id))?.id
    expect(grp('variety')).toBe('spreads')
    expect(grp('vig-own')).toBe('cover')
    expect(grp('names')).toBe('text')
    expect(grp('autobuild')).toBe('spreads')
  })
})
