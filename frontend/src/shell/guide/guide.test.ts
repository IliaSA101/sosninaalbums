import { describe, expect, it } from 'vitest'
import { FEATURES, rowOf } from '../../base/prefs/levels'
import { HOTKEYS } from '../menu/hotkeys'
import { ARTICLES, GUIDE_GROUPS, placesOf } from './articles'
import { HINT_SPOTS, menuPath, menus } from './show'

// Стражи справочника (спека 033, design §9): справка не устаревает молча. Новая функция без статьи роняет
// первый, переименованный или удалённый элемент разметки, пункт меню или клавиша — остальные.
const sources = import.meta.glob('/src/**/*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

// Якоря в разметке: только литералом data-guide="…" (несколько id — через пробел) или литералом в спреде
// {...cond ? { 'data-guide': '…' } : {}} (якорь только у части экземпляров: текущий номер, плавающая карточка).
// Динамический data-guide={…} запрещён.
const ANCHOR = /(?<![\w-])(?:data-guide="([^"]*)"|'data-guide': '([^']*)')/g
const anchors = (): Set<string> => {
  const out = new Set<string>()
  for (const src of Object.values(sources))
    for (const m of src.matchAll(ANCHOR))
      (m[1] ?? m[2])
        .split(/\s+/)
        .filter(Boolean)
        .forEach((id) => out.add(id))
  return out
}
// Пункты меню без своей статьи: причина обязательна. Новый пункт с командой роняет страж 6, пока ему не
// напишут статью (menu: или spot: menu:) или не добавят сюда с причиной.
const NO_ARTICLE: Record<string, string> = {
  guide: 'сам справочник: открывает это окно, мигать нечем',
  whatsNew: 'окно «Что нового» — список изменений версии, а не функция редактора',
}
const uiOf = (spot?: string): string | null => (spot?.startsWith('ui:') ? spot.slice(3) : null)

describe('справочник функций: стражи', () => {
  it('каждая функция уровня описана', () => {
    const described = new Set(ARTICLES.map((a) => a.feature))
    const missing: string[] = []
    for (const f of FEATURES) for (const id of [f.id, ...(f.also ?? [])]) if (!described.has(id)) missing.push(id)
    expect(missing).toEqual([])
  })

  it('у каждой строки матрицы есть статьи для «?»', () => {
    const rows = new Set(ARTICLES.map((a) => rowOf(a.feature)))
    expect(FEATURES.map((f) => f.id).filter((id) => !rows.has(id))).toEqual([])
  })

  it('якоря есть в разметке', () => {
    const have = anchors()
    const need = new Set<string>(HINT_SPOTS.map((s) => s.slice(3)))
    for (const a of ARTICLES) {
      for (const s of [...placesOf(a).map((p) => p.spot), a.miss?.spot]) {
        const id = uiOf(s)
        if (id) need.add(id)
      }
    }
    expect([...need].filter((id) => !have.has(id)).sort()).toEqual([])
  })

  it('нет лишних якорей', () => {
    const used = new Set<string>(HINT_SPOTS.map((s) => s.slice(3)))
    for (const a of ARTICLES) {
      for (const s of [...placesOf(a).map((p) => p.spot), a.miss?.spot]) {
        const id = uiOf(s)
        if (id) used.add(id)
      }
    }
    expect([...anchors()].filter((id) => !used.has(id)).sort()).toEqual([])
    // Динамический якорь поиск по литералам не увидит (кроме data-guide-hold у «⋯»).
    const dynamic = Object.entries(sources)
      .filter(([, src]) => /(?<![\w-])data-guide=\{/.test(src))
      .map(([p]) => p)
    expect(dynamic).toEqual([])
  })

  it('кнопки, видимые только при наведении, раскрываются показом', () => {
    // Полоса раскладок прячет группы (visibility:hidden): якорь внутри неё виден только при data-spot на контейнере.
    const bar = Object.entries(sources).find(([p]) => p.endsWith('/panels/layouts/LayoutBar.svelte'))?.[1] ?? ''
    expect(bar).toMatch(/data-guide-reveal/)
    expect(bar).toMatch(/\.bar:global\(\[data-spot\]\) \.group/)
    for (const id of ['layout-all', 'copy-grid', 'grid-gallery']) expect(bar, id).toContain(`data-guide="${id}"`)
    // ⊞ есть и у «Авто», и у виньетки (две ветки разметки): якорь нужен в обеих.
    expect(bar.match(/data-guide="layout-all"/g)?.length).toBe(2)
  })

  it('пункты меню и клавиши существуют', () => {
    const top = new Set(menus().flatMap((m) => m.items.map((i) => i.id)))
    const badMenu: string[] = []
    for (const a of ARTICLES) {
      for (const p of placesOf(a)) if (p.spot.startsWith('menu:') && !top.has(p.spot.slice(5))) badMenu.push(`${a.id}: ${p.spot}`)
      for (const m of a.menu ?? []) if (!top.has(m)) badMenu.push(`${a.id}: menu ${m}`)
    }
    expect(badMenu).toEqual([])

    const badKeys: string[] = []
    for (const a of ARTICLES)
      for (const k of a.keys ?? []) if (!HOTKEYS.some((h) => !h.hidden && (h.action === k || h.show === k))) badKeys.push(`${a.id}: ${k}`)
    expect(badKeys).toEqual([])
  })

  it('у пункта меню есть статья', () => {
    const covered = new Set<string>()
    for (const a of ARTICLES) {
      for (const p of placesOf(a)) if (p.spot.startsWith('menu:')) covered.add(p.spot.slice(5))
      for (const m of a.menu ?? []) covered.add(m)
    }
    const items = menus().flatMap((m) => m.items.filter((i) => i.cmd).map((i) => i.id))
    expect(items.filter((id) => !covered.has(id) && !NO_ARTICLE[id])).toEqual([])
    // Запись в NO_ARTICLE без причины или про несуществующий / уже описанный пункт — мусор.
    for (const [id, why] of Object.entries(NO_ARTICLE)) {
      expect(why.trim().length, `NO_ARTICLE.${id}: нужна причина`).toBeGreaterThan(0)
      expect(items.includes(id) && !covered.has(id), `NO_ARTICLE.${id}: пункт описан или не существует`).toBe(true)
    }
  })

  it('статьи целы', () => {
    const ids = ARTICLES.map((a) => a.id)
    expect(new Set(ids).size).toBe(ids.length)
    const groups = new Set(GUIDE_GROUPS.map((g) => g.id))
    for (const a of ARTICLES) {
      expect(placesOf(a).length > 0 || a.how, `${a.id}: нужно место или how`).toBeTruthy()
      expect(a.title.length, `${a.id}: title`).toBeLessThanOrEqual(40)
      expect(a.what.length, `${a.id}: what`).toBeLessThanOrEqual(220)
      expect(groups.has(a.group), `${a.id}: раздел`).toBe(true)
    }
    const html = Object.entries(sources).filter(([p, src]) => p.includes('/shell/guide/') && src.includes('{@html'))
    expect(html.map(([p]) => p)).toEqual([])
  })

  it('статьи с несколькими местами: модель цела', () => {
    const multi = ARTICLES.filter((a) => a.spots)
    expect(multi.length).toBeGreaterThan(0)
    for (const a of multi) {
      const list = a.spots!
      expect(a.spot, `${a.id}: spot и spots вместе`).toBeUndefined()
      expect(list.length, `${a.id}: spots — от двух мест`).toBeGreaterThanOrEqual(2)
      expect(new Set(list.map((p) => p.spot)).size, `${a.id}: места повторяются`).toBe(list.length)
      expect(new Set(list.map((p) => p.label)).size, `${a.id}: подписи кнопок повторяются`).toBe(list.length)
      for (const p of list) {
        expect(p.label.startsWith('Показать'), `${a.id}: «${p.label}» — кнопка начинается с «Показать»`).toBe(true)
        expect(!p.level || !!p.place, `${a.id}: у места с level нужно place`).toBe(true)
      }
      // Первая — основная: она открывается без повышения уровня.
      expect(list[0].level, `${a.id}: первое место не должно быть с level`).toBeUndefined()
    }
  })

  it('у каждой статьи с меню-местом путь строится из buildMenu', () => {
    const bad: string[] = []
    for (const a of ARTICLES)
      for (const p of placesOf(a)) if (p.spot.startsWith('menu:') && !menuPath(p.spot.slice(5))) bad.push(`${a.id}: ${p.spot}`)
    expect(bad).toEqual([])
  })

  it('041: статья про разнообразие раскладок — ручка, команда в «Правка» и галерея; в «Автосборке» — строка про ручку', () => {
    const a = ARTICLES.find((x) => x.id === 'variety')
    expect(a).toMatchObject({ title: 'Разнообразие раскладок', feature: 'variants', menu: ['varyLayouts'] })
    for (const w of ['Автосборка', 'Дополнительно', 'Разнообразить раскладки', 'Похожие', 'Другие варианты'])
      expect(a?.where, w).toContain(w)
    expect(a?.words).toContain('звезды')
    expect(ARTICLES.find((x) => x.id === 'autobuild')?.where).toContain('Разнообразие раскладок')
  })

  it('051: «Автосборка» — «Авто | Свой от … до …» и серии, без прежнего «не больше»; «Разнообразие» — ленты, фигуры, формы', () => {
    const b = ARTICLES.find((x) => x.id === 'autobuild')
    expect(b?.what).toContain('Авто')
    for (const w of ['Фото на разворот: Авто | Свой от … до …', 'Не разрывать серии съёмки']) expect(b?.where, w).toContain(w)
    const v = ARTICLES.find((x) => x.id === 'variety')
    for (const w of ['Ленты', 'Фигуры', 'Лесенка']) expect(v?.where, w).toContain(w)
    expect(v?.words).toContain('лесенка')
  })

  it('у каждого раздела есть статьи; метки версии у статей нет', () => {
    for (const g of GUIDE_GROUPS)
      expect(
        ARTICLES.some((a) => a.group === g.id),
        g.id,
      ).toBe(true)
    expect(ARTICLES.filter((a) => 'since' in a)).toEqual([])
    expect(GUIDE_GROUPS.some((g) => /^Новое/.test(g.title))).toBe(false)
  })
})
