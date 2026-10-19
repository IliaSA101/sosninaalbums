// Фон на набор листов (077): чистые решения окна «Фоны» — какие листы, подсказка, вопрос, итог.
// Образец — «Копировать оформление» (CopyGridDialog): те же слова и разбор номеров. Цели считает фронт
// по списку ID (геометрии здесь нет), области и фото фона считает Go (PlanBackgroundTo).
import type { BackdropScope, GridTarget, SpreadView, View } from '../../base/core/types'
import { isCover, type Unit } from '../../base/geom/sheet'
import { plural } from '../../base/words/plural'
import { parseRanges } from '../layouts/ranges'

export type BgPick = 'this' | 'sel' | 'nums' | 'all'

// Итог сухого прогона PlanBackgroundTo (model.BgPlan).
export interface BgPlanInfo {
  spreads: number
  photos: number
}

// Итог ApplyBackgroundTo (app.BgResult): вид и сколько листов изменилось.
export interface BgResult {
  view: View
  spreads: number
}

// Строка выбора листов видна на обычном листе альбома, где разворотов не меньше двух (обложка и один разворот — нет).
export const bgPickShown = (sp: Pick<SpreadView, 'id'> | undefined, spreadsN: number): boolean => !!sp && !isCover(sp) && spreadsN >= 2

// «На выделенные в ленте» есть только при двух и более выделенных; при открытии выбран, иначе «на этот».
export const bgSelShown = (selN: number): boolean => selN >= 2
export const bgPickDefault = (selN: number): BgPick => (bgSelShown(selN) ? 'sel' : 'this')

// Номера «3, 5-8» -> ID разворотов; ошибка — текст образца (parseRanges).
export function bgNums(text: string, all: readonly string[]): { ids: string[]; error: string | null } {
  const r = parseRanges(text, all.length)
  return r.error ? { ids: [], error: r.error } : { ids: r.nums.map((n) => all[n - 1]), error: null }
}

export interface BgTargetsIn {
  pick: BgPick
  cur: string // открытый лист
  scope: BackdropScope // «Куда»: одна область на все листы
  sel: readonly string[] // выделенные в ленте, в порядке альбома
  nums: readonly string[] // листы из поля номеров (bgNums)
  all: readonly string[] // все развороты, без обложки
}

export function bgTargets(i: BgTargetsIn): GridTarget[] {
  const ids = i.pick === 'this' ? [i.cur] : i.pick === 'sel' ? i.sel : i.pick === 'nums' ? i.nums : i.all
  // Корешка у разворотов нет (он только у обложки, где строки выбора нет) — страховка для типа
  const scope = i.scope === 'spine' ? 'spread' : i.scope
  return ids.map((spread) => ({ spread, scope }))
}

// Мимо прежнего ApplyBackground идёт всё, кроме единственной цели «этот лист» (номер текущего в поле — тоже один лист).
export const isMany = (pick: BgPick, targets: readonly GridTarget[], cur: string): boolean =>
  pick !== 'this' && !(targets.length === 1 && targets[0].spread === cur)

// «Затронет 12 разворотов» / «…, левые страницы».
export function bgHint(n: number, scope: BackdropScope, unit: Unit): string {
  const side = scope === 'left' ? ', левые страницы' : scope === 'right' ? ', правые страницы' : ''
  return `Затронет ${n} ${plural(n, unit.plural)}${side}`
}

// Вопрос перед постановкой: только если уходят фото фона. null — ставить сразу.
export function bgAskText(plan: BgPlanInfo, unit: Unit): string | null {
  const p = plan.photos
  if (!(p > 0)) return null
  const lost = `${p} ${plural(p, ['фото фона заменится', 'фото фона заменятся', 'фото фона заменятся'])}`
  return `${lost}. Поставить фон на ${plan.spreads} ${plural(plan.spreads, unit.plural)}?`
}

// Тост после постановки; 0 изменённых — холостая правка (на всех уже этот фон).
export function bgDoneText(n: number, unit: Unit): string {
  return n > 0 ? `Фон поставлен на ${n} ${plural(n, unit.plural)}` : 'У выбранных уже этот фон'
}
