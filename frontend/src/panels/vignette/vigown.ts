// Свои шаблоны виньетки (038): чистая логика и вызовы Go для окна «Настроить виньетку», галереи и карточки.
// Шаблоны хранит Go (templates.json, общий для компьютера); здесь тексты, условия кнопок и тосты.
import { Api, store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import type { GridResult, VigChoice } from '../../base/core/types'
import { tplCall } from '../layouts/gridactions'
import { tplError } from '../gallery/gridgallery'
import { RATIOS, choiceOf, leftText, portraitLines, type Ratio, type Shape } from './vignette'

export const OWN_TITLE = 'Мои шаблоны'
export const NAME_MAX = 60
export const SAVE_HINT = 'Сохранятся места с ролями, шапка и стиль подписей. Шаблон хранится на этом компьютере, общий для всех альбомов'

// Индекс первого своего шаблона (перед ним — заголовок «Мои шаблоны»); своих нет — −1.
export const firstOwn = (items: readonly Pick<VigChoice, 'own'>[]): number => items.findIndex((c) => c.own)

// Название плитки: удалённый из файла шаблон помечен.
export const ownLabel = (c: Pick<VigChoice, 'name' | 'gone'>): string => (c.gone ? `${c.name} (шаблон удалён)` : c.name)

// Имя шаблона листа для карточки справа и строки раскладки: по списку Go (переименование видно сразу),
// tplName на листе — снимок на момент применения, запасной, пока списка нет.
export const tplLabel = (items: readonly VigChoice[], tpl: string, fallback?: string): string => {
  const c = choiceOf(items, tpl)
  return c ? ownLabel(c) : (fallback ?? '')
}

// Свой шаблон: его имя можно изменить, поэтому строке раскладки нужен список Go.
export const isOwnTpl = (tpl: string): boolean => tpl.startsWith('my:')

// ✎ и × — только у своего, живого шаблона и при уровне «Шаблоны»: удалённого из файла переименовывать нечего.
export const canEditOwn = (c: Pick<VigChoice, 'own' | 'gone'>, allowed: boolean): boolean => allowed && !!c.own && !c.gone

// Строки под плиткой галереи: ширина портрета и пометка «удалён».
export const tileLines = (c: VigChoice): string[] => (c.gone ? [...portraitLines(c), 'удалён'] : portraitLines(c))

// Выбор своего шаблона ставит в окне его портрет (точный режим требует того же); числа не трогает.
// null — менять нечего (каталожный шаблон, портрет тот же, не пришёл или неизвестный).
export function pickedRatio(items: readonly VigChoice[], id: string, cur: string): Ratio | null {
  const c = items.find((x) => x.id === id)
  const r = c?.ratio as Ratio | undefined
  return c?.own && r && r !== cur && RATIOS.includes(r) ? r : null
}

// Смена шаблона в окне и в ▦ (049, решение №122): что поменять в форме. Каталожный шаблон с подсказкой круга
// ставит «Круг» и портрет 1:1 (видно и можно сменить); уход с него на шаблон без подсказки возвращает прямоугольник,
// если форму не меняли; портрет не трогается. Свой шаблон — только его портрет (как pickedRatio). null — менять нечего.
export function pickedForm(
  items: readonly VigChoice[],
  prevId: string,
  nextId: string,
  form: { ratio: string; shape?: string },
): { ratio?: Ratio; shape?: Shape } | null {
  const next = items.find((x) => x.id === nextId)
  const out: { ratio?: Ratio; shape?: Shape } = {}
  const own = pickedRatio(items, nextId, form.ratio)
  if (own) out.ratio = own
  if (next && !next.own && next.shape === 'oval') {
    if (form.shape !== 'oval') out.shape = 'oval'
    if (form.ratio !== '1:1') out.ratio = '1:1'
  } else if (form.shape === 'oval' && next?.shape !== 'oval' && items.find((x) => x.id === prevId)?.shape === 'oval') {
    out.shape = ''
  }
  return out.ratio || out.shape !== undefined ? out : null
}

// То же для формы мест (044): свой шаблон с формой переключает «Форму» в окне. null — менять нечего
// (каталожный шаблон, форма не задана или та же).
export function pickedShape(
  items: readonly VigChoice[],
  id: string,
  cur: { shape?: string; round?: number },
): { shape: Shape; round: number } | null {
  const c = items.find((x) => x.id === id)
  if (!c?.own || (c.shape === undefined && c.round === undefined)) return null
  const shape: Shape = c.shape === 'oval' ? 'oval' : ''
  const round = shape === 'oval' ? 0 : (c.round ?? 0)
  return shape === (cur.shape ?? '') && round === (cur.round ?? 0) ? null : { shape, round }
}

// Форма и пропорция мест на плитке шаблона (049): то, что получится после щелчка по ней при текущей форме и шаблоне curId,
// — плитки не меняют вид при выборе другого шаблона. Логика та же, что у pickedForm/pickedShape (они применяются при выборе).
// ratio — портрет мест после щелчка («круги» — 1:1); from — портрет, в котором Go посчитал коробки плитки
// (каталожный — портрет окна, свой — его собственный): отличие ratio от from VigCard гасит вписыванием (fitBox).
export type TileForm = { shape?: string; round?: number; ratio: string; from: string }

export function tileForm(
  items: readonly VigChoice[],
  curId: string,
  tileId: string,
  form: { ratio: string; shape?: string; round?: number },
): TileForm {
  const own = pickedShape(items, tileId, form)
  const p = pickedForm(items, curId, tileId, form)
  const ratio = p?.ratio ?? form.ratio
  const from = items.find((x) => x.id === tileId)?.own ? ratio : form.ratio
  if (own) return { ...own, ratio, from }
  return { shape: p?.shape ?? form.shape, round: form.round, ratio, from }
}

// Место портрета ratio внутри коробки плитки (доли листа bx,by,bw,bh; w,h — px плитки): вписано по меньшей стороне
// по центру, больше коробки не бывает. Пропорцию считает не Go, а выбранная форма: «круг» всегда 1:1.
export function fitBox(
  b: { x: number; y: number; w: number; h: number },
  w: number,
  h: number,
  ratio: string,
): { x: number; y: number; w: number; h: number } {
  const [rw, rh] = ratio.split(':').map(Number)
  const a = rw > 0 && rh > 0 ? rw / rh : 0
  const pw = b.w * w
  const ph = b.h * h
  if (!a || pw <= 0 || ph <= 0) return b
  const nw = pw / ph > a ? ph * a : pw
  const nh = pw / ph > a ? ph : pw / a
  return { x: b.x + (b.w - nw / w) / 2, y: b.y + (b.h - nh / h) / 2, w: nw / w, h: nh / h }
}

// Текст вопроса об удалении; cap — «Лист» или «Разворот» (unitOf.cap).
export const deleteText = (name: string, cap: string): string =>
  `Шаблон «${name}» будет удалён из списка. ${cap === 'Лист' ? 'Листы' : 'Развороты'}, где он уже применён, не изменятся.`

// Имя из поля: пробелы по краям убираются, длиннее NAME_MAX обрезается (Go зажимает так же).
export const nameOf = (raw: string): string => Array.from(raw.trim()).slice(0, NAME_MAX).join('')

// «Сохранить как шаблон…»: пустое имя — Go назовёт «Шаблон N». Успех — тост; ошибка Go — тост текста.
export async function saveVigTemplate(spreadId: string, name: string): Promise<boolean> {
  if (!(await tplCall(Api.SaveVignetteTemplate(spreadId, nameOf(name))))) return false
  store.notify('Шаблон сохранён')
  return true
}

// Id шаблона — VigChoice.id как есть («my:<id>»), префикс снимает Go.
export const renameVigTemplate = (id: string, name: string): Promise<boolean> => tplCall(Api.RenameGridTemplate(id, nameOf(name)))
export const deleteVigTemplate = (id: string): Promise<boolean> => tplCall(Api.DeleteGridTemplate(id))

// «Вернуть авто» в строке раскладки виньетки: места пересобираются по шаблону, один шаг отмены.
// Фото, что не встали, уходят в ленту — тост с «Вернуть», как у «Применить».
export async function resetVignette(spreadId: string): Promise<boolean> {
  try {
    const r = (await Api.ResetVignette(spreadId)) as unknown as GridResult
    store.apply(r.view)
    if (r.left > 0) store.undoToast(leftText(r.left))
    return true
  } catch (e) {
    store.notify(tplError(errText(e)), 'error')
    return false
  }
}
