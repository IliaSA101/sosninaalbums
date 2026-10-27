import { describe, expect, it } from 'vitest'
import type { MenuItem } from '../menu/menu'
import { TOUR } from '../tour/steps'
import { ARTICLES } from '../guidetext/articles'
import { menus } from './show'

// Страж путей «Правка ▸ …» и «Разворот ▸ …» в статьях справочника и шагах тура (124 U1, 0.26-124-26): пункт
// перенесли между меню — текст статьи не должен отстать. Пути «Файл», «Вид», «Справка» не проверяются: перенос их не задевает.
const PATH = /«(Правка|Разворот) ▸ ([^»]+?)»/g

// Пункт по подписи; «Сделать ч/б» и «Сделать цветным» — один пункт, подпись зависит от выделения.
const same = (label: string, seg: string): boolean =>
  label === seg || (seg === 'Сделать ч/б' && label === 'Сделать цветным') || (seg === 'Сделать цветным' && label === 'Сделать ч/б')

// Спуск по меню: путь вида «Правка ▸ Повернуть ▸» (хвост « ▸» у подменю допустим). null — путь есть, ошибки нет.
function pathError(menu: string, rest: string): string | null {
  const root = menus().find((m) => m.label === menu)
  if (!root) return `меню «${menu}» нет в строке`
  const parts = rest.replace(/\s*▸$/, '').split(' ▸ ')
  let list: MenuItem[] = root.items
  for (let i = 0; i < parts.length; i++) {
    const it = list.find((x) => same(x.label, parts[i]))
    if (!it) return `в «${menu}» нет пункта «${parts.slice(0, i + 1).join(' ▸ ')}»`
    if (i < parts.length - 1) list = it.sub ?? []
    else if (/▸$/.test(rest) && !it.sub) return `у «${parts[i]}» нет подменю`
  }
  return null
}

interface Found {
  where: string
  menu: string
  rest: string
}

// Тексты, где путь может встретиться: что/где/как статей и поля шагов тура.
function texts(): { from: string; text: string }[] {
  const out: { from: string; text: string }[] = []
  for (const a of ARTICLES) for (const f of [a.what, a.where, a.how]) if (f) out.push({ from: `статья ${a.id}`, text: f })
  for (const s of TOUR) for (const f of [s.title, s.ready, s.empty]) out.push({ from: `шаг тура ${s.id}`, text: f })
  return out
}

function found(): Found[] {
  return texts().flatMap(({ from, text }) => [...text.matchAll(PATH)].map((m) => ({ where: from, menu: m[1], rest: m[2] })))
}

describe('пути «Правка ▸ …» и «Разворот ▸ …» в справочнике и туре', () => {
  it('каждый путь ведёт к пункту своего меню', () => {
    const bad = found()
      .map((f) => ({ f, err: pathError(f.menu, f.rest) }))
      .filter((x) => x.err)
      .map((x) => `${x.f.where}: «${x.f.menu} ▸ ${x.f.rest}» — ${x.err}`)
    expect(bad).toEqual([])
  })

  it('страж видит пути: 10 «Разворот ▸ …» и не меньше 9 «Правка ▸ …»', () => {
    const all = found()
    expect(all.filter((f) => f.menu === 'Разворот').length).toBe(10)
    expect(all.filter((f) => f.menu === 'Правка').length).toBeGreaterThanOrEqual(9)
  })

  it('страж ловит перенесённый пункт по старому пути и несуществующее подменю', () => {
    expect(pathError('Правка', 'Галерея фонов…')).toMatch(/нет пункта/)
    expect(pathError('Разворот', 'Галерея фонов…')).toBeNull()
    expect(pathError('Правка', 'Повернуть ▸')).toBeNull()
    expect(pathError('Правка', 'Удалить ▸')).toMatch(/нет подменю/)
    expect(pathError('Правка', 'Слой ▸ Выше')).toBeNull()
    expect(pathError('Правка', 'Сделать цветным')).toBeNull()
    expect(pathError('Правка', 'Сделать ч/б')).toBeNull()
  })
})
