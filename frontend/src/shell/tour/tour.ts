// Первые шаги (спека 113, design §4.3–4.5): чистая логика тура — какие шаги идут на этом уровне, что сделать с видом
// при входе, живое ли место и где встанет карточка. Без DOM и Svelte; исполняет tourstate.svelte.ts.
import type { FeatureId } from '../../base/prefs/levels'
import { articleText } from '../guidetext/articletext'
import type { SpotKey } from '../guidetext/articles'
import type { SheetInfo, ShowCtx, Step } from '../guide/show'
import { MISSING, MORE_IDS, TOUR, type TourId, type TourStep } from './steps'

// Состояние окна для плана плюс «есть откуда брать фото» (папки или отдельные фото).
export type TourCtx = ShowCtx & { sources: boolean }

export interface Box {
  left: number
  top: number
  width: number
  height: number
}

// Меню и поповеры уходят в body (portal): по ним ловушка тура понимает, что человек открыл что-то из выреза.
export const POPUP_SEL = '[role="menu"], [data-barpop]'

export type TourFrom = 'auto' | 'menu' | 'more'

// Шаги тура на этом уровне: шаг, чья функция выключена «Своим», пропускается («из N» меньше). more — продолжение (шаги 5–7).
export function activeSteps(can: (f: FeatureId) => boolean, from: TourFrom = 'menu'): TourStep[] {
  return TOUR.filter((s) => can(s.feature) && (from !== 'more' || MORE_IDS.includes(s.id)))
}

const isAuto = (s: SheetInfo | null | undefined): boolean => !!s && !s.grid && s.kind !== 'vignette'

export interface Resolved {
  spot: SpotKey
  text: string
  live: boolean
}

// Место живое? Пересчитывается на каждое изменение вида без шагов вида (design §4.3).
export function isLive(id: TourId, ctx: TourCtx): boolean {
  switch (id) {
    case 'folders':
      return ctx.sources
    case 'library':
    case 'autobuild':
      return ctx.libPhotos > 0
    case 'timeline':
      return ctx.spreads.length > 0
    case 'layouts':
      return !ctx.onCover && isAuto(ctx.spreads[ctx.index])
    case 'photo':
      return ctx.selPhoto
    case 'export':
      return true
  }
}

// Место и текст шага по состоянию окна. Текст — через articleText (клавиши и куски по уровню).
export function resolve(step: TourStep, ctx: TourCtx): Resolved {
  const live = isLive(step.id, ctx)
  return {
    live,
    spot: live ? step.spot : (step.emptySpot ?? step.spot),
    text: articleText(live ? step.ready : step.empty, ctx.can),
  }
}

// Сброс вида, мешающего увидеть место: только включённое.
function resetSteps(ctx: TourCtx): Step[] {
  const out: Step[] = []
  if (ctx.big) out.push({ do: 'unbig' })
  if (ctx.bgMode) out.push({ do: 'bgExit' })
  if (ctx.galleryOpen) out.push({ do: 'closeGallery' })
  if (ctx.menusOpen) out.push({ do: 'closeMenus' })
  return out
}

// Шаг 5: с обложки, «Ручной» или виньетки — к первому «Авто»; такого нет — кольцо на холсте, лист не трогаем.
// Выделение снимается: при выделенном фото строку раскладок заменяет панель выравнивания, якоря ⊞ нет
// (решение 0.25-q113-step5; выделение — вид, не правка: шага отмены и rev нет).
function layoutsEnter(ctx: TourCtx): Step[] {
  const at = ctx.spreads.findIndex(isAuto)
  const here = !ctx.onCover && isAuto(ctx.spreads[ctx.index])
  const out: Step[] = at >= 0 && !here ? [{ do: 'go', index: at }] : []
  if (at >= 0 && ctx.selAny) out.push({ do: 'clearSel' })
  return out
}

// Шаг 6: к развороту с фото, если на текущем их нет; выделить первое, не трогая выделенное на этом же листе.
function photoEnter(ctx: TourCtx): Step[] {
  const cur = ctx.onCover ? null : ctx.spreads[ctx.index]
  const bare = !cur || cur.photos === 0
  const at = bare ? ctx.spreads.findIndex((s) => s.photos > 0) : -1
  const out: Step[] = at >= 0 ? [{ do: 'go', index: at }] : []
  if (at >= 0 || (!bare && !ctx.selPhoto)) out.push({ do: 'selectPhoto' })
  return out
}

// Шаги вида при входе (design §4.3). Уровень не меняется, проект не правится: лист, выделение, масштаб. first — первый шаг
// прогона: перед ним сбрасывается вид; шаги 5 и 6 сбрасывают его всегда (галерею, открытую щелчком по ⊞ на прошлом шаге).
export function enterSteps(step: TourStep, ctx: TourCtx, first: boolean): Step[] {
  const view = step.id === 'layouts' || step.id === 'photo'
  const out = first || view ? resetSteps(ctx) : []
  if (step.id === 'layouts') out.push(...layoutsEnter(ctx))
  else if (step.id === 'photo') out.push(...photoEnter(ctx))
  if (view && ctx.zoomed) out.push({ do: 'fit' })
  return out
}

// Какие поля решает автозапуск (design §4.5).
export interface AutoState {
  due: boolean // prefs.tourDue
  startOpen: boolean // стартовое окно
  modal: boolean // любое Modal (в том числе «Открываем альбом»)
  loaded: boolean // альбом загружен
  opening: boolean // ход открытия альбома
  running: boolean // тур уже открыт или уже стартовал сам
}

export const shouldAutostart = (s: AutoState): boolean => s.due && !s.startOpen && !s.modal && s.loaded && !s.opening && !s.running

// Есть ли на каком-то развороте фото (для разового продолжения).
export const hasPhotoOnSpread = (ctx: Pick<ShowCtx, 'spreads'>): boolean => ctx.spreads.some((s) => s.photos > 0)

// Карточка: ширина 320 px, отступы — как у подписи кольца.
export const CARD_W = 320
export const CARD_W_INTRO = 340
const GAP = 12 // карточка от кольца
const EDGE = 8 // карточка от края окна
const INSIDE = 16 // внутри у угла цели
const BIG = 0.6 // доля окна, с которой цель считается «холстом»

const clamp = (v: number, lo: number, hi: number): number => Math.max(lo, Math.min(v, Math.max(lo, hi)))

type Place = { left: number; top: number }

// Площадь пересечения карточки в точке p с рамкой.
function crossArea(p: Place, size: { w: number; h: number }, b: Box): number {
  const w = Math.min(p.left + size.w, b.left + b.width) - Math.max(p.left, b.left)
  const h = Math.min(p.top + size.h, b.top + b.height) - Math.max(p.top, b.top)
  return w > 0 && h > 0 ? w * h : 0
}

// Видимая в окне часть рамки элемента; ничего не видно — null (область шага пропускается).
export function visibleBox(r: Box, win: { w: number; h: number }): Box | null {
  const left = Math.max(0, r.left)
  const top = Math.max(0, r.top)
  const right = Math.min(win.w, r.left + r.width)
  const bottom = Math.min(win.h, r.top + r.height)
  return right > left && bottom > top ? { left, top, width: right - left, height: bottom - top } : null
}

// Где карточка: рядом с кольцом (снизу → сверху → справа → слева), кольцо не перекрывает; цель больше 60 % окна или
// рядом места нет — внутри у правого нижнего угла цели; без кольца — по центру. Всегда в окне.
// avoid (122) — области, которые карточка не закрывает (лента фото, текущий разворот): к сторонам кольца добавляются
// углы окна (ближние к кольцу первыми), берётся первое место без пересечения, иначе с наименьшим; кольцо не задето.
// Без avoid результат прежний.
export function cardPlace(ring: Box | null, size: { w: number; h: number }, win: { w: number; h: number }, avoid: Box[] = []): Place {
  const at = (x: number, y: number) => ({ left: clamp(x, EDGE, win.w - size.w - EDGE), top: clamp(y, EDGE, win.h - size.h - EDGE) })
  if (!ring) return at((win.w - size.w) / 2, (win.h - size.h) / 2)
  const midX = ring.left + ring.width / 2 - size.w / 2
  const midY = ring.top + ring.height / 2 - size.h / 2
  const right = ring.left + ring.width
  const bottom = ring.top + ring.height
  const corner = at(right - size.w - INSIDE, bottom - size.h - INSIDE)
  if (ring.width * ring.height > BIG * win.w * win.h) return corner
  const sides: Place[] = []
  if (bottom + GAP + size.h <= win.h - EDGE) sides.push(at(midX, bottom + GAP))
  if (ring.top - GAP - size.h >= EDGE) sides.push(at(midX, ring.top - GAP - size.h))
  if (right + GAP + size.w <= win.w - EDGE) sides.push(at(right + GAP, midY))
  if (ring.left - GAP - size.w >= EDGE) sides.push(at(ring.left - GAP - size.w, midY))
  if (avoid.length === 0) return sides[0] ?? corner
  const dist = (p: Place) => Math.hypot(p.left + size.w / 2 - (midX + size.w / 2), p.top + size.h / 2 - (midY + size.h / 2))
  const corners = [at(0, 0), at(win.w, 0), at(0, win.h), at(win.w, win.h)].sort((a, b) => dist(a) - dist(b))
  // Кольцо с отступом GAP: углы окна могут лечь на него, стороны — нет (отступ ровно GAP).
  const halo = { left: ring.left - GAP, top: ring.top - GAP, width: ring.width + 2 * GAP, height: ring.height + 2 * GAP }
  const free = [...sides, ...corners].filter((p) => crossArea(p, size, halo) === 0)
  if (free.length === 0) return sides[0] ?? corner
  const cost = (p: Place) => avoid.reduce((sum, b) => sum + crossArea(p, size, b), 0)
  return free.reduce((best, p) => (cost(p) < cost(best) ? p : best))
}

// Текст шага, когда места на экране нет: к тексту добавляется пояснение.
export const withMissing = (text: string, missing: boolean): string => (missing ? `${text} ${MISSING}` : text)

// Число шагов в приветствии (на любом уровне их от 5 до 7: слово «шагов» одно).
export const introText = (text: string, n: number): string => text.replace('{n}', String(n))
