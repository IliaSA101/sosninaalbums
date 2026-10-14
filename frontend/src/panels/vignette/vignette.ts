// Панель виньетки (спека 031, design §7): чистая логика под тесты. Места и превью считает Go;
// здесь только числа формы, подсказка «Всего мест» и условия кнопок.
import type { SpreadView, VigChoice, VignetteView } from '../../base/core/types'
import { isCover } from '../../base/geom/sheet'
import type { FeatureId } from '../../base/prefs/levels'

export const RATIOS = ['2:3', '3:4', '4:5', '1:1'] as const
export type Ratio = (typeof RATIOS)[number]

export const DEFAULT_RATIO: Ratio = '3:4'
export const DEFAULT_TPL = 'head-t-left' // «Шапка, учителя слева» — как у Go (DefaultVignette)
export const MAX_TEACHERS = 18
export const MAX_PUPILS = 64
export const MAX_PLACES = 64 // запасное значение: настоящее приходит в VignetteView.max

// Поля формы панели. Шапка (head) в форму не входит: её ID знает лист, Go читает его сам.
export interface VigForm {
  tpl: string
  teachers: number
  pupils: number
  ratio: Ratio
  ttext: string
}

// Параметры, которые уходят в Api.SetVignette (model.Vignette).
export interface VigParams {
  tpl: string
  teachers: number
  pupils: number
  ratio: string
  ttext: string
  head: string
}

const isRatio = (r: string | undefined): r is Ratio => RATIOS.includes(r as Ratio)

// Форма по виду листа. Пропорция неизвестная или не пришла — 3:4, как зажимает Go.
export function formOf(v: VignetteView): VigForm {
  return { tpl: v.tpl, teachers: v.teachers, pupils: v.pupils, ratio: isRatio(v.ratio) ? v.ratio : DEFAULT_RATIO, ttext: v.ttext ?? '' }
}

// «Сделать виньеткой» обычный разворот: фото садятся в места учеников, поэтому учеников столько, сколько фото
// (но так, чтобы с учителем вышло не больше предела); пустой разворот — 24, как у новой виньетки.
export function startForm(photos: number, max = MAX_PLACES): VigForm {
  const teachers = 1
  const pupils = photos > 0 ? Math.min(photos, MAX_PUPILS, max - teachers) : 24
  return { tpl: DEFAULT_TPL, teachers, pupils, ratio: DEFAULT_RATIO, ttext: '' }
}

export function paramsOf(f: VigForm): VigParams {
  return { tpl: f.tpl, teachers: f.teachers, pupils: f.pupils, ratio: f.ratio, ttext: f.ttext, head: '' }
}

// Число из поля: пусто и мусор — 0, дробное округляется вниз, в пределах 0…max.
export function clampCount(raw: unknown, max: number): number {
  const n = Math.floor(Number(raw))
  if (!Number.isFinite(n)) return 0
  return Math.max(0, Math.min(max, n))
}

// Превью выбранного шаблона среди карточек Go.
export const choiceOf = (list: readonly VigChoice[], tpl: string): VigChoice | undefined => list.find((c) => c.id === tpl)

// Сколько у выбранного шаблона групповых фото (каждое — место; в своём шаблоне их может быть несколько).
// Карточка Go надёжнее; пока её нет — вид листа (там только «есть/нет»).
export function groupCount(tpl: string, list: readonly VigChoice[], cur?: VignetteView): number {
  const c = choiceOf(list, tpl)
  if (c && c.roles.length > 0) return c.roles.filter((r) => r === 'group').length
  return cur?.tpl === tpl && cur.group ? 1 : 0
}

// Мест всего: учителя + ученики + групповые.
export const totalPlaces = (f: Pick<VigForm, 'teachers' | 'pupils'>, groups: number): number => f.teachers + f.pupils + groups

// Форма совпадает с тем, что уже стоит на листе: «Применить» вернёт только геометрию шаблона.
export function sameAsSheet(f: VigForm, v: VignetteView): boolean {
  const g = formOf(v)
  return f.tpl === g.tpl && f.teachers === g.teachers && f.pupils === g.pupils && f.ratio === g.ratio && f.ttext === g.ttext
}

export const leftText = (n: number): string => `Убрано в ленту: ${n}`

// «34,5 мм»: до десятых, запятая.
const mmText = (mm: number): string => `${String(Math.round(mm * 10) / 10).replace('.', ',')} мм`

// Подпись под превью: ширина портрета ученика; пометка «мелко» у Small.
export function portraitText(c: VigChoice): string {
  if (c.over) return 'не поместится'
  if (c.pupilW <= 0) return 'без учеников'
  return `портрет ${mmText(c.pupilW)}${c.small ? ' · мелко' : ''}`
}

// Подпись под превью в узкой плитке (галерея): строки без слова «портрет», каждая — без переноса, плитка
// растёт до самой длинной, поэтому соседние подписи не налезают. Полный текст — в title и aria-label.
export function portraitLines(c: VigChoice): string[] {
  if (c.over) return ['не влезет']
  if (c.pupilW <= 0) return ['без учеников']
  return c.small ? [mmText(c.pupilW), 'мелко'] : [mmText(c.pupilW)]
}

export type RoleTone = 'teacher' | 'pupil' | 'group'

// Тон места на превью: роль неизвестная и пустая — ученик.
export const toneOf = (role: string | undefined): RoleTone => (role === 'teacher' || role === 'group' ? role : 'pupil')

// Лист — виньетка с параметрами (kind и vig приходят вместе, но тип этого не знает).
export const isVignette = <T extends { kind?: string; vig?: VignetteView }>(s: T | undefined): s is T & { vig: VignetteView } =>
  s?.kind === 'vignette' && !!s.vig

// Следующий (delta > 0) или предыдущий шаблон по кругу — для Пробела и стрелок вариантов.
// Шаблоны «не поместится» пропускаются; нечего выбирать (подходящий один — тот, что стоит) — null.
export function stepTpl(list: readonly Pick<VigChoice, 'id' | 'over'>[], cur: string, delta: number): string | null {
  const n = list.length
  if (n === 0) return null
  const at = list.findIndex((c) => c.id === cur)
  const dir = delta < 0 ? -1 : 1
  // Текущего среди карточек нет (шаблон из файла) — начинаем с края, откуда идёт шаг.
  const start = at >= 0 ? at : dir > 0 ? -1 : n
  for (let k = 1; k <= n; k++) {
    const c = list[(((start + dir * k) % n) + n) % n]
    if (c.id !== cur && !c.over) return c.id
  }
  return null
}

// «Сделать виньеткой» (карточка, меню «Правка»): обычный разворот на уровне с виньеткой; не обложка и не виньетка.
export function canMakeVignette(sp: Pick<SpreadView, 'id' | 'kind' | 'vig'> | null | undefined, can: (f: FeatureId) => boolean): boolean {
  return !!sp && !isCover(sp) && !isVignette(sp) && can('vignette')
}
