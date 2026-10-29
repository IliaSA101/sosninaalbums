import { describe, expect, it } from 'vitest'
import frozenText from '../../../../testdata/guide-ids.txt?raw'
import { ARTICLES } from './articles'
import { RENAMED } from './renamed'

// Заморозка id статей F1 (спека 140, 0.30-140-11): id — ключи отметок «Изучил». Новый id нужно дописать в
// testdata/guide-ids.txt; пропавший — объяснить записью в RENAMED.

// Строки файла без пустых и комментариев (#).
function parseIds(text: string): string[] {
  return text
    .split(/\r?\n/)
    .map((s) => s.trim())
    .filter((s) => s !== '' && !s.startsWith('#'))
}

// То же правило id, что в Go (internal/prefs/guide.go): иначе MarkGuide молча отбросит отметку.
const ID_RULE = /^[a-z0-9][a-z0-9-]{0,39}$/

// Id и ключи/цели RENAMED не по правилу (пустая цель — «статью убрали», допустима).
function formatViolations(articles: readonly string[], renamed: Record<string, string>): string[] {
  const out: string[] = []
  const bad = (id: string): boolean => !ID_RULE.test(id)
  for (const id of articles) if (bad(id)) out.push(`id «${id}» не подходит под ${ID_RULE.source}`)
  for (const [old, to] of Object.entries(renamed)) {
    if (bad(old)) out.push(`RENAMED: ключ «${old}» не подходит под ${ID_RULE.source}`)
    if (to !== '' && bad(to)) out.push(`RENAMED: цель «${to}» не подходит под ${ID_RULE.source}`)
  }
  return out
}

// Все нарушения; пусто — страж зелёный. Отдельная функция, чтобы проверить сам страж на подменённых списках.
function idViolations(articles: readonly string[], frozen: readonly string[], renamed: Record<string, string>): string[] {
  const have = new Set(articles)
  const fro = new Set(frozen)
  const out: string[] = []
  for (const id of articles) if (!fro.has(id)) out.push(`id «${id}» нет в testdata/guide-ids.txt — допишите id`)
  for (const id of frozen) if (!have.has(id) && !(id in renamed)) out.push(`id «${id}» исчез — нужна запись в RENAMED`)
  for (const [old, to] of Object.entries(renamed)) {
    if (to !== '' && !have.has(to)) out.push(`RENAMED «${old}» → «${to}»: такой статьи нет`)
    if (have.has(old)) out.push(`RENAMED «${old}»: статья с этим id снова есть — уберите запись`)
  }
  out.push(...formatViolations(articles, renamed))
  if (fro.size !== frozen.length) out.push('в testdata/guide-ids.txt есть повторы')
  if (have.size !== articles.length) out.push('в ARTICLES есть повторы id')
  return out
}

describe('заморозка id статей: страж', () => {
  it('нарушений нет', () => {
    const ids = ARTICLES.map((a) => a.id)
    expect(idViolations(ids, parseIds(frozenText), RENAMED)).toEqual([])
  })
  it('файл не пуст и отсортирован', () => {
    const ids = parseIds(frozenText)
    expect(ids.length).toBeGreaterThan(100)
    expect(ids).toEqual([...ids].sort())
  })
})

describe('самопроверка стража: подмена роняет', () => {
  const frozen = ['a', 'b', 'c']
  it('чистый набор и учтённая запись RENAMED — без нарушений', () => {
    expect(idViolations(['a', 'b', 'c'], frozen, {})).toEqual([])
    expect(idViolations(['a', 'b', 'd'], [...frozen, 'd'], { c: 'd' })).toEqual([])
    expect(idViolations(['a', 'b'], frozen, { c: '' })).toEqual([])
  })
  it('новый id не в файле — провал «допишите»', () => {
    expect(idViolations(['a', 'b', 'c', 'n'], frozen, {}).join()).toContain('допишите id')
  })
  it('id пропал без RENAMED — провал', () => {
    expect(idViolations(['a', 'b'], frozen, {}).join()).toContain('нужна запись в RENAMED')
  })
  it('RENAMED на несуществующую статью — провал', () => {
    expect(idViolations(['a', 'b'], frozen, { c: 'zzz' }).join()).toContain('такой статьи нет')
  })
  it('RENAMED на id, который снова есть среди статей — провал', () => {
    expect(idViolations(['a', 'b', 'c'], frozen, { c: '' }).join()).toContain('статья с этим id снова есть')
  })
  it('id не по правилу (41 символ, заглавная, подчёркивание, пустой) — провал', () => {
    const long = 'a'.repeat(41)
    expect(idViolations([long], [long], {}).join()).toContain('не подходит')
    expect(idViolations(['autoBuild'], ['autoBuild'], {}).join()).toContain('не подходит')
    expect(idViolations(['a_b'], ['a_b'], {}).join()).toContain('не подходит')
    expect(idViolations(['a'.repeat(40)], ['a'.repeat(40)], {})).toEqual([])
  })
  it('RENAMED: ключ или цель не по правилу — провал', () => {
    expect(idViolations(['a'], ['a', 'Old'], { Old: 'a' }).join()).toContain('ключ «Old»')
    expect(idViolations(['a_b'], ['x'], { x: 'a_b' }).join()).toContain('цель «a_b»')
    expect(idViolations(['a'], ['a', 'x'], { x: '' })).toEqual([])
  })
  it('повторы в файле — провал', () => {
    expect(idViolations(['a'], ['a', 'a'], {}).join()).toContain('повторы')
  })
  it('разбор пропускает комментарии и пустые строки', () => {
    expect(parseIds('# x\n\na\r\n b \n')).toEqual(['a', 'b'])
  })
})
