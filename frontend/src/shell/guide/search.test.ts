import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { prefs } from '../../base/prefs/prefs.svelte'
import { ARTICLES } from '../guidetext/articles'
import { orderRank } from '../guidetext/order'
import { fromLatin, grouped, norm, search, stem, tokens } from './search'

const titles = (q: string) => search(q).hits.map((h) => h.a.title)

describe('слова фотографа про промежуток между фото', () => {
  it('«расстояние между фото» и «белая линия» находят «Настройки альбома»', () => {
    for (const q of ['расстояние между фото', 'белая линия', 'вплотную', 'стык']) {
      expect(
        search(q).hits.map((h) => h.a.id),
        q,
      ).toContain('settings')
    }
  })
})

// Клавиши статей — из реестра (074): без него поиск по клавише пуст.
beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

describe('нормализация', () => {
  it('регистр и «ё» не важны', () => {
    expect(norm('ЁЖ Звёзды')).toBe('еж звезды')
    expect(tokens('Ctrl+Z, отмена')).toEqual(['ctrl+z', 'отмена'])
  })
  it('основа: окончания отбрасываются по длине слова', () => {
    expect(stem('виньетку')).toBe('винье')
    expect(stem('имена')).toBe('име')
    expect(stem('фото')).toBe('фот')
    expect(stem('са')).toBe('са')
  })
  it('основа сводит «белое/белые» и «обрезать/обрезка»', () => {
    expect(stem('белое')).toBe(stem('белые'))
    expect(stem('обрезать')).toBe(stem('обрезка'))
  })
  it('слова с цифрой или «+» — целиком', () => {
    for (const w of ['ctrl+z', 'f1', 'p3', 'ctrl+shift+z']) expect(stem(w), w).toBe(w)
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
  it('правки агента (154): «фон альбома» и «стикеры на обложке» находят «Правки агента»', () => {
    for (const q of ['фон альбома', 'стикеры на обложке'])
      expect(
        search(q).hits.map((h) => h.a.id),
        q,
      ).toContain('agents-edit')
  })
  it('подрезка (104):«подрезка», «белые полосы», «закрыть лист» находят статью', () => {
    for (const q of ['подрезка', 'белые полосы', 'закрыть лист', 'обрезка'])
      expect(
        search(q).hits.map((h) => h.a.id),
        q,
      ).toContain('crop')
  })
  it('цветовой профиль (119): «xiaomi», «p3», «бледно», «srgb», «профиль» находят «Цветовой профиль фото»', () => {
    for (const q of ['xiaomi', 'p3', 'бледно', 'srgb', 'профиль', 'iphone'])
      expect(
        search(q).hits.map((h) => h.a.id),
        q,
      ).toContain('color-profile')
  })
  it('цветовой профиль PNG (125): «png» и «скриншот» находят «Цветовой профиль фото»', () => {
    for (const q of ['png', 'скриншот'])
      expect(
        search(q).hits.map((h) => h.a.id),
        q,
      ).toContain('color-profile')
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
  it('сначала все слова: точный проход, без пометки «похожие»', () => {
    const r = search('виньетка шаблон')
    expect(r.loose).toBe(false)
    expect(r.hits.map((h) => h.a.title)).toContain('Шаблоны виньетки')
  })
  it('нет статьи со всеми словами — «похожие» по части слов, у кого совпало больше — выше', () => {
    const r = search('виньетка шаблон ъъъъъ')
    expect(r.loose).toBe(true)
    const has = (id: string, w: string) => search(w).hits.some((h) => h.a.id === id)
    const n = (id: string) => ['виньетка', 'шаблон'].filter((w) => has(id, w)).length
    const counts = r.hits.map((h) => n(h.a.id))
    expect(counts[0]).toBe(2)
    expect(counts).toEqual([...counts].sort((x, y) => y - x))
  })
  it('служебные слова не мешают; если кроме них ничего нет — ищутся как есть', () => {
    expect(search('как сделать виньетку').hits.map((h) => h.a.id)).toContain('vignette')
    expect(search('как').hits.length).toBeGreaterThan(0)
  })
  it('«ctrl+z» и «f1» ищутся целиком: «ctrl+zz» ничего не находит', () => {
    expect(search('ctrl+z').hits[0].a.id).toBe('undo')
    expect(search('ctrl+zz').hits).toEqual([])
  })
  it('запрос режется до 200 знаков и не бросает на знаках RegExp', () => {
    const head = 'я'.repeat(200)
    expect(search(`${head} виньетка`).hits).toEqual(search(head).hits)
    for (const q of ['(', '[a-', '.*', '\\', 'a{2,1}', '+', '$^']) expect(() => search(q), q).not.toThrow()
  })
  it('«свой шаблон виньетки» → статья «Свои шаблоны виньетки»', () => {
    expect(search('свой шаблон виньетки').hits.map((h) => h.a.id)).toContain('vig-own')
  })
  it('ничего не нашлось — пустой список и без смены раскладки', () => {
    const r = search('ъъъъъ')
    expect(r.hits).toEqual([])
    expect(r.from).toBeNull()
    expect(r.loose).toBe(false)
  })
  it('название весомее текста: статья с названием идёт раньше', () => {
    expect(titles('обложка')[0]).toBe('Обложка')
  })
  it('ничья при равном весе — в порядке окна (GUIDE_ORDER: раздел, затем место в разделе)', () => {
    let ties = 0
    for (const q of ['фото', 'разворот', 'макет', 'текст', 'цвет', 'виньетка', 'шаблон', 'размер']) {
      const hits = search(q).hits
      for (let i = 1; i < hits.length; i++) {
        if (hits[i - 1].score !== hits[i].score) continue
        ties++
        expect(orderRank(hits[i - 1].a.id), `${q}: ${hits[i - 1].a.id} перед ${hits[i].a.id}`).toBeLessThanOrEqual(orderRank(hits[i].a.id))
      }
    }
    expect(ties).toBeGreaterThan(20) // проверка не пустая: ничьих много
  })
  it('ничья в запасном проходе — тот же порядок окна: сначала число слов, потом вес, потом место в окне', () => {
    const hits = search('виньетка шаблон ъъъъъ').hits
    expect(search('виньетка шаблон ъъъъъ').loose).toBe(true)
    const found = (id: string) => ['виньетка', 'шаблон'].filter((w) => search(w).hits.some((h) => h.a.id === id)).length
    let ties = 0
    for (let i = 1; i < hits.length; i++) {
      const [x, y] = [hits[i - 1], hits[i]]
      if (x.score !== y.score || found(x.a.id) !== found(y.a.id)) continue
      ties++
      expect(orderRank(x.a.id), `${x.a.id} перед ${y.a.id}`).toBeLessThanOrEqual(orderRank(y.a.id))
    }
    expect(ties).toBeGreaterThan(0)
  })
  it('orderRank: разделы по GUIDE_GROUPS, внутри — по GUIDE_ORDER; неизвестная статья — в конец', () => {
    expect(orderRank('new-album')).toBe(0)
    expect(orderRank('open')).toBe(1)
    expect(orderRank('folders')).toBeGreaterThan(orderRank('updates'))
    expect(orderRank('нет-такой')).toBeGreaterThan(orderRank('mirror'))
  })
  it('пустой запрос — все статьи', () => {
    expect(search('  ').hits).toHaveLength(ARTICLES.length)
  })
})

describe('индекс: действия фото и подменю', () => {
  it('подпись действия из acts находит статью', () => {
    expect(search('Копировать имя файла').hits[0].a.id).toBe('photo-about')
    expect(search('Сделать обычной надписью').hits.map((h) => h.a.id)).toContain('looks-draft')
  })
  it('подпись подменю связанного пункта находит статью', () => {
    // «Правка ▸ Зеркало ▸ ↔ По горизонтали»: подпись пункта подменю статьи flip (menu: flip).
    expect(search('↔ По горизонтали').hits.map((h) => h.a.id)).toContain('flip')
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
