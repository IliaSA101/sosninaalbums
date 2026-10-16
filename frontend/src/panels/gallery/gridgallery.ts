// Чистая логика галереи сеток и окна «Копировать сетку…» (спека 007, U9): без Svelte, под тесты.
// Геометрию считает Go; здесь только перевод долей области в мм разворота для предпросмотра
// и в доли карточки для рисунка, а также тексты вопросов.
import { plural } from '../../base/core/plural'
import type { SheetSize } from '../../base/geom/sheet'
import type { TplSource } from '../../base/tpl/tplsource'
import type { Box, GridChoice, GridChoices, GridPlan, GridTarget, LayoutScope, PlaceView, Rect, RotRect } from '../../base/core/types'

export const MAX_N = 7 // фильтр «7+»

const clampN = (n: number) => Math.max(1, Math.min(MAX_N, Math.round(n) || 1))

// Шаблон ставится на область scope? Шаблон разворота — только на разворот;
// шаблон страницы — на страницу или на обе (разворот).
export function fits(ch: GridChoice, scope: LayoutScope): boolean {
  return ch.scope === 'page' || scope === 'spread'
}

// Ряды из ответа Go — для области, под которую он посчитан (не текущей): пока идёт «Считаю…» после смены
// области, прежний ответ не перетолковывается и пункты переключателя не становятся серыми.
export const builtinOf = (r: GridChoices | null | undefined, scope: LayoutScope): GridChoice[] =>
  scope === 'spread' ? (r?.builtin ?? []) : []
export const mineOf = (r: GridChoices | null | undefined, scope: LayoutScope): GridChoice[] => (r?.mine ?? []).filter((x) => fits(x, scope))

// Фильтр по умолчанию: число мест (у сетки) или фото (у «Авто») в области, 1..7.
export function defaultN(spread: { rects: Rect[]; places: PlaceView[] | null }, mid: number, scope: LayoutScope): number {
  const cells: Rect[] = spread.places && spread.places.length > 0 ? spread.places : spread.rects
  const inScope = cells.filter((r) => {
    const c = r.x + r.w / 2
    return scope === 'spread' || (scope === 'left' ? c < mid : c >= mid)
  })
  return clampN(inScope.length)
}

// Угол и форма коробки (044) — только когда они есть: у обычной поля rot, shape, round нет.
const withRot = (r: Rect, b: Box): Box => ({
  ...r,
  ...(b.rot ? { rot: b.rot } : {}),
  ...(b.shape ? { shape: b.shape } : {}),
  ...(b.round ? { round: b.round } : {}),
})

// Доли карточки (0..1): шаблон страницы на разворот рисуется на обеих страницах.
// Угол идёт вместе с коробкой: поворот вокруг её центра, а центр при делении на 2 едет вместе с x и w.
export function cardBoxes(ch: GridChoice, scope: LayoutScope): Box[] {
  if (ch.scope === 'page' && scope === 'spread') {
    return [0, 0.5].flatMap((off) => ch.boxes.map((b) => withRot({ x: off + b.x / 2, y: b.y, w: b.w / 2, h: b.h }, b)))
  }
  return ch.boxes
}

// Прямоугольники карточки в мм разворота — для предпросмотра на большом развороте
// (store.previewChoice). Те же правила перевода, что у Go в applyGrid.
export function previewRects(ch: GridChoice, scope: LayoutScope, sheet: SheetSize): RotRect[] {
  return cardBoxes(ch, scope).map((b) => {
    // cardBoxes даёт доли листа (scope = spread) или страницы (left/right).
    const w = scope === 'spread' ? sheet.w : sheet.mid
    const off = scope === 'right' ? sheet.mid : 0
    return withRot({ x: off + b.x * w, y: b.y * sheet.h, w: b.w * w, h: b.h * sheet.h }, b)
  })
}

// Тексты источника шаблонов (050): пустой ряд и причина серого пункта — одни и те же слова.
// n — число мест в фильтре (7 = «7 мест и больше»), unitOne — «разворот» / «лист».
export function gridSourceTexts(scope: LayoutScope, n: number, unitOne: string) {
  const mineN = `Нет своих шаблонов на ${n === MAX_N ? '7 мест и больше' : `${n} ${plural(n, ['место', 'места', 'мест'])}`}`
  const builtin = scope === 'spread' ? 'Нет шаблонов на такое число мест' : `Встроенные шаблоны ставятся только на весь ${unitOne}`
  const empty: Record<TplSource, string> = { all: scope === 'spread' ? builtin : mineN, builtin, mine: mineN }
  return { empty, off: { builtin, mine: mineN } }
}

// Подсказка у значка «−N».
export function dropTitle(n: number): string {
  return `${n} ${plural(n, ['фото вернётся', 'фото вернутся', 'фото вернутся'])} в ленту`
}

// Предупреждение перед сбросом ручных мест: вопрос «Вернуть авто?» страницы, ручная виньетка и окно «Настроить виньетку».
export const RESET_WARN = 'Ваши правки мест сбросятся.'

// Ошибки шаблонов: «сохранены более новой версией» — понятным текстом.
export function tplError(msg: string): string {
  if (/более новой версией/i.test(msg)) {
    return 'Шаблоны сохранены более новой версией программы. Обновите программу, чтобы менять их'
  }
  return msg
}

// ---------- копирование ----------

export type CopyWhere = 'next' | 'spreads' | 'all' | 'new'
export type CopySide = 'left' | 'right' | 'spread'

export interface CopyPlanInput {
  src: string
  from: LayoutScope
  where: CopyWhere
  side: CopySide // для страницы-источника: на какие страницы целевых разворотов
  spreadIds: string[] // «развороты: 3, 5-8» уже переведены в id
  allIds: string[]
}

// Цели CopyGrid. С разворота цель всегда «разворот»; со страницы — выбранная сторона,
// «соседняя страница» — другая сторона того же разворота. Для 'new' целей нет (DuplicateGrid).
export function copyTargets(i: CopyPlanInput): GridTarget[] {
  if (i.where === 'new') return []
  if (i.where === 'next') return i.from === 'spread' ? [] : [{ spread: i.src, scope: i.from === 'left' ? 'right' : 'left' }]
  const ids = i.where === 'all' ? i.allIds : i.spreadIds
  const scope: LayoutScope = i.from === 'spread' ? 'spread' : i.side
  return ids.map((spread) => ({ spread, scope }))
}

// Сколько разворотов реально затронет копирование: цель «та же область того же разворота» Go пропускает.
export function touched(targets: GridTarget[], src: string, from: LayoutScope): number {
  return new Set(targets.filter((t) => !(t.spread === src && t.scope === from)).map((t) => t.spread)).size
}

// Вопрос до копирования (design §6). null — копировать сразу.
export function planQuestion(p: GridPlan): string | null {
  const stickers = p.stickers
  if (!(p.frozen > 1 || p.left > 0 || p.texts > 0 || stickers > 0)) return null
  const parts: string[] = []
  if (p.frozen > 0)
    parts.push(`${p.frozen} ${plural(p.frozen, ['разворот станет ручным', 'разворота станут ручными', 'разворотов станут ручными'])}`)
  if (p.left > 0) parts.push(`${p.left} ${plural(p.left, ['фото вернётся', 'фото вернутся', 'фото вернутся'])} в ленту`)
  if (p.texts > 0)
    parts.push(`${p.texts} ${plural(p.texts, ['блок текста заменится', 'блока текста заменятся', 'блоков текста заменятся'])}`)
  if (stickers > 0) parts.push(`${stickers} ${plural(stickers, ['стикер заменится', 'стикера заменятся', 'стикеров заменятся'])}`)
  return `${parts.join(', ')}. Копировать?`
}

export interface CopyOpts {
  design: boolean // «С текстом и фоном»
  mirror: boolean // «Зеркально на другой стороне»
}

// Окно «Копировать оформление» (049 Ф2): «С надписями» вместо «С текстом и фоном», выключен по умолчанию.
export interface DecorOpts {
  texts: boolean // «С надписями»
  mirror: boolean
}

const KEY = 'copyGrid'
const DECOR_KEY = 'copyDecor'

// Настройки окна — свойство программы, а не проекта: localStorage в try/catch, как у view.svelte.ts.
function load(key: string, o: Record<string, boolean>) {
  try {
    const v = JSON.parse(localStorage.getItem(key) ?? 'null')
    if (v && typeof v === 'object') for (const k of Object.keys(o)) if (typeof v[k] === 'boolean') o[k] = v[k]
  } catch {
    // Без хранилища — значения по умолчанию.
  }
  return o
}

function save(key: string, o: object) {
  try {
    localStorage.setItem(key, JSON.stringify(o))
  } catch {
    // Без хранилища настройки не запомнятся.
  }
}

export const loadCopyOpts = (): CopyOpts => load(KEY, { design: true, mirror: false }) as unknown as CopyOpts
export const saveCopyOpts = (o: CopyOpts) => save(KEY, o)
export const loadDecorOpts = (): DecorOpts => load(DECOR_KEY, { texts: false, mirror: false }) as unknown as DecorOpts
export const saveDecorOpts = (o: DecorOpts) => save(DECOR_KEY, o)
