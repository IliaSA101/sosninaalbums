// Окно «Макеты альбома» (131 U1): чистые решения — плитки темы, подписи, лента шагов, режим «Что сделать», запрос картинки.
// Серию, места и вопрос считает Go (LookInfo / LookCount / PlanLook); здесь только отбор и слова.
import { cmText, newSheetSize, pageSize, type SheetSettings } from '../../base/geom/sheet'
import { plural } from '../../base/words/plural'
import type { SegItem } from '../../base/tpl/Seg.svelte'
import type { PutToast } from '../../panels/decor/spreadtpl'
import type { LookInfo, LookItem, LookMode, LookPlan, LookRole } from './looktypes'

export const LOOKS_TITLE = 'Макеты альбома'
export const EMPTY_LOOKS = 'Макетов пока нет — их приносят наборы библиотеки'
// Пример темы (142): в компактной редакции его ещё нет — пустое окно ведёт в «Наборы»; с примером указатель стоит под плитками.
export const EMPTY_DEMO = 'Макетов пока нет — пример свадебного макета есть в «Наборах»'
export const MORE_LOOKS = 'Ещё макеты — в наборах библиотеки'
export const EMPTY_THEME = 'В этой теме пока пусто'
export const EMPTY_MINE = 'Здесь появятся ваши макеты'
export const PICK_HINT = 'Выберите макет слева'
export const SWAP_NOTE = 'Сменить макет целиком можно, пока в местах нет фото'
export const BG_NOTE = 'Стикеры макета легли бы на ваши фото — их ставит «Поставить на разворот»'
export const hideText = (name: string): string => `Макет «${name}» будет убран из библиотеки. Альбомы, где он уже стоит, не изменятся.`
export const COVER_CHIP = 'Обложка'

// Ключ картинки в /media/look: 16 hex после префикса записи (`my:<hex>`); не похоже на ключ — '' (картинку не просим).
export function lookKey(id: string): string {
  const k = id.slice(id.lastIndexOf(':') + 1)
  return /^[0-9a-f]{16}$/.test(k) ? k : ''
}

// Подпись плитки: «от 26 фото · 7 разворотов»; чего нет в карточке — того нет в подписи.
export function cardNote(it: Pick<LookItem, 'from' | 'steps'>): string {
  const parts: string[] = []
  if (it.from) parts.push(`от ${it.from} фото`)
  if (it.steps) parts.push(`${it.steps} ${plural(it.steps, ['разворот', 'разворота', 'разворотов'])}`)
  return parts.join(' · ')
}

// Шаг, который показывает плитка: лицевая обложки, нет её — разворот 1.
export const tileStep = (it: Pick<LookItem, 'cover'>): number => (it.cover ? 0 : 1)

// Число разворотов в пределах Go (LookInfo.min…max); мусор — нижний предел.
export function clampSpreads(n: number, info: Pick<LookInfo, 'min' | 'max'> | null): number {
  const lo = info?.min ?? 1
  const hi = Math.max(lo, info?.max ?? 200)
  const v = Math.trunc(n)
  return Number.isFinite(v) ? Math.min(hi, Math.max(lo, v)) : lo
}

// По умолчанию — все шаги серии по разу (0.28-131-22).
export const defaultSpreads = (info: LookInfo): number => clampSpreads(info.steps.length, info)

// Лента шагов: «Обложка · 1 Титул · 2 Сборы ↻» — i совпадает с параметром i в /media/look.
export interface StripChip {
  i: number
  label: string
  num: number // 0 — обложка
  role: LookRole
}
export function strip(info: Pick<LookInfo, 'steps' | 'cover'> | null): StripChip[] {
  if (!info) return []
  const out: StripChip[] = info.cover ? [{ i: 0, label: COVER_CHIP, num: 0, role: '' }] : []
  info.steps.forEach((s, k) => out.push({ i: k + 1, label: s.name, num: k + 1, role: s.role }))
  return out
}

export const REPEAT_MARK = '↻'
export const roleTitle = (r: LookRole): string => (r === 'repeat' ? 'Повторяется по кругу' : r === 'vignette' ? 'Виньетка' : '')

// Шаг предпросмотра после ‹ ›: по ленте, без заворота; шага нет в ленте — первый.
export function moveStep(chips: readonly StripChip[], cur: number, delta: number): number {
  if (chips.length === 0) return cur
  const at = chips.findIndex((c) => c.i === cur)
  if (at < 0) return chips[0].i
  return chips[Math.min(chips.length - 1, Math.max(0, at + delta))].i
}

// Режим применения: в собранном альбоме выбирает фотограф («Фоны по серии» по умолчанию), иначе — серия целиком.
export type FullMode = 'bg' | 'add'
export const MODE_ITEMS: SegItem<FullMode>[] = [
  { id: 'bg', label: 'Фоны по серии', title: 'Разворотам встанет только фон шага; фото, места и надписи не меняются' },
  { id: 'add', label: 'Добавить развороты макета', title: 'Повторяемые развороты макета встанут перед концом серии, места пустые' },
]
export const effMode = (plan: Pick<LookPlan, 'full'> | null, pick: FullMode): LookMode => (plan?.full ? pick : 'series')

// Вопрос (U2) нужен, только если заменится своё или фото уйдут в ленту (design §4, как 067).
export const needsAsk = (p: Pick<LookPlan, 'touched' | 'replaced' | 'left'>): boolean => p.touched > 0 || p.replaced > 0 || p.left > 0

// ---------- «Применить»: вопрос и тосты (U2) ----------

export const ASK_TITLE = 'Применить макет?'
// Заголовок вопроса «Поставить на…»: как «Поставить разворот?» 067; у обложки — «Поставить обложку?»
export const stepAskTitle = (cover: boolean): string => (cover ? 'Поставить обложку?' : 'Поставить разворот?')

// Вопрос перед «Применить» (design §4, образец — putQuestion 067): тронутые развороты, свой фон, фото в ленту.
// Развороты, где макет поставит фон, становятся оформленными (якоря 107) — «Весь альбом заново» их не пересоберёт. null — ставить сразу.
export function applyQuestion(
  p: Pick<LookPlan, 'touched' | 'replaced' | 'left' | 'anchors' | 'coverTouched' | 'coverReplaced'>,
): string | null {
  const parts: string[] = []
  // touched и replaced — суммы с обложкой (0.28-131-174): доля обложки приходит флагами, число разворотов — разностью.
  const spreadsTouched = p.touched - (p.coverTouched ? 1 : 0)
  if (p.touched > 0) {
    const where = p.coverTouched
      ? spreadsTouched > 0
        ? `${spreadsTouched} ${plural(spreadsTouched, ['развороте', 'разворотах', 'разворотах'])} и обложке`
        : 'обложке'
      : `${p.touched} ${plural(p.touched, ['развороте', 'разворотах', 'разворотах'])}`
    parts.push(`Ваши правки на ${where} заменятся`)
  }
  if (p.replaced > 0) {
    const n = p.replaced - (p.coverReplaced ? 1 : 0)
    const word = n > 0 ? `${n} ${plural(n, ['разворота', 'разворотов', 'разворотов'])}${p.coverReplaced ? ' и обложки' : ''}` : 'обложки'
    parts.push(`У ${word} заменится свой фон`)
  }
  if (p.left > 0) parts.push(`${p.left} ${plural(p.left, ['фото вернётся', 'фото вернутся', 'фото вернутся'])} в ленту`)
  if (parts.length === 0) return null
  if (p.anchors > 0) {
    const n = p.anchors
    const f = plural(n, [
      'разворот станет оформленным — «Весь альбом заново» его',
      'разворота станут оформленными — «Весь альбом заново» их',
      'разворотов станут оформленными — «Весь альбом заново» их',
    ])
    parts.push(`${n} ${f} не пересоберёт`)
  }
  return `${parts.join('. ')}. Применить?`
}

// Тост после «Применить»: «Макет «…»: 9 разворотов, 46 мест». «Добавить» и «Фоны» называют своё. Версия не выросла —
// Go ничего не менял (повтор того же), шага отмены нет. asked — число из запроса: у «Добавить» это «сколько добавить»
// (plan.spreads в Go — разворотов макета после постановки).
export function applyToast(
  name: string,
  p: Pick<LookPlan, 'mode' | 'spreads' | 'places' | 'missing'>,
  changed: boolean,
  asked = p.spreads,
): PutToast {
  const lack = p.missing > 0 ? `Нет в библиотеке: ${p.missing}` : ''
  if (!changed) return { text: [`Макет «${name}» уже стоит`, lack].filter(Boolean).join(' · '), undo: false }
  const word = (k: number) => `${k} ${plural(k, ['разворот', 'разворота', 'разворотов'])}`
  const m = `${p.places} ${plural(p.places, ['место', 'места', 'мест'])}`
  const what =
    p.mode === 'bg' ? `фоны у ${word(p.spreads)}` : p.mode === 'add' ? `добавлено ${word(asked)}, ${m}` : `${word(p.spreads)}, ${m}`
  return { text: [`Макет «${name}»: ${what}`, lack].filter(Boolean).join(' · '), undo: true }
}

// «Поставить на разворот 5» / «на обложку»: подпись по текущему листу. Номер — место листа в ленте без обложки; one — «разворот» или «лист» (альбом листов).
export const stepLabel = (onCover: boolean, no: number, one = 'разворот'): string =>
  onCover ? 'Поставить на обложку' : `Поставить на ${one} ${no}`
export const STEP_COVER_ONLY = 'Оформление обложки ставится только на обложку: откройте её'
export const STEP_NO_COVER = 'На обложку встаёт шаг «Обложка»: выберите его в ленте шагов'
export const STEP_ALBUM_NO_COVER = 'В альбоме нет обложки: её оформление встанет при «Создать обложку»'
export const STEP_LOOK_NO_COVER = 'У этого макета нет оформления обложки'

// Почему «Поставить на…» серая ('' — не серая). Шаг 0 — оформление обложки: встаёт только на обложку (её нет в альбоме —
// оформление придёт с «Создать обложку»); шаг разворота на обложку не встаёт (у макета нет обложки — так и говорим).
export function stepWhy(step: number, onCover: boolean, albumCover: boolean, lookCover: boolean): string {
  if ((step === 0) === onCover) return ''
  if (step === 0) return albumCover ? STEP_COVER_ONLY : STEP_ALBUM_NO_COVER
  return lookCover ? STEP_NO_COVER : STEP_LOOK_NO_COVER
}

// Пропорция картинки (ширина/высота) для /media/look: страница шага — одна страница формата, разворот — лист альбома,
// обложка — её лист (нет обложки — как разворот). Какие места и как вписаны — решает Go, здесь только размер запроса.
const R_MIN = 0.2
const R_MAX = 5
export function lookRatio(st: SheetSettings, pages: 1 | 2, cover?: { w: number; h: number } | null): number {
  let r: number
  if (cover && cover.w > 0 && cover.h > 0) r = cover.w / cover.h
  else if (pages === 1) r = pageSize(st).w / pageSize(st).h
  else {
    const s = newSheetSize(st)
    r = s.w / s.h
  }
  return Math.min(R_MAX, Math.max(R_MIN, r))
}

// Пропорция плитки карточки: у обложки — лицевая сторона (половина листа), у разворота — весь лист.
export function tileRatio(st: SheetSettings, it: Pick<LookItem, 'cover'>, cover?: { w: number; h: number } | null): number {
  if (!it.cover) return lookRatio(st, 2)
  return Math.min(R_MAX, Math.max(R_MIN, lookRatio(st, 2, cover) / 2))
}

export const formatText = (st: SheetSettings): string => `${cmText(pageSize(st).w)}×${cmText(pageSize(st).h)}`

export const placesText = (places: number | null): string => (places === null ? '' : `мест ${places}`)
// С выделением — по выбранным, без — по свободным фото ленты (как у автосборки): слово следует за источником K.
export const fitText = (k: number, selected = true): string =>
  selected
    ? `Подобрать под ${k} ${plural(k, ['выбранное фото', 'выбранных фото', 'выбранных фото'])}`
    : `Подобрать под ${k} ${plural(k, ['свободное фото', 'свободных фото', 'свободных фото'])}`
