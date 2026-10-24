import { afterEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { can, minLevel, rowOf, type Level } from '../../base/prefs/levels'
import { features, fromBase } from '../../base/prefs/levelcustom'
import { articleById, placesOf } from './articles'
import {
  closedLine,
  closedPlaces,
  keyRows,
  levelNeed,
  menuOf,
  menuPath,
  openPlaces,
  placeOpen,
  planShow,
  requiredFeatures,
  type Plan,
  type SheetInfo,
  type ShowCtx,
} from './show'

const sheet = (o: Partial<SheetInfo> = {}): SheetInfo => ({ kind: '', grid: false, photos: 2, texts: 0, ...o })

// Полный уровень, один обычный разворот, ничего не выделено; тест меняет только нужное.
const ctx = (o: Partial<ShowCtx> = {}): ShowCtx => ({
  level: 'full',
  custom: null,
  can: (f) => can(f, 'full'),
  sheet: false,
  spreads: [sheet()],
  index: 0,
  cover: null,
  onCover: false,
  selPhoto: false,
  selText: false,
  selAny: false,
  libPhotos: 3,
  libAll: 3,
  libSel: false,
  cardBody: true,
  big: false,
  bgMode: false,
  galleryOpen: false,
  menusOpen: false,
  zoomed: false,
  ...o,
})
const at = (level: Level, o: Partial<ShowCtx> = {}) => ctx({ level, can: (f) => can(f, level), ...o })
const art = (id: string) => articleById(id)!
const plan = (id: string, o: Partial<ShowCtx> = {}): Plan => planShow(art(id), ctx(o))
const kinds = (p: Plan) => p.steps.map((s) => s.do)

afterEach(() => dropCatalog())

describe('уровень', () => {
  it('функция есть — шагов уровня нет', () => {
    expect(levelNeed(art('frame'), ctx()).steps).toEqual([])
  })
  it('скрыта на «Простом» → шаг уровня «Стандартный» и тост', () => {
    const p = planShow(art('frame'), at('simple'))
    expect(p.steps[0]).toEqual({ do: 'level', to: 'standard' })
    expect(p.toast).toBe('Уровень «Стандартный» — для «Кадр»')
  })
  it('берётся наибольший нужный уровень: «Ручная» требует «Полный»', () => {
    const p = planShow(art('rotate'), at('standard'))
    expect(p.steps[0]).toEqual({ do: 'level', to: 'full' })
  })
  it('«Свой»: функция добавляется в набор вместе с опорами (toggle строки)', () => {
    const c = fromBase('simple')
    const set = features('custom', c)
    const p = planShow(art('frame'), ctx({ level: 'custom', custom: c, can: (f) => set.has(f) }))
    expect(p.steps[0]).toEqual({ do: 'custom', row: rowOf('frame') })
    expect(p.toast).toMatch(/^Включено в «Свой»: .*Кадр/)
  })
  it('rowOf: also-id указывает на главную строку', () => {
    expect(rowOf('bgColor')).toBe('captions')
    expect(rowOf('frame')).toBe('frame')
    expect(minLevel('frame')).toBe('standard')
  })
})

describe('нужные состояния: проект не меняется', () => {
  it('нет обложки → подсказка и cover-tile', () => {
    const p = plan('cover-settings')
    expect(p.spot).toBe('ui:cover-tile')
    expect(p.hint).toMatch(/Обложки ещё нет/)
  })
  it('проект-лист и нужна обложка → только текст, подсвечивать нечем', () => {
    const p = plan('cover-settings', { sheet: true })
    expect(p.spot).toBeNull()
    expect(p.toast).toBe('В отдельном листе обложки нет')
  })
  it('обложка есть, открыт разворот → переход на обложку', () => {
    const p = plan('cover-settings', { cover: sheet() })
    expect(kinds(p)).toContain('cover')
    expect(p.spot).toBe('ui:cover-print')
  })
  it('виньетка на другом листе → переход на неё', () => {
    const p = plan('vig-settings', { spreads: [sheet(), sheet({ kind: 'vignette' })] })
    expect(p.steps).toContainEqual({ do: 'go', index: 1 })
    expect(p.spot).toBe('ui:vig-setup')
  })
  it('виньетки нет → мигает add-vignette', () => {
    const p = plan('vig-settings')
    expect(p.spot).toBe('ui:add-vignette')
    expect(p.hint).toMatch(/Виньетки ещё нет/)
  })
  it('нужна «Ручная» на «Авто»-развороте → подсказка и mode-toggle, переключения нет', () => {
    const p = plan('rotate')
    expect(p.spot).toBe('ui:mode-toggle')
    expect(p.hint).toMatch(/Ручную/)
    expect(kinds(p)).not.toContain('selectPhoto')
    // Никаких шагов, меняющих проект: только вид.
    expect(kinds(p).every((k) => ['details', 'clearSel', 'unbig', 'bgExit', 'closeGallery', 'closeMenus'].includes(k))).toBe(true)
  })
  it('«Ручная» уже стоит, фото есть → выделяется первое', () => {
    const p = plan('rotate', { spreads: [sheet({ grid: true })] })
    expect(kinds(p)).toContain('selectPhoto')
    expect(p.spot).toBe('ui:rotate-handle')
  })
  it('нужно фото, на листе их нет → library и подсказка', () => {
    const p = plan('frame', { spreads: [sheet({ photos: 0 })] })
    expect(p.spot).toBe('ui:library')
    expect(p.hint).toMatch(/Поставьте фото/)
  })
  it('нужно фото, фото есть → выделить первое', () => {
    const p = plan('frame')
    expect(kinds(p)).toContain('selectPhoto')
    expect(p.spot).toBe('ui:frame')
  })
  it('фото уже выделено → выделять не надо', () => {
    expect(kinds(plan('frame', { selPhoto: true, selAny: true }))).not.toContain('selectPhoto')
  })
  it('нужен текст: есть → выделить, нет → мигает лента «Текст»', () => {
    expect(kinds(plan('text-rotate', { spreads: [sheet({ texts: 1 })] }))).toContain('selectText')
    expect(plan('text-rotate').spot).toBe('ui:rail')
  })
  it('нет разворотов → автосборка', () => {
    const p = plan('frame', { spreads: [], index: -1 })
    expect(p.spot).toBe('ui:autobuild')
  })
})

describe('кнопки полосы раскладок (скрыты до наведения)', () => {
  it('«Все раскладки» на «Авто»-развороте: шагов нет, мигает layout-all, выделение снимается', () => {
    const p = plan('layout-gallery', { selAny: true })
    expect(p.spot).toBe('ui:layout-all')
    expect(kinds(p)).toContain('clearSel')
    expect(p.hint).toBeUndefined()
  })
  it('«Все раскладки»: текущий — «Ручной», есть «Авто» — переход на него', () => {
    const p = plan('layout-gallery', { spreads: [sheet({ grid: true }), sheet()] })
    expect(p.steps).toContainEqual({ do: 'go', index: 1 })
    expect(p.spot).toBe('ui:layout-all')
  })
  it('«Все раскладки»: все развороты «Ручные» или виньетки — показываем переключатель', () => {
    const p = plan('layout-gallery', { spreads: [sheet({ grid: true }), sheet({ kind: 'vignette' })] })
    expect(p.spot).toBe('ui:mode-toggle')
    expect(p.hint).toMatch(/«Авто»/)
  })
  it('«Все раскладки» с обложки — на первый «Авто»-разворот', () => {
    const p = plan('layout-gallery', { onCover: true, cover: sheet() })
    expect(p.steps).toContainEqual({ do: 'go', index: 0 })
  })
  it('«Копировать раскладку» и «Шаблоны»: нужна «Ручная», выделение снимается', () => {
    for (const id of ['copy-grid', 'templates']) {
      const p = plan(id, { spreads: [sheet({ grid: true })], selAny: true })
      expect(kinds(p), id).toContain('clearSel')
      expect(p.spot, id).toBe(art(id).spot)
    }
    expect(plan('copy-grid').spot).toBe('ui:mode-toggle')
  })
})

describe('отметки в ленте и виньетка', () => {
  it('«Звёзды и метки»: фото ленты не выделено — выделяем первое, мигает полоса отметок', () => {
    const p = plan('marks')
    expect(kinds(p)).toContain('selectLibrary')
    expect(p.spot).toBe('ui:mark-bar')
  })
  it('«Звёзды и метки»: фото уже выделено — шаг не нужен', () => {
    expect(kinds(plan('marks', { libSel: true }))).not.toContain('selectLibrary')
  })
  it('«Звёзды и метки»: лента пуста — мигает лента с подсказкой', () => {
    const p = plan('marks', { libPhotos: 0, libAll: 0 })
    expect(p.spot).toBe('ui:library')
    expect(p.hint).toMatch(/добавьте папку/)
  })
  it('«Звёзды и метки»: лента пуста из-за поиска или фильтра — подсказка про сброс, а не «добавьте папку»', () => {
    const p = plan('marks', { libPhotos: 0, libAll: 255 })
    expect(p.spot).toBe('ui:library')
    expect(p.hint).toBe('Лента пуста из-за поиска или фильтра: сбросьте их')
    expect(kinds(p)).not.toContain('selectLibrary')
  })
  it('«Виньетка в альбоме»: виньетка есть — переход и мигает она; нет — место создания', () => {
    const p = plan('vignette', { spreads: [sheet(), sheet({ kind: 'vignette' })] })
    expect(p.steps).toContainEqual({ do: 'go', index: 1 })
    expect(p.spot).toBe('ui:vig-mark')
    expect(plan('vignette').spot).toBe('ui:add-vignette')
  })
})

describe('лента действий', () => {
  it('«Место» на «Стандартном»: лента есть только на «Полном» → шаг уровня', () => {
    const p = planShow(art('place-add'), at('standard', { spreads: [sheet({ grid: true })] }))
    expect(p.steps[0]).toEqual({ do: 'level', to: 'full' })
    expect(p.spot).toBe('ui:rail-place')
  })
  it('«Отразить раскладку» тоже требует плавающих панелей', () => {
    expect(requiredFeatures(art('mirror'))).toContain('panels')
  })
})

describe('карточка и панели', () => {
  it('карточка на «Простом» (панелей нет) → раскрывается карточка в панели деталей', () => {
    const p = planShow(art('person'), at('full', { can: (f) => f !== 'panels' }))
    expect(p.steps).toContainEqual({ do: 'details', section: 'card' })
  })
  it('на «Полном» карточка у холста — панель не открывается', () => {
    expect(plan('person', { selPhoto: true, selAny: true }).steps).not.toContainEqual({ do: 'details', section: 'card' })
  })
  it('карточка листа: выделение снимается', () => {
    expect(kinds(plan('bg-color', { selAny: true }))).toContain('clearSel')
  })
  it('раздел панели деталей', () => {
    expect(plan('settings').steps).toContainEqual({ do: 'details', section: 'settings' })
  })
  it('фото через корешок (064): «Показать» раскрывает «Настройки альбома» и подсвечивает переключатель', () => {
    const p = plan('fold')
    expect(p.steps).toContainEqual({ do: 'details', section: 'settings' })
    expect(placesOf(articleById('fold')!).map((x) => x.spot)).toEqual(['ui:fold-mode'])
  })
  it('точные px (123): у «Файла разворота» и «Файла обложки» свои якоря, не вся форма', () => {
    expect(placesOf(articleById('print-px')!).map((x) => x.spot)).toEqual(['menu:export', 'ui:page-px', 'ui:cover-px'])
    const [, page] = placesOf(art('print-px'))
    expect(planShow(art('print-px'), ctx(), page).steps).toContainEqual({ do: 'details', section: 'settings' })
    expect(planShow(art('print-px'), ctx(), page).spot).toBe('ui:page-px')
  })
  it('подрезка (104): «Показать» раскрывает «Настройки альбома» и подсвечивает блок', () => {
    const p = plan('crop')
    expect(p.steps).toContainEqual({ do: 'details', section: 'settings' })
    expect(placesOf(articleById('crop')!).map((x) => x.spot)).toEqual(['ui:crop-mode'])
  })
})

describe('сброс вида и меню', () => {
  it('«Во весь экран», режим фона, галерея и меню фото закрываются', () => {
    const p = plan('frame', { big: true, bgMode: true, galleryOpen: true, menusOpen: true })
    expect(kinds(p).slice(0, 4)).toEqual(['unbig', 'bgExit', 'closeGallery', 'closeMenus'])
  })
  it('пункт меню → раскрывается нужное меню', () => {
    const p = plan('export')
    expect(p.spot).toBe('menu:export')
    expect(p.steps).toContainEqual({ do: 'menu', menu: 'file' })
  })
  it('зум на холсте вписывается, когда нужен вид целиком', () => {
    expect(kinds(plan('big'))).not.toContain('fit')
  })
})

describe('вспомогательное', () => {
  it('menuOf и menuPath строятся из buildMenu', () => {
    expect(menuOf('export')).toBe('file')
    expect(menuOf('нет такого')).toBeNull()
    expect(menuPath('export')).toMatch(/^Файл ▸ Экспорт/)
    expect(menuPath('guide')).toBe('Справка ▸ Справочник функций…')
  })
  it('пункты «Разворота» ведут в «Разворот», а не в «Правку» (124 U1)', () => {
    expect(menuOf('bgGallery')).toBe('spread')
    expect(menuOf('makeVignette')).toBe('spread')
    expect(menuOf('undo')).toBe('edit')
    expect(menuOf('flip')).toBe('edit')
    expect(menuPath('borderGallery')).toBe('Разворот ▸ Рамочки…')
    expect(menuPath('removeCell')).toBe('Правка ▸ Удалить')
  })
  it('keyRows: по id команды (текущие клавиши) и по справочной строке мыши', () => {
    useCatalog()
    const rows = keyRows(art('undo'))
    expect(rows.map((r) => r.show)).toEqual(['Ctrl+Z', 'Ctrl+Y / Ctrl+Shift+Z'])
    expect(rows.map((r) => r.label)).toEqual(['Отменить', 'Повторить'])
    expect(keyRows(art('hotkeys')).length).toBeGreaterThan(0)
    // справочная строка мыши и жеста — по её show
    expect(keyRows(art('zoom')).map((r) => r.show)).toContain('Ctrl+колесо, щипок тачпада')
  })
  it('keyRows: своя клавиша видна сразу; команда без клавиши строки не даёт; без реестра пусто', () => {
    expect(keyRows(art('undo'))).toEqual([])
    useCatalog({ undo: ['KeyU'], redo: [] })
    expect(keyRows(art('undo')).map((r) => r.show)).toEqual(['U'])
  })
})

describe('несколько мест у статьи (решение 133)', () => {
  const labels = (id: string, level: Level) => openPlaces(art(id), at(level)).map((p) => p.label)

  it('одно spot — одна кнопка «Показать», как раньше', () => {
    expect(placesOf(art('frame')).map((p) => p.label)).toEqual(['Показать'])
    expect(labels('frame', 'full')).toEqual(['Показать'])
    expect(placesOf(art('marks'))[0].spot).toBe('ui:mark-bar')
  })
  it('«Свободный текст» на «Полном»: меню и лента действий, первая — меню', () => {
    expect(labels('text', 'full')).toEqual(['Показать в меню', 'Показать в ленте действий'])
    expect(closedPlaces(art('text'), at('full'))).toEqual([])
  })
  it('на «Стандартном» ленты нет: одна кнопка и серая строка про «Полном»', () => {
    expect(labels('text', 'standard')).toEqual(['Показать в меню'])
    expect(closedPlaces(art('text'), at('standard')).map((p) => closedLine(p, at('standard')))).toEqual([
      'на «Полном»: лента действий слева',
    ])
  })
  it('на «Своём» без панелей серая строка называет функцию, а не уровень', () => {
    const c = ctx({ level: 'custom', can: (f) => f !== 'panels' })
    const later = closedPlaces(art('text'), c)
    expect(later.map((p) => closedLine(p, c))).toEqual(['с «Плавающими панелями»: лента действий слева'])
  })
  it('«Карточка у холста»: место несёт подсказку на случай открытой панели деталей', () => {
    const card = placesOf(art('panels')).find((p) => p.spot === 'ui:inspector')!
    expect(card.miss).toMatch(/панель деталей/)
    expect(planShow(art('panels'), ctx(), card).miss).toBe(card.miss)
  })
  it('«Сделать виньеткой»: текущий — виньетка, есть обычный — переход на него', () => {
    const vm = placesOf(art('vignette')).find((p) => p.spot === 'ui:vig-make')!
    const p = planShow(art('vignette'), ctx({ spreads: [sheet({ kind: 'vignette' }), sheet({ grid: true })], index: 0 }), vm)
    expect(p.steps).toContainEqual({ do: 'go', index: 1 })
    expect(p.spot).toBe('ui:vig-make')
  })
  it('«Сделать виньеткой»: обычный (и ручной) разворот — переход не нужен; с обложки — на первый обычный', () => {
    const vm = placesOf(art('vignette')).find((p) => p.spot === 'ui:vig-make')!
    expect(kinds(planShow(art('vignette'), ctx({ spreads: [sheet({ grid: true })] }), vm))).not.toContain('go')
    const c = ctx({ spreads: [sheet({ kind: 'vignette' }), sheet()], onCover: true, cover: sheet() })
    expect(planShow(art('vignette'), c, vm).steps).toContainEqual({ do: 'go', index: 1 })
  })
  it('«Сделать виньеткой»: все развороты — виньетки: мигает виньетка в ленте с подсказкой', () => {
    const vm = placesOf(art('vignette')).find((p) => p.spot === 'ui:vig-make')!
    const p = planShow(art('vignette'), ctx({ spreads: [sheet({ kind: 'vignette' })] }), vm)
    expect(p.spot).toBe('ui:vig-mark')
    expect(p.hint).toMatch(/виньетки/)
  })
  it('«Свой»: место с level открывается, когда включены плавающие панели', () => {
    const rail = placesOf(art('text')).find((p) => p.spot === 'ui:rail-text')!
    const set = (on: boolean) => ({ level: 'custom' as const, can: (f: string) => on || f !== 'panels' })
    expect(placeOpen(rail, set(true) as never)).toBe(true)
    expect(placeOpen(rail, set(false) as never)).toBe(false)
  })
  it('план идёт к выбранному месту; без выбора — к первому доступному', () => {
    const a = art('text')
    const [menu, rail] = placesOf(a)
    expect(planShow(a, ctx(), menu).spot).toBe('menu:addText')
    expect(planShow(a, ctx(), menu).steps).toContainEqual({ do: 'menu', menu: 'spread' })
    const p = planShow(a, ctx(), rail)
    expect(p.spot).toBe('ui:rail-text')
    expect(kinds(p)).not.toContain('menu')
    expect(planShow(a, ctx()).spot).toBe('menu:addText')
  })
  it('needs берётся у места, иначе у статьи', () => {
    // «Зеркало»: карточка нужна только месту в карточке, меню обходится выделенным фото.
    const a = art('flip')
    const [card, menu] = placesOf(a)
    const c = at('full', { can: (f) => f !== 'panels' })
    expect(planShow(a, c, card).steps).toContainEqual({ do: 'details', section: 'card' })
    expect(planShow(a, c, menu).steps).not.toContainEqual({ do: 'details', section: 'card' })
    // Место без своего needs берёт needs статьи: «Текст» требует разворот.
    expect(planShow(art('text'), ctx({ spreads: [], index: -1 })).spot).toBe('ui:autobuild')
  })
  it('место с level само уровень не повышает; одиночное spot ленты — как раньше', () => {
    const rail = placesOf(art('text')).find((p) => p.spot === 'ui:rail-text')!
    expect(requiredFeatures(art('text'), rail)).not.toContain('panels')
    expect(requiredFeatures(art('place-add'))).toContain('panels')
  })
})
