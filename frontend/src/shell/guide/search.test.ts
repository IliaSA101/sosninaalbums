import { describe, expect, it } from 'vitest'
import { ARTICLES } from './articles'
import { fromLatin, grouped, norm, search, stem, tokens } from './search'

const titles = (q: string) => search(q).hits.map((h) => h.a.title)

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

describe('поиск (спека 033)', () => {
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
  it('«клавиши» → «Горячие клавиши»', () => {
    expect(titles('клавиши')).toContain('Горячие клавиши')
  })
  it('поиск по клавише: Ctrl+Z находит отмену', () => {
    expect(titles('ctrl+z')).toContain('Отменить и повторить')
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
