import { describe, expect, it } from 'vitest'
import { FEATURES, rowOf, type FeatureId } from '../../base/prefs/levels'
import { prefs } from '../../base/prefs/prefs.svelte'
import { catalogView } from '../../base/keys/keymap.fixture'
import { MOUSE_HELP } from '../menu/mousehelp'
import { ARTICLES, GUIDE_GROUPS, placesOf } from './articles'
import { articleText } from './articletext'
import { levelMarks, nestedLevels } from './leveltext'
import { HINT_SPOTS, menuPath, menus } from './show'
import { TOUR } from '../tour/steps'

// Стражи справочника (спека 033, design §9): справка не устаревает молча. Новая функция без статьи роняет
// первый, переименованный или удалённый элемент разметки, пункт меню или клавиша — остальные.
const sources = import.meta.glob('/src/**/*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

// Якоря в разметке: только литералом data-guide="…" (несколько id — через пробел) или литералом в спреде
// {...cond ? { 'data-guide': '…' } : {}} (якорь только у части экземпляров: текущий номер, плавающая карточка).
// Динамический data-guide={…} запрещён. Третья форма — запасной якорь «⋯» группы карточки: moreGuide="…" литералом
// (CardGroup кладёт его в data-guide-hold; кнопка ушла в «⋯», основного места в разметке нет — спека 124 U5, vig-save).
const ANCHOR = /(?<![\w-])(?:data-guide="([^"]*)"|'data-guide': '([^']*)'|moreGuide="([^"]*)")/g
const anchors = (): Set<string> => {
  const out = new Set<string>()
  for (const src of Object.values(sources))
    for (const m of src.matchAll(ANCHOR))
      (m[1] ?? m[2] ?? m[3])
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
  about: 'окно «О программе»: версия, автор, лицензия и ссылки, а не функция редактора',
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
    // Якорь только для кольца тура (122: layout-auto — вся группа раскладок) тоже нужен.
    for (const t of TOUR) for (const s of [t.spot, t.emptySpot]) if (s) used.add(s.slice(3))
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
    const known = new Set([...catalogView.commands.map((e) => e.id), ...MOUSE_HELP.map((r) => r.show)])
    for (const a of ARTICLES) for (const k of a.keys ?? []) if (!known.has(k)) badKeys.push(`${a.id}: ${k}`)
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

  it('041: статья про разнообразие раскладок — ручка, команда в «Разворот» и галерея; в «Автосборке» — строка про ручку', () => {
    const a = ARTICLES.find((x) => x.id === 'variety')
    expect(a).toMatchObject({ title: 'Разнообразие раскладок', feature: 'variants', menu: ['varyLayouts'] })
    for (const w of ['Автосборка', 'Раскладки', 'Разнообразить раскладки', 'Похожие', 'Другие варианты']) expect(a?.where, w).toContain(w)
    expect(a?.words).toContain('звезды')
    expect(ARTICLES.find((x) => x.id === 'autobuild')?.where).toContain('Разнообразие раскладок')
  })

  it('119: «Цветовой профиль фото» — группа «Фото и лента», значок в ленте, слова про Xiaomi, P3 и бледные цвета', () => {
    const a = ARTICLES.find((x) => x.id === 'color-profile')
    expect(a).toMatchObject({ title: 'Цветовой профиль фото', group: 'photos', feature: 'base', spot: 'ui:color-badge' })
    for (const w of ['xiaomi', 'p3', 'бледно', 'srgb']) expect(a?.words, w).toContain(w)
    expect(a?.how).toContain('Lightroom')
  })

  it('078: «Похожие кадры» — блок «Фото» окна автосборки, якорь строки и запасное место; «Автосборка» называет блок и вариант', () => {
    const a = ARTICLES.find((x) => x.id === 'similar')
    expect(a).toMatchObject({ title: 'Похожие кадры', feature: 'base', spot: 'ui:auto-similar', miss: { spot: 'ui:autobuild' } })
    for (const w of ['Убирать похожие кадры: Нет · Дубли · Очень похожие · Похожие', '«Вернуть все»', 'Вариант ‹ 1 ›'])
      expect(a?.where, w).toContain(w)
    for (const w of ['«беру» и 4–5★ не убираются никогда', 'Убранные остаются в ленте']) expect(a?.how, w).toContain(w)
    for (const w of ['дубли', 'вернуть', 'вариант']) expect(a?.words, w).toContain(w)
    expect(ARTICLES.find((x) => x.id === 'autobuild')?.where).toMatch(/блок «Фото»[^.]*«Убирать похожие кадры»/)
    expect(ARTICLES.find((x) => x.id === 'autobuild')?.words).toContain('вариант')
  })

  it('078: «Автосборка» рассказывает про «Пересобрать…»: тост, вариант, отмена одним шагом, что её закрывает', () => {
    const a = ARTICLES.find((x) => x.id === 'autobuild')
    for (const w of ['«Пересобрать…»', '«Вариант»', 'одним шагом', 'Звёзды и метки пересборку не закрывают', 'свободных фото нет'])
      expect(a?.how, w).toContain(w)
    expect(a?.how).toContain('{key:undo|')
    for (const w of ['пересобрать', 'пересборка']) expect(a?.words, w).toContain(w)
  })

  it('107: «Автосборка» рассказывает про «Весь альбом заново»: что остаётся на месте, вопрос, один шаг отмены, похожие в ленту', () => {
    const a = ARTICLES.find((x) => x.id === 'autobuild')
    for (const w of ['Весь альбом заново', 'остаются на своих местах', 'Пересобрать альбом?', 'одним шагом', 'уйдут в ленту', 'обложка'])
      expect(a?.how, w).toContain(w)
    expect(a?.words).toContain('весь альбом')
  })

  it('065: фильтр семейств — в «Автосборка ▸ Раскладки»; «Разнообразить раскладки» берёт его же; серии остались в «Дополнительно»', () => {
    const b = ARTICLES.find((x) => x.id === 'autobuild')
    for (const w of ['«Раскладки»', 'фильтр семейств', 'Варианты · Что соберётся', 'Разнообразие раскладок'])
      expect(b?.where, w).toContain(w)
    expect(b?.where).toMatch(/Не разрывать серии съёмки[^.]*«Дополнительно»/)
    expect(b?.words).toContain('семейства')
    const v = ARTICLES.find((x) => x.id === 'variety')
    expect(v?.where).toContain('фильтр семейств')
    expect(v?.where).toContain('берёт их же')
    expect(v?.where).not.toContain('«Дополнительно»') // ручка переехала из «Дополнительно» в «Раскладки»
  })

  it('051: «Автосборка» — «Авто | Свой от … до …» и серии, без прежнего «не больше»; «Разнообразие» — ленты, фигуры, формы', () => {
    const b = ARTICLES.find((x) => x.id === 'autobuild')
    expect(b?.what).toContain('Авто')
    for (const w of ['Фото на разворот: Авто | Свой от … до …', 'Не разрывать серии съёмки']) expect(b?.where, w).toContain(w)
    const v = ARTICLES.find((x) => x.id === 'variety')
    for (const w of ['Ленты', 'Фигуры', 'Лесенка']) expect(v?.where, w).toContain(w)
    expect(v?.words).toContain('лесенка')
  })

  it('067: «Готовые развороты» — на функции «Фоны и стикеры», вход в «Фон» карточки и в «Разворот»', () => {
    const a = ARTICLES.find((x) => x.id === 'spread-tpl')
    expect(a).toMatchObject({ title: 'Готовые развороты', feature: 'decor', menu: ['spreadTpl'] })
    for (const w of ['«Фон»', 'Разворот ▸ Готовые развороты…', '«Левую» и «Правую»']) expect(a?.where, w).toContain(w)
    expect(placesOf(a!).map((p) => p.spot)).toEqual(['ui:spread-tpl', 'menu:spreadTpl'])
  })

  it('077: «Фоны и стикеры» — фон сразу на много листов, слова поиска', () => {
    const a = ARTICLES.find((x) => x.id === 'decor')
    for (const w of [
      'строка «На»',
      'Выделенные',
      'Номера: 3, 5-8',
      'Все развороты',
      'один шаг отмены',
      '«QR-код по ссылке…»',
      '«Добавить свой стикер…»',
    ])
      expect(a?.where, w).toContain(w)
    for (const w of ['все', 'развороты', 'сразу', 'несколько', 'набор', 'страниц', 'логотип', 'qr']) expect(a?.words, w).toContain(w)
  })

  it('079: «Рамочки» и «Прозрачность фото» — свои функции уровня, якоря карточки, вход в «Разворот», слова поиска', () => {
    const b = ARTICLES.find((x) => x.id === 'border')
    expect(b).toMatchObject({ title: 'Рамочки', feature: 'border', menu: ['borderGallery'] })
    for (const w of ['«Рамочка» ▸ «Выбрать…»', 'Разворот ▸ Рамочки…', 'правая кнопка', 'все места разворота', '«Добавить свою рамочку…»'])
      expect(b?.where, w).toContain(w)
    expect(placesOf(b!).map((p) => p.spot)).toEqual(['ui:border', 'menu:borderGallery'])
    for (const w of ['рамочка', 'рамка', 'паспарту', 'полароид', 'обводка']) expect(b?.words, w).toContain(w)
    const f = ARTICLES.find((x) => x.id === 'photo-fade')
    expect(f).toMatchObject({ title: 'Прозрачность фото', feature: 'photoFade', spot: 'ui:photo-fade' })
    expect(f?.where).toContain('«Прозрачность»')
    expect(f?.what).toContain('«Авто» не закрепляется')
    // строка уровня у обеих — прежняя: нового выбора в «Своём» нет
    expect(rowOf('border')).toBe('decor')
    expect(rowOf('photoFade')).toBe('adjust')
  })

  it('067 Ф2: «Наборы библиотеки» — на функции «Фоны и стикеры», вход в «Файл» и в окна', () => {
    const a = ARTICLES.find((x) => x.id === 'library-packs')
    expect(a).toMatchObject({ title: 'Наборы библиотеки', feature: 'decor', menu: ['packs'] })
    for (const w of ['Файл ▸ Наборы библиотеки…', '«Наборы…»', '«Добавить из файла…»', '«Заменить?»']) expect(a?.where, w).toContain(w)
    // Ф3: сеть — согласие, ход, флажок и отзыв названы теми же словами, что в окне
    for (const w of ['«Обновить библиотеку»', '«Обратиться к GitHub?»', '«Отменить»', '«Проверять при запуске»', '«Не обращаться к сети»'])
      expect(a?.where, w).toContain(w)
    expect(a?.what).toContain('только с вашего согласия')
    expect(placesOf(a!).map((p) => p.spot)).toEqual(['menu:packs'])
  })

  it('086: «Папка данных и перенос» — вход в «Файл», что лежит в папке, перенос целиком и шаблоны в файл', () => {
    const a = ARTICLES.find((x) => x.id === 'data-folder')
    expect(a).toMatchObject({ title: 'Папка данных и перенос', feature: 'base' })
    for (const w of ['Файл ▸ Открыть папку данных', 'Мои шаблоны ▸ Сохранить в файл', 'Загрузить из файла'])
      expect(a?.where, w).toContain(w)
    for (const w of ['config.json', 'templates.json', 'log.txt', 'Резервные копии', 'миниатюры']) expect(a?.how, w).toContain(w)
    expect(placesOf(a!).map((p) => p.spot)).toEqual(['menu:showData'])
  })

  it('080-9: «Ч/б развороты» — отдельная статья на функции «adjust», якорь строки, запасное место — окно', () => {
    const a = ARTICLES.find((x) => x.id === 'auto-gray')
    expect(a).toMatchObject({
      title: 'Ч/б развороты',
      group: 'spreads',
      feature: 'adjust',
      spot: 'ui:auto-gray',
      miss: { spot: 'ui:autobuild' },
    })
    for (const w of ['Ч/б развороты: Нет · 10 % · 20 % · 30 %', '«Что соберётся»']) expect(a?.where, w).toContain(w)
    for (const w of ['ч/б', 'чёрно-белые', 'автосборка']) expect(a?.words, w).toContain(w)
  })

  it('080-9: на «Простом» «Похожие кадры» и «Автосборка» не говорят о ч/б; на «Стандартном» — говорят', () => {
    const texts = (id: string) => {
      const a = ARTICLES.find((x) => x.id === id)
      return [a?.what, a?.where, a?.how, a?.words].map((t) => articleText(t ?? '')).join(' ')
    }
    prefs.load({ level: 'simple', showStart: false, first: false, custom: null })
    for (const id of ['similar', 'autobuild']) expect(texts(id), id).not.toMatch(/ч\/б/i)
    expect(texts('autobuild')).not.toContain('{can:')
    prefs.load({ level: 'standard', showStart: false, first: false, custom: null })
    expect(texts('autobuild')).toContain('«Ч/б развороты» (статья «Ч/б развороты»)')
    expect(texts('similar')).not.toMatch(/ч\/б/i) // ч/б теперь только в своей статье
  })

  it('080-9: «Свой» без «Вариантов раскладки» — «Автосборка» не называет фильтр семейств', () => {
    const where = () => articleText(ARTICLES.find((x) => x.id === 'autobuild')?.where ?? '')
    prefs.load({ level: 'standard', showStart: false, first: false, custom: null })
    expect(where()).toContain('фильтр семейств')
    prefs.load({ level: 'custom', showStart: false, first: false, custom: { base: 'standard', on: [], off: ['variants'] } })
    expect(where()).not.toContain('семейств')
    expect(where()).toContain('Раскладки»: переключатель') // без двойных пробелов и «,»
    prefs.load({ level: 'simple', showStart: false, first: false, custom: null })
  })

  it('080-9: каждая метка {can:id} называет FeatureId, вложенной {can} нет', () => {
    const ids = new Set<string>(FEATURES.flatMap((f) => [f.id, ...(f.also ?? [])]))
    const bad: string[] = []
    for (const a of ARTICLES)
      for (const t of [a.what, a.where, a.how, a.miss?.hint, a.words]) {
        for (const id of levelMarks(t ?? '')) if (!ids.has(id as FeatureId)) bad.push(`${a.id}: ${id}`)
        for (const body of nestedLevels(t ?? '')) bad.push(`${a.id}: вложенная метка в «${body}»`)
      }
    expect(bad).toEqual([])
  })

  it('096: «Порядок разворотов» — статья в «Развороты» на всех уровнях со строкой мыши; «Все развороты» называет перетаскивание', () => {
    const a = ARTICLES.find((x) => x.id === 'spread-order')
    expect(a).toMatchObject({ title: 'Порядок разворотов', group: 'spreads', feature: 'base', spot: 'ui:timeline' })
    const row = MOUSE_HELP.find((r) => r.show === 'Перетащить разворот в ленте или в «Все развороты»')
    expect(row).toMatchObject({ label: 'Поменять порядок разворотов; Esc — отмена', group: 'spreads' })
    expect(row?.feature).toBeUndefined()
    expect(a?.keys).toContain(row?.show)
    expect(ARTICLES.find((x) => x.id === 'all-spreads')?.what).toContain('порядок — перетаскиванием')
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
