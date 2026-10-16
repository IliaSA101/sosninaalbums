// Окно «Настроить виньетку» и карточка-сводка (031 dialog §2–3): чистая логика под тесты.
// Места и предпросмотр считает Go; здесь тексты, поля шапки, размеры рамок и условия «Применить».
import { plural } from '../../base/core/plural'
import type { SpreadView, Vars, VigChoice, VigSetup, VignetteView } from '../../base/core/types'
import { RESET_WARN } from '../gallery/gridgallery'
import { paramsOf, portraitText, sameAsSheet, stepTpl, type VigForm } from './vignette'

// ---------- Поля шапки ----------

export type VarKey = keyof Vars

// Порядок и подписи полей шапки; hint — образец в пустом поле (школа, детсад, организация).
export const VAR_FIELDS: readonly { k: VarKey; label: string; hint: string }[] = [
  { k: 'title', label: 'Заголовок', hint: 'Гимназия № 1' },
  { k: 'subtitle', label: 'Подзаголовок', hint: '11 «А» класс' },
  { k: 'year', label: 'Год', hint: '2026' },
  { k: 'lead', label: 'Руководитель', hint: 'Иванова И. И.' },
]

// Знаков в одном поле (model.MaxVarRunes); Go зажимает сам, поле не даёт набрать больше.
export const VAR_MAX = 120

// Форма полей шапки: все четыре строки всегда есть (пустая — «»).
export type VarsForm = Record<VarKey, string>

export const varsFormOf = (v: Vars | undefined): VarsForm => ({
  title: v?.title ?? '',
  subtitle: v?.subtitle ?? '',
  year: v?.year ?? '',
  lead: v?.lead ?? '',
})

// В Go уходят только непустые: пустое поле — нет ключа, как в `omitempty`.
export function varsOf(f: VarsForm): Vars {
  const out: Vars = {}
  for (const { k } of VAR_FIELDS) if (f[k] !== '') out[k] = f[k]
  return out
}

export const sameVars = (a: VarsForm, b: VarsForm): boolean => VAR_FIELDS.every(({ k }) => a[k] === b[k])

export const setupOf = (f: VigForm, v: VarsForm): VigSetup => ({ vig: paramsOf(f), vars: varsOf(v) })

// ---------- Форма мест (044) ----------

export type ShapeKey = 'rect' | 'round' | 'oval'

// «Скруглённые» — 10 % меньшей стороны (решение 044 №35); «Круг» у виньетки ещё и ставит портрет 1:1.
export const VIG_ROUND = 10
export const SHAPE_KEYS: readonly { k: ShapeKey; label: string; hint: string }[] = [
  { k: 'rect', label: 'Прямоугольник', hint: 'Места учеников и учителей — прямоугольные' },
  { k: 'round', label: 'Скруглённые', hint: 'Прямоугольники со скруглёнными углами (10 %)' },
  { k: 'oval', label: 'Круг', hint: 'Круглые портреты; пропорция портрета станет 1:1' },
]

// Какая кнопка подсвечена: круг, любое скругление (в том числе из своего шаблона) или прямоугольник.
export function shapeKey(f: Pick<VigForm, 'shape' | 'round'>): ShapeKey {
  if (f.shape === 'oval') return 'oval'
  return (f.round ?? 0) > 0 ? 'round' : 'rect'
}

// Что поменять в форме при выборе кнопки. Круг — портрет 1:1 (в овале 3:4 были бы овалы; вернёте 3:4 — овалы).
export function shapePatch(k: ShapeKey): Pick<VigForm, 'shape' | 'round'> & Partial<Pick<VigForm, 'ratio'>> {
  if (k === 'oval') return { shape: 'oval', round: 0, ratio: '1:1' }
  return { shape: '', round: k === 'round' ? VIG_ROUND : 0 }
}

// ---------- Тексты ----------

const TEACHERS: [string, string, string] = ['учитель', 'учителя', 'учителей']
const PUPILS: [string, string, string] = ['ученик', 'ученика', 'учеников']

// «1 учитель · 23 ученика · групповое»: учителей 0 — без этой части.
export function countsText(v: Pick<VignetteView, 'teachers' | 'pupils' | 'group'>): string {
  const parts: string[] = []
  if (v.teachers > 0) parts.push(`${v.teachers} ${plural(v.teachers, TEACHERS)}`)
  parts.push(`${v.pupils} ${plural(v.pupils, PUPILS)}`)
  if (v.group) parts.push('групповое')
  return parts.join(' · ')
}

// Вторая строка сводки: числа и размер портрета (из карточки шаблона Go; пока её нет — одни числа).
export function summaryText(v: VignetteView, c: VigChoice | undefined): string {
  const counts = countsText(v)
  return c && !c.over ? `${counts} · ${portraitText(c)}` : counts
}

// Места роли без фото на листе (виньетка только что пересобрана или фото мало).
export const emptyPlaces = (sp: Pick<SpreadView, 'places'>): number => sp.places?.filter((p) => !p.photo).length ?? 0

export const placesLine = (total: number, max: number): string => `Мест ${total} из ${max}`

// «в ленту уйдут 2 фото»: «фото» не склоняется. Нечего убирать — пусто.
export const leftLine = (n: number): string => (n > 0 ? `в ленту уйдут ${n} фото` : '')

// ---------- «Применить» ----------

// Строка перед «Применить»: у ручной виньетки (vig.manual) любая правка пересобирает места по шаблону и сбрасывает
// ручные — те же слова, что в вопросе страницы «Вернуть авто?». Нечего применять или лист нетронут — пусто.
export const resetWarning = (manual: boolean | undefined, unchanged: boolean): string => (manual && !unchanged ? RESET_WARN : '')

// Почему «Применить» закрыто; null — можно. Порядок: идёт ли применение, шаблон, предел, «ничего не изменилось».
export function dialogBlock(a: { busy: boolean; tpl: string; total: number; max: number; same: boolean }): string | null {
  if (a.busy) return 'Идёт пересборка…'
  if (!a.tpl) return 'Выберите шаблон'
  if (a.total > a.max) return `Не больше ${a.max} мест на листе: уменьшите число учителей или учеников`
  if (a.same) return 'Ничего не изменилось'
  return null
}

// Форма и поля совпадают с листом: применять нечего.
export const sameAsDialog = (f: VigForm, v: VarsForm, sp: { vig: VignetteView; vars?: Vars }): boolean =>
  sameAsSheet(f, sp.vig) && sameVars(v, varsFormOf(sp.vars))

// ---------- Размеры ----------

// Вписать лист sw×sh в рамку maxW×maxH с сохранением пропорции; px, округляется вниз.
export function fitBox(sw: number, sh: number, maxW: number, maxH: number): { w: number; h: number } {
  if (!(sw > 0 && sh > 0 && maxW > 0 && maxH > 0)) return { w: 0, h: 0 }
  const k = Math.min(maxW / sw, maxH / sh)
  return { w: Math.floor(sw * k), h: Math.floor(sh * k) }
}

// Рамка предпросмотра: колонка ≤ 560×440; по высоте окна — min(440, окно − reserve), но не меньше 240.
// reserve — что кроме предпросмотра занимает окно: шапка, низ и сетка шаблонов (300 — старая полоса в один ряд).
export const PREVIEW_W = 560
export const PREVIEW_H = 440
export const PREVIEW_MIN_H = 240
export const PREVIEW_RESERVE = 400
export function previewBox(sw: number, sh: number, colW: number, winH: number, reserve = 300): { w: number; h: number } {
  const maxH = Math.max(PREVIEW_MIN_H, Math.min(PREVIEW_H, winH - reserve))
  return fitBox(sw, sh, Math.min(PREVIEW_W, colW), maxH)
}

// Карточка шаблона: пропорция листа в рамке CARD_W × CARD_MAX_H (px).
export const CARD_W = 164
export const CARD_MAX_H = 110
export const cardBox = (sw: number, sh: number): { w: number; h: number } => fitBox(sw, sh, CARD_W, CARD_MAX_H)

// Высота сетки шаблонов: что осталось в окне под предпросмотром (отступ окна от краёв 32, шапка и низ ≈ 120,
// зазор 10), но не меньше STRIP_MIN; выше — сетка прокручивается сама, а не окно.
export const STRIP_MIN = 130
export const stripMax = (winH: number, previewH: number): number => Math.max(STRIP_MIN, winH - 32 - 120 - previewH - 10)

// Куда сдвинуть выбор стрелкой в сетке из cols колонок: ← → — соседний шаблон по кругу, ↑ ↓ — на ряд;
// «не поместится» пропускается, за краем сетки — null (остаёмся).
export type ArrowKey = 'ArrowLeft' | 'ArrowRight' | 'ArrowUp' | 'ArrowDown'
export function gridStep(list: readonly Pick<VigChoice, 'id' | 'over'>[], cur: string, key: ArrowKey, cols: number): string | null {
  if (key === 'ArrowLeft') return stepTpl(list, cur, -1)
  if (key === 'ArrowRight') return stepTpl(list, cur, 1)
  const dir = key === 'ArrowUp' ? -1 : 1
  const step = Math.max(1, cols)
  const at = list.findIndex((c) => c.id === cur)
  if (at < 0) return null
  for (let i = at + dir * step; i >= 0 && i < list.length; i += dir * step) {
    if (!list[i].over) return list[i].id
  }
  return null
}

// Сдвиг прокрутки сетки, чтобы элемент [a, b] (координаты внутри сетки, по любой оси) был виден в окне размером vw.
export function revealX(scroll: number, a: number, b: number, vw: number): number {
  if (a < scroll) return a
  if (b > scroll + vw) return Math.max(0, b - vw)
  return scroll
}
