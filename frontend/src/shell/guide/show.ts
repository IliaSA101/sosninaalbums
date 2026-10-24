// План показа функции (спека 033, design §4). Чистая логика: по статье и состоянию окна считает, что
// переключить (уровень, лист, выделение, панели) и что подсветить. Проект не меняется никогда:
// шаги правят только вид. Исполняет план guide.svelte.ts.
import { buildMenu, type Menu, type MenuCtx, type MenuId } from '../menu/menu'
import { keymap } from '../../base/keys/keymap.svelte'
import { chordsText } from '../../base/keys/chordtext'
import { isMac } from '../../base/core/platform'
import { MOUSE_HELP } from '../menu/mousehelp'
import { features, fromBase, shortOf, toggle } from '../../base/prefs/levelcustom'
import { LEVEL_TITLE, RANK, minLevel, rowOf, type Base, type Custom, type FeatureId, type Level, type RowId } from '../../base/prefs/levels'
import type { SectionName } from '../../base/prefs/view.svelte'
import { placesOf, type Article, type Need, type Place } from './articles'

// Что известно о листе (разворот или обложка): хватает, чтобы решить про вид, не обращаясь к View.
export interface SheetInfo {
  kind: '' | 'vignette'
  grid: boolean
  photos: number
  texts: number
}

export interface ShowCtx {
  level: Level
  custom: Custom | null
  can: (f: FeatureId) => boolean
  sheet: boolean // проект-лист (031): обложки нет
  spreads: SheetInfo[] // развороты в порядке альбома
  index: number // номер текущего разворота; -1 — разворотов нет
  cover: SheetInfo | null // лист обложки; null — обложки нет
  onCover: boolean
  selPhoto: boolean
  selText: boolean
  selAny: boolean // выделено что угодно на текущем листе (места, фото, тексты)
  libPhotos: number // фото в ленте с учётом фильтра и поиска
  libAll: number // фото в библиотеке вообще: libPhotos = 0 при libAll > 0 — их скрыл поиск или фильтр
  libSel: boolean // в ленте выделено фото
  cardBody: boolean // у текущего листа есть карточка (разворот есть, галерея закрыта)
  big: boolean
  bgMode: boolean
  galleryOpen: boolean
  menusOpen: boolean // меню фото, просмотр фото или меню слоя
  zoomed: boolean
}

export type Step =
  | { do: 'level'; to: Base }
  | { do: 'custom'; row: RowId }
  | { do: 'unbig' }
  | { do: 'bgExit' }
  | { do: 'closeGallery' }
  | { do: 'closeMenus' }
  | { do: 'cover' }
  | { do: 'go'; index: number }
  | { do: 'selectPhoto' }
  | { do: 'selectText' }
  | { do: 'selectLibrary' }
  | { do: 'clearSel' }
  | { do: 'details'; section: SectionName }
  | { do: 'gallery' }
  | { do: 'fit' }
  | { do: 'menu'; menu: MenuId }

export interface Plan {
  steps: Step[]
  spot: string | null // 'ui:<id>' или 'menu:<id>'; null — мигать нечем
  hint?: string // подпись под кольцом, когда мигает не сама функция
  note?: string // строка в карточке: чего сейчас не хватает
  toast?: string // уровень изменён («Вернуть» добавляет исполнитель) или мигать нечем
  miss?: string // подсказка тоста места, если якоря на экране не нашлось (Place.miss)
}

// Места, куда план ведёт, когда нужного состояния нет (design §4): их якоря должны быть в разметке.
export const HINT_SPOTS = ['ui:autobuild', 'ui:cover-tile', 'ui:add-vignette', 'ui:library', 'ui:rail', 'ui:mode-toggle'] as const

const FIRST = (list: SheetInfo[], pick: (s: SheetInfo) => boolean): number => list.findIndex(pick)

// Кнопки ленты действий («Место», «Разделить», «Отразить») есть только при плавающих панелях.
const RAIL_SPOTS: string[] = ['ui:rail', 'ui:rail-place', 'ui:rail-split', 'ui:mirror']

// Строки уровня, нужные статье: её собственная и те, что требуют состояния (виньетка, ручная, текст).
// С place — по этому месту: его needs и его якорь (у статьи с spots плавающие панели требует только место с level).
export function requiredFeatures(a: Article, place?: Place): FeatureId[] {
  const out: FeatureId[] = [a.feature]
  const needs = place?.needs ?? a.needs ?? []
  const spot = place ? place.spot : a.spot
  if (needs.includes('vignette')) out.push('vignette')
  if (needs.includes('grid')) out.push('manual')
  if (needs.includes('text')) out.push('text')
  if (spot && RAIL_SPOTS.includes(spot)) out.push('panels')
  return [...new Set(out)]
}

export interface LevelNeed {
  steps: Step[]
  can: (f: FeatureId) => boolean // что будет доступно после шагов
  toast?: string
  to?: Base // готовый уровень, на который перейдём
  added: RowId[] // строки, которые добавятся в «Свой»
}

// Что нужно включить, чтобы функция стала доступна. Пусто — она уже доступна.
export function levelNeed(a: Article, ctx: Pick<ShowCtx, 'level' | 'custom' | 'can'>, place?: Place): LevelNeed {
  const lack = requiredFeatures(a, place).filter((f) => !ctx.can(f))
  if (lack.length === 0) return { steps: [], can: ctx.can, added: [] }
  const names = [...new Set(lack.map((f) => shortOf(rowOf(f))))].join(', ')
  if (ctx.level === 'custom') {
    let c = ctx.custom ?? fromBase('simple')
    const added: RowId[] = []
    const steps: Step[] = []
    for (const row of new Set(lack.map(rowOf))) {
      const t = toggle(c, row, true)
      c = t.custom
      added.push(...t.added)
      steps.push({ do: 'custom', row })
    }
    const set = features('custom', c)
    return { steps, can: (f) => set.has(f), toast: `Включено в «Свой»: ${[...new Set(added.map(shortOf))].join(', ')}`, added }
  }
  const to = lack.map(minLevel).reduce<Base>((m, b) => (RANK[b] > RANK[m] ? b : m), 'simple')
  const rank = RANK[ctx.level]
  return {
    steps: [{ do: 'level', to }],
    can: (f) => RANK[minLevel(f)] <= Math.max(RANK[to], rank),
    toast: `Уровень «${LEVEL_TITLE[to]}» — для «${names}»`,
    to,
    added: [],
  }
}

// У «Своего» нет ранга: есть ли места уровня, решает характерная функция уровня (плавающие панели — «Полный»).
const LEVEL_GATE: Partial<Record<Base, FeatureId>> = { full: 'panels' }

// Место есть на текущем уровне: без level — всегда; иначе ранг готового уровня не ниже, у «Своего» — по функции.
export function placeOpen(p: Place, ctx: Pick<ShowCtx, 'level' | 'can'>): boolean {
  if (!p.level) return true
  if (ctx.level === 'custom') {
    const gate = LEVEL_GATE[p.level]
    return !gate || ctx.can(gate)
  }
  return RANK[ctx.level] >= RANK[p.level]
}

// Места статьи, доступные сейчас (кнопки), и те, что выше текущего уровня (серые строки).
export const openPlaces = (a: Article, ctx: Pick<ShowCtx, 'level' | 'can'>): Place[] => placesOf(a).filter((p) => placeOpen(p, ctx))
export const closedPlaces = (a: Article, ctx: Pick<ShowCtx, 'level' | 'can'>): Place[] => placesOf(a).filter((p) => !placeOpen(p, ctx))

// Название уровня в предложном падеже: «на «Полном»».
const LEVEL_ON: Record<Base, string> = { simple: 'Простом', standard: 'Стандартном', full: 'Полном' }
// То же для «Своего», где вместо уровня — функция из LEVEL_GATE, в творительном падеже: «с «Плавающими панелями»».
const GATE_WITH: Partial<Record<Base, string>> = { full: 'Плавающими панелями' }

// Серая строка про место, которого на этом уровне нет: «на «Полном»: лента действий слева». На «Своём» уровней
// нет — условие называем функцией: «с «Плавающими панелями»: …».
export function closedLine(p: Place, ctx: Pick<ShowCtx, 'level'>): string {
  const what = p.place ?? p.label
  if (ctx.level === 'custom' && LEVEL_GATE[p.level ?? 'full']) return `с «${GATE_WITH[p.level ?? 'full']}»: ${what}`
  return `на «${LEVEL_ON[p.level ?? 'full']}»: ${what}`
}

// Верхнее меню целиком, всё доступно: подписи и пути строятся из него, а не пишутся руками.
const FULL: MenuCtx = {
  level: 'full',
  spreads: 1,
  hasSpread: true,
  canUndo: true,
  canRedo: true,
  zone: 'canvas',
  libraryPhotos: 1,
  anySelection: true,
  deletable: true,
  flippable: true,
  varyable: true,
  layerable: true,
  missing: 1,
  big: false,
  details: false,
  safe: false,
  rulers: false,
  snap: false,
  alignCount: 2,
}
let fullMenu: Menu[] | null = null
export const menus = (): Menu[] => (fullMenu ??= buildMenu(FULL))

// Меню, в котором верхний пункт itemId; null — такого пункта нет.
export const menuOf = (itemId: string): MenuId | null => menus().find((m) => m.items.some((i) => i.id === itemId))?.id ?? null

// «Файл ▸ Экспорт…» для верхнего пункта.
export function menuPath(itemId: string): string | null {
  const m = menus().find((x) => x.items.some((i) => i.id === itemId))
  const it = m?.items.find((i) => i.id === itemId)
  return m && it ? `${m.label} ▸ ${it.label}` : null
}

export interface KeyRow {
  show: string
  label: string
}

// Клавиши статьи: id команды из реестра (текущие сочетания, в том числе свои) или show справочной строки (мышь, жесты).
// Команда без клавиши строки не даёт.
export function keyRows(a: Article): KeyRow[] {
  const out: KeyRow[] = []
  for (const k of a.keys ?? []) {
    const e = keymap.entry(k)
    if (e) {
      if (e.keys.length > 0 && !out.some((r) => r.label === e.label)) out.push({ show: chordsText(e.keys, isMac()), label: e.label })
      continue
    }
    const m = MOUSE_HELP.find((r) => r.show === k)
    if (m && !out.some((r) => r.show === m.show)) out.push({ show: m.show, label: m.label })
  }
  return out
}

// Якорь статьи → селектор в разметке: ui:x — data-guide, menu:x — пункт выпадающего меню.
export const spotId = (spot: string): string => spot.slice(spot.indexOf(':') + 1)

// Ход плана: где мы окажемся после уже набранных шагов (лист и выделение), чтобы следующие условия
// проверялись по новому состоянию, а не по исходному.
interface Walk {
  ctx: ShowCtx
  can: (f: FeatureId) => boolean
  steps: Step[]
  toast?: string
  onCover: boolean
  index: number
  selPhoto: boolean
  selText: boolean
  selAny: boolean
}

const curOf = (w: Walk): SheetInfo | null => (w.onCover ? w.ctx.cover : (w.ctx.spreads[w.index] ?? null))

// Условие не выполнить: подсвечиваем место, где его создают.
const stop = (w: Walk, to: string, hint: string, note: string): Plan => ({ steps: w.steps, spot: to, hint, note, toast: w.toast })

// Переход на другой лист: выделение остаётся на прежнем.
function jump(w: Walk, s: Step, onCover: boolean, index: number): void {
  w.steps.push(s)
  w.onCover = onCover
  w.index = index
  w.selPhoto = w.selText = w.selAny = false
}

// Сброс вида: то, что мешает увидеть место.
function resetView(w: Walk, needs: Set<Need>): void {
  const { ctx, steps } = w
  if (ctx.big) steps.push({ do: 'unbig' })
  if (ctx.bgMode) steps.push({ do: 'bgExit' })
  if (ctx.galleryOpen && !needs.has('gallery')) steps.push({ do: 'closeGallery' })
  if (ctx.menusOpen) steps.push({ do: 'closeMenus' })
}

// Карточка выделенного видна у холста (панели) или в панели деталей — её раскрываем, если нужно.
function cardSteps(w: Walk): void {
  if (w.selAny) w.steps.push({ do: 'clearSel' })
  if (!(w.can('panels') && w.ctx.cardBody)) w.steps.push({ do: 'details', section: 'card' })
}

function spreadNeed(w: Walk, needs: Set<Need>): Plan | null {
  if (!needs.has('spread') || !(w.onCover || !curOf(w))) return null
  if (w.ctx.spreads.length === 0)
    return stop(
      w,
      'ui:autobuild',
      'Сначала соберите альбом: «Автосборка» или перетащите фото на разворот',
      'Сейчас в альбоме нет разворотов — покажу, как их получить',
    )
  jump(w, { do: 'go', index: 0 }, false, 0)
  return null
}

function coverNeed(w: Walk, needs: Set<Need>): Plan | null {
  if (!needs.has('cover') || w.onCover) return null
  if (w.ctx.cover) {
    jump(w, { do: 'cover' }, true, w.index)
    return null
  }
  if (w.ctx.sheet) return { steps: [], spot: null, note: 'В отдельном листе обложки нет', toast: 'В отдельном листе обложки нет' }
  return stop(w, 'ui:cover-tile', 'Обложки ещё нет: «+ Обложка» или перетащите фото сюда', 'Сейчас обложки нет — покажу, где её создать')
}

function vignetteNeed(w: Walk, needs: Set<Need>): Plan | null {
  if (!needs.has('vignette') || curOf(w)?.kind === 'vignette') return null
  const at = FIRST(w.ctx.spreads, (s) => s.kind === 'vignette')
  if (at < 0) return stop(w, 'ui:add-vignette', 'Виньетки ещё нет — создайте её здесь', 'Сейчас нет виньетки — покажу, где её создать')
  jump(w, { do: 'go', index: at }, false, at)
  return null
}

// Включить «Ручную» — правка проекта: сами не переключаем, показываем переключатель.
function gridNeed(w: Walk, needs: Set<Need>): Plan | null {
  if (!needs.has('grid') || curOf(w)?.grid) return null
  cardSteps(w)
  return stop(w, 'ui:mode-toggle', 'Переключите разворот на «Ручную»', 'Сейчас разворот «Авто» — покажу, где включить «Ручную»')
}

// «Все раскладки» есть только у «Авто»-разворота (не «Ручной», не виньетка): ищем такой лист; нет — показываем
// переключатель (сами в «Авто» не возвращаем: это правка проекта).
function autoNeed(w: Walk, needs: Set<Need>): Plan | null {
  if (!needs.has('auto')) return null
  const ok = (s: SheetInfo | null): boolean => !!s && !s.grid && s.kind !== 'vignette'
  if (!w.onCover && ok(curOf(w))) return null
  const at = FIRST(w.ctx.spreads, ok)
  if (at < 0) {
    cardSteps(w)
    return stop(w, 'ui:mode-toggle', 'Переключите разворот на «Авто»', 'Сейчас нет «Авто»-разворота — покажу, где его включить')
  }
  jump(w, { do: 'go', index: at }, false, at)
  return null
}

// «Сделать виньеткой» есть у любого разворота, кроме виньетки и обложки (ручной тоже): ищем такой лист.
// Все развороты — виньетки: показать нечего, мигаем виньеткой в ленте.
function notvigNeed(w: Walk, needs: Set<Need>): Plan | null {
  if (!needs.has('notvig')) return null
  const ok = (s: SheetInfo | null): boolean => !!s && s.kind !== 'vignette'
  if (!w.onCover && ok(curOf(w))) return null
  const at = FIRST(w.ctx.spreads, ok)
  if (at < 0) return stop(w, 'ui:vig-mark', 'Все развороты уже виньетки', 'Сейчас все развороты — виньетки: «Сделать виньеткой» нечему')
  jump(w, { do: 'go', index: at }, false, at)
  return null
}

// Полоса отметок появляется над лентой, когда в ней выделено фото.
function libNeed(w: Walk, needs: Set<Need>): Plan | null {
  if (!needs.has('libsel') || w.ctx.libSel) return null
  if (w.ctx.libPhotos === 0 && w.ctx.libAll > 0)
    return stop(
      w,
      'ui:library',
      'Лента пуста из-за поиска или фильтра: сбросьте их',
      'Сейчас поиск или фильтр скрыл все фото ленты — покажу ленту',
    )
  if (w.ctx.libPhotos === 0) return stop(w, 'ui:library', 'В ленте пока нет фото: добавьте папку', 'Сейчас в ленте нет фото — покажу ленту')
  w.steps.push({ do: 'selectLibrary' })
  return null
}

function photoNeed(w: Walk, needs: Set<Need>): Plan | null {
  if (!needs.has('photo') || w.selPhoto) return null
  if ((curOf(w)?.photos ?? 0) === 0)
    return stop(w, 'ui:library', 'Поставьте фото на разворот и щёлкните по нему', 'Сейчас на листе нет фото — покажу ленту фото')
  w.steps.push({ do: 'selectPhoto' })
  w.selAny = true
  return null
}

function textNeed(w: Walk, needs: Set<Need>): Plan | null {
  if (!needs.has('text') || w.selText) return null
  if ((curOf(w)?.texts ?? 0) > 0) {
    w.steps.push({ do: 'selectText' })
    w.selAny = true
    return null
  }
  const to = w.can('panels') ? 'ui:rail' : 'menu:addText'
  if (to.startsWith('menu:')) w.steps.push({ do: 'menu', menu: menuOf('addText') ?? 'spread' })
  return stop(w, to, 'Добавьте текст', 'Сейчас на листе нет текста — покажу, где его добавить')
}

// Панели, галерея и зум: то, что показывает само место. Карточка листа видна, когда ничего не выделено;
// карточка выделенного — всегда, где бы она ни была.
function viewSteps(w: Walk, needs: Set<Need>): void {
  const { steps } = w
  if (needs.has('idle') && w.selAny) steps.push({ do: 'clearSel' })
  if (needs.has('card') && !(w.can('panels') && w.ctx.cardBody)) steps.push({ do: 'details', section: 'card' })
  for (const n of needs) if (n.startsWith('details:')) steps.push({ do: 'details', section: n.slice(8) as SectionName })
  if (needs.has('gallery') && !w.ctx.galleryOpen) steps.push({ do: 'gallery' })
  if (needs.has('canvas') && w.ctx.zoomed) steps.push({ do: 'fit' })
}

// Условия в порядке проверки: каждое либо готово (null), либо переводит лист, либо обрывает план.
const NEEDS: ((w: Walk, needs: Set<Need>) => Plan | null)[] = [
  spreadNeed,
  coverNeed,
  vignetteNeed,
  gridNeed,
  autoNeed,
  notvigNeed,
  libNeed,
  photoNeed,
  textNeed,
]

// place — место, выбранное кнопкой; без него — первое из доступных на уровне.
export function planShow(a: Article, ctx: ShowCtx, place?: Place): Plan {
  const pl = place ?? openPlaces(a, ctx)[0]
  const lv = levelNeed(a, ctx, pl)
  const needs = new Set<Need>(pl?.needs ?? a.needs ?? [])
  const w: Walk = {
    ctx,
    can: lv.can,
    steps: [...lv.steps],
    toast: lv.toast,
    onCover: ctx.onCover,
    index: ctx.index,
    selPhoto: ctx.selPhoto,
    selText: ctx.selText,
    selAny: ctx.selAny,
  }
  resetView(w, needs)
  // Первое невыполнимое условие обрывает план: дальше не идём.
  for (const check of NEEDS) {
    const halted = check(w, needs)
    if (halted) return halted
  }
  viewSteps(w, needs)

  return { steps: w.steps, spot: spotStep(w, pl?.spot), toast: lv.toast, miss: pl?.miss }
}

// Место-пункт меню: добавляем шаг раскрытия меню; пункта в меню нет — мигать нечем (null).
function spotStep(w: Walk, spot: string | undefined): string | null {
  if (!spot?.startsWith('menu:')) return spot ?? null
  const m = menuOf(spotId(spot))
  if (!m) return null
  w.steps.push({ do: 'menu', menu: m })
  return spot
}
