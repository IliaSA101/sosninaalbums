import { describe, expect, it } from 'vitest'
import { ARTICLES, GUIDE_GROUPS, articleById, type Article, type GroupId } from '../guidetext/articles'
import { GUIDE_ORDER, inOrder } from '../guidetext/order'
import { countLine, learnedOf, openGroups, orderOf, progress, sectionMark, sectionsOf, stepSel } from './guidelist'

// Чистая логика списка окна F1 (спека 136, design §1).
describe('sectionsOf: разделы и порядок', () => {
  it('разделы идут в порядке GUIDE_GROUPS, статьи — в порядке GUIDE_ORDER, каждая один раз', () => {
    const s = sectionsOf()
    expect(s.map((x) => x.id)).toEqual(GUIDE_GROUPS.map((g) => g.id))
    for (const x of s) expect(x.items.map((a) => a.id)).toEqual(GUIDE_ORDER[x.id])
    expect(s.flatMap((x) => x.items)).toHaveLength(ARTICLES.length)
  })
  it('входной порядок не важен: развёрнутый массив даёт тот же показ', () => {
    const rev = sectionsOf([...ARTICLES].reverse())
    expect(rev.map((x) => x.items.map((a) => a.id))).toEqual(sectionsOf().map((x) => x.items.map((a) => a.id)))
  })
  it('статья, не названная в GUIDE_ORDER, встаёт в конец своего раздела; пустые разделы не показываются', () => {
    const extra = { ...articleById('undo')!, id: 'new-one' }
    const s = sectionsOf([extra, articleById('first-steps')!, articleById('start-window')!])
    expect(s).toHaveLength(1)
    expect(s[0].items.map((a) => a.id)).toEqual(['start-window', 'first-steps', 'new-one'])
  })
  it('inOrder не меняет входной массив и устойчив для равных', () => {
    const src = [{ id: 'x1' }, { id: 'crop' }, { id: 'x2' }, { id: 'start-window' }]
    const out = inOrder(src, 'start')
    expect(out.map((a) => a.id)).toEqual(['crop', 'start-window', 'x1', 'x2']) // crop стоит в GUIDE_ORDER раньше start-window
    expect(src[0].id).toBe('x1')
  })
})

describe('openGroups: свои ∪ раздел выбора', () => {
  const mine = new Set<GroupId>(['text'])
  it('раздел выбранной статьи открыт всегда, свои — тоже', () => {
    expect([...openGroups(mine, 'cover')].sort()).toEqual(['cover', 'text'])
  })
  it('нет выбора — только свои; набор своих не меняется', () => {
    expect([...openGroups(mine, undefined)]).toEqual(['text'])
    openGroups(mine, 'view')
    expect([...mine]).toEqual(['text'])
  })
  it('выбор в уже открытом руками разделе ничего не дублирует', () => {
    expect(openGroups(mine, 'text').size).toBe(1)
  })
})

describe('stepSel: шаг стрелкой по всем статьям', () => {
  const order = orderOf(sectionsOf())
  const lastOf = (g: GroupId) => GUIDE_ORDER[g][GUIDE_ORDER[g].length - 1]
  it('через границу раздела: последняя статья раздела → первая следующего', () => {
    expect(stepSel(order, lastOf('start'), 'ArrowDown')).toBe(GUIDE_ORDER.photos[0])
    expect(stepSel(order, GUIDE_ORDER.photos[0], 'ArrowUp')).toBe(lastOf('start'))
  })
  it('Home и End — первая и последняя статья всего списка', () => {
    expect(stepSel(order, 'details', 'Home')).toBe(GUIDE_ORDER.start[0])
    expect(stepSel(order, 'details', 'End')).toBe(lastOf('manual'))
  })
  it('на краях остаёмся на месте; нет выбора — вход с начала или с конца; пусто — null', () => {
    expect(stepSel(order, order[0], 'ArrowUp')).toBe(order[0])
    expect(stepSel(order, order[order.length - 1], 'ArrowDown')).toBe(order[order.length - 1])
    expect(stepSel(order, null, 'ArrowDown')).toBe(order[0])
    expect(stepSel(order, null, 'ArrowUp')).toBe(order[order.length - 1])
    expect(stepSel([], 'x', 'ArrowDown')).toBeNull()
  })
})

describe('countLine: счётчик в шапке', () => {
  it('без запроса — число статей со склонением', () => {
    const o = { shown: 115, total: 115, searching: false, filtered: false }
    expect(countLine(o)).toBe('115 статей')
    expect(countLine({ ...o, total: 121 })).toBe('121 статья')
    expect(countLine({ ...o, total: 22 })).toBe('22 статьи')
  })
  it('с описью — «115 статей · N функций»; поиск и фильтр число функций не показывают', () => {
    const o = { shown: 115, total: 115, searching: false, filtered: false, functions: 170 }
    expect(countLine(o)).toBe('115 статей · 170 функций')
    expect(countLine({ ...o, functions: 171 })).toBe('115 статей · 171 функция')
    expect(countLine({ ...o, shown: 7, searching: true })).toBe('Найдено 7 из 115')
    expect(countLine({ ...o, shown: 9, filtered: true })).toBe('9 из 115')
  })
  it('поиск — «Найдено N из M», фильтр строки уровней — «N из M»', () => {
    expect(countLine({ shown: 7, total: 115, searching: true, filtered: false })).toBe('Найдено 7 из 115')
    expect(countLine({ shown: 9, total: 115, searching: false, filtered: true })).toBe('9 из 115')
    expect(countLine({ shown: 0, total: 115, searching: true, filtered: true })).toBe('Найдено 0 из 115')
  })
})

describe('отметки «Изучил»: learnedOf, progress, sectionMark', () => {
  const known = new Set(['a', 'b', 'c'])
  it('learnedOf: свои id остаются, неизвестные отбрасываются', () => {
    expect([...learnedOf(['a', 'zzz', 'c'], {}, known)]).toEqual(['a', 'c'])
    expect(learnedOf([], {}, known).size).toBe(0)
  })
  it('learnedOf: отметка старого id засчитывается новому; убранная статья («») и цель без статьи не считаются', () => {
    expect([...learnedOf(['old', 'gone', 'lost'], { old: 'b', gone: '', lost: 'nope' }, known)]).toEqual(['b'])
  })
  it('learnedOf: старый и новый id вместе — одна отметка', () => {
    expect(learnedOf(['old', 'b'], { old: 'b' }, known).size).toBe(1)
  })
  const arts = [
    { id: 'a', feature: 'base' },
    { id: 'b', feature: 'full' },
    { id: 'c', feature: 'base' },
  ] as unknown as Article[]
  it('progress: m — доступные на уровне; изученная, но скрытая уровнем, в n не входит', () => {
    const can = (f: string) => f === 'base'
    expect(progress(arts, new Set(['a', 'b']), can)).toEqual({ n: 1, m: 2 })
    expect(progress(arts, new Set(), can)).toEqual({ n: 0, m: 2 })
    expect(progress(arts, new Set(['a', 'b', 'c']), () => true)).toEqual({ n: 3, m: 3 })
    expect(progress([], new Set(['a']), () => true)).toEqual({ n: 0, m: 0 })
  })
  it('sectionMark: «k / n» при k > 0, иначе «n»', () => {
    const items = [{ id: 'a' }, { id: 'b' }, { id: 'c' }]
    expect(sectionMark(items, new Set())).toBe('3')
    expect(sectionMark(items, new Set(['b', 'x']))).toBe('1 / 3')
    expect(sectionMark(items, new Set(['a', 'b', 'c']))).toBe('3 / 3')
    expect(sectionMark([], new Set(['a']))).toBe('0')
  })
})
